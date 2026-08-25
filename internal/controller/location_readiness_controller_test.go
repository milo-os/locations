// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

const testControllerName = "locations.miloapis.com/shared-edge"

func newReadinessReconciler(
	t *testing.T,
	objects ...client.Object,
) (*LocationReadinessReconciler, client.Client, *stubRecorder) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := locationsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed building the scheme: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&locationsv1alpha1.Location{}, &locationsv1alpha1.LocationClass{}).
		Build()

	recorder := &stubRecorder{}
	return &LocationReadinessReconciler{
		Client:         c,
		ControllerName: testControllerName,
		Recorder:       recorder,
	}, c, recorder
}

func classNamingController(name, controllerName string) *locationsv1alpha1.LocationClass {
	return &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       locationsv1alpha1.LocationClassSpec{ControllerName: controllerName},
	}
}

func locationWithTopology(
	name, className, project string,
	topology map[string]string,
) *locationsv1alpha1.Location {
	return &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: locationsv1alpha1.LocationSpec{
			LocationClassRef: locationsv1alpha1.LocationClassReference{
				Name:    className,
				Project: project,
			},
			Topology: topology,
		},
	}
}

func reconcileLocation(t *testing.T, r *LocationReadinessReconciler, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: client.ObjectKey{Name: name}}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
}

func locationReadyCondition(t *testing.T, c client.Client, name string) *metav1.Condition {
	t.Helper()
	var location locationsv1alpha1.Location
	if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &location); err != nil {
		t.Fatalf("failed reading location %q: %v", name, err)
	}
	return meta.FindStatusCondition(location.Status.Conditions, locationsv1alpha1.LocationConditionReady)
}

func TestAClaimedLocationBecomesReady(t *testing.T) {
	reconciler, c, recorder := newReadinessReconciler(t,
		classNamingController("shared-edge", testControllerName),
		locationWithTopology("sjc-1", "shared-edge", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}))

	reconcileLocation(t, reconciler, "sjc-1")

	condition := locationReadyCondition(t, c, "sjc-1")
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True, got %+v", condition)
	}
	if condition.Reason != locationsv1alpha1.LocationReasonReady {
		t.Fatalf("expected reason %q, got %q", locationsv1alpha1.LocationReasonReady, condition.Reason)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %+v", recorder.events)
	}
}

func TestAMissingClassLeavesTheLocationNotReady(t *testing.T) {
	reconciler, c, _ := newReadinessReconciler(t,
		locationWithTopology("sjc-1", "shared-edge", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}))

	reconcileLocation(t, reconciler, "sjc-1")

	condition := locationReadyCondition(t, c, "sjc-1")
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("expected Ready=False, got %+v", condition)
	}
	if condition.Reason != locationsv1alpha1.LocationReasonLocationClassNotFound {
		t.Fatalf("expected reason %q, got %q",
			locationsv1alpha1.LocationReasonLocationClassNotFound, condition.Reason)
	}
}

func TestAClassNamingAnotherControllerIsLeftAlone(t *testing.T) {
	reconciler, c, recorder := newReadinessReconciler(t,
		classNamingController("shared-edge", "locations.miloapis.com/somebody-else"),
		locationWithTopology("sjc-1", "shared-edge", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}))

	reconcileLocation(t, reconciler, "sjc-1")

	if condition := locationReadyCondition(t, c, "sjc-1"); condition != nil {
		t.Fatalf("a location claimed by another controller must be left untouched, got %+v", condition)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("expected no events, got %+v", recorder.events)
	}
}

func TestAForeignProjectClassRefIsNeverClaimed(t *testing.T) {
	reconciler, c, _ := newReadinessReconciler(t,
		classNamingController("shared-edge", testControllerName),
		locationWithTopology("sjc-1", "shared-edge", "other-project",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}))

	reconcileLocation(t, reconciler, "sjc-1")

	if condition := locationReadyCondition(t, c, "sjc-1"); condition != nil {
		t.Fatalf("a class in another control plane must not be claimed, got %+v", condition)
	}
}

