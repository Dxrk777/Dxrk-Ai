package lsptool

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ── JSON-RPC 2.0 wire types ──────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// ── LSPClient ────────────────────────────────────────────────────────────────

// LSPClient communicates with a single language server over stdin/stdout.
type LSPClient struct {
	serverCmd  string
	serverArgs []string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	mu         sync.Mutex
	nextID     atomic.Int32

	// Capabilities negotiated during initialize.
	capabilities map[string]any

	// Open documents tracked by URI.
	documents map[string]*docState

	// Diagnostics cache keyed by document URI.
	diagnosticsMu sync.RWMutex
	diagnostics   map[string][]Diagnostic
}

type docState struct {
	version int
	text    string
}

// NewLSPClient prepares a client but does not start the server yet.
func NewLSPClient(serverCmd string, args []string) *LSPClient {
	return &LSPClient{
		serverCmd:   serverCmd,
		serverArgs:  args,
		documents:   make(map[string]*docState),
		diagnostics: make(map[string][]Diagnostic),
	}
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

// Initialize starts the server process, performs the handshake, and stores
// the server's capabilities.
func (c *LSPClient) Initialize(rootURI string) error {
	c.cmd = exec.Command(c.serverCmd, c.serverArgs...)
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)

	// Stderr goes to the void for now; a production version might log it.
	c.cmd.Stderr = nil

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", c.serverCmd, err)
	}

	params := map[string]any{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			strconst.StrTextdocument: map[string]any{
				"hover":      map[string]any{"contentFormat": []string{strconst.StrMarkdown, "plaintext"}},
				"completion": map[string]any{"completionItem": map[string]any{"snippetSupport": false}},
				"references": map[string]any{},
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
				"publishDiagnostics": map[string]any{},
				"formatting":         map[string]any{},
				"rename":             map[string]any{},

				"syncKind": 1,
			},
		},
		"clientInfo": map[string]any{
			"name":              "Dxrk-Ai",
			strconst.StrVersion: "1.0.0",
		},
	}

	var result initializeResult
	if err := c.sendRequest("initialize", params, &result); err != nil {
		_ = c.killProcess()
		return fmt.Errorf("initialize: %w", err)
	}
	c.capabilities = result.Capabilities

	// Send the initialized notification.
	if err := c.sendNotification("initialized", nil); err != nil {
		_ = c.killProcess()
		return fmt.Errorf("initialized notification: %w", err)
	}

	// Start the reader goroutine for diagnostics.
	go c.readLoop()

	return nil
}

// Shutdown sends the shutdown request followed by the exit notification.
func (c *LSPClient) Shutdown() error {
	_ = c.sendRequest("shutdown", nil, nil)
	_ = c.sendNotification("exit", nil)
	return c.killProcess()
}

