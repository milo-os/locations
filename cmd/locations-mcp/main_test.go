// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	"go.miloapis.com/locations/internal/agent"
)

const (
	testToken = "caller-token"
	// testHost stands in for the API a deployment is pointed at, and
	// testProjectName for the project a request names in its header.
	testHost        = "https://api.datum.example"
	testProjectName = "my-project"
	// serverToken is the identity this process might hold. It must never reach
	// an API server.
	serverToken = "server-service-account-token"
)

func baseConfig() *rest.Config {
	// Shaped like an in-cluster config: endpoint and CA, plus a server
	// identity that must not survive into a caller's read.
	return &rest.Config{
		Host:            testHost,
		BearerToken:     serverToken,
		BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		TLSClientConfig: rest.TLSClientConfig{CAFile: "/var/run/secrets/ca.crt"},
	}
}

// TestClientConfigAddressesTheCallersProject pins the addressing: a project is
// reached by rewriting the host path, not by a namespace named after it.
func TestClientConfigAddressesTheCallersProject(t *testing.T) {
	cfg, err := clientConfig(baseConfig(), testToken, testProjectName)
	if err != nil {
		t.Fatalf("clientConfig: %v", err)
	}

	want := "https://api.datum.example/apis/resourcemanager.miloapis.com/v1alpha1/projects/my-project/control-plane"
	if cfg.Host != want {
		t.Errorf("Host = %q, want %q", cfg.Host, want)
	}
}

// TestClientConfigCarriesOnlyTheCallerCredential is the security property the
// whole design rests on: the server must never read as itself.
func TestClientConfigCarriesOnlyTheCallerCredential(t *testing.T) {
	base := baseConfig()
	cfg, err := clientConfig(base, testToken, testProjectName)
	if err != nil {
		t.Fatalf("clientConfig: %v", err)
	}

	if cfg.BearerToken != testToken {
		t.Errorf("BearerToken = %q, want the caller's token", cfg.BearerToken)
	}
	if cfg.BearerTokenFile != "" {
		t.Errorf("BearerTokenFile = %q, want it cleared", cfg.BearerTokenFile)
	}
	if cfg.Impersonate.UserName != "" {
		t.Errorf("Impersonate = %q, want none", cfg.Impersonate.UserName)
	}
	if cfg.CAFile != base.CAFile {
		t.Errorf("CAFile = %q, want the base config's %q", cfg.CAFile, base.CAFile)
	}
	// The base config is shared by every request and must be left alone.
	if base.BearerToken != serverToken || base.Host != testHost {
		t.Error("clientConfig mutated the shared base config")
	}
}

// TestClientConfigRejectsAHostileProject: the project comes off a header and
// is interpolated into a URL path, so a value that could reshape that path
// into another API route has to be refused rather than escaped.
func TestClientConfigRejectsAHostileProject(t *testing.T) {
	for _, project := range []string{
		"../../../api/v1/nodes",
		"a/b",
		"proj/../other",
		"proj%2f..",
		"UPPER",
		"has space",
		"..",
		"",
	} {
		if _, err := clientConfig(baseConfig(), testToken, project); err == nil {
			t.Errorf("clientConfig(%q) succeeded, want a rejection", project)
		}
	}
}

// TestNoAmbientCredential: a request with no identity on it fails as a tool
// error, and never falls back to whatever this process holds.
func TestNoAmbientCredential(t *testing.T) {
	tests := []struct {
		name    string
		auth    string
		project string
		want    string
	}{
		{name: "no token", project: testProjectName, want: "no credentials"},
		{name: "wrong scheme", auth: "Basic abc", project: testProjectName, want: "no credentials"},
		{name: "no project", auth: "Bearer " + testToken, want: "no project"},
		{name: "invalid project", auth: "Bearer " + testToken, project: "a/b", want: "invalid project"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.auth != "" {
				r.Header.Set("Authorization", tc.auth)
			}
			if tc.project != "" {
				r.Header.Set(projectHeader, tc.project)
			}

			deps, err := depsFromRequest(r, baseConfig())(context.Background())
			if err == nil {
				t.Fatal("expected an error")
			}
			if deps.Reader != nil {
				t.Error("a Reader was built without a caller identity")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			// A tool error is handed back to the model; it must not carry
			// either credential.
			if strings.Contains(err.Error(), testToken) || strings.Contains(err.Error(), serverToken) {
				t.Errorf("error leaks a bearer token: %q", err)
			}
		})
	}
}