func TestAMissingCityCodeIsReportedOnTheLocation(t *testing.T) {
	reconciler, c, recorder := newReadinessReconciler(t,
		classNamingController("shared-edge", testControllerName),
		locationWithTopology("sjc-1", "shared-edge", "", map[string]string{"topology.datum.net/rack": "a1"}))

	reconcileLocation(t, reconciler, "sjc-1")

	condition := locationReadyCondition(t, c, "sjc-1")
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("expected Ready=False, got %+v", condition)
	}
	if condition.Reason != locationsv1alpha1.LocationReasonMissingTopology {
		t.Fatalf("expected reason %q, got %q",
			locationsv1alpha1.LocationReasonMissingTopology, condition.Reason)
	}
	if len(recorder.events) != 1 || recorder.events[0].reason != locationsv1alpha1.LocationReasonMissingTopology {
		t.Fatalf("expected a %q event, got %+v", locationsv1alpha1.LocationReasonMissingTopology, recorder.events)
	}
}

func TestReadinessIsRewrittenWhenTheClassAppears(t *testing.T) {
	reconciler, c, _ := newReadinessReconciler(t,
		locationWithTopology("sjc-1", "shared-edge", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}))

	reconcileLocation(t, reconciler, "sjc-1")
	if err := c.Create(context.Background(),
		classNamingController("shared-edge", testControllerName)); err != nil {
		t.Fatalf("failed creating the class: %v", err)
	}
	reconcileLocation(t, reconciler, "sjc-1")

	condition := locationReadyCondition(t, c, "sjc-1")
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True once the class exists, got %+v", condition)
	}
}

func TestEnqueuesTheLocationsNamingAChangedClass(t *testing.T) {
	reconciler, _, _ := newReadinessReconciler(t,
		classNamingController("shared-edge", testControllerName),
		locationWithTopology("sjc-1", "shared-edge", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"}),
		locationWithTopology("iad-1", "other-class", "",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "IAD"}),
		locationWithTopology("lhr-1", "shared-edge", "other-project",
			map[string]string{locationsv1alpha1.TopologyCityCodeKey: "LHR"}))

	requests := reconciler.enqueueLocationsNamingClass(context.Background(),
		classNamingController("shared-edge", testControllerName))

	if len(requests) != 1 || requests[0].Name != "sjc-1" {
		t.Fatalf("expected only sjc-1 to be enqueued, got %+v", requests)
	}
}

func newAcceptedReconciler(
	t *testing.T,
	objects ...client.Object,
) (*LocationClassAcceptedReconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := locationsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed building the scheme: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&locationsv1alpha1.LocationClass{}).
		Build()

	return &LocationClassAcceptedReconciler{
		Client:         c,
		ControllerName: testControllerName,
		Recorder:       &stubRecorder{},
	}, c
}

func acceptedCondition(t *testing.T, c client.Client, name string) *metav1.Condition {
	t.Helper()
	var class locationsv1alpha1.LocationClass
	if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &class); err != nil {
		t.Fatalf("failed reading location class %q: %v", name, err)
	}
	return meta.FindStatusCondition(class.Status.Conditions,
		locationsv1alpha1.LocationClassConditionAccepted)
}

func TestAClassNamingThisControllerIsAccepted(t *testing.T) {
	reconciler, c := newAcceptedReconciler(t, classNamingController("shared-edge", testControllerName))

	if _, err := reconciler.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: client.ObjectKey{Name: "shared-edge"}}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	condition := acceptedCondition(t, c, "shared-edge")
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected Accepted=True, got %+v", condition)
	}
}

func TestAClassNamingAnotherControllerIsNotAccepted(t *testing.T) {
	reconciler, c := newAcceptedReconciler(t,
		classNamingController("shared-edge", "locations.miloapis.com/somebody-else"))

	if _, err := reconciler.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: client.ObjectKey{Name: "shared-edge"}}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if condition := acceptedCondition(t, c, "shared-edge"); condition != nil {
		t.Fatalf("a class naming another controller must stay unclaimed, got %+v", condition)
	}
}
