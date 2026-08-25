// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

const (
	locationReadinessEventAction     = "ReconcileLocation"
	locationClassAcceptedEventAction = "AcceptLocationClass"
)

// LocationReadinessReconciler reports Ready on the Locations backed by a
// LocationClass naming this controller, and writes nothing on any other
// Location.
type LocationReadinessReconciler struct {
	Client client.Client

	// ControllerName is matched against a LocationClass spec.controllerName to
	// decide whether this operator claims the Locations of that class.
	ControllerName string

	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations,verbs=get;list;watch
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locationclasses,verbs=get;list;watch

func (r *LocationReadinessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("location", req.Name))

	var location locationsv1alpha1.Location
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &location); err != nil {
		if apierrors.IsNotFound(err) {
			locationReady.DeleteLabelValues(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed reading location %q: %w", req.Name, err)
	}

	if !location.DeletionTimestamp.IsZero() {
		locationReady.DeleteLabelValues(location.Name)
		return ctrl.Result{}, nil
	}

	condition, claimed, err := r.evaluate(ctx, &location)
	if err != nil || !claimed {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.report(ctx, &location, condition)
}

func (r *LocationReadinessReconciler) evaluate(
	ctx context.Context,
	location *locationsv1alpha1.Location,
) (metav1.Condition, bool, error) {
	classNames := IndexLocationsByClass(location)
	if len(classNames) == 0 {
		return metav1.Condition{}, false, nil
	}
	className := classNames[0]

	var class locationsv1alpha1.LocationClass
	err := r.Client.Get(ctx, client.ObjectKey{Name: className}, &class)
	if apierrors.IsNotFound(err) {
		return readyCondition(location, metav1.ConditionFalse,
			locationsv1alpha1.LocationReasonLocationClassNotFound,
			fmt.Sprintf("LocationClass %q does not exist in this control plane", className)), true, nil
	}
	if err != nil {
		return metav1.Condition{}, false, fmt.Errorf("failed reading location class %q: %w", className, err)
	}

	if class.Spec.ControllerName != r.ControllerName {
		return metav1.Condition{}, false, nil
	}

	if location.Spec.Topology[locationsv1alpha1.TopologyCityCodeKey] == "" {
		return readyCondition(location, metav1.ConditionFalse,
			locationsv1alpha1.LocationReasonMissingTopology,
			fmt.Sprintf("Location %q carries no %s, so it cannot be placed or published",
				location.Name, locationsv1alpha1.TopologyCityCodeKey)), true, nil
	}

	return readyCondition(location, metav1.ConditionTrue,
		locationsv1alpha1.LocationReasonReady,
		fmt.Sprintf("Backed by LocationClass %q", class.Name)), true, nil
}

func readyCondition(
	location *locationsv1alpha1.Location,
	status metav1.ConditionStatus,
	reason, message string,
) metav1.Condition {
	return metav1.Condition{
		Type:               locationsv1alpha1.LocationConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: location.Generation,
	}
}

func (r *LocationReadinessReconciler) report(
	ctx context.Context,
	location *locationsv1alpha1.Location,
	condition metav1.Condition,
) error {
	ready := 0.0
	if condition.Status == metav1.ConditionTrue {
		ready = 1
	}
	locationReady.WithLabelValues(location.Name).Set(ready)

	patch := client.MergeFrom(location.DeepCopy())
	if !meta.SetStatusCondition(&location.Status.Conditions, condition) {
		return nil
	}
	if err := r.Client.Status().Patch(ctx, location, patch); err != nil {
		return fmt.Errorf("failed reporting readiness for location %q: %w", location.Name, err)
	}

	eventType := "Normal"
	if condition.Status != metav1.ConditionTrue {
		eventType = "Warning"
	}
	r.event(location, eventType, condition.Reason, locationReadinessEventAction, condition.Message)
	log.FromContext(ctx).Info("reported location readiness",
		"status", condition.Status, "reason", condition.Reason)
	return nil
}

func (r *LocationReadinessReconciler) event(obj client.Object, eventType, reason, action, message string) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(obj, nil, eventType, reason, action, "%s", message)
}

func (r *LocationReadinessReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("location-readiness")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("location_readiness").
		For(&locationsv1alpha1.Location{}).
		Watches(&locationsv1alpha1.LocationClass{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueLocationsNamingClass)).
		Complete(r)
}

func (r *LocationReadinessReconciler) enqueueLocationsNamingClass(
	ctx context.Context,
	class client.Object,
) []reconcile.Request {
	var locations locationsv1alpha1.LocationList
	if err := r.Client.List(ctx, &locations); err != nil {
		log.FromContext(ctx).Error(err, "failed listing the locations naming a class")
		return nil
	}

	var requests []reconcile.Request
	for i := range locations.Items {
		for _, name := range IndexLocationsByClass(&locations.Items[i]) {
			if name != class.GetName() {
				continue
			}
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: locations.Items[i].Name},
			})
		}
	}
	return requests
}

// LocationClassAcceptedReconciler reports Accepted on every LocationClass
// naming this controller, so a class no running controller recognises stays
// visibly unclaimed.
type LocationClassAcceptedReconciler struct {
	Client client.Client

	ControllerName string

	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=locations.miloapis.com,resources=locationclasses/status,verbs=get;update;patch

func (r *LocationClassAcceptedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("locationclass", req.Name))

	var class locationsv1alpha1.LocationClass
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &class); err != nil {
		if apierrors.IsNotFound(err) {
			locationClassAccepted.DeleteLabelValues(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed reading location class %q: %w", req.Name, err)
	}

	if class.Spec.ControllerName != r.ControllerName {
		return ctrl.Result{}, nil
	}
	if !class.DeletionTimestamp.IsZero() {
		locationClassAccepted.DeleteLabelValues(class.Name)
		return ctrl.Result{}, nil
	}

	locationClassAccepted.WithLabelValues(class.Name).Set(1)

	condition := metav1.Condition{
		Type:               locationsv1alpha1.LocationClassConditionAccepted,
		Status:             metav1.ConditionTrue,
		Reason:             locationsv1alpha1.LocationClassReasonAccepted,
		Message:            acceptedMessage(&class),
		ObservedGeneration: class.Generation,
	}
	patch := client.MergeFrom(class.DeepCopy())
	if !meta.SetStatusCondition(&class.Status.Conditions, condition) {
		return ctrl.Result{}, nil
	}
	if err := r.Client.Status().Patch(ctx, &class, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed accepting location class %q: %w", class.Name, err)
	}
	if r.Recorder != nil {
		r.Recorder.Eventf(&class, nil, "Normal", condition.Reason,
			locationClassAcceptedEventAction, "%s", condition.Message)
	}
	return ctrl.Result{}, nil
}

func acceptedMessage(class *locationsv1alpha1.LocationClass) string {
	message := fmt.Sprintf("Taken on by controller %q", class.Spec.ControllerName)
	if class.Spec.ParametersRef != nil {
		message += "; spec.parametersRef is not read by this controller"
	}
	return message
}

func (r *LocationClassAcceptedReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("location-class-accepted")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("location_class_accepted").
		For(&locationsv1alpha1.LocationClass{}).
		Complete(r)
}
