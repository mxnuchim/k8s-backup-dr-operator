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
	"fmt"
	"sort"
	"time"

	backupv1alpha1 "github.com/mxnuchim/k8s-backup-operator/api/v1alpha1"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backups,verbs=get;list;watch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling BackupPolicy", "name", req.Name, "namespace", req.Namespace)

	// Fetch the BackupPolicy instance
	var backupPolicy backupv1alpha1.BackupPolicy
	if err := r.Get(ctx, req.NamespacedName, &backupPolicy); err != nil {
		log.Error(err, "unable to fetch BackupPolicy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Found backup policy", "schedule", backupPolicy.Spec.Schedule, "pvcName", backupPolicy.Spec.Target.PVCName)

	// Parse the cron schedule
	schedule, err := cron.ParseStandard(backupPolicy.Spec.Schedule)
	if err != nil {
		log.Error(err, "invalid cron schedule", "schedule", backupPolicy.Spec.Schedule)
		backupPolicy.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "InvalidSchedule",
				Message:            fmt.Sprintf("Invalid cron schedule: %v", err),
				LastTransitionTime: metav1.Now(),
			},
		}
		r.Recorder.Eventf(
			&backupPolicy,
			corev1.EventTypeWarning,
			"InvalidSchedule",
			"Invalid cron schedule %q: %v",
			backupPolicy.Spec.Schedule,
			err,
		)
		r.Status().Update(ctx, &backupPolicy)
		return ctrl.Result{}, nil
	}

	now := time.Now()

	// Calculate when the next backup should run
	var nextBackupTime time.Time
	if backupPolicy.Status.LastBackupTime != nil {
		// Find next scheduled time after the last backup
		nextBackupTime = schedule.Next(backupPolicy.Status.LastBackupTime.Time)
	} else {
		// No previous backup, schedule from now
		nextBackupTime = schedule.Next(now.Add(-1 * time.Second))
	}

	// Check if it's time to create a backup
	if now.Before(nextBackupTime) {
		// Not time yet, requeue for the scheduled time
		requeueAfter := nextBackupTime.Sub(now)
		log.Info("Next backup scheduled", "nextBackupTime", nextBackupTime, "requeueAfter", requeueAfter)

		r.Recorder.Eventf(
			&backupPolicy,
			corev1.EventTypeNormal,
			"BackupScheduled",
			"Next backup scheduled for %s",
			nextBackupTime.Format(time.RFC3339),
		)

		// Update status with next scheduled time
		backupPolicy.Status.NextScheduledBackup = &metav1.Time{Time: nextBackupTime}
		backupPolicy.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Scheduled",
				Message:            fmt.Sprintf("Next backup scheduled for %s", nextBackupTime.Format(time.RFC3339)),
				LastTransitionTime: metav1.Now(),
			},
		}
		r.Status().Update(ctx, &backupPolicy)

		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Time to create a backup!
	log.Info("Creating scheduled backup", "scheduledTime", nextBackupTime)

	// Create a new Backup
	backup := &backupv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupPolicy.Name + "-" + time.Now().Format("20060102-150405"),
			Namespace: backupPolicy.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backupPolicy.APIVersion,
					Kind:       backupPolicy.Kind,
					Name:       backupPolicy.Name,
					UID:        backupPolicy.UID,
					Controller: pointer.Bool(true),
				},
			},
		},
		Spec: backupv1alpha1.BackupSpec{
			PolicyRef: backupPolicy.Name,
			Target:    backupPolicy.Spec.Target,
		},
	}

	if err := r.Create(ctx, backup); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "unable to create Backup")
			return ctrl.Result{}, err
		}
		log.Info("Backup already exists (race condition), continuing")
	}

	log.Info("Created scheduled Backup", "backupName", backup.Name)

	r.Recorder.Eventf(
		&backupPolicy,
		corev1.EventTypeNormal,
		"BackupCreated",
		"Created backup %s",
		backup.Name,
	)

	// Clean up old backups based on retention policy
	if err := r.cleanupOldBackups(ctx, &backupPolicy); err != nil {
		log.Error(err, "failed to clean up old backups")
		// Don't fail the reconciliation, just log the error
	}

	// Update status with last backup time
	backupPolicy.Status.LastBackupTime = &metav1.Time{Time: now}
	nextScheduledTime := schedule.Next(now)
	backupPolicy.Status.NextScheduledBackup = &metav1.Time{Time: nextScheduledTime}
	backupPolicy.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "BackupCreated",
			Message:            fmt.Sprintf("Backup %s created, next backup scheduled for %s", backup.Name, nextScheduledTime.Format(time.RFC3339)),
			LastTransitionTime: metav1.Now(),
		},
	}

	if err := r.Status().Update(ctx, &backupPolicy); err != nil {
		log.Error(err, "unable to update BackupPolicy status")
		return ctrl.Result{}, err
	}

	// Requeue for the next scheduled backup
	requeueAfter := nextScheduledTime.Sub(time.Now())
	log.Info("Requeuing for next backup", "nextBackupTime", nextScheduledTime, "requeueAfter", requeueAfter)

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// cleanupOldBackups deletes backups beyond the retention policy
func (r *BackupPolicyReconciler) cleanupOldBackups(ctx context.Context, backupPolicy *backupv1alpha1.BackupPolicy) error {
	log := logf.FromContext(ctx)

	// If no retention policy, don't clean up
	if backupPolicy.Spec.Retention == nil || backupPolicy.Spec.Retention.KeepLast == nil {
		return nil
	}

	keepLast := *backupPolicy.Spec.Retention.KeepLast

	// List all backups for this policy
	var backups backupv1alpha1.BackupList
	if err := r.List(ctx, &backups, client.InNamespace(backupPolicy.Namespace)); err != nil {
		return err
	}

	// Filter backups owned by this policy and that are completed
	var ownedBackups []backupv1alpha1.Backup
	for _, backup := range backups.Items {
		if backup.Spec.PolicyRef == backupPolicy.Name &&
			backup.Status.Phase == backupv1alpha1.BackupPhaseCompleted {
			ownedBackups = append(ownedBackups, backup)
		}
	}

	// Sort by creation time (newest first)
	sort.Slice(ownedBackups, func(i, j int) bool {
		return ownedBackups[i].CreationTimestamp.After(ownedBackups[j].CreationTimestamp.Time)
	})

	// If we have more backups than allowed, delete the extras
	deletedCount := 0

	// Delete backups beyond keepLast
	if len(ownedBackups) > keepLast {
		backupsToDelete := ownedBackups[keepLast:]
		for _, backup := range backupsToDelete {
			log.Info("Deleting old backup due to retention policy",
				"backupName", backup.Name,
				"keepLast", keepLast)

			if err := r.Delete(ctx, &backup); err != nil {
				log.Error(err, "failed to delete old backup", "backupName", backup.Name)
				// Continue trying to delete others
				continue
			}
			deletedCount++

		}
	}

	// Emit ONE event if we deleted anything
	if deletedCount > 0 {
		r.Recorder.Eventf(
			backupPolicy,
			corev1.EventTypeNormal,
			"CleanupTriggered",
			"Deleted %d old backups (keepLast=%d)",
			deletedCount,
			keepLast,
		)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("backuppolicy-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.BackupPolicy{}).
		Owns(&backupv1alpha1.Backup{}). // Watch backups owned by this policy
		Named("backuppolicy").
		Complete(r)
}
