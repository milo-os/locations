// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"flag"
	"fmt"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	"go.miloapis.com/locations/internal/config"
	"go.miloapis.com/locations/internal/controller"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme, serializer.EnableStrict)
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(config.RegisterDefaults(scheme))
	utilruntime.Must(locationsv1alpha1.AddToScheme(scheme))
}

func newOperatorCommand(info BuildInfo) *cobra.Command {
	var (
		enableLeaderElection    bool
		leaderElectionNamespace string
		probeAddr               string
		serverConfigFile        string
	)

	opts := zap.Options{
		Development: true,
	}

	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the locations operator (controller-runtime manager)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

			setupLog := ctrl.Log.WithName("setup")
			setupLog.Info("starting locations operator",
				"version", info.Version,
				"gitCommit", info.GitCommit,
				"gitTreeState", info.GitTreeState,
				"buildDate", info.BuildDate,
			)

			var serverConfig config.LocationOperator
			var configData []byte
			if len(serverConfigFile) > 0 {
				var err error
				configData, err = os.ReadFile(serverConfigFile)
				if err != nil {
					return fmt.Errorf("reading server config from %q: %w", serverConfigFile, err)
				}
			}

			if err := runtime.DecodeInto(codecs.UniversalDecoder(), configData, &serverConfig); err != nil {
				return fmt.Errorf("decoding server config: %w", err)
			}

			setupLog.Info("server config loaded", "kubeconfigPath", serverConfig.KubeconfigPath)

			if err := serverConfig.Validate(); err != nil {
				return fmt.Errorf("invalid server config: %w", err)
			}

			cfg, err := serverConfig.RestConfig()
			if err != nil {
				return fmt.Errorf("loading rest config: %w", err)
			}

			ctx := ctrl.SetupSignalHandler()

			bootstrapClient, err := client.New(cfg, client.Options{Scheme: scheme})
			if err != nil {
				return fmt.Errorf("creating bootstrap client: %w", err)
			}

			metricsServerOptions := serverConfig.MetricsServer.Options(ctx, bootstrapClient)


			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:                  scheme,
				Metrics:                 metricsServerOptions,
				HealthProbeBindAddress:  probeAddr,
				LeaderElection:          enableLeaderElection,
				LeaderElectionID:        "locations.miloapis.com",
				LeaderElectionNamespace: leaderElectionNamespace,
			})
			if err != nil {
				return fmt.Errorf("starting manager: %w", err)
			}

			if err := (&controller.LocationClassReconciler{}).SetupWithManager(ctx, mgr); err != nil {
				return fmt.Errorf("creating location class controller: %w", err)
			}

			if serverConfig.LocationPublisher.Enabled() {
				if err := setupLocationPublisher(serverConfig, mgr); err != nil {
					return fmt.Errorf("creating location publisher: %w", err)
				}
				setupLog.Info("location publisher enabled",
					"hubKubeconfigPath", serverConfig.LocationPublisher.HubKubeconfigPath)
			} else {
				setupLog.Info("locationPublisher.hubKubeconfigPath not set; publishing disabled")
			}


			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				return fmt.Errorf("setting up health check: %w", err)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				return fmt.Errorf("setting up ready check: %w", err)
			}

			setupLog.Info("starting manager")
			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("running manager: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().StringVar(&leaderElectionNamespace, "leader-elect-namespace", "", "The namespace to use for leader election.")
	cmd.Flags().StringVar(&serverConfigFile, "server-config", "", "Path to the server config file.")

	// zap.Options.BindFlags accepts *flag.FlagSet (stdlib). Bridge via pflag's
	// AddGoFlagSet so the zap flags are surfaced on the cobra command.
	zapFlags := flag.NewFlagSet("zap", flag.ContinueOnError)
	opts.BindFlags(zapFlags)
	cmd.Flags().AddGoFlagSet(zapFlags)

	return cmd
}

// leaderElectedRunnable states a runnable's leader-election intent explicitly.
type leaderElectedRunnable struct {
	manager.Runnable
	leaderElected bool
}

func (r leaderElectedRunnable) NeedLeaderElection() bool { return r.leaderElected }

// setupLocationPublisher connects the publisher to the control plane it reads
// Locations from and the federation hub it writes copies to. Both connections
// run only on the leader.
func setupLocationPublisher(serverConfig config.LocationOperator, mgr manager.Manager) error {
	sourceRestConfig, err := serverConfig.LocationPublisher.SourceRestConfig(serverConfig.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("loading the location source kubeconfig: %w", err)
	}
	hubRestConfig, err := serverConfig.LocationPublisher.HubRestConfig()
	if err != nil {
		return fmt.Errorf("loading the federation hub kubeconfig: %w", err)
	}

	sourceCluster, err := cluster.New(sourceRestConfig, func(o *cluster.Options) {
		o.Scheme = scheme
	})
	if err != nil {
		return fmt.Errorf("constructing the location source cluster: %w", err)
	}

	hubCluster, err := cluster.New(hubRestConfig, func(o *cluster.Options) {
		o.Scheme = scheme
		o.Client = client.Options{Cache: &client.CacheOptions{Unstructured: true}}
	})
	if err != nil {
		return fmt.Errorf("constructing the federation hub cluster: %w", err)
	}

	for _, c := range []cluster.Cluster{sourceCluster, hubCluster} {
		if err := mgr.Add(leaderElectedRunnable{Runnable: c, leaderElected: true}); err != nil {
			return fmt.Errorf("adding a location publisher cluster: %w", err)
		}
	}

	return (&controller.LocationPublisherReconciler{
		Config:        serverConfig,
		SourceCluster: sourceCluster,
		HubCluster:    hubCluster,
	}).SetupWithManager(mgr)
}
