// SPDX-License-Identifier: AGPL-3.0-only

// Package agentdocs embeds the knowledge this service publishes to an AI
// assistant.
//
// Embedded rather than read from disk, so the server is one self-contained
// binary and a stripped image cannot quietly start answering 404 for a
// document the capability document promises.
package agentdocs

import "embed"

// FS holds the knowledge document, at the path named by KnowledgeFile.
//
//go:embed llms-full.txt
var FS embed.FS

// KnowledgeFile is the knowledge document: what a place is, what it declares
// about itself, and how to choose between places.
const KnowledgeFile = "llms-full.txt"
