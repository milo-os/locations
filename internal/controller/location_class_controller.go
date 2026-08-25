// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

const (
	// LocationClassFinalizer is held on a LocationClass for as long as a
	// Location in this control plane names it, so the class cannot be deleted
	// out from under the locations it backs.
	LocationClassFinalizer = "location-exists-finalizer.locations.miloapis.com"

	// LocationClassRefIndex indexes Locations by the name of the local
	// LocationClass they name. A Location naming a class in another control
	// plane is not indexed.
	LocationClassRefIndex = "spec.locationClassRef.name"

	locationClassBlockedReason = "LocationsStillReference"
	locationClassEventAction   = "DeleteLocationClass"

	blockedReferenceSample = 10
)

// LocationClassReconciler holds a finalizer on every LocationClass and refuses
// to release it while Locations still name that class.
type LocationClassReconciler struct {
	Client client.Client

	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locationclasses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locationclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations,verbs=get;list;watch

func (r *LocationClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("locationclass", req.Name))

	var class locationsv1alpha1.LocationClass
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &class); err != nil {
		if apierrors.IsNotFound(err) {
			locationClassDeletionBlocked.DeletePartialMatch(map[string]string{metricLabelClass: req.Name})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed reading location class %q: %w", req.Name, err)
	}

	if class.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.hold(ctx, &class)
	}
	return r.release(ctx, &class)
}

func (r *LocationClassReconciler) hold(ctx context.Context, class *locationsv1alpha1.LocationClass) error {
	locationClassDeletionBlocked.DeletePartialMatch(map[string]string{metricLabelClass: class.Name})

	if controllerutil.ContainsFinalizer(class, LocationClassFinalizer) {
		return nil
	}

	patch := client.MergeFrom(class.DeepCopy())
	controllerutil.AddFinalizer(class, LocationClassFinalizer)
	if err := r.Client.Patch(ctx, class, patch); err != nil {
		return fmt.Errorf("failed holding location class %q: %w", class.Name, err)
	}
	return nil
}

func (r *LocationClassReconciler) release(
	ctx context.Context,
	class *locationsv1alpha1.LocationClass,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	referencing, err := r.referencingLocations(ctx, class.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(referencing) > 0 {
		locationClassDeletionBlocked.WithLabelValues(class.Name, locationClassBlockedReason).Set(1)
		message := blockedMessage(referencing)
		if err := r.recordBlockedDeletion(ctx, class, message); err != nil {
			return ctrl.Result{}, err
		}
		r.event(class, "Warning", locationClassBlockedReason, message)
		logger.Info("deletion blocked, holding the location class", "message", message)
		return ctrl.Result{RequeueAfter: removalBlockedRequeue}, nil
	}

	if controllerutil.ContainsFinalizer(class, LocationClassFinalizer) {
		patch := client.MergeFrom(class.DeepCopy())
		controllerutil.RemoveFinalizer(class, LocationClassFinalizer)
		if err := r.Client.Patch(ctx, class, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed releasing location class %q: %w", class.Name, err)
		}
	}

	locationClassDeletionBlocked.DeletePartialMatch(map[string]string{metricLabelClass: class.Name})
	return ctrl.Result{}, nil
}

func (r *LocationClassReconciler) referencingLocations(ctx context.Context, name string) ([]string, error) {
	var locations locationsv1alpha1.LocationList
	if err := r.Client.List(ctx, &locations,
		client.MatchingFields{LocationClassRefIndex: name}); err != nil {
		return nil, fmt.Errorf("failed listing locations naming class %q: %w", name, err)
	}

	names := make([]string, 0, len(locations.Items))
	for i := range locations.Items {
		names = append(names, locations.Items[i].Name)
	}
	sort.Strings(names)
	return names, nil
}

func blockedMessage(referencing []string) string {
	sample := referencing
	suffix := ""
	if len(sample) > blockedReferenceSample {
		sample = sample[:blockedReferenceSample]
		suffix = ", ..."
	}
	return fmt.Sprintf("%d location(s) still name this class: %s%s",
		len(referencing), strings.Join(sample, ", "), suffix)
}

func (r *LocationClassReconciler) recordBlockedDeletion(
	ctx context.Context,
	class *locationsv1alpha1.LocationClass,
	message string,
) error {
	annotated := locationClassBlockedReason + ": " + message
	if class.Annotations[LocationRemovalBlockedAnnotation] == annotated {
		return nil
	}

	patch := client.MergeFrom(class.DeepCopy())
	if class.Annotations == nil {
		class.Annotations = map[string]string{}
	}
	class.Annotations[LocationRemovalBlockedAnnotation] = annotated
	if err := r.Client.Patch(ctx, class, patch); err != nil {
		return fmt.Errorf("failed recording the blocked deletion: %w", err)
	}
	return nil
}

func (r *LocationClassReconciler) event(obj client.Object, eventType, reason, message string) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(obj, nil, eventType, reason, locationClassEventAction, "%s", message)
}

// IndexLocationsByClass indexes a Location under the class it names, and only
// when that class is local. A reference qualified by a project names a class in
// another control plane, which this operator neither reads nor protects.
func IndexLocationsByClass(obj client.Object) []string {
	location, ok := obj.(*locationsv1alpha1.Location)
	if !ok {
		return nil
	}
	if strings.TrimSpace(location.Spec.LocationClassRef.Project) != "" {
		return nil
	}
	name := strings.TrimSpace(location.Spec.LocationClassRef.Name)
	if name == "" {
		return nil
	}
	return []string{name}
}

func (r *LocationClassReconciler) SetupWithManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("location-class")
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &locationsv1alpha1.Location{},
		LocationClassRefIndex, IndexLocationsByClass); err != nil {
		return fmt.Errorf("failed indexing locations by class: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("location_class").
		For(&locationsv1alpha1.LocationClass{}).
		Watches(&locationsv1alpha1.Location{},
			handler.EnqueueRequestsFromMapFunc(enqueueLocalClass)).
		Complete(r)
}

func enqueueLocalClass(_ context.Context, obj client.Object) []reconcile.Request {
	names := IndexLocationsByClass(obj)
	requests := make([]reconcile.Request, 0, len(names))
	for _, name := range names {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	}
	return requests
}
