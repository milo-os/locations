// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LocationPhase represents the lifecycle state of a Location.
// +kubebuilder:validation:Enum=Provisioning;Ready;Failed
type LocationPhase string

const (
	// LocationPhaseProvisioning indicates the location is being set up.
	LocationPhaseProvisioning LocationPhase = "Provisioning"

	// LocationPhaseReady indicates the location is active and healthy.
	LocationPhaseReady LocationPhase = "Ready"

	// LocationPhaseFailed indicates the location has encountered an unrecoverable error.
	LocationPhaseFailed LocationPhase = "Failed"
)

// LocationSpec defines the desired state of Location.
type LocationSpec struct {
	// Description is a human-readable description of this location.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=256
	Description string `json:"description,omitempty"`
}

// LocationStatus defines the observed state of Location.
type LocationStatus struct {
	// Phase represents the current lifecycle phase of the location.
	//
	// +kubebuilder:validation:Optional
	Phase LocationPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the location's state.
	//
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Location is the Schema for the locations API.
//
// +kubebuilder:object:root=true
// +kubebuilder:sublocation:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// Milo-specific: update or remove this annotation for your location type
// +kubebuilder:metadata:annotations="discovery.miloapis.com/parent-contexts=Organization"
type Location struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocationSpec   `json:"spec,omitempty"`
	Status LocationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LocationList contains a list of Location.
type LocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Location `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Location{}, &LocationList{})
}
