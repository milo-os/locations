// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// LocationOperator is the configuration for the locations operator.
type LocationOperator struct {
	metav1.TypeMeta

	MetricsServer MetricsServerConfig `json:"metricsServer"`


	// KubeconfigPath is the path to the kubeconfig file pointing at the Milo
	// control plane API server where locations are stored. When empty, the
	// controller falls back to in-cluster config / $KUBECONFIG via
	// ctrl.GetConfig(), which is useful for local development.
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`

	// LocationPublisher configures the controller that publishes Locations to
	// the federation hub as ServingLocations.
	LocationPublisher LocationPublisherConfig `json:"locationPublisher,omitempty"`
}

// Validate reports whether the operator configuration can serve.
func (c *LocationOperator) Validate() error {
	if err := c.LocationPublisher.Validate(); err != nil {
		return fmt.Errorf("locationPublisher: %w", err)
	}
	return nil
}

// RestConfig returns the *rest.Config used to connect to the Milo control plane.
// When KubeconfigPath is empty it falls back to the standard
// controller-runtime config resolution (in-cluster / $KUBECONFIG).
func (c *LocationOperator) RestConfig() (*rest.Config, error) {
	if c.KubeconfigPath == "" {
		return ctrl.GetConfig()
	}
	return clientcmd.BuildConfigFromFlags("", c.KubeconfigPath)
}


// +k8s:deepcopy-gen=true

// MetricsServerConfig configures the metrics server.
type MetricsServerConfig struct {
	// SecureServing enables serving metrics via https.
	SecureServing *bool `json:"secureServing,omitempty"`

	// BindAddress is the bind address for the metrics server.
	BindAddress string `json:"bindAddress"`

	// TLS is the TLS configuration for the metrics server.
	TLS TLSConfig `json:"tls"`
}

func SetDefaults_MetricsServerConfig(obj *MetricsServerConfig) {
	if obj.SecureServing == nil {
		obj.SecureServing = ptr.To(true)
	}

	if obj.BindAddress == "" {
		obj.BindAddress = "0"
	}

	if len(obj.TLS.CertDir) == 0 {
		obj.TLS.CertDir = filepath.Join(os.TempDir(), "k8s-metrics-server", "serving-certs")
	}
}

func (c *MetricsServerConfig) Options(ctx context.Context, secretsClient client.Client) metricsserver.Options {
	secureServing := c.SecureServing != nil && *c.SecureServing

	opts := metricsserver.Options{
		SecureServing: secureServing,
		BindAddress:   c.BindAddress,
		CertDir:       c.TLS.CertDir,
		CertName:      c.TLS.CertName,
		KeyName:       c.TLS.KeyName,
	}

	if secureServing {
		opts.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if secretRef := c.TLS.SecretRef; secretRef != nil {
		opts.TLSOpts = c.TLS.Options(ctx, secretsClient)
	}

	return opts
}

// +k8s:deepcopy-gen=true

// TLSConfig configures TLS certificate management.
type TLSConfig struct {
	// SecretRef is a reference to a secret that contains the server key and certificate.
	SecretRef *corev1.ObjectReference `json:"secretRef,omitempty"`

	// CertDir is the directory that contains the server key and certificate.
	CertDir string `json:"certDir"`

	// CertName is the server certificate name. Defaults to tls.crt.
	CertName string `json:"certName"`

	// KeyName is the server key name. Defaults to tls.key.
	KeyName string `json:"keyName"`
}

func (c *TLSConfig) Options(ctx context.Context, secretsClient client.Client) []func(*tls.Config) {
	var tlsOpts []func(*tls.Config)

	if secretRef := c.SecretRef; secretRef != nil {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			logger := ctrl.Log.WithName("tls-client")
			c.GetCertificate = func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				logger.Info("getting certificate")

				var secret corev1.Secret
				secretObjectKey := types.NamespacedName{
					Name:      secretRef.Name,
					Namespace: secretRef.Namespace,
				}
				if err := secretsClient.Get(ctx, secretObjectKey, &secret); err != nil {
					return nil, fmt.Errorf("failed to get secret: %w", err)
				}

				cert, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
				if err != nil {
					return nil, fmt.Errorf("failed to parse certificate: %w", err)
				}

				return &cert, nil
			}
		})
	}

	return tlsOpts
}

func SetDefaults_TLSConfig(obj *TLSConfig) {
	if len(obj.CertName) == 0 {
		obj.CertName = "tls.crt"
	}

	if len(obj.KeyName) == 0 {
		obj.KeyName = "tls.key"
	}
}

// SetDefaults_LocationOperator sets defaults for LocationOperator.
// The generated SetObjectDefaults_LocationOperator handles calling nested
// defaults (MetricsServerConfig, WebhookServerConfig, TLSConfig), so this
// function only sets top-level defaults.
func SetDefaults_LocationOperator(obj *LocationOperator) {
	// Top-level defaults are handled by nested SetDefaults_* functions
	// which are called by the generated SetObjectDefaults_LocationOperator.
}

func init() {
	SchemeBuilder.Register(&LocationOperator{})
}

// +k8s:deepcopy-gen=true

// LocationPublisherConfig configures the controller that copies Locations to
// the federation hub as ServingLocations, so that each cell is told which
// location it serves.
//
// Set hubKubeconfigPath to turn it on, and set it on exactly one deployment:
// two publishers writing the same hub fight over the same objects.
type LocationPublisherConfig struct {
	// SourceKubeconfigPath is the path to a kubeconfig for the control plane
	// holding the Locations to publish. Defaults to the operator's own
	// kubeconfigPath.
	SourceKubeconfigPath string `json:"sourceKubeconfigPath,omitempty"`

	// HubKubeconfigPath is the path to a kubeconfig for the federation hub the
	// copies are written to. Publishing stays off while this is empty.
	HubKubeconfigPath string `json:"hubKubeconfigPath,omitempty"`

	// SafetyResyncPeriod is how often the publisher re-reads both ends and
	// repairs any difference it finds. Publishing itself is driven by watches,
	// so this only bounds how long an edit made directly on the hub survives.
	// Shorten it to repair such edits sooner, at the cost of more API traffic.
	//
	// Defaults to 30 minutes.
	SafetyResyncPeriod metav1.Duration `json:"safetyResyncPeriod,omitempty"`

	// LocationScopedResources are the resource kinds carried to the cells
	// serving a location alongside the ServingLocation itself.
	//
	// A resource belongs to one location, so it is selected by the location it
	// names rather than fleet-wide: a cell serving nowhere would otherwise be
	// given every location's objects. Leave this empty and only the
	// ServingLocation is carried.
	LocationScopedResources []LocationScopedResource `json:"locationScopedResources,omitempty"`

	// Client configures the Kubernetes client connections to both the source
	// and the hub.
	Client ClientConnectionConfig `json:"client,omitempty"`
}

// +k8s:deepcopy-gen=true

// LocationScopedResource is a resource kind that belongs to a single location
// and is carried to the cells serving it.
type LocationScopedResource struct {
	// APIVersion of the resource, such as "networking.datumapis.com/v1alpha".
	APIVersion string `json:"apiVersion"`

	// Kind of the resource, such as "Subnet".
	Kind string `json:"kind"`

	// LocationLabel is the label on those objects naming the location they
	// belong to.
	LocationLabel string `json:"locationLabel"`
}

func SetDefaults_LocationPublisherConfig(obj *LocationPublisherConfig) {
	if obj.SafetyResyncPeriod.Duration == 0 {
		obj.SafetyResyncPeriod = metav1.Duration{Duration: 30 * time.Minute}
	}
}

func (c *LocationPublisherConfig) Enabled() bool {
	return c.HubKubeconfigPath != ""
}

func (c *LocationPublisherConfig) Validate() error {
	if !c.Enabled() {
		return nil
	}
	var errs []error
	if c.SafetyResyncPeriod.Duration < 0 {
		errs = append(errs, errors.New("safetyResyncPeriod must not be negative"))
	}
	for i, resource := range c.LocationScopedResources {
		if resource.APIVersion == "" || resource.Kind == "" || resource.LocationLabel == "" {
			errs = append(errs, fmt.Errorf(
				"locationScopedResources[%d]: apiVersion, kind and locationLabel are all required", i))
		}
	}
	return errors.Join(errs...)
}

// SourceRestConfig resolves the connection to the control plane the Location
// records are read from.
func (c *LocationPublisherConfig) SourceRestConfig(fallbackKubeconfigPath string) (*rest.Config, error) {
	path := c.SourceKubeconfigPath
	if path == "" {
		path = fallbackKubeconfigPath
	}
	return c.restConfig(path)
}

// HubRestConfig resolves the connection to the federation hub the published
// copies are written to.
func (c *LocationPublisherConfig) HubRestConfig() (*rest.Config, error) {
	return c.restConfig(c.HubKubeconfigPath)
}

func (c *LocationPublisherConfig) restConfig(path string) (*rest.Config, error) {
	var (
		cfg *rest.Config
		err error
	)
	if path == "" {
		cfg, err = ctrl.GetConfig()
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", path)
	}
	if err != nil {
		return nil, err
	}

	c.Client.ApplyTo(cfg)
	return cfg, nil
}

// +k8s:deepcopy-gen=true

// ClientConnectionConfig tunes client-side throttling on a connection.
type ClientConnectionConfig struct {
	// QPS is the maximum sustained queries per second before client-side
	// throttling kicks in.
	//
	// +default=50
	QPS float32 `json:"qps,omitempty"`

	// Burst is the maximum burst size for throttle. Requests above QPS but
	// below Burst are allowed immediately.
	//
	// +default=100
	Burst int `json:"burst,omitempty"`
}

// ApplyTo applies the client connection settings to a rest.Config.
func (c *ClientConnectionConfig) ApplyTo(cfg *rest.Config) {
	if c.QPS > 0 {
		cfg.QPS = c.QPS
	}
	if c.Burst > 0 {
		cfg.Burst = c.Burst
	}
}

func SetDefaults_ClientConnectionConfig(obj *ClientConnectionConfig) {
	if obj.QPS == 0 {
		obj.QPS = 50
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}
