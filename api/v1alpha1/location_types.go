// SPDX-License-Identifier: AGPL-3.0-only

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LocationSpec defines the desired state of Location.
type LocationSpec struct {
	// LocationClassName is the name of the LocationClass backing this location.
	// The class decides which controller brings the location up and what
	// capacity sits behind it.
	//
	// The class must exist and report Accepted=True before the location becomes
	// Ready. Naming a class that does not exist leaves the location not Ready
	// rather than rejecting it, so a location can be declared ahead of the
	// class that will serve it.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	LocationClassName string `json:"locationClassName"`

	// ManagedBy says who is responsible for operating the control plane for
	// this location. It is independent of the class: the class decides what
	// capacity backs the location, this decides who runs it.
	//
	// +kubebuilder:validation:Required
	ManagedBy LocationManagementResponsibility `json:"managedBy"`

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

// LocationManagementResponsibility says who operates the control plane for a
// location.
//
// +kubebuilder:validation:Enum=Platform;Self
type LocationManagementResponsibility string

const (
	// LocationManagedByPlatform means the platform operator runs the control
	// plane for the location, and you consume it as a managed service.
	LocationManagedByPlatform LocationManagementResponsibility = "Platform"

	// LocationManagedBySelf means whoever registered the location runs its
	// control plane themselves. The platform records the location and publishes
	// it, but does not operate it.
	LocationManagedBySelf LocationManagementResponsibility = "Self"
)

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
// +kubebuilder:printcolumn:name="Class",type="string",JSONPath=".spec.locationClassName"
// +kubebuilder:printcolumn:name="Managed By",type="string",JSONPath=".spec.managedBy"
// +kubebuilder:printcolumn:name="City",type="string",JSONPath=`.spec.topology.topology\.datum\.net/city-code`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Location is a place the platform serves traffic or runs workloads from,
// typically a city rather than a cloud vendor's region.
//
// Operators declare Locations once, on the platform control plane. The same
// kind then appears in your project's control plane for every location you can
// actually use, and on each cell as a ServingLocation telling it where it sits.
// A Location in your control plane is the statement that the location is
// offered to you; edit the platform copy, not the projections.
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
