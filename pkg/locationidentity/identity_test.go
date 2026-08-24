// SPDX-License-Identifier: AGPL-3.0-only

package locationidentity

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

func deliveredCopy(name string) *locationsv1alpha1.ServingLocation {
	return &locationsv1alpha1.ServingLocation{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: locationsv1alpha1.ServingLocationSpec{
			Topology: map[string]string{locationsv1alpha1.TopologyCityCodeKey: "SJC"},
		},
	}
}

func identityScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := locationsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed building the scheme: %v", err)
	}
	return scheme
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name           string
		delivered      []client.Object
		configured     LocationConfig
		expectName     string
		expectSource   string
		expectMismatch bool
		expectReason   string
	}{
		{
			name:         "a single delivered copy is the cell's identity",
			delivered:    []client.Object{deliveredCopy("sjc-1")},
			expectName:   "sjc-1",
			expectSource: LocationIdentitySourceDelivered,
		},
		{
			name:         "delivery wins over configuration",
			delivered:    []client.Object{deliveredCopy("sjc-1")},
			configured:   LocationConfig{Name: "sjc-1"},
			expectName:   "sjc-1",
			expectSource: LocationIdentitySourceDelivered,
		},
		{
			name:           "a disagreement prefers delivered and is reported",
			delivered:      []client.Object{deliveredCopy("sjc-1")},
			configured:     LocationConfig{Name: "lhr-1"},
			expectName:     "sjc-1",
			expectSource:   LocationIdentitySourceDelivered,
			expectMismatch: true,
		},
		{
			name:         "with nothing delivered the configured location is used",
			configured:   LocationConfig{Name: "lhr-1"},
			expectName:   "lhr-1",
			expectSource: LocationIdentitySourceConfigured,
		},
		{
			name:         "more than one delivered copy never guesses",
			delivered:    []client.Object{deliveredCopy("sjc-1"), deliveredCopy("lhr-1")},
			expectReason: LocationUnresolvedAmbiguous,
		},
		{
			name:         "an ambiguous delivery falls back to the explicit configured answer",
			delivered:    []client.Object{deliveredCopy("sjc-1"), deliveredCopy("lhr-1")},
			configured:   LocationConfig{Name: "fra-1"},
			expectName:   "fra-1",
			expectSource: LocationIdentitySourceConfigured,
		},
		{
			name:         "with neither the cell waits visibly",
			expectReason: LocationUnresolvedNoIdentity,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := identityScheme(t)
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(test.delivered...).Build()

			identity, err := Resolve(context.Background(), reader, test.configured)

			if test.expectReason != "" {
				var unresolved *LocationUnresolved
				if !errors.As(err, &unresolved) {
					t.Fatalf("expected a waiting state, got identity=%+v err=%v", identity, err)
				}
				if unresolved.Reason != test.expectReason {
					t.Fatalf("expected reason %q, got %q", test.expectReason, unresolved.Reason)
				}
				if unresolved.Message == "" {
					t.Fatal("a waiting state must name what is missing")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if identity.Reference.Name != test.expectName {
				t.Fatalf("expected location %q, got %q", test.expectName, identity.Reference.Name)
			}
			if identity.Source != test.expectSource {
				t.Fatalf("expected source %q, got %q", test.expectSource, identity.Source)
			}
			if identity.Mismatch != test.expectMismatch {
				t.Fatalf("expected mismatch=%v, got %v", test.expectMismatch, identity.Mismatch)
			}
		})
	}
}
