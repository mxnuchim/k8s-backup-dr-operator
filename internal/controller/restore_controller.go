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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"fmt"
	"time"

	backupv1alpha1 "github.com/mxnuchim/k8s-backup-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=restores/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Restore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling Restore", "name", req.Name, "namespace", req.Namespace)

	// Fetch the Restore instance
	var restore backupv1alpha1.Restore
	if err := r.Get(ctx, req.NamespacedName, &restore); err != nil {
		log.Error(err, "unable to fetch Restore")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If restore is already completed or failed, nothing to do
	if restore.Status.Phase == backupv1alpha1.RestorePhaseCompleted ||
		restore.Status.Phase == backupv1alpha1.RestorePhaseFailed {
		log.Info("Restore already in terminal state", "phase", restore.Status.Phase)
		return ctrl.Result{}, nil
	}

	// Validate that the Backup exists and is completed
	targetNamespace := restore.Spec.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = restore.Namespace
	}

	var backup backupv1alpha1.Backup
	backupKey := client.ObjectKey{Name: restore.Spec.BackupName, Namespace: targetNamespace}
	if err := r.Get(ctx, backupKey, &backup); err != nil {
		log.Error(err, "unable to fetch Backup", "backupName", restore.Spec.BackupName)
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		restore.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "BackupNotFound",
				Message:            fmt.Sprintf("Backup %s not found", restore.Spec.BackupName),
				LastTransitionTime: metav1.Now(),
			},
		}
		r.Status().Update(ctx, &restore)
		return ctrl.Result{}, err
	}

	// Verify backup is completed
	if backup.Status.Phase != backupv1alpha1.BackupPhaseCompleted {
		log.Info("Backup not completed yet", "backupPhase", backup.Status.Phase)
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		restore.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "BackupNotReady",
				Message:            fmt.Sprintf("Backup is in phase %s, not Completed", backup.Status.Phase),
				LastTransitionTime: metav1.Now(),
			},
		}
		r.Status().Update(ctx, &restore)
		return ctrl.Result{}, nil
	}

	// Set phase to Running if not already set
	if restore.Status.Phase == "" {
		restore.Status.Phase = backupv1alpha1.RestorePhaseRunning
		now := metav1.Now()
		restore.Status.StartTime = &now
		restore.Status.Conditions = []metav1.Condition{
			{
				Type:               "Progressing",
				Status:             metav1.ConditionTrue,
				Reason:             "RestoreStarted",
				Message:            "Restore job is being created",
				LastTransitionTime: metav1.Now(),
			},
		}
		if err := r.Status().Update(ctx, &restore); err != nil {
			log.Error(err, "unable to update Restore status to Running")
			return ctrl.Result{}, err
		}
		log.Info("Updated Restore status to Running")
	}

	// Check if Job already exists
	var existingJob batchv1.Job
	jobName := restore.Name + "-job"
	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: restore.Namespace}, &existingJob)
	if err == nil {
		// Job exists, check its status
		if existingJob.Status.Succeeded > 0 {
			log.Info("Restore Job completed successfully")
			restore.Status.Phase = backupv1alpha1.RestorePhaseCompleted
			now := metav1.Now()
			restore.Status.CompletionTime = &now
			restore.Status.Conditions = []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "RestoreCompleted",
					Message:            fmt.Sprintf("Successfully restored from backup %s", backup.Name),
					LastTransitionTime: metav1.Now(),
				},
			}
			if err := r.Status().Update(ctx, &restore); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else if existingJob.Status.Failed > 0 {
			log.Info("Restore Job failed")
			restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
			now := metav1.Now()
			restore.Status.CompletionTime = &now
			restore.Status.Conditions = []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "RestoreFailed",
					Message:            "Restore job failed - check job logs for details",
					LastTransitionTime: metav1.Now(),
				},
			}
			if err := r.Status().Update(ctx, &restore); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// Job still running, requeue to check later
		log.Info("Restore Job still running")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !apierrors.IsNotFound(err) {
		log.Error(err, "unable to fetch Job")
		return ctrl.Result{}, err
	}

	// Job doesn't exist, create it
	job := r.createRestoreJob(&restore, &backup)
	if err := r.Create(ctx, job); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "unable to create Restore Job")
			restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
			r.Status().Update(ctx, &restore)
			return ctrl.Result{}, err
		}
		log.Info("Job already exists (race condition), continuing")
	}

	log.Info("Created Restore Job", "jobName", job.Name)
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *RestoreReconciler) createRestoreJob(restore *backupv1alpha1.Restore, backup *backupv1alpha1.Backup) *batchv1.Job {
	jobName := restore.Name + "-job"

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: restore.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: restore.APIVersion,
					Kind:       restore.Kind,
					Name:       restore.Name,
					UID:        restore.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "prepare-restore",
							Image: "busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"echo 'Starting restore operation...' && " +
									"if [ -f /backup-source/" + backup.Name + ".tar.gz ]; then " +
									"  tar -xzf /backup-source/" + backup.Name + ".tar.gz -C /restore-target && " +
									"  echo 'Restore completed successfully' && " +
									"  echo 'Restored files:' && " +
									"  ls -lh /restore-target/; " +
									"else " +
									"  echo 'ERROR: Backup file not found at /backup-source/" + backup.Name + ".tar.gz' && exit 1; " +
									"fi",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup-source",
									MountPath: "/backup-source",
									ReadOnly:  true,
								},
								{
									Name:      "restore-target",
									MountPath: "/restore-target",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "restore",
							Image: "busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"echo 'Starting restore operation...' && " +
									"if [ -f /backup-source/" + backup.Name + ".tar.gz ]; then " +
									"  tar -xzf /backup-source/" + backup.Name + ".tar.gz -C /restore-target && " +
									"  echo 'Restore completed successfully' && " +
									"  echo 'Restored files:' && " +
									"  ls -lh /restore-target/; " +
									"else " +
									"  echo 'ERROR: Backup file not found at /backup-source/" + backup.Name + ".tar.gz' && exit 1; " +
									"fi",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup-source",
									MountPath: "/backup-source",
									ReadOnly:  true,
								},
								{
									Name:      "restore-target",
									MountPath: "/restore-target",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "backup-source",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "backup-storage",
									ReadOnly:  true,
								},
							},
						},
						{
							Name: "restore-target",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: restore.Spec.TargetPVC,
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
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("restore-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Named("restore").
		Complete(r)
}
