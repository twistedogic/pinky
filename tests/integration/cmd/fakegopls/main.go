// fakegopls is a minimal language server used by integration tests to
// drive pinky's LSP wire-up without needing a real gopls.
//
// It speaks stdio JSON-RPC 2.0 with Content-Length framing
// (LSP-standard, the format powernap expects). Behavior is governed
// by env vars set by the test driver:
//
//   FAKE_LSP_FILE  absolute path of the file to treat as the
//                  "current" file (used as the URI base for response
//                  Locations)
//   FAKE_LSP_DEFS  JSON array of {query:{line,col},
//                  target:{uri,range:{start,end}}} objects
//   FAKE_LSP_REFS  JSON array of {line,col} refs
//   FAKE_LSP_HOVER markdown hover content
//
// Build:  go build -o fakegopls ./tests/integration/cmd/fakegopls
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/sourcegraph/jsonrpc2"
)

type Position struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Def struct {
	Query  Position `json:"query"`
	Target struct {
		URI   string `json:"uri"`
		Range Range  `json:"range"`
	} `json:"target"`
}

type config struct {
	file  string
	defs  []Def
	refs  []Position
	hover string
}

func parseDefs(s string) []Def {
	if s == "" {
		return nil
	}
	var out []Def
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func parseRefs(s string) []Position {
	if s == "" {
		return nil
	}
	var out []Position
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func main() {
	cfg := &config{
		file:  os.Getenv("FAKE_LSP_FILE"),
		defs:  parseDefs(os.Getenv("FAKE_LSP_DEFS")),
		refs:  parseRefs(os.Getenv("FAKE_LSP_REFS")),
		hover: os.Getenv("FAKE_LSP_HOVER"),
	}

	handler := jsonrpc2.HandlerWithError(cfg.handle).SuppressErrClosed()

	stream := jsonrpc2.NewBufferedStream(
		&rw{rd: os.Stdin, wc: os.Stdout},
		jsonrpc2.VSCodeObjectCodec{},
	)
	conn := jsonrpc2.NewConn(context.Background(), stream, handler)

	<-conn.DisconnectNotify()
}

// rw wraps os.Stdin (Reader) + os.Stdout (Writer) into a ReadWriteCloser.
type rw struct {
	rd *os.File
	wc *os.File
}

func (r *rw) Read(p []byte) (int, error)         { return r.rd.Read(p) }
func (r *rw) Write(p []byte) (int, error)        { return r.wc.Write(p) }
func (r *rw) Close() error                       { return nil }

func (c *config) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
	switch req.Method {
	case "initialize":
		return &jsonInitializeResult{
			Capabilities: jsonCapabilities{
				DefinitionProvider: true,
				ReferencesProvider: true,
				HoverProvider:      true,
				TextDocumentSync:   1,
			},
			ServerInfo: jsonServerInfo{Name: "fakegopls", Version: "0.0.0"},
		}, nil
	case "textDocument/didOpen":
		return nil, nil
	case "textDocument/didChange":
		return nil, nil
	case "initialized":
		return nil, nil
	case "textDocument/definition":
		if req.Params == nil {
			return nil, errors.New("missing params")
		}
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position Position `json:"position"`
		}
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		for _, d := range c.defs {
			if d.Query.Line == params.Position.Line && d.Query.Col == params.Position.Col {
				return []jsonLocation{{
					URI: d.Target.URI,
					Range: jsonRange{
						Start: jsonPos{Line: d.Target.Range.Start.Line, Char: d.Target.Range.Start.Col},
						End:   jsonPos{Line: d.Target.Range.End.Line, Char: d.Target.Range.End.Col},
					},
				}}, nil
			}
		}
		return []jsonLocation{}, nil
	case "textDocument/references":
		if req.Params == nil {
			return nil, errors.New("missing params")
		}
		out := make([]jsonLocation, 0, len(c.refs))
		for _, r := range c.refs {
			out = append(out, jsonLocation{
				URI: "file://" + c.file,
				Range: jsonRange{
					Start: jsonPos{Line: r.Line, Char: r.Col},
					End:   jsonPos{Line: r.Line, Char: r.Col + 1},
				},
			})
		}
		return out, nil
	case "textDocument/hover":
		return jsonHover{
			Contents: jsonMarkup{
				Kind:  "markdown",
				Value: c.hover,
			},
		}, nil
	case "exit":
		go func() { os.Exit(0) }()
		return nil, nil
	default:
		return nil, errors.New("not implemented: " + req.Method)
	}
}

type jsonInitializeResult struct {
	Capabilities jsonCapabilities `json:"capabilities"`
	ServerInfo   jsonServerInfo   `json:"serverInfo"`
}

type jsonCapabilities struct {
	DefinitionProvider bool `json:"definitionProvider,omitempty"`
	ReferencesProvider bool `json:"referencesProvider,omitempty"`
	HoverProvider      bool `json:"hoverProvider,omitempty"`
	TextDocumentSync   int  `json:"textDocumentSync,omitempty"`
}

type jsonServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type jsonLocation struct {
	URI   string   `json:"uri"`
	Range jsonRange `json:"range"`
}

type jsonRange struct {
	Start jsonPos `json:"start"`
	End   jsonPos `json:"end"`
}

type jsonPos struct {
	Line int `json:"line"`
	Char int `json:"character"`
}

type jsonHover struct {
	Contents jsonMarkup `json:"contents"`
}

type jsonMarkup struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

var _ = fmt.Sprint