// TestCredentialErrorsBlameTheClientNotThePerson pins the copy on the errors a
// request fails with before any read happens. They reach a person through an
// assistant, so they are customer-facing text, and the credential is the
// calling client's to send — an error that tells the person to re-authenticate
// sends them somewhere they cannot help.
//
// internal/agent/copy_test.go holds the repo's plain-language denylist, but it
// scans values inside that package and cannot reach a string here, so the
// terms that could plausibly turn up in this copy are repeated.
func TestCredentialErrorsBlameTheClientNotThePerson(t *testing.T) {
	banned := regexp.MustCompile(`(?i)\b(rbac|control planes?|reconcil|milo|pods?|controllers?|cells?)\b`)
	// Second person is the failure mode itself: an error that says "you" hands
	// the model somebody to instruct, and the only person it can instruct is
	// the one who cannot fix this.
	secondPerson := regexp.MustCompile(`(?i)\byou(r|rs)?\b`)

	for name, err := range credentialErrors(t) {
		t.Run(name, func(t *testing.T) {
			msg := err.Error()
			if !strings.Contains(msg, "re-authenticating will not help") {
				t.Errorf("error = %q, want it to rule out re-authenticating", msg)
			}
			if !strings.Contains(msg, "client that called this tool") ||
				!strings.Contains(msg, "whoever operates that client") {
				t.Errorf("error = %q, want it to name the client as the actor", msg)
			}
			if m := secondPerson.FindString(msg); m != "" {
				t.Errorf("error = %q addresses the reader as %q; the reader cannot fix it", msg, m)
			}
			if m := banned.FindString(msg); m != "" {
				t.Errorf("error = %q uses %q, which a customer has no way to read", msg, m)
			}
		})
	}
}

// credentialErrors collects one error per way a request can fail before a
// read, keyed by what went missing.
func credentialErrors(t *testing.T) map[string]error {
	t.Helper()

	out := map[string]error{}
	for name, h := range map[string]struct{ auth, project string }{
		"no-token":   {project: "acme-prod"},
		"no-project": {auth: "Bearer " + testToken},
	} {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if h.auth != "" {
			r.Header.Set("Authorization", h.auth)
		}
		if h.project != "" {
			r.Header.Set(projectHeader, h.project)
		}
		_, err := depsFromRequest(r, baseConfig())(context.Background())
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		out[name] = err
	}

	_, err := clientConfig(baseConfig(), testToken, "Not A Project")
	if err == nil {
		t.Fatal("invalid-project: expected an error")
	}
	out["invalid-project"] = err
	return out
}

// TestProjectComesFromTheHeaderOnly guards the prompt-injection defense: a
// query parameter or body field a model could influence must not reach the
// addressing.
func TestProjectComesFromTheHeaderOnly(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/mcp?project=attacker-project", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	r.Header.Set(projectHeader, " my-project ")

	cfg, err := clientConfig(baseConfig(), bearerToken(r), strings.TrimSpace(r.Header.Get(projectHeader)))
	if err != nil {
		t.Fatalf("clientConfig: %v", err)
	}
	if strings.Contains(cfg.Host, "attacker-project") {
		t.Errorf("Host = %q, want the header's project", cfg.Host)
	}
	if !strings.Contains(cfg.Host, "/projects/my-project/control-plane") {
		t.Errorf("Host = %q, want the trimmed header project", cfg.Host)
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		auth string
		want string
	}{
		{auth: "Bearer " + testToken, want: testToken},
		{auth: "bearer " + testToken, want: testToken},
		{auth: "Bearer  " + testToken + " ", want: testToken},
		{auth: "Basic " + testToken, want: ""},
		{auth: testToken, want: ""},
		{auth: "", want: ""},
	}
	for _, tc := range tests {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		if got := bearerToken(r); got != tc.want {
			t.Errorf("bearerToken(%q) = %q, want %q", tc.auth, got, tc.want)
		}
	}
}

