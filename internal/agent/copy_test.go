// SPDX-License-Identifier: AGPL-3.0-only

package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	locationsv1alpha1 "go.miloapis.com/locations/api/v1alpha1"
	agentdocs "go.miloapis.com/locations/docs/agent"
)

// Every string in this package is read by a customer, through an assistant
// that will repeat it nearly verbatim. These tests pin the words: they are the
// customer's, and the evidence they need in order to escalate survives.

// bannedTerm is one piece of vocabulary the customer has no way to know, with
// the reason it is banned, so a future reader can argue with the list rather
// than guess at it.
type bannedTerm struct {
	// pattern matches the term in prose, applied only after every API
	// identifier is removed: an identifier quoted as evidence is allowed, the
	// same word used as English is not.
	pattern *regexp.Regexp
	// why justifies the ban.
	why string
}

func banned(pattern, why string) bannedTerm {
	return bannedTerm{pattern: regexp.MustCompile(`(?i)` + pattern), why: why}
}

// internalVocabulary is the denylist.
//
// The test for a word: does the customer meet it in the API they write, or in
// output they already read? Then it is theirs and it survives. Does it exist
// only inside the implementation? Then a customer reading it learns nothing,
// and it is banned.
func internalVocabulary() []bannedTerm {
	return []bannedTerm{
		banned(`\bcontrollers?\b`,
			"the reader does not know what one is, and never has to. Say what happened to the "+
				"place, or say the platform."),
		banned(`\breconcil`,
			"loop vocabulary. It describes how the platform works, never what the person asked."),
		banned(`\bcontrol planes?\b`,
			"how the platform is built, not something a person choosing a city reasons about. "+
				"The thing they own and can see is their project."),
		banned(`\brbac\b`,
			"internal authorization vocabulary. Say who has not been granted access."),
		banned(`\badmission\b`,
			"the API extension point that rejected a request. The person needs to know they "+
				"were refused and by what, not which piece did it."),
		banned(`\bcrds?\b`,
			"the shape of the API, not the answer. Name the records that are missing instead."),
		banned(`\bpods?\b`,
			"a runtime unit nobody asking where a service runs has ever seen."),
		banned(`\bcells?\b`,
			"an internal unit of infrastructure, absent from the public docs, so a customer "+
				"reading it learns nothing. Say the place, or say the platform."),
		banned(`\bmilo\b`,
			"the name of an internal service. It means nothing outside the platform."),
		banned(`\bprojections?\b|\bprojected\b`,
			"how a place reaches a project, which is the platform's business. To the person "+
				"asking, the place is either offered to them or it is not."),
		banned(`\bkubernetes\b|\bk8s\b`,
			"what the platform is built on, never the answer to where a service runs."),
	}
}

// apiIdentifiers are the strings that may appear verbatim: kinds, condition
// types, reasons, topology keys and tool names. A customer quotes them when
// escalating, so they are removed before the prose is scanned.
func apiIdentifiers() []string {
	ids := []string{
		"ServiceAvailability",
		"LocationClass",
		"Location",
		"locations.miloapis.com/v1alpha1",
		"services.miloapis.com/v1alpha1",
		"locations.miloapis.com",
		"services.miloapis.com",
		"topology.datum.net/city-code",
		"topology.datum.net/region",
		"spec.topology",
		"spec.coordinates",
		"metadata.name",
		"cityCode",
		"readyReason",
		"readyMessage",
		"coordinates",
		"topology",
		ToolLocationsList,
		ToolLocationsGet,
		AvailableCondition,
		locationsv1alpha1.LocationConditionReady,
		locationsv1alpha1.LocationReasonLocationClassNotFound,
		locationsv1alpha1.LocationReasonMissingTopology,
	}
	// Longest first, so a longer identifier is removed before a shorter one
	// could match inside it and leave a fragment behind.
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if len(ids[j]) > len(ids[i]) {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

// prose strips the API identifiers, and any extras the caller supplies, out of
// a string. The extras matter: place names are chosen by whoever runs the
// platform, and a place called "cell-1" must not be read as prose.
func prose(s string, extra ...string) string {
	for _, id := range append(apiIdentifiers(), extra...) {
		if id != "" {
			s = strings.ReplaceAll(s, id, " ")
		}
	}
	return s
}

// checkCopy scans one piece of prose against the denylist.
func checkCopy(t *testing.T, where, text string, extra ...string) {
	t.Helper()
	p := prose(text, extra...)
	for _, term := range internalVocabulary() {
		if m := term.pattern.FindString(p); m != "" {
			t.Errorf("%s uses %q, which a customer has no way to read.\n  why: %s\n  in: %s",
				where, m, term.why, strings.TrimSpace(text))
		}
	}
}

// TestToolCopyUsesNoInternalVocabulary covers what a model reads before it
// decides whether to call a tool, and repeats when it explains why it did.
func TestToolCopyUsesNoInternalVocabulary(t *testing.T) {
	session := connect(t, depsWith(NewClientReader(fixture(t))))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("no tools published; there is nothing to check")
	}
	for _, tool := range res.Tools {
		checkCopy(t, tool.Name+".Description", tool.Description)
		checkCopy(t, tool.Name+".Title", tool.Title)
		for field, description := range schemaDescriptions(t, tool) {
			checkCopy(t, tool.Name+".input."+field, description)
		}
	}
}

// schemaDescriptions pulls every field description out of a published input
// schema. They are the sentences that tell a model what to put in an argument.
func schemaDescriptions(t *testing.T, tool *mcp.Tool) map[string]string {
	t.Helper()

	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshalling %s input schema: %v", tool.Name, err)
	}
	var parsed struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parsing %s input schema: %v", tool.Name, err)
	}
	out := make(map[string]string, len(parsed.Properties))
	for field, property := range parsed.Properties {
		out[field] = property.Description
	}
	return out
}

