// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

type recordedEvent struct {
	reason  string
	message string
}

type stubRecorder struct {
	events []recordedEvent
}

func (r *stubRecorder) Eventf(
	_ runtime.Object,
	_ runtime.Object,
	_, reason, _, note string,
	args ...any,
) {
	r.events = append(r.events, recordedEvent{reason: reason, message: fmt.Sprintf(note, args...)})
}

func newLocationClassReconciler(
	t *testing.T,
	objects ...client.Object,
) (*LocationClassReconciler, client.Client, *stubRecorder) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := locationsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed building the scheme: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&locationsv1alpha1.Location{}, LocationClassRefIndex, IndexLocationsByClass).
		WithObjects(objects...).
		Build()

	recorder := &stubRecorder{}
	return &LocationClassReconciler{Client: c, Recorder: recorder}, c, recorder
}

func locationClass(name string) *locationsv1alpha1.LocationClass {
	return &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: locationsv1alpha1.LocationClassSpec{
			ControllerName: "locations.miloapis.com/shared-edge",
		},
	}
}

func locationNamingClass(name, className, project string) *locationsv1alpha1.Location {
	return &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: locationsv1alpha1.LocationSpec{
			LocationClassRef: locationsv1alpha1.LocationClassReference{
				Name:    className,
				Project: project,
			},
			Topology: map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"},
		},
	}
}

func reconcileClass(t *testing.T, r *LocationClassReconciler, name string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	return result
}

func getClass(t *testing.T, c client.Client, name string) *locationsv1alpha1.LocationClass {
	t.Helper()
	var class locationsv1alpha1.LocationClass
	if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &class); err != nil {
		t.Fatalf("failed reading location class %q: %v", name, err)
	}
	return &class
}

func TestAClassIsHeldAsSoonAsItExists(t *testing.T) {
	reconciler, c, _ := newLocationClassReconciler(t, locationClass("shared-edge"))

	reconcileClass(t, reconciler, "shared-edge")

	if !controllerutil.ContainsFinalizer(getClass(t, c, "shared-edge"), LocationClassFinalizer) {
		t.Fatalf("expected the class to carry %q", LocationClassFinalizer)
	}
}

func TestDeletionIsBlockedWhileALocationNamesTheClass(t *testing.T) {
	reconciler, c, recorder := newLocationClassReconciler(t,
		locationClass("shared-edge"),
		locationNamingClass("sjc-1", "shared-edge", ""))

	reconcileClass(t, reconciler, "shared-edge")
	if err := c.Delete(context.Background(), locationClass("shared-edge")); err != nil {
		t.Fatalf("failed deleting the class: %v", err)
	}

	result := reconcileClass(t, reconciler, "shared-edge")
	if result.RequeueAfter == 0 {
		t.Fatal("a blocked deletion must keep re-checking")
	}

	class := getClass(t, c, "shared-edge")
	if !controllerutil.ContainsFinalizer(class, LocationClassFinalizer) {
		t.Fatal("the finalizer must be held while a location names the class")
	}
	blocked := class.Annotations[LocationRemovalBlockedAnnotation]
	if !strings.Contains(blocked, "sjc-1") {
		t.Fatalf("the blocked reason must name the referencing locations, got %q", blocked)
	}
	if len(recorder.events) != 1 || recorder.events[0].reason != locationClassBlockedReason {
		t.Fatalf("expected one %q event, got %+v", locationClassBlockedReason, recorder.events)
	}
}

func TestAForeignProjectReferenceDoesNotBlockDeletion(t *testing.T) {
	reconciler, c, _ := newLocationClassReconciler(t,
		locationClass("shared-edge"),
		locationNamingClass("sjc-1", "shared-edge", "other-project"))

	reconcileClass(t, reconciler, "shared-edge")
	if err := c.Delete(context.Background(), locationClass("shared-edge")); err != nil {
		t.Fatalf("failed deleting the class: %v", err)
	}
	reconcileClass(t, reconciler, "shared-edge")

	var class locationsv1alpha1.LocationClass
	err := c.Get(context.Background(), client.ObjectKey{Name: "shared-edge"}, &class)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("a class named across control planes must not block the local delete, err=%v", err)
	}
}

func TestDeletionProceedsWhenNothingNamesTheClass(t *testing.T) {
	reconciler, c, _ := newLocationClassReconciler(t,
		locationClass("shared-edge"),
		locationNamingClass("sjc-1", "other-class", ""))

	reconcileClass(t, reconciler, "shared-edge")
	if err := c.Delete(context.Background(), locationClass("shared-edge")); err != nil {
		t.Fatalf("failed deleting the class: %v", err)
	}
	reconcileClass(t, reconciler, "shared-edge")

	var class locationsv1alpha1.LocationClass
	err := c.Get(context.Background(), client.ObjectKey{Name: "shared-edge"}, &class)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the class to be gone, err=%v", err)
	}
}

func TestRemovingTheLastLocationReleasesTheClass(t *testing.T) {
	reconciler, c, _ := newLocationClassReconciler(t,
		locationClass("shared-edge"),
		locationNamingClass("sjc-1", "shared-edge", ""))

	reconcileClass(t, reconciler, "shared-edge")
	if err := c.Delete(context.Background(), locationClass("shared-edge")); err != nil {
		t.Fatalf("failed deleting the class: %v", err)
	}
	reconcileClass(t, reconciler, "shared-edge")

	if err := c.Delete(context.Background(), locationNamingClass("sjc-1", "shared-edge", "")); err != nil {
		t.Fatalf("failed deleting the location: %v", err)
	}
	reconcileClass(t, reconciler, "shared-edge")

	var class locationsv1alpha1.LocationClass
	err := c.Get(context.Background(), client.ObjectKey{Name: "shared-edge"}, &class)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the class to go once nothing names it, err=%v", err)
	}
}

func TestIndexLocationsByClass(t *testing.T) {
	tests := []struct {
		name     string
		location *locationsv1alpha1.Location
		expect   []string
	}{
		{
			name:     "a local reference is indexed",
			location: locationNamingClass("sjc-1", "shared-edge", ""),
			expect:   []string{"shared-edge"},
		},
		{
			name:     "a project-qualified reference names another control plane",
			location: locationNamingClass("sjc-1", "shared-edge", "other-project"),
			expect:   nil,
		},
		{
			name:     "a whitespace-only project is still local",
			location: locationNamingClass("sjc-1", "shared-edge", "   "),
			expect:   []string{"shared-edge"},
		},
		{
			name:     "an empty class name indexes nothing",
			location: locationNamingClass("sjc-1", "", ""),
			expect:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IndexLocationsByClass(test.location)
			if len(got) != len(test.expect) {
				t.Fatalf("expected %v, got %v", test.expect, got)
			}
			for i := range got {
				if got[i] != test.expect[i] {
					t.Fatalf("expected %v, got %v", test.expect, got)
				}
			}
		})
	}
}
