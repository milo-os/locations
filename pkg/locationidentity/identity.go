// SPDX-License-Identifier: AGPL-3.0-only

// Package locationidentity answers the question a cell cannot answer for
// itself: which location am I serving? It is exported so the services running
// on a cell resolve that identity the same way.
package locationidentity

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

// Identity sources a cell can resolve its location from.
const (
	LocationIdentitySourceDelivered  = "Delivered"
	LocationIdentitySourceConfigured = "Configured"
)

// Reasons a cell cannot name the location it serves.
const (
	LocationUnresolvedNoIdentity = "NoLocationIdentity"
	LocationUnresolvedAmbiguous  = "AmbiguousLocationIdentity"
)

// LocationConfig names a Location. On a cell it is a fallback, used only while
// no ServingLocation has been delivered to that cell.
type LocationConfig struct {
	// Name is the name of the Location, such as "us-east-1-iad".
	Name string `json:"name,omitempty"`
}

// LocationIdentity is the location a cell serves and where that answer came
// from. Mismatch reports that a delivered copy and a configured location
// disagree; the delivered copy wins.
type LocationIdentity struct {
	Reference locationsv1alpha1.LocationReference
	Source    string
	Mismatch  bool
}

// LocationUnresolved reports that a cell cannot yet name the location it
// serves. Callers report it on the objects that are stuck rather than failing.
type LocationUnresolved struct {
	Reason  string
	Message string
}

func (e *LocationUnresolved) Error() string { return e.Message }

// Resolve returns the location a cell serves. A single
// delivered ServingLocation wins over the configured location. More than one
// delivered copy falls back to the configured location, and returns
// LocationUnresolved when there is none.
func Resolve(
	ctx context.Context,
	reader client.Reader,
	configured LocationConfig,
) (LocationIdentity, error) {
	var delivered locationsv1alpha1.ServingLocationList
	if err := reader.List(ctx, &delivered); err != nil {
		return LocationIdentity{}, fmt.Errorf("failed listing delivered serving locations: %w", err)
	}

	configuredIdentity := LocationIdentity{
		Reference: locationsv1alpha1.LocationReference{
			Name: configured.Name,
		},
		Source: LocationIdentitySourceConfigured,
	}

	switch {
	case len(delivered.Items) == 1:
		identity := LocationIdentity{
			Reference: locationsv1alpha1.LocationReference{Name: delivered.Items[0].Name},
			Source:    LocationIdentitySourceDelivered,
			Mismatch:  configured.Name != "" && configured.Name != delivered.Items[0].Name,
		}
		return identity, nil

	case len(delivered.Items) > 1:
		if configured.Name != "" {
			return configuredIdentity, nil
		}
		return LocationIdentity{}, &LocationUnresolved{
			Reason: LocationUnresolvedAmbiguous,
			Message: fmt.Sprintf(
				"%d ServingLocations have been delivered to this cell; exactly one cluster label %s is expected and no location is configured to fall back to",
				len(delivered.Items), locationsv1alpha1.ServingLocationTopologyLabel),
		}

	case configured.Name != "":
		return configuredIdentity, nil

	default:
		return LocationIdentity{}, &LocationUnresolved{
			Reason: LocationUnresolvedNoIdentity,
			Message: fmt.Sprintf(
				"no ServingLocation has been delivered to this cell and location.name is not configured; label the cluster with %s or configure a location",
				locationsv1alpha1.ServingLocationTopologyLabel),
		}
	}
}
