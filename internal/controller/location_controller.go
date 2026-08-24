// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

const (
	locationFinalizer = "locations.miloapis.com/location"
	ConditionTypeReady = "Ready"
)

// LocationReconciler reconciles a Location object.
type LocationReconciler struct {
	client client.Client
}

// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations/finalizers,verbs=update

func (r *LocationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var location locationsv1alpha1.Location
	if err := r.client.Get(ctx, req.NamespacedName, &location); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion with finalizer
	if !location.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &location)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(&location, locationFinalizer) {
		controllerutil.AddFinalizer(&location, locationFinalizer)
		if err := r.client.Update(ctx, &location); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Reconcile desired state
	location.Status.Phase = locationsv1alpha1.LocationPhaseReady
	location.Status.ObservedGeneration = location.Generation

	apimeta.SetStatusCondition(&location.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: location.Generation,
		Reason:             "LocationReady",
		Message:            "Location is ready.",
	})

	if err := r.client.Status().Update(ctx, &location); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	logger.Info("reconciled location", "phase", location.Status.Phase)
	return ctrl.Result{}, nil
}

func (r *LocationReconciler) reconcileDelete(ctx context.Context, location *locationsv1alpha1.Location) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	controllerutil.RemoveFinalizer(location, locationFinalizer)
	if err := r.client.Update(ctx, location); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}

	logger.Info("finalized location")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LocationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).
		Named("location").
		For(&locationsv1alpha1.Location{}).
		Complete(r)
}
