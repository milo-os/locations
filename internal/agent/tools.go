// SPDX-License-Identifier: AGPL-3.0-only

// Package agent publishes what this service knows about where things are
// offered, as tools an AI assistant can call.
//
// Both tools are read-only. Nothing here creates, changes or deletes anything,
// and every read runs as the person who asked — see [Reader].
package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
)

// The tools this service publishes.
const (
	ToolLocationsList = "locations_list"
	ToolLocationsGet  = "locations_get"
)

// CityCodeKey is the well-known topology key carrying a location's city,
// lifted out of the topology map because almost every question about where
// something runs is really a question about which city.
const CityCodeKey = "topology.datum.net/city-code"

// Tool descriptions. They live here, rather than inline at registration, so
// copy_test.go can hold them to the same plain-language rules as every other
// sentence a customer reads.
const (
	locationsListDescription = "List the places a named service is offered to this project, each with what it " +
		"declares about where it is and whether it is serving right now. Take names from here verbatim " +
		"when something has to name a place: a name that is not on this list can never be satisfied, and " +
		"a request that uses one sits unplaced instead of failing loudly. A place that is missing means " +
		"either that the service is not offered there or that this project is not entitled to it there; " +
		"either way it cannot be used. An empty list is a real answer — the service is offered nowhere " +
		"this project can use — and says so. Read-only: this changes nothing."

	locationsGetDescription = "Get one place by name, with everything it declares about where it is — its city, " +
		"its region, and whatever else is published — its coordinates when it has them, and whether it " +
		"is serving, with what was reported if it is not. Use it after " + ToolLocationsList + " when a " +
		"decision turns on a detail the list does not carry. A name that is not offered to this project " +
		"is not found here, even if it exists somewhere else. Read-only: this changes nothing."
)

// ToolDeps is what one request's tool calls read through.
type ToolDeps struct {
	Reader Reader
}

// DepsFor resolves the dependencies for a tool call. A function rather than a
// value, so the caller decides how identity is established: the server derives
// it from each HTTP request, tests supply it directly.
type DepsFor func(context.Context) (ToolDeps, error)

// LocationView is one place a service is offered to this project.
type LocationView struct {
	Name string `json:"name"`
	// Ready reports whether the place is serving. A place can be offered and
	// still not be serving; the two are worth telling apart before promising
	// anything will start there.
	Ready bool `json:"ready"`
	// Topology is everything the place declares about where it is — its city,
	// its region, and whatever else is published. A selector that picks places
	// by region is matched against exactly these keys.
	Topology map[string]string `json:"topology,omitempty"`
	// CityCode is the city, lifted out of Topology because it is the key most
	// questions are really about.
	CityCode string `json:"cityCode,omitempty"`
}

// LocationsListInput names the service to ask about. There is deliberately no
// project field: see the package comment on [DepsFor] and the server's own.
type LocationsListInput struct {
	Service string `json:"service" jsonschema:"The service to ask about, named the way the platform names it, e.g. \"compute\"."`
}

// LocationsListOutput is where one service is offered.
type LocationsListOutput struct {
	Service   string         `json:"service"`
	Locations []LocationView `json:"locations"`
	// Note explains an empty list, so it is never read as a failure.
	Note string `json:"note,omitempty"`
}

// LocationsGetInput names one place.
type LocationsGetInput struct {
	Name string `json:"name" jsonschema:"The name of the place, taken verbatim from locations_list, e.g. \"us-east-1\"."`
}

// CoordinatesView is where a place is on a map, in decimal degrees.
type CoordinatesView struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

// LocationsGetOutput is one place in full.
type LocationsGetOutput struct {
	Name        string            `json:"name"`
	Ready       bool              `json:"ready"`
	Topology    map[string]string `json:"topology,omitempty"`
	CityCode    string            `json:"cityCode,omitempty"`
	Coordinates *CoordinatesView  `json:"coordinates,omitempty"`
	// ReadyReason and ReadyMessage carry what was reported about a place that
	// is not serving. Both are empty when nothing has been reported either way,
	// which is not the same as a reported failure.
	ReadyReason  string `json:"readyReason,omitempty"`
	ReadyMessage string `json:"readyMessage,omitempty"`
}

// RegisterTools adds both tools to s. deps is consulted per call rather than
// captured once, so no caller can inherit another's identity.
func RegisterTools(s *mcp.Server, deps DepsFor) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolLocationsList,
		Title:       "List where a service is offered",
		Description: locationsListDescription,
	}, locationsList(deps))

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolLocationsGet,
		Title:       "Get one place in full",
		Description: locationsGetDescription,
	}, locationsGet(deps))
}

