// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

var locationLog = logf.Log.WithName("location-webhook")

// SetupWebhookWithManager registers the Location webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	webhook := &locationWebhook{}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&locationsv1alpha1.Location{}).
		WithDefaulter(webhook).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-locations-miloapis-com-v1alpha1-location,mutating=true,failurePolicy=fail,sideEffects=None,groups=locations.miloapis.com,resources=locations,verbs=create;update,versions=v1alpha1,name=mlocation.kb.io,admissionReviewVersions=v1

// +kubebuilder:webhook:path=/validate-locations-miloapis-com-v1alpha1-location,mutating=false,failurePolicy=fail,sideEffects=None,groups=locations.miloapis.com,resources=locations,verbs=create;update;delete,versions=v1alpha1,name=vlocation.kb.io,admissionReviewVersions=v1

type locationWebhook struct{}

var _ admission.CustomDefaulter = &locationWebhook{}
var _ admission.CustomValidator = &locationWebhook{}

// Default implements admission.CustomDefaulter.
func (r *locationWebhook) Default(ctx context.Context, obj runtime.Object) error {
	location, ok := obj.(*locationsv1alpha1.Location)
	if !ok {
		return fmt.Errorf("unexpected type %T", obj)
	}

	locationLog.Info("defaulting", "name", location.GetName())

	// TODO: add defaulting logic here

	return nil
}

// ValidateCreate implements admission.CustomValidator.
func (r *locationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	location, ok := obj.(*locationsv1alpha1.Location)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", obj)
	}

	locationLog.Info("validating create", "name", location.GetName())

	// TODO: add validation logic here

	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator.
func (r *locationWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := oldObj.(*locationsv1alpha1.Location)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", oldObj)
	}

	newLocation, ok := newObj.(*locationsv1alpha1.Location)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", newObj)
	}

	locationLog.Info("validating update", "name", newLocation.GetName())

	// TODO: add validation logic here

	return nil, nil
}

// ValidateDelete implements admission.CustomValidator.
func (r *locationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	location, ok := obj.(*locationsv1alpha1.Location)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", obj)
	}

	locationLog.Info("validating delete", "name", location.GetName())

	// TODO: add validation logic here

	return nil, nil
}
