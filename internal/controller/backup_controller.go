/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"time"

	backupv1alpha1 "github.com/mxnuchim/k8s-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Backup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling Backup", "name", req.Name, "namespace", req.Namespace)

	// Fetch the Backup instance
	var backup backupv1alpha1.Backup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		log.Error(err, "unable to fetch Backup")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If backup is already completed or failed, nothing to do
	if backup.Status.Phase == backupv1alpha1.BackupPhaseCompleted ||
		backup.Status.Phase == backupv1alpha1.BackupPhaseFailed {
		log.Info("Backup already in terminal state", "phase", backup.Status.Phase)
		return ctrl.Result{}, nil
	}

	// Set phase to Running if not already set
	if backup.Status.Phase == "" {
		backup.Status.Phase = backupv1alpha1.BackupPhaseRunning
		now := metav1.Now()
		backup.Status.StartTime = &now
		if err := r.Status().Update(ctx, &backup); err != nil {
			log.Error(err, "unable to update Backup status to Running")
			return ctrl.Result{}, err
		}
		log.Info("Updated Backup status to Running")
	}

	// Check if Job already exists
	var existingJob batchv1.Job
	jobName := backup.Name + "-job"
	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: backup.Namespace}, &existingJob)
	if err == nil {
		// Job exists, check its status
		if existingJob.Status.Succeeded > 0 {
			log.Info("Backup Job completed successfully")
			backup.Status.Phase = backupv1alpha1.BackupPhaseCompleted
			now := metav1.Now()
			backup.Status.CompletionTime = &now
			backup.Status.BackupLocation = "/backups/" + backup.Name + ".tar.gz"
			if err := r.Status().Update(ctx, &backup); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else if existingJob.Status.Failed > 0 {
			log.Info("Backup Job failed")
			backup.Status.Phase = backupv1alpha1.BackupPhaseFailed
			now := metav1.Now()
			backup.Status.CompletionTime = &now
			if err := r.Status().Update(ctx, &backup); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// Job still running, requeue to check later
		log.Info("Backup Job still running")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !apierrors.IsNotFound(err) {
		log.Error(err, "unable to fetch Job")
		return ctrl.Result{}, err
	}

	// Job doesn't exist, create it
	job := r.createBackupJob(&backup)
	if err := r.Create(ctx, job); err != nil {
		// Ignore "already exists" errors (race condition from multiple reconciles)
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "unable to create Backup Job")
			backup.Status.Phase = backupv1alpha1.BackupPhaseFailed
			r.Status().Update(ctx, &backup)
			return ctrl.Result{}, err
		}
		log.Info("Job already exists (race condition), continuing")
	}

	log.Info("Created Backup Job", "jobName", job.Name)
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *BackupReconciler) createBackupJob(backup *backupv1alpha1.Backup) *batchv1.Job {
	jobName := backup.Name + "-job"

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "backup",
							Image: "busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"echo 'Starting backup of PVC: " + backup.Spec.Target.PVCName + "' && " +
									"tar -czf /backup-output/" + backup.Name + ".tar.gz -C /data . && " +
									"echo 'Backup completed successfully' && " +
									"ls -lh /backup-output/",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "source-data",
									MountPath: "/data",
									ReadOnly:  true,
								},
								{
									Name:      "backup-output",
									MountPath: "/backup-output",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "source-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: backup.Spec.Target.PVCName,
									ReadOnly:  true,
								},
							},
						},
						{
							Name: "backup-output",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "backup-storage", // Shared storage
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.Backup{}).
		Owns(&batchv1.Job{}).
		Named("backup").
		Complete(r)
}
