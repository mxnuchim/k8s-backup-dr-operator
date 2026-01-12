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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"time"

	backupv1alpha1 "github.com/mxnuchim/k8s-backup-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.manuchim.dev,resources=backuppolicies/finalizers,verbs=update

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

	// Check if a Backup already exists for this policy
	var existingBackups backupv1alpha1.BackupList
	if err := r.List(ctx, &existingBackups, client.InNamespace(backupPolicy.Namespace)); err != nil {
		log.Error(err, "unable to list existing Backups")
		return ctrl.Result{}, err
	}

	// Filter backups owned by this policy
	var ownedBackups []backupv1alpha1.Backup
	for _, backup := range existingBackups.Items {
		if backup.Spec.PolicyRef == backupPolicy.Name {
			ownedBackups = append(ownedBackups, backup)
		}
	}

	if len(ownedBackups) > 0 {
		log.Info("Backup already exists, skipping creation", "count", len(ownedBackups))
		return ctrl.Result{}, nil
	}

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
		log.Error(err, "unable to create Backup")
		return ctrl.Result{}, err
	}

	log.Info("Created Backup", "backupName", backup.Name)

	// Update BackupPolicy status
	backupPolicy.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "BackupCreated",
			Message:            "Initial backup created successfully",
			LastTransitionTime: metav1.Now(),
		},
	}

	if err := r.Status().Update(ctx, &backupPolicy); err != nil {
		log.Error(err, "unable to update BackupPolicy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.BackupPolicy{}).
		Named("backuppolicy").
		Complete(r)
}