// TestFailureCopyUsesNoInternalVocabulary covers the sentences assembled at
// read time, which the description test cannot see. These are the ones a
// person meets on their worst day, so they matter most.
func TestFailureCopyUsesNoInternalVocabulary(t *testing.T) {
	for where, text := range failureCopy(t) {
		checkCopy(t, where, text)
	}
}

// TestFailureCopyNamesWhoCanAct is the counterweight to the denylist. Plain
// language is not vague language: a person told only that something failed can
// do nothing with it, so each failure has to say whether they are the one who
// can act.
func TestFailureCopyNamesWhoCanAct(t *testing.T) {
	// Second person is the failure mode itself: copy that says "you" hands the
	// model somebody to instruct, and on these paths the only person it can
	// instruct is the one who cannot fix it.
	secondPerson := regexp.MustCompile(`(?i)\byou(r|rs)?\b`)

	for where, text := range failureCopy(t) {
		if !strings.HasPrefix(where, "not-served") && !strings.HasPrefix(where, "forbidden") {
			continue
		}
		if m := secondPerson.FindString(text); m != "" {
			t.Errorf("%s addresses the reader as %q; the reader is not who can fix it: %s", where, m, text)
		}
		if !strings.Contains(text, "did nothing wrong") && !strings.Contains(text, "may simply not have been granted") {
			t.Errorf("%s does not clear the person who asked: %s", where, text)
		}
	}
}

// failureCopy collects one string per way an answer can go wrong, keyed by
// what went wrong.
func failureCopy(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}

	for _, kind := range []string{"ServiceAvailability", "Location"} {
		reader := notServedReader{Reader: NewClientReader(fixture(t)), kind: kind}
		if _, err := listWith(t, reader, LocationsListInput{Service: testService}); err != nil {
			out["not-served."+kind] = err.Error()
		} else {
			t.Fatalf("%s absent produced no error", kind)
		}
	}

	if _, err := listWith(t, forbiddenReader{Reader: NewClientReader(fixture(t))},
		LocationsListInput{Service: testService}); err != nil {
		out["forbidden"] = err.Error()
	} else {
		t.Fatal("no access produced no error")
	}

	if _, err := listWith(t, NewClientReader(fixture(t)), LocationsListInput{}); err != nil {
		out["no-service"] = err.Error()
	} else {
		t.Fatal("an empty service produced no error")
	}

	if _, err := getWith(t, NewClientReader(fixture(t)), LocationsGetInput{Name: "nowhere"}); err != nil {
		out["no-such-place"] = err.Error()
	} else {
		t.Fatal("an unknown name produced no error")
	}

	empty, err := listWith(t, NewClientReader(fixture(t)), LocationsListInput{Service: "nothing-runs-here"})
	if err != nil {
		t.Fatalf("locations_list: %v", err)
	}
	out["empty-note"] = empty.Note

	// The generic wrapper, for a failure that is none of the above.
	out["unreadable"] = unreadable(fmt.Errorf("the network went away")).Error()
	return out
}

// TestKnowledgeDocumentUsesNoInternalVocabulary covers the document served at
// /llms-full.txt. It addresses an assistant rather than a person, but an
// assistant that reads "cell" produces an answer that says "cell".
func TestKnowledgeDocumentUsesNoInternalVocabulary(t *testing.T) {
	body, err := agentdocs.FS.ReadFile(agentdocs.KnowledgeFile)
	if err != nil {
		t.Fatalf("reading %s: %v", agentdocs.KnowledgeFile, err)
	}
	if len(body) == 0 {
		t.Fatalf("%s is empty; the capability document promises it", agentdocs.KnowledgeFile)
	}

	line := 0
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line++
		checkCopy(t, fmt.Sprintf("%s:%d", agentdocs.KnowledgeFile, line), scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning %s: %v", agentdocs.KnowledgeFile, err)
	}
}

// TestKnowledgeDocumentKeepsTheEvidence: plain language must not cost the
// assistant the identifiers it needs to actually use the tools.
func TestKnowledgeDocumentKeepsTheEvidence(t *testing.T) {
	body, err := agentdocs.FS.ReadFile(agentdocs.KnowledgeFile)
	if err != nil {
		t.Fatalf("reading %s: %v", agentdocs.KnowledgeFile, err)
	}
	for _, want := range []string{
		ToolLocationsList,
		ToolLocationsGet,
		CityCodeKey,
		"locations.miloapis.com/v1alpha1",
		// The distinction the whole document exists to teach.
		"ready",
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("%s does not mention %q, which an assistant needs in order to act",
				agentdocs.KnowledgeFile, want)
		}
	}
}
