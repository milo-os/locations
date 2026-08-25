// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LocationSpec defines the desired state of Location.
type LocationSpec struct {
	// LocationClassRef names the LocationClass backing this location. The class
	// decides which controller brings the location up and what capacity sits
	// behind it.
	//
	// Leave the project qualifier empty to name a class in the same control
	// plane as this Location. Set it to name a class in the provider's control
	// plane, which is what you do to place a location on capacity the provider
	// operates.
	//
	// The class must exist and report Accepted=True before the location becomes
	// Ready. Naming a class that does not exist leaves the location not Ready
	// rather than rejecting it, so a location can be declared ahead of the
	// class that will serve it.
	//
	// +kubebuilder:validation:Required
	LocationClassRef LocationClassReference `json:"locationClassRef"`

	// The topology of the location
	//
	// This may contain arbitrary topology keys. Some keys may be well known, such
	// as:
	//	- topology.datum.net/city-code
	//
	// +kubebuilder:validation:Required
	Topology map[string]string `json:"topology"`

	// The geographic coordinates of the location, used by consumers that need
	// to plot the location on a map.
	//
	// +kubebuilder:validation:Optional
	Coordinates *Coordinates `json:"coordinates,omitempty"`
}

// LocationClassReference names a LocationClass, in this control plane or in
// the provider's.
type LocationClassReference struct {
	// Name is the name of the LocationClass.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Project names the project whose control plane holds the class. Leave it
	// unset, or empty, for a class that lives alongside this Location.
	//
	// Set it when you are consuming capacity somebody else operates: the class
	// stays in their control plane, where they own it, and your Location points
	// across at it. You do not get a copy of the class, and you cannot change
	// what it offers.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=253
	Project string `json:"project,omitempty"`
}

// Coordinates describes a geographic point in decimal degrees (WGS 84).
//
// Latitude and longitude are serialized as strings rather than floats, per
// Kubernetes API convention (float precision/serialization varies across
// client languages).
type Coordinates struct {
	// Latitude in decimal degrees, in the range [-90, 90].
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^-?\d{1,2}(\.\d+)?$`
	// +kubebuilder:validation:XValidation:message="latitude must be between -90 and 90",rule="double(self) >= -90.0 && double(self) <= 90.0"
	Latitude string `json:"latitude"`

	// Longitude in decimal degrees, in the range [-180, 180].
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^-?\d{1,3}(\.\d+)?$`
	// +kubebuilder:validation:XValidation:message="longitude must be between -180 and 180",rule="double(self) >= -180.0 && double(self) <= 180.0"
	Longitude string `json:"longitude"`
}

// LocationStatus defines the observed state of Location.
type LocationStatus struct {
	// Represents the observations of a location's current state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:validation:XValidation:message="name must be at most 63 characters, because it is published as a label value",rule="size(self.metadata.name) <= 63"
// +kubebuilder:printcolumn:name="Class",type="string",JSONPath=".spec.locationClassRef.name"
// +kubebuilder:printcolumn:name="City",type="string",JSONPath=`.spec.topology.topology\.datum\.net/city-code`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Location is a place the platform serves traffic or runs workloads from,
// typically a city rather than a cloud vendor's region.
//
// Every Location names a LocationClass, and where that class lives tells you
// whose capacity is behind the location. A class in the provider's control
// plane is the provider's own footprint, offered to you: the provider owns the
// capacity and operates it. A class in your control plane is capacity you
// brought, such as your own cloud account, and you operate it. Either way the
// controller named on the class is the one that acts, so nothing about the
// location has to restate who runs it.
//
// Locations the provider offers are declared once on the platform control
// plane and projected into your project's control plane, and onto each cell as
// a ServingLocation telling it where it sits. A projected Location is the
// statement that the location is offered to you; edit the platform copy, not
// the projection. Locations you declare yourself, on your own class, are yours
// to edit.
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

type LocationReference struct {
	// Name of a datum location
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

func init() {
	SchemeBuilder.Register(&Location{}, &LocationList{})
}
