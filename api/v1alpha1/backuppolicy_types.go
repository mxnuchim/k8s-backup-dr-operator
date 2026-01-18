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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupPolicySpec defines the desired state of BackupPolicy
type BackupPolicySpec struct {
	// Schedule in cron format (e.g., "0 2 * * *" for daily at 2 AM)
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// Target defines what to backup
	// +kubebuilder:validation:Required
	Target BackupTarget `json:"target"`

	// Retention defines how many backups to keep
	// +optional
	Retention *RetentionPolicy `json:"retention,omitempty"`
}

// BackupTarget defines the resource to backup
type BackupTarget struct {
	// PVCName is the name of the PersistentVolumeClaim to backup
	// +kubebuilder:validation:Required
	PVCName string `json:"pvcName"`

	// Namespace where the PVC lives (defaults to BackupPolicy's namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// RetentionPolicy defines backup retention rules
type RetentionPolicy struct {
	// KeepLast is the number of most recent backups to retain
	// +kubebuilder:validation:Minimum=1
	// +optional
	KeepLast *int `json:"keepLast,omitempty"`
}

// BackupPolicyStatus defines the observed state of BackupPolicy.
type BackupPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LastBackupTime is when the last backup was created
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// NextScheduledBackup is when the next backup will be created
	// +optional
	NextScheduledBackup *metav1.Time `json:"nextScheduledBackup,omitempty"`

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the BackupPolicy resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BackupPolicy is the Schema for the backuppolicies API
type BackupPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of BackupPolicy
	// +required
	Spec BackupPolicySpec `json:"spec"`

	// status defines the observed state of BackupPolicy
	// +optional
	Status BackupPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// BackupPolicyList contains a list of BackupPolicy
type BackupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []BackupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicy{}, &BackupPolicyList{})
}
