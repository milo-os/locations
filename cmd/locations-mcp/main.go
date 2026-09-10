// SPDX-License-Identifier: AGPL-3.0-only

// Command locations-mcp publishes where services are offered, as tools an AI
// assistant can call, alongside the knowledge it reads before calling them:
//
//	POST /mcp             Streamable HTTP MCP, stateless
//	GET  /llms-full.txt   Knowledge: places, topology, offered vs serving
//	GET  /healthz         liveness
//
// One process, because the capability document naming this server names both
// URLs. Only /mcp takes a credential; see docs.go for why the document does
// not.
//
// The server holds no credential of its own for a project: it reads through a
// client built from the caller's own bearer token. A tool call can therefore
// never see more than the person who asked, the platform stays the single
// place where access is decided, and there is no privilege here to escalate
// with.
//
// The project a request reads is taken from a header, never from a tool
// argument. Arguments are chosen by a model, and a model that could name its
// own project would be one prompt injection away from another tenant's data.
// The header is set by the already-authenticated caller.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	"go.miloapis.com/locations/internal/agent"
	servicesv1alpha1 "go.miloapis.com/service-catalog/api/v1alpha1"
)

const (
	// projectHeader names the project a request reads. Set by the caller, not
	// by the model.
	projectHeader = "X-Datum-Project"

	serverName    = "milo-locations-mcp"
	serverVersion = "0.1.0"

	// readHeaderTimeout bounds how long a slow client may hold a connection
	// before sending headers.
	readHeaderTimeout = 10 * time.Second

	// misconfiguredClientNote closes every error a request can fail with
	// before a read is attempted. All of them are the calling client's
	// configuration, and an assistant relaying a vaguer wording told the
	// person to re-authenticate — so the sentence names the actor and rules
	// that out.
	misconfiguredClientNote = "The person who asked did nothing wrong and re-authenticating will not " +
		"help: this is a configuration problem for whoever operates that client"
)

// Build metadata set via -ldflags at build time. See Dockerfile.
var (
	version      = "dev"
	gitCommit    = "unknown"
	gitTreeState = "unknown"
	buildDate    = "unknown"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// The scheme carries both groups the tools join: the places themselves, and
// the records saying which services are offered at them. A group missing here
// fails at the first read with a message about the scheme, which says nothing
// about what was being asked.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(locationsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(servicesv1alpha1.AddToScheme(scheme))
}

func main() {
	var addr string

	flag.StringVar(&addr, "addr", envOr("LOCATIONS_MCP_ADDR", ":8080"),
		"address to serve MCP on")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog.Info("starting locations-mcp",
		"version", version,
		"gitCommit", gitCommit,
		"gitTreeState", gitTreeState,
		"buildDate", buildDate,
	)

	// GetConfig resolves the --kubeconfig flag controller-runtime registers,
	// then KUBECONFIG, then in-cluster config, then ~/.kube/config. Only the
	// endpoint and CA are used; see clientForToken.
	baseConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to load platform API configuration")
		os.Exit(1)
	}

	if err := checkAPIEndpoint(baseConfig); err != nil {
		setupLog.Error(err, "refusing to start")
		os.Exit(1)
	}

	if err := run(addr, baseConfig); err != nil {
		setupLog.Error(err, "server failed")
		os.Exit(1)
	}
}

// checkAPIEndpoint refuses to start when the configuration resolved to the API
// server of the cluster this process runs in.
//
// With no kubeconfig, GetConfig falls back to in-cluster config, and
// clientConfig then hangs a project path off the local API server. Every tool
// call fails with a 401 that reads like the caller's token is bad when the
// deployment is what is wrong; failing at boot puts the error where the
// mistake is.
//
// The check matches the SHAPE of the mistake, never an address: which API a
// deployment reads is deployment configuration, not this repo's.
func checkAPIEndpoint(cfg *rest.Config) error {
	local := localClusterEndpoint()
	if local == "" || !sameEndpoint(cfg.Host, local) {
		return nil
	}
	return fmt.Errorf(
		"endpoint %s is this cluster's own API server: locations-mcp reads Datum projects, not the "+
			"cluster it runs in, so the in-cluster fallback is never correct. Give the deployment an "+
			"explicit endpoint: mount a kubeconfig naming the platform API's address and CA, and point "+
			"KUBECONFIG at it (or pass --kubeconfig)",
		cfg.Host)
}

