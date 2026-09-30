// Package lsp is pinky's thin layer on top of charmbracelet/x/powernap.
//
// It exposes a Manager that lazily spawns language servers on first
// use, a 30 s unavailable retry window for missing binaries, and
// the read-only enforcement the file viewer expects. The bridge
// surfaces the three LSP queries pinky wires into the file viewer
// (definition, references, hover) as Bubble Tea messages.
//
// Power-nap is the only LSP JSON-RPC client in the charmbracelet
// ecosystem; pinky talks to it directly (no agentutil-style wrapper).
package lsp

import (
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// URIToPath converts a file:// URI to an absolute filesystem path.
// Wrapper around protocol.DocumentURI.Path() exposed at the pinky
// layer so callers don't need to import the powernap protocol
// package directly.
func URIToPath(uri string) (string, error) {
	return protocol.DocumentURI(uri).Path()
}

// Kind discriminates the three LSP queries pinky wires into the
// file viewer. The bridge dispatches on Kind to choose the right
// powernap call and the right result type for the reply channel.
type Kind int

const (
	KindDefinition Kind = iota // textDocument/definition
	KindReferences             // textDocument/references
	KindHover                  // textDocument/hover
)

// Location is powernap's protocol.Location re-exported with pinky-
// facing (URI string + 1-based line/col) coordinates. URIs are
// kept as protocol.DocumentURI so the powernap helper for path
// conversion is available at the model layer when needed.
type Location = protocol.Location

// Hover is powernap's protocol.Hover re-exported under the pinky
// name. Used by the bridge to extract the first line for the
// one-line hover footer.
type Hover = protocol.Hover

// Request is one in-flight LSP query. ID disambiguates replies
// when the user presses a key while a previous query is still
// outstanding.
type Request struct {
	ID    int64
	Kind  Kind
	URI   string
	Line  int // 1-based line, matches powernap's expectations after offset conversion
	Char  int // 0-based byte offset into the line
}

// ServerState is the lifecycle state of one language server as
// observed by the model layer's status-line render. Sourced from
// the Manager's internal clientEntry.state plus the 30s
// unavailable window.
type ServerState int

const (
	ServerNone ServerState = iota // no server registered for this extension
	ServerStarting                // binary on PATH, spawn in progress
	ServerReady                   // Initialize succeeded, queries can run
	ServerMissing                 // binary not on PATH (inside unavailableWindow)
	ServerError                   // other spawn / init failure
)

// ServerStatus is a snapshot of one language server's state for
// a given file path. Returned by Manager.ServerStatus so the
// model can render a "LSP: gopls ●" chip in the file-viewer
// status line without keeping a parallel cache.
type ServerStatus struct {
	LangID  string // canonical name (e.g. "gopls", "rust-analyzer")
	Command string // binary on PATH (e.g. "gopls")
	State   ServerState
}

// Result is the reply delivered to the model layer. Exactly one
// of Locations / Hover carries a non-nil value, dispatching on
// req.Kind. Err is non-nil when the LSP call failed; the model
// layer surfaces the message as a footer.
type Result struct {
	ID        int64
	Kind      Kind
	Locations []Location
	Hover     *Hover
	Err       error
	// ErrServerMissing is set when the language server's binary
	// was not on PATH. The model layer renders the install hint
	// and starts the 30 s quiet window.
	ErrServerMissing bool
	// InstallHint is the human-readable install command (e.g.
	// "go install golang.org/x/tools/gopls@latest"). Empty when
	// not a missing-server error.
	InstallHint string
}