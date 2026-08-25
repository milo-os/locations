// SPDX-License-Identifier: AGPL-3.0-only

// Package crd runs the generated CRDs against a real apiserver (envtest) to
// guard schema rules the fake client cannot enforce: enums, required fields,
// and CEL validations never reach a fake client.
package crd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

var testClient client.Client

func TestMain(m *testing.M) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		os.Exit(m.Run())
	}

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "base", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "envtest start (installs generated CRDs): %v\n", err)
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	if err := locationsv1alpha1.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "add scheme: %v\n", err)
		os.Exit(1)
	}
	testClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build client: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = env.Stop()
	os.Exit(code)
}

func requireEnv(t *testing.T) client.Client {
	t.Helper()
	if testClient == nil {
		t.Skip("KUBEBUILDER_ASSETS unset; envtest binaries are required to exercise CRD schemas")
	}
	return testClient
}

func locationSpec() locationsv1alpha1.LocationSpec {
	return locationsv1alpha1.LocationSpec{
		LocationClassName: "shared-edge",
		ManagedBy:         locationsv1alpha1.LocationManagedByPlatform,
		Topology:          map[string]string{locationsv1alpha1.TopologyCityCodeKey: "DFW"},
	}
}

func TestLocationClassAcceptsParametersRef(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	class := &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: "with-parameters"},
		Spec: locationsv1alpha1.LocationClassSpec{
			ControllerName: "locations.miloapis.com/shared-edge",
			ParametersRef: &locationsv1alpha1.ParametersReference{
				Group: "compute.miloapis.com",
				Kind:  "EdgeCapacityPool",
				Name:  "shared-edge-pool",
			},
		},
	}
	if err := cl.Create(ctx, class); err != nil {
		t.Fatalf("create LocationClass: %v", err)
	}
	t.Cleanup(func() { _ = cl.Delete(ctx, class) })

	var got locationsv1alpha1.LocationClass
	if err := cl.Get(ctx, client.ObjectKeyFromObject(class), &got); err != nil {
		t.Fatalf("get LocationClass: %v", err)
	}
	if got.Spec.ParametersRef == nil || got.Spec.ParametersRef.Kind != "EdgeCapacityPool" {
		t.Fatalf("parametersRef did not round-trip: %+v", got.Spec.ParametersRef)
	}
}

func TestLocationClassParametersRefOptional(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	class := &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: "no-parameters"},
		Spec:       locationsv1alpha1.LocationClassSpec{ControllerName: "locations.miloapis.com/self-managed"},
	}
	if err := cl.Create(ctx, class); err != nil {
		t.Fatalf("create LocationClass without parametersRef: %v", err)
	}
	t.Cleanup(func() { _ = cl.Delete(ctx, class) })
}

func TestLocationClassRequiresControllerName(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	class := &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: "no-controller"},
	}
	err := cl.Create(ctx, class)
	if err == nil {
		t.Cleanup(func() { _ = cl.Delete(ctx, class) })
		t.Fatal("a LocationClass without controllerName must be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid error, got %v", err)
	}
}

func TestLocationClassControllerNameIsImmutable(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	class := &locationsv1alpha1.LocationClass{
		ObjectMeta: metav1.ObjectMeta{Name: "immutable-controller"},
		Spec:       locationsv1alpha1.LocationClassSpec{ControllerName: "locations.miloapis.com/shared-edge"},
	}
	if err := cl.Create(ctx, class); err != nil {
		t.Fatalf("create LocationClass: %v", err)
	}
	t.Cleanup(func() { _ = cl.Delete(ctx, class) })

	class.Spec.ControllerName = "locations.miloapis.com/other"
	err := cl.Update(ctx, class)
	if err == nil {
		t.Fatal("retargeting controllerName must be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid error, got %v", err)
	}
}

func TestLocationAcceptsKnownManagementResponsibilities(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	for _, managedBy := range []locationsv1alpha1.LocationManagementResponsibility{
		locationsv1alpha1.LocationManagedByPlatform,
		locationsv1alpha1.LocationManagedBySelf,
	} {
		t.Run(string(managedBy), func(t *testing.T) {
			spec := locationSpec()
			spec.ManagedBy = managedBy
			location := &locationsv1alpha1.Location{
				ObjectMeta: metav1.ObjectMeta{Name: "managed-by-" + strings.ToLower(string(managedBy))},
				Spec:       spec,
			}
			if err := cl.Create(ctx, location); err != nil {
				t.Fatalf("create Location managed by %s: %v", managedBy, err)
			}
			t.Cleanup(func() { _ = cl.Delete(ctx, location) })

			var got locationsv1alpha1.Location
			if err := cl.Get(ctx, client.ObjectKeyFromObject(location), &got); err != nil {
				t.Fatalf("get Location: %v", err)
			}
			if got.Spec.ManagedBy != managedBy {
				t.Fatalf("managedBy did not round-trip: got %q", got.Spec.ManagedBy)
			}
		})
	}
}

func TestLocationRejectsUnknownManagementResponsibility(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	spec := locationSpec()
	spec.ManagedBy = "datum-managed"
	location := &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: "unknown-managed-by"},
		Spec:       spec,
	}
	err := cl.Create(ctx, location)
	if err == nil {
		t.Cleanup(func() { _ = cl.Delete(ctx, location) })
		t.Fatal("a managedBy outside the enum must be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid error, got %v", err)
	}
}

func TestLocationRequiresManagementResponsibility(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	spec := locationSpec()
	spec.ManagedBy = ""
	location := &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: "no-managed-by"},
		Spec:       spec,
	}
	err := cl.Create(ctx, location)
	if err == nil {
		t.Cleanup(func() { _ = cl.Delete(ctx, location) })
		t.Fatal("a Location without managedBy must be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid error, got %v", err)
	}
}

func TestLocationRequiresLocationClassName(t *testing.T) {
	cl := requireEnv(t)
	ctx := context.Background()

	spec := locationSpec()
	spec.LocationClassName = ""
	location := &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: "no-class"},
		Spec:       spec,
	}
	err := cl.Create(ctx, location)
	if err == nil {
		t.Cleanup(func() { _ = cl.Delete(ctx, location) })
		t.Fatal("a Location without locationClassName must be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid error, got %v", err)
	}
}
