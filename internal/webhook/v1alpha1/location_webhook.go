// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

var locationLog = logf.Log.WithName("location-webhook")

// SetupWebhookWithManager registers the Location webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	webhook := &locationWebhook{}

	return ctrl.NewWebhookManagedBy(mgr, &locationsv1alpha1.Location{}).
		WithDefaulter(webhook).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-locations-miloapis-com-v1alpha1-location,mutating=true,failurePolicy=fail,sideEffects=None,groups=locations.miloapis.com,resources=locations,verbs=create;update,versions=v1alpha1,name=mlocation.kb.io,admissionReviewVersions=v1

// +kubebuilder:webhook:path=/validate-locations-miloapis-com-v1alpha1-location,mutating=false,failurePolicy=fail,sideEffects=None,groups=locations.miloapis.com,resources=locations,verbs=create;update;delete,versions=v1alpha1,name=vlocation.kb.io,admissionReviewVersions=v1

type locationWebhook struct{}

var _ admission.Defaulter[*locationsv1alpha1.Location] = &locationWebhook{}
var _ admission.Validator[*locationsv1alpha1.Location] = &locationWebhook{}

// Default implements admission.Defaulter.
func (r *locationWebhook) Default(ctx context.Context, location *locationsv1alpha1.Location) error {
	locationLog.Info("defaulting", "name", location.GetName())
	return nil
}

// ValidateCreate implements admission.Validator.
func (r *locationWebhook) ValidateCreate(ctx context.Context, location *locationsv1alpha1.Location) (admission.Warnings, error) {
	locationLog.Info("validating create", "name", location.GetName())
	return nil, nil
}

// ValidateUpdate implements admission.Validator.
func (r *locationWebhook) ValidateUpdate(ctx context.Context, oldLocation, newLocation *locationsv1alpha1.Location) (admission.Warnings, error) {
	locationLog.Info("validating update", "name", newLocation.GetName())
	return nil, nil
}

// ValidateDelete implements admission.Validator.
func (r *locationWebhook) ValidateDelete(ctx context.Context, location *locationsv1alpha1.Location) (admission.Warnings, error) {
	locationLog.Info("validating delete", "name", location.GetName())
	return nil, nil
}