func locationsList(deps DepsFor) mcp.ToolHandlerFor[LocationsListInput, LocationsListOutput] {
	return func(
		ctx context.Context, _ *mcp.CallToolRequest, in LocationsListInput,
	) (*mcp.CallToolResult, LocationsListOutput, error) {
		d, err := deps(ctx)
		if err != nil {
			return nil, LocationsListOutput{}, err
		}

		service := strings.TrimSpace(in.Service)
		if service == "" {
			return nil, LocationsListOutput{}, fmt.Errorf(
				"%s needs the name of a service to ask about, e.g. \"compute\"", ToolLocationsList)
		}

		offered, err := offeredAt(ctx, d.Reader, service)
		if err != nil {
			return nil, LocationsListOutput{}, err
		}

		all, err := d.Reader.ListLocations(ctx)
		if err != nil {
			return nil, LocationsListOutput{}, unreadable(err)
		}

		out := LocationsListOutput{Service: service, Locations: []LocationView{}}
		for i := range all {
			location := &all[i]
			if !offered[location.Name] {
				continue
			}
			out.Locations = append(out.Locations, LocationView{
				Name:     location.Name,
				Ready:    ready(location),
				Topology: topology(location.Spec.Topology),
				CityCode: location.Spec.Topology[CityCodeKey],
			})
		}
		sort.Slice(out.Locations, func(i, j int) bool {
			return out.Locations[i].Name < out.Locations[j].Name
		})

		if len(out.Locations) == 0 {
			out.Note = fmt.Sprintf(
				"%q is not offered anywhere this project can use today. That is the current answer, "+
					"not a failure to look it up.", service)
		}
		return nil, out, nil
	}
}

func locationsGet(deps DepsFor) mcp.ToolHandlerFor[LocationsGetInput, LocationsGetOutput] {
	return func(
		ctx context.Context, _ *mcp.CallToolRequest, in LocationsGetInput,
	) (*mcp.CallToolResult, LocationsGetOutput, error) {
		d, err := deps(ctx)
		if err != nil {
			return nil, LocationsGetOutput{}, err
		}

		name := strings.TrimSpace(in.Name)
		if name == "" {
			return nil, LocationsGetOutput{}, fmt.Errorf(
				"%s needs the name of a place, taken from %s", ToolLocationsGet, ToolLocationsList)
		}

		location, err := d.Reader.GetLocation(ctx, name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, LocationsGetOutput{}, fmt.Errorf(
					"there is no place called %q available to this project. Names are exact — take one "+
						"verbatim from %s rather than guessing at it", name, ToolLocationsList)
			}
			return nil, LocationsGetOutput{}, unreadable(err)
		}

		out := LocationsGetOutput{
			Name:     location.Name,
			Ready:    ready(location),
			Topology: topology(location.Spec.Topology),
			CityCode: location.Spec.Topology[CityCodeKey],
		}
		if c := location.Spec.Coordinates; c != nil {
			out.Coordinates = &CoordinatesView{Latitude: c.Latitude, Longitude: c.Longitude}
		}
		if c := apimeta.FindStatusCondition(location.Status.Conditions, locationsv1alpha1.LocationConditionReady); c != nil {
			out.ReadyReason, out.ReadyMessage = c.Reason, c.Message
		}
		return nil, out, nil
	}
}

// offeredAt returns the names of the places where service is offered to this
// project.
func offeredAt(ctx context.Context, reader Reader, service string) (map[string]bool, error) {
	records, err := reader.ListAvailability(ctx)
	if err != nil {
		return nil, unreadable(err)
	}

	offered := map[string]bool{}
	for i := range records {
		record := &records[i]
		if !strings.EqualFold(record.Spec.ServiceRef.Name, service) {
			continue
		}
		if !apimeta.IsStatusConditionTrue(record.Status.Conditions, AvailableCondition) {
			continue
		}
		if name := record.Spec.LocationRef.Name; name != "" {
			offered[name] = true
		}
	}
	return offered, nil
}

func ready(location *locationsv1alpha1.Location) bool {
	return apimeta.IsStatusConditionTrue(location.Status.Conditions, locationsv1alpha1.LocationConditionReady)
}

// topology returns a copy of the declared topology, or nil when there is
// nothing to report, so an empty map never renders as one.
func topology(declared map[string]string) map[string]string {
	if len(declared) == 0 {
		return nil
	}
	out := make(map[string]string, len(declared))
	for k, v := range declared {
		out[k] = v
	}
	return out
}

// unreadable turns a failed read into the sentence a person should get.
//
// The loud case is the important one. When a project does not carry one of the
// two records this joins, the question was not asked at all, and saying "no
// places" would tell somebody to wait for something that is already there.
// Naming the missing record, and saying plainly that the asker did nothing
// wrong, is what separates the two.
func unreadable(err error) error {
	if ns, ok := asNotServed(err); ok {
		return fmt.Errorf(
			"the places this project can use cannot be read: the project does not carry the %s "+
				"records that answer that, so the question was not asked rather than answered with "+
				"nothing. The person who asked did nothing wrong — whoever operates this deployment "+
				"needs to fix it", ns.Kind)
	}
	if apierrors.IsForbidden(err) {
		return fmt.Errorf(
			"the places this project can use are not readable with the access this request carries. " +
				"The person who asked may simply not have been granted it for this project; whoever " +
				"administers the project can grant it")
	}
	return fmt.Errorf("the places this project can use could not be read: %w", err)
}