// TestReadsGoThroughTheCallersProject drives a real client at a stub API
// server: a read must land on the project's path with the caller's token.
func TestReadsGoThroughTheCallersProject(t *testing.T) {
	var gotPath, gotAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"locations.miloapis.com/v1alpha1","kind":"LocationList","items":[]}`))
	}))
	defer api.Close()

	cfg, err := clientConfig(&rest.Config{Host: api.URL}, testToken, testProjectName)
	if err != nil {
		t.Fatalf("clientConfig: %v", err)
	}

	// A static mapper keeps the test off discovery; the path under test is the
	// one clientConfig put on the config, not one the mapper chose.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{locationsv1alpha1.GroupVersion})
	mapper.AddSpecific(
		locationsv1alpha1.GroupVersion.WithKind("Location"),
		locationsv1alpha1.GroupVersion.WithResource("locations"),
		locationsv1alpha1.GroupVersion.WithResource("location"),
		meta.RESTScopeRoot,
	)

	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	if _, err := agent.NewClientReader(c).ListLocations(context.Background()); err != nil {
		t.Fatalf("ListLocations: %v", err)
	}

	wantPath := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/my-project/control-plane" +
		"/apis/locations.miloapis.com/v1alpha1/locations"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if gotAuth != "Bearer "+testToken {
		t.Errorf("Authorization = %q, want the caller's bearer token", gotAuth)
	}
}

// The address a cluster advertises for its own API server. The guard's whole
// job is spotting this endpoint.
const (
	localClusterHost = "10.0.0.1"
	localClusterPort = "443"
)

// TestCheckAPIEndpointRejectsTheInClusterFallback pins the startup guard: a
// deployment given no kubeconfig still gets in-cluster config, and every tool
// call then fails with a 401 that reads like the caller's credentials are bad.
// The guard turns that into a boot failure naming the actual mistake.
func TestCheckAPIEndpointRejectsTheInClusterFallback(t *testing.T) {
	tests := []struct {
		name        string
		serviceHost string
		servicePort string
		host        string
		wantErr     bool
	}{
		{
			name:        "in-cluster fallback",
			serviceHost: localClusterHost,
			servicePort: localClusterPort,
			host:        "https://10.0.0.1:443",
			wantErr:     true,
		},
		{
			name:        "in-cluster fallback, trailing slash",
			serviceHost: localClusterHost,
			servicePort: localClusterPort,
			host:        "https://10.0.0.1:443/",
			wantErr:     true,
		},
		{
			name:        "IPv6 in-cluster fallback",
			serviceHost: "fd00::1",
			servicePort: localClusterPort,
			host:        "https://[fd00::1]:443",
			wantErr:     true,
		},
		{
			// The deployment supplied a kubeconfig naming the platform API.
			// This is the only correct configuration.
			name:        "explicit endpoint",
			serviceHost: localClusterHost,
			servicePort: localClusterPort,
			host:        "https://api.datum.example:6443",
			wantErr:     false,
		},
		{
			// Same address, different port: an API fronted by the local
			// cluster is still not the local API server.
			name:        "same host, different port",
			serviceHost: localClusterHost,
			servicePort: localClusterPort,
			host:        "https://10.0.0.1:6443",
			wantErr:     false,
		},
		{
			// Outside a cluster there is no local API server to have fallen
			// back to, so the guard must not fire on a developer's kubeconfig.
			name:    "not in a cluster",
			host:    "https://127.0.0.1:6443",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.serviceHost)
			t.Setenv("KUBERNETES_SERVICE_PORT", tc.servicePort)

			err := checkAPIEndpoint(&rest.Config{Host: tc.host})
			if tc.wantErr == (err == nil) {
				t.Fatalf("checkAPIEndpoint(%q) error = %v, wantErr = %v", tc.host, err, tc.wantErr)
			}
			if err == nil {
				return
			}
			// Actionable: it must say what to set, not just that something is
			// wrong.
			if !strings.Contains(err.Error(), "KUBECONFIG") {
				t.Errorf("error = %q, want it to name the setting to fix", err)
			}
		})
	}
}

// TestGuardNamesNoEnvironment: the guard exists so deployment topology stays
// out of this repo, so it must not smuggle any in. It recognises the mistake
// from KUBERNETES_SERVICE_HOST/_PORT, which every cluster sets for itself.
func TestGuardNamesNoEnvironment(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	if got := localClusterEndpoint(); got != "" {
		t.Errorf("localClusterEndpoint() = %q with no cluster env, want %q", got, "")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "198.51.100.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	if got, want := localClusterEndpoint(), "https://198.51.100.1:443"; got != want {
		t.Errorf("localClusterEndpoint() = %q, want %q", got, want)
	}
}

// TestKnowledgeIsServedWithoutACredential: an assistant fetches the document
// before it holds any project context, so the route must not ask for one. It
// is static text with no tenant data in it.
func TestKnowledgeIsServedWithoutACredential(t *testing.T) {
	docs, err := newKnowledgeHandler()
	if err != nil {
		t.Fatalf("newKnowledgeHandler: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, knowledgePath, nil)
	w := httptest.NewRecorder()
	docs.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", knowledgePath, w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if len(body) == 0 {
		t.Fatal("knowledge document is empty")
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
	if paths := docs.paths(); len(paths) != 1 || paths[0] != knowledgePath {
		t.Errorf("paths() = %v, want just %q", paths, knowledgePath)
	}
}

// TestKnowledgeRoutingHasNothingToTraverse: routing is an exact-match lookup,
// never a path join, so nothing outside the embedded set can be reached.
func TestKnowledgeRoutingHasNothingToTraverse(t *testing.T) {
	docs, err := newKnowledgeHandler()
	if err != nil {
		t.Fatalf("newKnowledgeHandler: %v", err)
	}

	for _, path := range []string{
		"/../etc/passwd",
		"/llms-full.txt/../../etc/passwd",
		"/llms-full",
		"/",
	} {
		w := httptest.NewRecorder()
		docs.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://x"+path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, w.Code)
		}
	}

	w := httptest.NewRecorder()
	docs.ServeHTTP(w, httptest.NewRequest(http.MethodPost, knowledgePath, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST %s = %d, want 405", knowledgePath, w.Code)
	}
}

// TestCapabilityDocumentExampleMatchesWhatIsServed keeps the published example
// honest. An allow-list naming a tool this server does not publish loses that
// tool silently — the assistant simply never offers it — and a knowledge URL
// pointing at a path this process does not serve is a 404 nobody notices until
// an answer is worse than it should be.
func TestCapabilityDocumentExampleMatchesWhatIsServed(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "agent", "capability-document.json"))
	if err != nil {
		t.Fatalf("reading the capability document example: %v", err)
	}

	var doc struct {
		Metadata struct {
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			ServiceRef struct {
				Name string `json:"name"`
			} `json:"serviceRef"`
			ServiceName string `json:"serviceName"`
			Knowledge   struct {
				Sources []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"sources"`
			} `json:"knowledge"`
			Tools struct {
				MCPServers []struct {
					Name         string `json:"name"`
					Endpoint     string `json:"endpoint"`
					ToolSelector struct {
						Include []string `json:"include"`
					} `json:"toolSelector"`
				} `json:"mcpServers"`
			} `json:"tools"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the capability document example: %v", err)
	}

	// No namespace: this is a platform capability, composed into every
	// project rather than entitled to one.
	if doc.Metadata.Namespace != "" {
		t.Errorf("metadata.namespace = %q, want none — this capability belongs to every project",
			doc.Metadata.Namespace)
	}
	if doc.Spec.ServiceName != "locations.miloapis.com" {
		t.Errorf("serviceName = %q, want locations.miloapis.com", doc.Spec.ServiceName)
	}
	if doc.Spec.ServiceRef.Name != "locations" {
		t.Errorf("serviceRef.name = %q, want locations", doc.Spec.ServiceRef.Name)
	}

	published := map[string]bool{
		agent.ToolLocationsList: true,
		agent.ToolLocationsGet:  true,
	}
	if len(doc.Spec.Tools.MCPServers) != 1 {
		t.Fatalf("mcpServers = %d, want exactly one", len(doc.Spec.Tools.MCPServers))
	}
	server := doc.Spec.Tools.MCPServers[0]
	if !strings.HasSuffix(server.Endpoint, "/mcp") {
		t.Errorf("endpoint = %q, want the /mcp route this process serves", server.Endpoint)
	}
	if len(server.ToolSelector.Include) == 0 {
		t.Error("toolSelector.include is empty; no tool would reach the assistant")
	}
	for _, name := range server.ToolSelector.Include {
		if !published[name] {
			t.Errorf("toolSelector.include names %q, which this server does not publish", name)
		}
	}

	if len(doc.Spec.Knowledge.Sources) == 0 {
		t.Fatal("knowledge.sources is empty; the assistant would read nothing before calling a tool")
	}
	for _, source := range doc.Spec.Knowledge.Sources {
		if source.Type != "LLMDocs" {
			t.Errorf("knowledge source type = %q, want LLMDocs", source.Type)
		}
		if !strings.HasSuffix(source.URL, knowledgePath) {
			t.Errorf("knowledge source %q does not end in %q, the path this process serves",
				source.URL, knowledgePath)
		}
	}
}
