// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LocationClassSpec describes a kind of location the platform can offer.
type LocationClassSpec struct {
	// ControllerName is the controller that reconciles Locations of this class.
	// It reads as a domain-qualified path, for example
	// `locations.miloapis.com/shared-edge`.
	//
	// Only the controller named here acts on a Location of this class. Every
	// other controller ignores it, so two providers can serve locations side by
	// side in the same control plane without contending for the same objects.
	//
	// You cannot change this field after creation. Retarget a class by creating
	// a new one and moving Locations across.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:message="controllerName is immutable",rule="self == oldSelf"
	ControllerName string `json:"controllerName"`

	// ParametersRef points at a resource holding the provider's own
	// configuration for this class: which capacity backs it, and whatever else
	// that provider needs to stand a location up. The shape of that resource is
	// the provider's to define, and this API makes no claim about it.
	//
	// The resource is owned by the provider, not by you. Read it to understand
	// what a class offers; expect writes to be rejected.
	//
	// Leave it unset for a class that needs no configuration. If it is set and
	// the resource cannot be read, or does not carry what the controller needs,
	// the class reports Accepted=False with reason InvalidParameters and no
	// Location of this class comes up.
	//
	// +kubebuilder:validation:Optional
	ParametersRef *ParametersReference `json:"parametersRef,omitempty"`
}

// ParametersReference identifies a provider-owned resource by group, kind and
// name.
type ParametersReference struct {
	// Group is the API group of the referent, for example
	// `compute.miloapis.com`. Use the empty string for the core API group.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=253
	Group string `json:"group"`

	// Kind is the kind of the referent, for example `EdgeCapacityPool`.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Kind string `json:"kind"`

	// Name is the name of the referent.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// LocationClassStatus reports what the controller behind a class has made of
// it.
type LocationClassStatus struct {
	// Conditions describe the current state of the class.
	//
	// `Accepted` tells you whether the controller named in spec.controllerName
	// has taken the class on. Until it is True, Locations of this class do
	// nothing. A class no controller recognises stays Accepted=Unknown with
	// reason Pending, which is what you see when the class names a controller
	// that is not running.
	//
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	// LocationClassConditionAccepted reports whether the controller named by
	// the class has taken it on.
	LocationClassConditionAccepted = "Accepted"

	// LocationClassReasonAccepted is set when the class is usable.
	LocationClassReasonAccepted = "Accepted"

	// LocationClassReasonInvalidParameters is set when spec.parametersRef
	// cannot be resolved or does not carry what the controller needs.
	LocationClassReasonInvalidParameters = "InvalidParameters"

	// LocationClassReasonPending is set when no controller has reported on the
	// class yet.
	LocationClassReasonPending = "Pending"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Controller",type="string",JSONPath=".spec.controllerName"
// +kubebuilder:printcolumn:name="Accepted",type="string",JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LocationClass is a kind of location the platform can offer.
//
// A Location says where; its class says what backs it. The class names the
// controller that brings locations of that kind up, and points at the
// provider's configuration for the capacity behind them. Two locations in the
// same city with different classes are different products.
//
// Classes are declared by whoever supplies the capacity. If you are placing
// workloads, you pick a class by name on a Location and read its Accepted
// condition to know it is usable; you do not create classes yourself.
type LocationClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocationClassSpec   `json:"spec,omitempty"`
	Status LocationClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LocationClassList contains a list of LocationClass.
type LocationClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LocationClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LocationClass{}, &LocationClassList{})
}
