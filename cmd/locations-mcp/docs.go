// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"time"

	agentdocs "go.miloapis.com/locations/docs/agent"
)

const (
	// knowledgePath serves the knowledge document.
	knowledgePath = "/llms-full.txt"

	textContentType = "text/plain; charset=utf-8"
)

// document is one static file, read out of the embedded FS at startup.
type document struct {
	body        []byte
	contentType string
}

// knowledgeHandler serves this service's knowledge over plain HTTP.
//
// Routing is an exact-match lookup in a table built at startup, never a path
// join against a directory, so "..", an absolute path, or an escaped separator
// has nothing to traverse to.
//
// The document is public and carries no auth check: an assistant fetches it
// before it holds any project context, and it is static text with no tenant
// data in it. /mcp is the credential-bearing surface.
type knowledgeHandler struct {
	docs map[string]document
}

// newKnowledgeHandler reads every embedded document into memory. Failing here
// is a build problem, not a runtime one, so the server refuses to start rather
// than serve a promised URL as a 404.
func newKnowledgeHandler() (*knowledgeHandler, error) {
	knowledge, err := agentdocs.FS.ReadFile(agentdocs.KnowledgeFile)
	if err != nil {
		return nil, fmt.Errorf("reading embedded knowledge: %w", err)
	}
	if len(knowledge) == 0 {
		return nil, fmt.Errorf("embedded knowledge %s is empty", agentdocs.KnowledgeFile)
	}
	return &knowledgeHandler{
		docs: map[string]document{
			knowledgePath: {body: knowledge, contentType: textContentType},
		},
	}, nil
}

func (h *knowledgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	doc, ok := h.docs[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Every byte is fixed at build time and already in memory: one write of a
	// known-size buffer, no per-request I/O.
	w.Header().Set("Content-Type", doc.contentType)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(doc.body))
}

// paths returns the served URL paths, sorted, for the startup log.
func (h *knowledgeHandler) paths() []string {
	out := make([]string, 0, len(h.docs))
	for p := range h.docs {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