// localClusterEndpoint returns the API server address in-cluster config
// resolves to, or "" outside a cluster. It mirrors rest.InClusterConfig's own
// derivation rather than calling it, so the check still holds when the
// projected token InClusterConfig also requires is not mounted.
func localClusterEndpoint() string {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return ""
	}
	return "https://" + net.JoinHostPort(host, port)
}

// sameEndpoint compares two API server addresses, ignoring a trailing slash.
func sameEndpoint(a, b string) bool {
	return strings.TrimSuffix(a, "/") == strings.TrimSuffix(b, "/")
}

func run(addr string, baseConfig *rest.Config) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			// A server per request, bound to that caller's identity and
			// project. Nothing is shared between callers.
			s := mcp.NewServer(&mcp.Implementation{
				Name:    serverName,
				Version: serverVersion,
			}, nil)
			agent.RegisterTools(s, depsFromRequest(r, baseConfig))
			return s
		},
		// Stateless: nothing here needs session state, and it keeps the server
		// robust against clients that disappear mid-call.
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	docs, err := newKnowledgeHandler()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle(knowledgePath, docs)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	setupLog.Info("listening", "addr", addr, "mcp", "/mcp", "docs", docs.paths())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving on %s: %w", addr, err)
	}
	return nil
}

// depsFromRequest returns a DepsFor resolving the caller's identity and
// project from r. Resolution is deferred to call time so a request carrying no
// credentials fails as a tool error the model can report, rather than as a nil
// client panic deep in a handler.
func depsFromRequest(r *http.Request, baseConfig *rest.Config) agent.DepsFor {
	token := bearerToken(r)
	project := strings.TrimSpace(r.Header.Get(projectHeader))

	return func(context.Context) (agent.ToolDeps, error) {
		if token == "" {
			return agent.ToolDeps{}, fmt.Errorf(
				"no credentials on this request: the client that called this tool did not forward "+
					"the person's identity. %s", misconfiguredClientNote)
		}
		if project == "" {
			return agent.ToolDeps{}, fmt.Errorf(
				"no project on this request: the client that called this tool did not set the %s "+
					"header. %s", projectHeader, misconfiguredClientNote)
		}

		c, err := clientForToken(baseConfig, token, project)
		if err != nil {
			return agent.ToolDeps{}, err
		}
		return agent.ToolDeps{Reader: agent.NewClientReader(c)}, nil
	}
}

// projectPath returns the API path a project is read through. A project is
// addressed by rewriting the host path, not by a namespace named after it.
func projectPath(project string) string {
	return fmt.Sprintf("/apis/resourcemanager.miloapis.com/v1alpha1/projects/%s/control-plane", project)
}

// clientConfig derives the REST config one request reads through: the caller's
// token, pointed at their project. The base config supplies the endpoint and
// CA only — every credential field it might carry is cleared first, so the
// server's own identity can never leak into a caller's read.
func clientConfig(baseConfig *rest.Config, token, project string) (*rest.Config, error) {
	// The project arrives in a header and is interpolated into a URL path, so
	// it is validated before it can reshape that path into another API route.
	if errs := validation.IsDNS1123Subdomain(project); len(errs) > 0 {
		return nil, fmt.Errorf("invalid project %q on the %s header sent by the client that called "+
			"this tool: %s. %s", project, projectHeader, strings.Join(errs, "; "), misconfiguredClientNote)
	}

	cfg := rest.AnonymousClientConfig(rest.CopyConfig(baseConfig))
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""

	host, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("parsing API host: %w", err)
	}
	host.Path = projectPath(project)
	cfg.Host = host.String()

	return cfg, nil
}

// clientForToken builds a client that reads project as the bearer of token.
func clientForToken(baseConfig *rest.Config, token, project string) (client.Client, error) {
	cfg, err := clientConfig(baseConfig, token, project)
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("building client for caller: %w", err)
	}
	return c, nil
}

// bearerToken extracts a bearer token from the Authorization header.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
