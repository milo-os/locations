// SPDX-License-Identifier: AGPL-3.0-only

package agent

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	servicesv1alpha1 "go.miloapis.com/service-catalog/api/v1alpha1"
)

// AvailableCondition is what an availability record reports when the service
// is genuinely offered at the location it names.
const AvailableCondition = "Available"

// Reader fetches the two records this server joins.
//
// The identity a read runs under is decided by whoever constructs the Reader,
// never inside a tool: the server builds one per request from the caller's own
// bearer token, so a tool call can never see more than the person who asked
// could see themselves.
type Reader interface {
	// ListAvailability returns every service availability record visible to
	// the caller.
	ListAvailability(ctx context.Context) ([]servicesv1alpha1.ServiceAvailability, error)
	// ListLocations returns every location visible to the caller.
	ListLocations(ctx context.Context) ([]locationsv1alpha1.Location, error)
	// GetLocation returns one location by name.
	GetLocation(ctx context.Context, name string) (*locationsv1alpha1.Location, error)
}

// NotServedError reports that a project does not carry a kind at all, as
// distinct from carrying it and holding nothing.
//
// The distinction is the whole point. "No locations" and "nobody looked" call
// for opposite actions — one waits for the service to arrive somewhere, the
// other is a misconfiguration someone has to fix — so they must never reach a
// person as the same sentence.
type NotServedError struct {
	Kind string
	Err  error
}

func (e *NotServedError) Error() string {
	return fmt.Sprintf("%s is not served by this project: %v", e.Kind, e.Err)
}

func (e *NotServedError) Unwrap() error { return e.Err }

// notServed reports whether err means the kind itself is absent.
func notServed(err error) bool {
	return apimeta.IsNoMatchError(err) || apierrors.IsNotFound(err)
}

// ClientReader implements Reader against a controller-runtime client. Every
// read carries whatever credentials that client holds and no others.
type ClientReader struct {
	Client client.Client
}

var _ Reader = (*ClientReader)(nil)

// NewClientReader returns a Reader backed by c.
func NewClientReader(c client.Client) *ClientReader {
	return &ClientReader{Client: c}
}

func (r *ClientReader) ListAvailability(ctx context.Context) ([]servicesv1alpha1.ServiceAvailability, error) {
	var list servicesv1alpha1.ServiceAvailabilityList
	if err := r.Client.List(ctx, &list); err != nil {
		if notServed(err) {
			return nil, &NotServedError{Kind: "ServiceAvailability", Err: err}
		}
		return nil, fmt.Errorf("listing service availability: %w", err)
	}
	return list.Items, nil
}

func (r *ClientReader) ListLocations(ctx context.Context) ([]locationsv1alpha1.Location, error) {
	var list locationsv1alpha1.LocationList
	if err := r.Client.List(ctx, &list); err != nil {
		if notServed(err) {
			return nil, &NotServedError{Kind: "Location", Err: err}
		}
		return nil, fmt.Errorf("listing locations: %w", err)
	}
	return list.Items, nil
}

// GetLocation distinguishes an absent kind from an absent object: a name that
// is not there is an ordinary answer, a kind that is not there is not.
func (r *ClientReader) GetLocation(ctx context.Context, name string) (*locationsv1alpha1.Location, error) {
	var location locationsv1alpha1.Location
	if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &location); err != nil {
		if apimeta.IsNoMatchError(err) {
			return nil, &NotServedError{Kind: "Location", Err: err}
		}
		return nil, fmt.Errorf("getting location %s: %w", name, err)
	}
	return &location, nil
}

// asNotServed returns the NotServedError in err's chain, if any.
func asNotServed(err error) (*NotServedError, bool) {
	var ns *NotServedError
	if errors.As(err, &ns) {
		return ns, true
	}
	return nil, false
}