func (c *LSPClient) killProcess() error {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

type initializeResult struct {
	Capabilities map[string]any `json:"capabilities"`
}

// ── Document sync ────────────────────────────────────────────────────────────

// DidOpen sends textDocument/didOpen.
func (c *LSPClient) DidOpen(uri, languageID string, version int, text string) error {
	c.mu.Lock()
	c.documents[uri] = &docState{version: version, text: text}
	c.mu.Unlock()
	return c.sendNotification("textDocument/didOpen", map[string]any{
		strconst.StrTextdocument: map[string]any{
			"uri": uri, "languageId": languageID, strconst.StrVersion: version, "text": text,
		},
	})
}

// DidChange sends textDocument/didChange.
func (c *LSPClient) DidChange(uri string, changes []TextDocumentContentChangeEvent) error {
	c.mu.Lock()
	doc, ok := c.documents[uri]
	if !ok {
		doc = &docState{}
		c.documents[uri] = doc
	}
	doc.version++
	v := doc.version
	c.mu.Unlock()
	return c.sendNotification("textDocument/didChange", map[string]any{
		strconst.StrTextdocument: map[string]any{"uri": uri, strconst.StrVersion: v},
		"contentChanges":         changes,
	})
}

// DidSave sends textDocument/didSave.
func (c *LSPClient) DidSave(uri string) error {
	return c.sendNotification("textDocument/didSave", map[string]any{
		strconst.StrTextdocument: map[string]any{"uri": uri},
	})
}

// ── Language features ────────────────────────────────────────────────────────

// Hover returns hover information at the given position.
func (c *LSPClient) Hover(params TextDocumentPositionParams) (*HoverResult, error) {
	var result HoverResult
	if err := c.sendRequest("textDocument/hover", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GotoDefinition returns the location of the definition at the given position.
func (c *LSPClient) GotoDefinition(params TextDocumentPositionParams) (*Location, error) {
	var result Location
	if err := c.sendRequest("textDocument/definition", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FindReferences returns all references to the symbol at the given position.
func (c *LSPClient) FindReferences(params TextDocumentPositionParams) ([]Location, error) {
	var result []Location
	if err := c.sendRequest("textDocument/references", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Completion returns completion items at the given position.
func (c *LSPClient) Completion(params TextDocumentPositionParams) ([]CompletionItem, error) {
	raw, err := c.sendRequestRaw("textDocument/completion", params)
	if err != nil {
		return nil, err
	}
	var list CompletionList
	if err := json.Unmarshal(raw, &list); err == nil && list.Items != nil {
		return list.Items, nil
	}
	var items []CompletionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode completion items: %w", err)
	}
	return items, nil
}

// DocumentSymbols returns the symbol outline for the given document.
func (c *LSPClient) DocumentSymbols(uri string) ([]DocumentSymbol, error) {
	raw, err := c.sendRequestRaw("textDocument/documentSymbol", map[string]any{
		strconst.StrTextdocument: map[string]any{"uri": uri},
	})
	if err != nil {
		return nil, err
	}
	var symbols []DocumentSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("decode document symbols: %w", err)
	}
	return symbols, nil
}

// WorkspaceSymbols returns symbols matching the query string.
func (c *LSPClient) WorkspaceSymbols(query string) ([]SymbolInformation, error) {
	var result []SymbolInformation
	if err := c.sendRequest("workspace/symbol", map[string]any{strconst.StrQuery: query}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Diagnostics returns cached diagnostics for the given document URI.
func (c *LSPClient) Diagnostics(uri string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()
	return c.diagnostics[uri]
}

// FormatDocument returns text edits that reformat the document.
func (c *LSPClient) FormatDocument(uri string, opts FormattingOptions) ([]TextEdit, error) {
	var result []TextEdit
	if err := c.sendRequest("textDocument/formatting", map[string]any{
		strconst.StrTextdocument: map[string]any{"uri": uri},
		"options":                opts,
	}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Rename returns a workspace edit that renames the symbol at the position.
func (c *LSPClient) Rename(params RenameParams) (*WorkspaceEdit, error) {
	var result WorkspaceEdit
	if err := c.sendRequest("textDocument/rename", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CodeAction returns available code actions for the given range.
func (c *LSPClient) CodeAction(uri string, rng Range, diagnostics []Diagnostic) ([]CodeAction, error) {
	var result []CodeAction
	if err := c.sendRequest("textDocument/codeAction", CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Context:      CodeActionContext{Diagnostics: diagnostics},
	}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ── JSON-RPC transport ───────────────────────────────────────────────────────

func (c *LSPClient) sendRequest(method string, params any, result any) error {
	raw, err := c.sendRequestRaw(method, params)
	if err != nil {
		return err
	}
	if result != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}

func (c *LSPClient) sendRequestRaw(method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	c.mu.Lock()
	if err := c.writeMessage(req); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("write request %s: %w", method, err)
	}

	// Read the response synchronously (sufficient for single-threaded use).
	resp, err := c.readResponse()
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("read response %s: %w", method, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("LSP error %s: %s (code %d)", method, resp.Error.Message, resp.Error.Code)
	}
	return resp.Result, nil
}

func (c *LSPClient) sendNotification(method string, params any) error {
	notif := rpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeMessage(notif)
}

// writeMessage writes a Content-Length framed JSON-RPC message.
func (c *LSPClient) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err = c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

// readResponse reads a single Content-Length framed response from the server.
func (c *LSPClient) readResponse() (*rpcResponse, error) {
	for {
		length, err := c.readContentLength()
		if err != nil {
			return nil, err
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		var probe struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &probe); err == nil && probe.ID == nil && probe.Method != "" {
			c.handleNotification(body)
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		return &resp, nil
	}
}

// readContentLength parses the Content-Length header line.
func (c *LSPClient) readContentLength() (int, error) {
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length: ")))
			if err != nil {
				return 0, fmt.Errorf("parse Content-Length: %w", err)
			}
			return n, nil
		}
	}
}

// handleNotification processes server-to-client notifications.
func (c *LSPClient) handleNotification(data []byte) {
	var notif struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(data, &notif); err != nil {
		return
	}
	if notif.Method == "textDocument/publishDiagnostics" {
		var diag PublishDiagnosticsParams
		if err := json.Unmarshal(notif.Params, &diag); err != nil {
			return
		}
		c.diagnosticsMu.Lock()
		c.diagnostics[diag.URI] = diag.Diagnostics
		c.diagnosticsMu.Unlock()
	}
}

// readLoop continuously reads notifications from the server.
func (c *LSPClient) readLoop() {
	for {
		length, err := c.readContentLength()
		if err != nil {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			return
		}
		c.handleNotification(body)
	}
}

// PublishDiagnosticsParams is the shape of textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// filePathToURI converts a filesystem path to a file:// URI.
func filePathToURI(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "file://" + p
	}
	return "file://" + abs
}

// uriToFilePath strips the file:// scheme and returns the local path.
func uriToFilePath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}
