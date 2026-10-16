// Package langserver answers an editor over the Language Server Protocol. Each
// open file gets the findings the CI gate reports for it, while it is still
// being written.
package langserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Source names this server on every diagnostic it publishes.
const Source = "slopfix"

// familyOrder ranks rule families, most structural first.
var familyOrder = []string{"yaml", "ste", "english"}

// lastFamily is the most numerous family and the most mechanical to repair.
const lastFamily = "wrap"

func rank(id string) int {
	// A warning never crowds out a finding that fails the gate.
	if ste.WarningIDs.Contains(id) {
		return len(familyOrder) + 2
	}
	family, _, _ := strings.Cut(id, "/")
	if family == lastFamily {
		return len(familyOrder) + 1
	}
	if i := slices.Index(familyOrder, family); i >= 0 {
		return i
	}
	return len(familyOrder)
}

// Server holds the open documents. The connection hands it one message at a
// time, so nothing here is shared between goroutines.
type Server struct {
	// maxPerFile caps what a file publishes. The last diagnostic sent says how many were held back.
	maxPerFile int
	// claudeDir is the user's own configuration, which no build reads.
	claudeDir string
	open      map[lsp.DocumentURI]string
}

// New answers for every open file under a work tree, except under home's .claude directory.
func New(maxPerFile int, home string) *Server {
	return &Server{
		maxPerFile: maxPerFile,
		claudeDir:  filepath.Join(home, ".claude"),
		open:       map[lsp.DocumentURI]string{},
	}
}

// Serve answers one client over rwc until it disconnects.
func (s *Server) Serve(ctx context.Context, rwc io.ReadWriteCloser) {
	stream := jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(s.handle))
	<-conn.DisconnectNotify()
}

func (s *Server) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
	switch req.Method {
	case "initialize":
		// Full sync: a finding is a property of the whole document.
		kind := lsp.TDSKFull
		return initializeResult{Capabilities: capabilities{
			ServerCapabilities: lsp.ServerCapabilities{TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{Kind: &kind}},
			DiagnosticProvider: diagnosticOptions{},
		}}, nil
	case "textDocument/diagnostic":
		var p documentDiagnosticParams
		if err := params(req, &p); err != nil {
			return nil, err
		}
		items := []lsp.Diagnostic{}
		if content, ok := s.open[p.TextDocument.URI]; ok {
			items = s.Diagnostics(PathOf(p.TextDocument.URI), content)
		}
		return fullDocumentDiagnosticReport{Kind: "full", Items: items}, nil
	case "initialized", "shutdown":
		return nil, nil
	case "exit":
		return nil, conn.Close()
	case "textDocument/didOpen":
		var p lsp.DidOpenTextDocumentParams
		if err := params(req, &p); err != nil {
			return nil, err
		}
		s.open[p.TextDocument.URI] = p.TextDocument.Text
		return nil, s.publish(ctx, conn, p.TextDocument.URI)
	case "textDocument/didChange":
		var p lsp.DidChangeTextDocumentParams
		if err := params(req, &p); err != nil {
			return nil, err
		}
		if len(p.ContentChanges) == 0 {
			return nil, nil
		}
		s.open[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
		return nil, s.publish(ctx, conn, p.TextDocument.URI)
	case "textDocument/didSave":
		var p lsp.DidSaveTextDocumentParams
		if err := params(req, &p); err != nil {
			return nil, err
		}
		return nil, s.publish(ctx, conn, p.TextDocument.URI)
	case "textDocument/didClose":
		var p lsp.DidCloseTextDocumentParams
		if err := params(req, &p); err != nil {
			return nil, err
		}
		delete(s.open, p.TextDocument.URI)
		// An empty list is what clears a closed file's findings from the client.
		return nil, conn.Notify(ctx, "textDocument/publishDiagnostics",
			lsp.PublishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []lsp.Diagnostic{}})
	}
	if req.Notif {
		return nil, nil
	}
	return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "unknown method: " + req.Method}
}

