// SPDX-License-Identifier: AGPL-3.0-only

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	servicesv1alpha1 "go.miloapis.com/service-catalog/api/v1alpha1"
)

const (
	testService     = "compute"
	testClientName  = "test-client"
	testServerName  = "test-server"
	testImplVersion = "0.0.1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(locationsv1alpha1.AddToScheme(s))
	utilruntime.Must(servicesv1alpha1.AddToScheme(s))
	return s
}

func location(name, city string, ready bool, extra map[string]string) *locationsv1alpha1.Location {
	topology := map[string]string{CityCodeKey: city}
	for k, v := range extra {
		topology[k] = v
	}
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	return &locationsv1alpha1.Location{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: locationsv1alpha1.LocationSpec{
			LocationClassRef: locationsv1alpha1.LocationClassReference{Name: "datum-managed"},
			Topology:         topology,
		},
		Status: locationsv1alpha1.LocationStatus{
			Conditions: []metav1.Condition{{
				Type:               locationsv1alpha1.LocationConditionReady,
				Status:             status,
				Reason:             locationsv1alpha1.LocationReasonReady,
				Message:            "Reported.",
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}

func availability(name, service, atLocation string, available bool) *servicesv1alpha1.ServiceAvailability {
	status := metav1.ConditionFalse
	if available {
		status = metav1.ConditionTrue
	}
	return &servicesv1alpha1.ServiceAvailability{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: servicesv1alpha1.ServiceAvailabilitySpec{
			ServiceRef:  servicesv1alpha1.ServiceRef{Name: service},
			LocationRef: servicesv1alpha1.LocationRef{Name: atLocation},
		},
		Status: servicesv1alpha1.ServiceAvailabilityStatus{
			Conditions: []metav1.Condition{{
				Type:               AvailableCondition,
				Status:             status,
				Reason:             "Reported",
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}

// fixture is the shape every join test reasons over: four places, and
// availability records covering every way a place can fail to make the list.
func fixture(t *testing.T) client.Client {
	t.Helper()
	dfw := location("dfw", "DFW", true, map[string]string{"topology.datum.net/region": "us-central-1"})
	iad := location("iad", "IAD", true, nil)
	ams := location("ams", "AMS", false, nil)
	lhr := location("lhr", "LHR", true, nil)

	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(dfw, iad, ams, lhr,
			availability("compute-dfw", testService, "dfw", true),
			// Offered, but not serving: still a real choice.
			availability("compute-ams", testService, "ams", true),
			// Recorded but not yet available: not a choice.
			availability("compute-iad", testService, "iad", false),
			// Another service entirely: must not leak into compute's answer.
			availability("storage-lhr", "storage", "lhr", true),
			// Names a place this project cannot see: must not invent a row.
			availability("compute-syd", testService, "syd", true),
		).
		Build()
}

func depsWith(reader Reader) DepsFor {
	return func(context.Context) (ToolDeps, error) { return ToolDeps{Reader: reader}, nil }
}

func listWith(t *testing.T, reader Reader, in LocationsListInput) (LocationsListOutput, error) {
	t.Helper()
	_, out, err := locationsList(depsWith(reader))(context.Background(), nil, in)
	return out, err
}

func getWith(t *testing.T, reader Reader, in LocationsGetInput) (LocationsGetOutput, error) {
	t.Helper()
	_, out, err := locationsGet(depsWith(reader))(context.Background(), nil, in)
	return out, err
}

// TestLocationsListJoinsAvailabilityToThePlacesItNames is the tool's whole
// job: only the places where the named service is really offered, in a stable
// order, with what each one declares about itself.
func TestLocationsListJoinsAvailabilityToThePlacesItNames(t *testing.T) {
	out, err := listWith(t, NewClientReader(fixture(t)), LocationsListInput{Service: testService})
	if err != nil {
		t.Fatalf("locations_list: %v", err)
	}

	// Sorted by name, so two calls in a conversation cannot disagree about the
	// order and read as a change.
	want := []string{"ams", "dfw"}
	got := make([]string, 0, len(out.Locations))
	for _, l := range out.Locations {
		got = append(got, l.Name)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("locations = %v, want exactly %v in that order", got, want)
	}
	if out.Service != testService {
		t.Errorf("service = %q, want %q", out.Service, testService)
	}
	if out.Note != "" {
		t.Errorf("note = %q, want none for a non-empty list", out.Note)
	}

	// Offered and serving are separate answers and both travel.
	byName := map[string]LocationView{}
	for _, l := range out.Locations {
		byName[l.Name] = l
	}
	if !byName["dfw"].Ready {
		t.Error("dfw reports Ready=True and must come back ready")
	}
	if byName["ams"].Ready {
		t.Error("ams is offered but not serving; ready must say so")
	}
	if byName["dfw"].CityCode != "DFW" {
		t.Errorf("cityCode = %q, want %q", byName["dfw"].CityCode, "DFW")
	}
	if byName["dfw"].Topology["topology.datum.net/region"] != "us-central-1" {
		t.Errorf("topology = %v, want every declared key to survive", byName["dfw"].Topology)
	}
}

// TestLocationsListFiltersByService: an availability record for another
// service must never widen this one's answer.
func TestLocationsListFiltersByService(t *testing.T) {
	out, err := listWith(t, NewClientReader(fixture(t)), LocationsListInput{Service: "storage"})
	if err != nil {
		t.Fatalf("locations_list: %v", err)
	}
	if len(out.Locations) != 1 || out.Locations[0].Name != "lhr" {
		t.Fatalf("locations = %+v, want only lhr", out.Locations)
	}
}

// TestLocationsListEmptyIsARealAnswer: nowhere offered is an answer, and it
// has to say that it is one.
func TestLocationsListEmptyIsARealAnswer(t *testing.T) {
	out, err := listWith(t, NewClientReader(fixture(t)), LocationsListInput{Service: "nothing-runs-here"})
	if err != nil {
		t.Fatalf("locations_list: %v", err)
	}
	if len(out.Locations) != 0 {
		t.Fatalf("locations = %+v, want none", out.Locations)
	}
	if !strings.Contains(out.Note, "not a failure to look it up") {
		t.Errorf("note = %q, want it to rule out a failed lookup", out.Note)
	}
}

func TestLocationsListNeedsAServiceName(t *testing.T) {
	for _, in := range []LocationsListInput{{}, {Service: "   "}} {
		if _, err := listWith(t, NewClientReader(fixture(t)), in); err == nil {
			t.Errorf("locations_list(%+v) succeeded, want a rejection", in)
		}
	}
}

// notServedReader refuses one kind and answers normally for the other, which
// is exactly how a project missing one of them behaves.
type notServedReader struct {
	Reader
	kind string
}

func (r notServedReader) ListAvailability(ctx context.Context) ([]servicesv1alpha1.ServiceAvailability, error) {
	if r.kind == "ServiceAvailability" {
		return nil, &NotServedError{Kind: r.kind, Err: &apimeta.NoKindMatchError{}}
	}
	return r.Reader.ListAvailability(ctx)
}

func (r notServedReader) ListLocations(ctx context.Context) ([]locationsv1alpha1.Location, error) {
	if r.kind == "Location" {
		return nil, &NotServedError{Kind: r.kind, Err: &apimeta.NoKindMatchError{}}
	}
	return r.Reader.ListLocations(ctx)
}

func (r notServedReader) GetLocation(ctx context.Context, name string) (*locationsv1alpha1.Location, error) {
	if r.kind == "Location" {
		return nil, &NotServedError{Kind: r.kind, Err: &apimeta.NoKindMatchError{}}
	}
	return r.Reader.GetLocation(ctx, name)
}

// TestNothingLookedNeverReadsAsNothingFound is the failure this tool exists to
// keep honest. "No places" and "nobody looked" call for opposite actions, so
// a project missing either record must fail loudly, name what is missing, and
// never degrade to an empty list.
func TestNothingLookedNeverReadsAsNothingFound(t *testing.T) {
	for _, kind := range []string{"ServiceAvailability", "Location"} {
		t.Run(kind, func(t *testing.T) {
			reader := notServedReader{Reader: NewClientReader(fixture(t)), kind: kind}

			out, err := listWith(t, reader, LocationsListInput{Service: testService})
			if err == nil {
				t.Fatalf("locations_list succeeded with %s absent, returning %+v", kind, out)
			}
			if len(out.Locations) != 0 {
				t.Errorf("locations = %+v alongside the error; nothing may be reported as looked at", out.Locations)
			}
			if !strings.Contains(err.Error(), kind) {
				t.Errorf("error = %q, want it to name the missing %s records", err, kind)
			}
			if !strings.Contains(err.Error(), "did nothing wrong") {
				t.Errorf("error = %q, want it to clear the person who asked", err)
			}

			if kind == "Location" {
				if _, err := getWith(t, reader, LocationsGetInput{Name: "dfw"}); err == nil {
					t.Fatal("locations_get succeeded with Location absent")
				} else if !strings.Contains(err.Error(), kind) {
					t.Errorf("error = %q, want it to name the missing %s records", err, kind)
				}
			}
		})
	}
}

// forbiddenReader is a project the caller may not read.
type forbiddenReader struct{ Reader }

func (forbiddenReader) ListAvailability(context.Context) ([]servicesv1alpha1.ServiceAvailability, error) {
	return nil, apierrors.NewForbidden(
		schema.GroupResource{Group: "services.miloapis.com", Resource: "serviceavailabilities"},
		"", fmt.Errorf("not allowed"))
}

// TestForbiddenIsNotAnEmptyList: no access is its own answer, distinct from
// both "nowhere" and "nobody looked".
func TestForbiddenIsNotAnEmptyList(t *testing.T) {
	out, err := listWith(t, forbiddenReader{Reader: NewClientReader(fixture(t))},
		LocationsListInput{Service: testService})
	if err == nil {
		t.Fatalf("locations_list succeeded without access, returning %+v", out)
	}
	if !strings.Contains(err.Error(), "not readable with the access this request carries") {
		t.Errorf("error = %q, want it to name the missing access", err)
	}
}

func TestLocationsGetReturnsOnePlaceInFull(t *testing.T) {
	c := fixture(t)
	withCoordinates := location("syd", "SYD", false, nil)
	withCoordinates.Spec.Coordinates = &locationsv1alpha1.Coordinates{
		Latitude:  "-33.8688",
		Longitude: "151.2093",
	}
	withCoordinates.Status.Conditions[0].Reason = locationsv1alpha1.LocationReasonLocationClassNotFound
	withCoordinates.Status.Conditions[0].Message = "Reported by the platform."
	if err := c.Create(context.Background(), withCoordinates); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	out, err := getWith(t, NewClientReader(c), LocationsGetInput{Name: "syd"})
	if err != nil {
		t.Fatalf("locations_get: %v", err)
	}
	if out.Name != "syd" || out.CityCode != "SYD" {
		t.Errorf("out = %+v, want syd/SYD", out)
	}
	if out.Ready {
		t.Error("syd is not serving; ready must say so")
	}
	if out.ReadyReason != locationsv1alpha1.LocationReasonLocationClassNotFound {
		t.Errorf("readyReason = %q, want what was reported", out.ReadyReason)
	}
	if out.Coordinates == nil || out.Coordinates.Latitude != "-33.8688" {
		t.Errorf("coordinates = %+v, want the declared point", out.Coordinates)
	}
}

func TestLocationsGetRejectsANameThatIsNotThere(t *testing.T) {
	_, err := getWith(t, NewClientReader(fixture(t)), LocationsGetInput{Name: "nowhere"})
	if err == nil {
		t.Fatal("locations_get succeeded for a name that does not exist")
	}
	if !strings.Contains(err.Error(), ToolLocationsList) {
		t.Errorf("error = %q, want it to point at %s for real names", err, ToolLocationsList)
	}
	if _, err := getWith(t, NewClientReader(fixture(t)), LocationsGetInput{Name: " "}); err == nil {
		t.Error("locations_get succeeded with no name")
	}
}

// TestNotServedClassification pins what counts as a kind this project does not
// carry. Everything downstream of it depends on the distinction.
func TestNotServedClassification(t *testing.T) {
	if !notServed(&apimeta.NoKindMatchError{}) {
		t.Error("a kind with no match must read as not served")
	}
	if !notServed(apierrors.NewNotFound(schema.GroupResource{Resource: "locations"}, "")) {
		t.Error("a missing resource path must read as not served")
	}
	if notServed(apierrors.NewForbidden(schema.GroupResource{Resource: "locations"}, "dfw", fmt.Errorf("no"))) {
		t.Error("no access is not the same as no kind; they call for opposite actions")
	}
}

// ---------------------------------------------------------------- MCP round trip

func connect(t *testing.T, deps DepsFor) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: testServerName, Version: testImplVersion}, nil)
	RegisterTools(server, deps)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: testClientName, Version: testImplVersion}, nil)
	clientSession, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

// TestRegisterToolsPublishesExactlyTheDocumentedSet inspects what a registered
// server actually advertises.
//
// The set is closed, and named, so a third tool cannot arrive without someone
// editing this list — the capability document's allow-list is the enforcement
// point, but a tool that does not exist cannot be reached through any path at
// all. It also catches a schema that fails to infer, since AddTool panics on a
// bad one.
func TestRegisterToolsPublishesExactlyTheDocumentedSet(t *testing.T) {
	session := connect(t, depsWith(NewClientReader(fixture(t))))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}

	got := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	want := []string{ToolLocationsList, ToolLocationsGet}
	if len(got) != len(want) {
		t.Errorf("published %d tools, want exactly %d", len(got), len(want))
	}
	for _, name := range want {
		tool, ok := got[name]
		if !ok {
			t.Errorf("tool %q is not published", name)
			continue
		}
		// The description is what tells a model when to reach for a tool; an
		// empty one silently degrades every answer.
		if tool.Description == "" {
			t.Errorf("tool %q has no description", name)
		}
		// Both tools change nothing, and saying so is what lets a model run
		// them without asking first.
		if !strings.Contains(tool.Description, "Read-only") {
			t.Errorf("tool %q does not promise it changes nothing: %q", name, tool.Description)
		}
	}
}

// TestNoToolTakesAProject is the prompt-injection guard, checked where it is
// actually enforceable: on the published schema. A project field the model
// could fill in would make another tenant's data one injected instruction
// away, so no input schema may have one.
func TestNoToolTakesAProject(t *testing.T) {
	session := connect(t, depsWith(NewClientReader(fixture(t))))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	for _, tool := range res.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshalling %s input schema: %v", tool.Name, err)
		}
		var parsed struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("parsing %s input schema: %v", tool.Name, err)
		}
		for field := range parsed.Properties {
			if strings.Contains(strings.ToLower(field), "project") {
				t.Errorf("%s takes %q as an argument; the project comes from the request only",
					tool.Name, field)
			}
		}
	}
}

// TestToolCallRoundTrip drives a real call over a real session, so the
// registered schemas are exercised end to end rather than only the handlers.
func TestToolCallRoundTrip(t *testing.T) {
	ctx := context.Background()
	session := connect(t, depsWith(NewClientReader(fixture(t))))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      ToolLocationsList,
		Arguments: map[string]any{"service": testService},
	})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tools/call returned an error result: %+v", res.Content)
	}

	var out LocationsListOutput
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshalling result: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if len(out.Locations) != 2 || out.Locations[0].Name != "ams" {
		t.Fatalf("locations = %+v, want ams and dfw", out.Locations)
	}
}
