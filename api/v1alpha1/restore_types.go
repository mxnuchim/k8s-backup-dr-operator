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

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	// BackupName is the name of the Backup to restore from
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// TargetPVC is the PVC to restore data into
	// +kubebuilder:validation:Required
	TargetPVC string `json:"targetPVC"`

	// TargetNamespace is where to create/restore the PVC (defaults to Restore's namespace)
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	// Phase represents the current phase of the restore
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// StartTime is when the restore started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the restore finished
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RestoredDataSize is the size of restored data
	// +optional
	RestoredDataSize string `json:"restoredDataSize,omitempty"`

	// conditions represent the current state of the Restore resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// RestorePhase represents the phase of a restore operation
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
type RestorePhase string

const (
	RestorePhasePending   RestorePhase = "Pending"
	RestorePhaseRunning   RestorePhase = "Running"
	RestorePhaseCompleted RestorePhase = "Completed"
	RestorePhaseFailed    RestorePhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupName`
// +kubebuilder:printcolumn:name="Target PVC",type=string,JSONPath=`.spec.targetPVC`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Restore
	// +required
	Spec RestoreSpec `json:"spec"`

	// status defines the observed state of Restore
	// +optional
	Status RestoreStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