type (
	diagnosticOptions struct {
		InterFileDependencies bool `json:"interFileDependencies"`
		WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
	}
	capabilities struct {
		lsp.ServerCapabilities
		DiagnosticProvider diagnosticOptions `json:"diagnosticProvider"`
	}
	initializeResult struct {
		Capabilities capabilities `json:"capabilities"`
	}
	documentDiagnosticParams struct {
		TextDocument lsp.TextDocumentIdentifier `json:"textDocument"`
	}
	fullDocumentDiagnosticReport struct {
		Kind  string           `json:"kind"`
		Items []lsp.Diagnostic `json:"items"`
	}
)

func params(req *jsonrpc2.Request, v any) error {
	if req.Params == nil {
		return fmt.Errorf("%s carries no params", req.Method)
	}
	return json.Unmarshal(*req.Params, v)
}

func (s *Server) publish(ctx context.Context, conn *jsonrpc2.Conn, uri lsp.DocumentURI) error {
	content, ok := s.open[uri]
	if !ok {
		return nil
	}
	return conn.Notify(ctx, "textDocument/publishDiagnostics",
		lsp.PublishDiagnosticsParams{URI: uri, Diagnostics: s.Diagnostics(PathOf(uri), content)})
}

// PathOf answers the file a URI names, or "" for a URI that names no file.
func PathOf(uri lsp.DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil || u.Scheme != "file" {
		return ""
	}
	path := u.Path
	// A Windows URI spells the drive as /C:/..., and the leading slash is not part of the path.
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

// Judged reports whether the gate reads this file. A build reads a file in a
// work tree, and never the user's own Claude configuration.
func (s *Server) Judged(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	path = filepath.Clean(path)
	if path == s.claudeDir || strings.HasPrefix(path, s.claudeDir+string(filepath.Separator)) {
		return false
	}
	return workTree(path) != "" && slopfix.Reads(path)
}

// workTree answers the root of the work tree holding path, or "" outside one.
// The .git entry is a directory in a clone and a file in a worktree or a
// submodule, so only its existence is asked.
func workTree(path string) string {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Diagnostics answers what the gate reports for content headed for path,
// ranked so a structural finding is never crowded out by a voluminous one.
func (s *Server) Diagnostics(path, content string) []lsp.Diagnostic {
	out := []lsp.Diagnostic{}
	if !s.Judged(path) {
		return out
	}
	findings := slopfix.CheckContent(path, content)
	slices.SortStableFunc(findings, func(a, b ste.Finding) int {
		if r := rank(a.ID) - rank(b.ID); r != 0 {
			return r
		}
		return a.Line - b.Line
	})
	lines := strings.Split(content, "\n")
	for _, f := range findings {
		out = append(out, diagnostic(f, lines))
	}
	if s.maxPerFile <= 0 || len(out) <= s.maxPerFile {
		return out
	}
	hidden := len(out) - s.maxPerFile
	out = out[:s.maxPerFile]
	out[len(out)-1].Message += fmt.Sprintf(" (+%d more %s findings in this file)", hidden, Source)
	return out
}

// diagnostic places a finding on whole lines. A finding that fails the gate is
// an error, and a warning stays a warning.
func diagnostic(f ste.Finding, lines []string) lsp.Diagnostic {
	level := lsp.Error
	if f.Warning() {
		level = lsp.Warning
	}
	start := max(0, f.Line-1)
	end := max(start, f.EndLine-1)
	width := 0
	if end < len(lines) {
		// A position's character counts UTF-16 code units.
		width = len(utf16.Encode([]rune(strings.TrimSuffix(lines[end], "\r"))))
	}
	message := f.Rule
	if f.Detail != "" {
		message += fmt.Sprintf(" %q", f.Detail)
	}
	message += "."
	if f.Fix != "" {
		message += " " + f.Fix
	}
	return lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{Line: start},
			End:   lsp.Position{Line: end, Character: width},
		},
		Severity: level,
		Code:     f.ID,
		Source:   Source,
		Message:  message,
	}
}
