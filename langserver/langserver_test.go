package langserver

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workflowWithCommentBlock = "name: CI\n# one\n# two\n# three\non: push\n"

// repo makes a work tree and answers its root.
func repo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	return root
}

func codes(diags []lsp.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func TestDiagnosticsReportAWorkflowFindingOnItsLines(t *testing.T) {
	root := repo(t)
	s := New(10, t.TempDir())
	diags := s.Diagnostics(filepath.Join(root, ".github", "workflows", "ci.yml"), workflowWithCommentBlock)
	require.Contains(t, codes(diags), "yaml/comment-block")
	for _, d := range diags {
		if d.Code != "yaml/comment-block" {
			continue
		}
		assert.Equal(t, lsp.Error, d.Severity)
		assert.Equal(t, Source, d.Source)
		assert.Equal(t, 1, d.Range.Start.Line)
		assert.GreaterOrEqual(t, d.Range.End.Line, d.Range.Start.Line)
		assert.NotEmpty(t, d.Message)
	}
}

func TestDiagnosticsLeaveAFileOutsideEveryWorkTreeAlone(t *testing.T) {
	s := New(10, t.TempDir())
	path := filepath.Join(t.TempDir(), ".github", "workflows", "ci.yml")
	assert.Empty(t, s.Diagnostics(path, workflowWithCommentBlock))
}

func TestDiagnosticsLeaveTheClaudeConfigurationAlone(t *testing.T) {
	home := repo(t)
	s := New(10, home)
	inside := filepath.Join(home, ".claude", "plans", "plan.md")
	assert.Empty(t, s.Diagnostics(inside, "This shouldn't run; it is banned.\n"))

	// The control: the same text in the same work tree, outside .claude, is judged.
	outside := filepath.Join(home, "docs", "plan.md")
	assert.NotEmpty(t, s.Diagnostics(outside, "This shouldn't run; it is banned.\n"))
}

func TestDiagnosticsLeaveAFileNoRuleReadsAlone(t *testing.T) {
	root := repo(t)
	s := New(10, t.TempDir())
	assert.Empty(t, s.Diagnostics(filepath.Join(root, "data.bin"), "This shouldn't run; it is banned.\n"))
}

func TestDiagnosticsRankProseAheadOfTheWrapRule(t *testing.T) {
	root := repo(t)
	s := New(10, t.TempDir())
	doc := "A paragraph that is\nwrapped by hand.\n\nThis shouldn't run.\n"
	got := codes(s.Diagnostics(filepath.Join(root, "README.md"), doc))
	require.Contains(t, got, "wrap/hard-wrap")
	wrapAt := -1
	steAt := -1
	for i, code := range got {
		if code == "wrap/hard-wrap" {
			wrapAt = i
		}
		if strings.HasPrefix(code, "ste/") && steAt == -1 {
			steAt = i
		}
	}
	require.NotEqual(t, -1, steAt, "no ste finding in %v", got)
	assert.Less(t, steAt, wrapAt, "the wrap rule sits last: %v", got)
}

func TestDiagnosticsCountWhatTheCapHeldBack(t *testing.T) {
	root := repo(t)
	path := filepath.Join(root, "README.md")
	doc := "This shouldn't run.\n\nThat won't run.\n\nIt can't run.\n"

	all := New(0, t.TempDir()).Diagnostics(path, doc)
	require.Greater(t, len(all), 1)

	capped := New(1, t.TempDir()).Diagnostics(path, doc)
	require.Len(t, capped, 1)
	assert.Contains(t, capped[0].Message, "more slopfix findings in this file")
	assert.Contains(t, capped[0].Message, "+"+strconv.Itoa(len(all)-1)+" ")
}

func TestPathOfDecodesAFileURI(t *testing.T) {
	assert.Equal(t, filepath.FromSlash("/tmp/a b/c.md"), PathOf("file:///tmp/a%20b/c.md"))
	assert.Equal(t, "", PathOf("untitled:Untitled-1"))
}

// client drives a Server over a pipe the way an editor does, and collects what
// it publishes.
type client struct {
	conn      *jsonrpc2.Conn
	published chan lsp.PublishDiagnosticsParams
}

func dial(t *testing.T, s *Server) *client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	serverSide, clientSide := net.Pipe()
	go s.Serve(ctx, serverSide)

	c := &client{published: make(chan lsp.PublishDiagnosticsParams, 16)}
	c.conn = jsonrpc2.NewConn(ctx, jsonrpc2.NewBufferedStream(clientSide, jsonrpc2.VSCodeObjectCodec{}),
		jsonrpc2.HandlerWithError(func(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
			if req.Method != "textDocument/publishDiagnostics" {
				return nil, nil
			}
			var p lsp.PublishDiagnosticsParams
			if err := json.Unmarshal(*req.Params, &p); err != nil {
				return nil, err
			}
			c.published <- p
			return nil, nil
		}))
	t.Cleanup(func() { c.conn.Close() })
	return c
}

func (c *client) next(t *testing.T) lsp.PublishDiagnosticsParams {
	t.Helper()
	select {
	case p := <-c.published:
		return p
	case <-time.After(10 * time.Second):
		t.Fatal("the server published nothing")
		return lsp.PublishDiagnosticsParams{}
	}
}

func TestServerPublishesOverTheProtocol(t *testing.T) {
	root := repo(t)
	c := dial(t, New(10, t.TempDir()))
	ctx := context.Background()

	var init json.RawMessage
	require.NoError(t, c.conn.Call(ctx, "initialize", lsp.InitializeParams{}, &init))
	var caps struct {
		Capabilities struct {
			TextDocumentSync int `json:"textDocumentSync"`
		} `json:"capabilities"`
	}
	require.NoError(t, json.Unmarshal(init, &caps))
	assert.Equal(t, 1, caps.Capabilities.TextDocumentSync)
	require.NoError(t, c.conn.Notify(ctx, "initialized", struct{}{}))

	uri := lsp.DocumentURI("file://" + filepath.ToSlash(filepath.Join(root, ".github", "workflows", "ci.yml")))
	require.NoError(t, c.conn.Notify(ctx, "textDocument/didOpen", lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{URI: uri, LanguageID: "yaml", Text: workflowWithCommentBlock},
	}))
	opened := c.next(t)
	assert.Equal(t, uri, opened.URI)
	assert.Contains(t, codes(opened.Diagnostics), "yaml/comment-block")

	require.NoError(t, c.conn.Notify(ctx, "textDocument/didChange", lsp.DidChangeTextDocumentParams{
		TextDocument:   lsp.VersionedTextDocumentIdentifier{TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: uri}, Version: 2},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{{Text: "name: CI\non: push\n"}},
	}))
	changed := c.next(t)
	assert.NotContains(t, codes(changed.Diagnostics), "yaml/comment-block")

	require.NoError(t, c.conn.Notify(ctx, "textDocument/didClose", lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}))
	closed := c.next(t)
	assert.Equal(t, uri, closed.URI)
	assert.NotNil(t, closed.Diagnostics)
	assert.Empty(t, closed.Diagnostics)

	// A request always gets an answer, and one the server does not know is an error rather than silence.
	err := c.conn.Call(ctx, "textDocument/hover", struct{}{}, nil)
	var rpcErr *jsonrpc2.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, int64(jsonrpc2.CodeMethodNotFound), rpcErr.Code)

	// shutdown answers an explicit null result, which a client needs to see a well-formed reply.
	var shutdown json.RawMessage
	require.NoError(t, c.conn.Call(ctx, "shutdown", nil, &shutdown))
	assert.Equal(t, "null", string(shutdown))
}
