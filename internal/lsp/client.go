package lsp

import (
	"bufio"
	"context"
	"encoding/json" // Keep for potential future use within client.go
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	// Import the protocol package
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// Client manages the connection and state for an LSP server.
type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	// Request ID counter
	nextID atomic.Int32

	// Response handlers (Managed in transport.go)
	handlers   map[int32]chan *Message // Use Message from this package (lsp)
	handlersMu sync.RWMutex

	// Server request handlers (Managed via Register method)
	serverRequestHandlers map[string]ServerRequestHandler // Use ServerRequestHandler from transport.go
	serverHandlersMu      sync.RWMutex

	// Notification handlers (Managed via Register method)
	notificationHandlers map[string]NotificationHandler // Use NotificationHandler from transport.go
	notificationMu       sync.RWMutex

	// Diagnostic cache
	diagnostics   map[protocol.DocumentUri][]protocol.Diagnostic // Use protocol types
	diagnosticsMu sync.RWMutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex

	// Debug flag
	debug bool
}

// OpenFileInfo stores information about an open file.
type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentUri // Use protocol.DocumentUri
}

// --- Helper Functions ---

// Ptr returns a pointer to the given value. Useful for optional boolean fields.
func Ptr[T any](v T) *T {
	return &v
}

// NewClient creates and starts a new LSP client.
func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &Client{
		Cmd:                   cmd,
		stdin:                 stdin,
		stdout:                bufio.NewReader(stdout),
		stderr:                stderr,
		handlers:              make(map[int32]chan *Message), // Use Message from this package
		notificationHandlers:  make(map[string]NotificationHandler), // Use NotificationHandler from transport.go
		serverRequestHandlers: make(map[string]ServerRequestHandler), // Use ServerRequestHandler from transport.go
		diagnostics:           make(map[protocol.DocumentUri][]protocol.Diagnostic), // Use protocol types
		openFiles:             make(map[string]*OpenFileInfo),
		debug:                 os.Getenv("MCP_LSP_DEBUG") == "true",
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "LSP Server: %s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stderr: %v\n", err)
		}
	}()

	// handleMessages is defined in transport.go, start it here
	go client.handleMessages()

	return client, nil
}

// RegisterNotificationHandler registers a handler for a specific notification method.
// Assumes NotificationHandler type is defined in transport.go
func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	c.notificationHandlers[method] = handler
}

// RegisterServerRequestHandler registers a handler for a specific server request method.
// Assumes ServerRequestHandler type is defined in transport.go
func (c *Client) RegisterServerRequestHandler(method string, handler ServerRequestHandler) {
	c.serverHandlersMu.Lock()
	defer c.serverHandlersMu.Unlock()
	c.serverRequestHandlers[method] = handler
}

// InitializeLSPClient sends the initialize request and initialized notification.
// This method orchestrates the initialization sequence using methods defined elsewhere.
func (c *Client) InitializeLSPClient(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	rootURI := "file://" + workspaceDir
	// Corrected: Trace field is *protocol.TraceValue
	traceValue := protocol.TraceValue("off")
	tracePtr := &traceValue

	// Prepare SymbolKind ValueSet
	symbolKinds := []protocol.SymbolKind{
		protocol.File, protocol.Module, protocol.Namespace, protocol.Package, protocol.Class, protocol.Method, protocol.Property, protocol.Field, protocol.Constructor,
		protocol.Enum, protocol.Interface, protocol.Function, protocol.Variable, protocol.Constant, protocol.String, protocol.Number, protocol.Boolean, protocol.Array,
		protocol.Object, protocol.Key, protocol.Null, protocol.EnumMember, protocol.Struct, protocol.Event, protocol.Operator, protocol.TypeParameter,
	}

	initParams := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: []protocol.WorkspaceFolder{
				{
					URI:  protocol.URI(rootURI),
					Name: workspaceDir,
				},
			},
		},
		XInitializeParams: protocol.XInitializeParams{
			ProcessID: int32(os.Getpid()),
			ClientInfo: &protocol.ClientInfo{
				Name:    "mcp-language-server",
				Version: "0.1.0", // Consider making version dynamic
			},
			RootURI: protocol.DocumentUri(rootURI),
			Capabilities: protocol.ClientCapabilities{
				// Corrected: Workspace field is protocol.WorkspaceClientCapabilities (not pointer)
				Workspace: protocol.WorkspaceClientCapabilities{
					ApplyEdit: true, // bool
					WorkspaceEdit: &protocol.WorkspaceEditClientCapabilities{
						DocumentChanges: true, // bool
					},
					// Configuration: true, // Deprecated
					// Corrected: DidChangeConfiguration is protocol.DidChangeConfigurationClientCapabilities (not pointer)
					DidChangeConfiguration: protocol.DidChangeConfigurationClientCapabilities{
						DynamicRegistration: false, // bool
					},
					Symbol: &protocol.WorkspaceSymbolClientCapabilities{
						DynamicRegistration: false, // bool
						// Corrected: SymbolKind field type is *protocol.ClientSymbolKindOptions
						SymbolKind: &protocol.ClientSymbolKindOptions{
							ValueSet: symbolKinds,
						},
					},
				},
				// Corrected: TextDocument field is protocol.TextDocumentClientCapabilities (not pointer)
				TextDocument: protocol.TextDocumentClientCapabilities{
					Synchronization: &protocol.TextDocumentSyncClientCapabilities{
						DynamicRegistration: false, // bool
						WillSave:            false, // bool
						WillSaveWaitUntil:   false, // bool
						DidSave:             true,  // bool
					},
					Rename: &protocol.RenameClientCapabilities{
						DynamicRegistration: false, // bool
						PrepareSupport:      false, // bool
					},
					// Corrected: DocumentSymbol is protocol.DocumentSymbolClientCapabilities (not pointer)
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{
						DynamicRegistration:               false, // bool
						HierarchicalDocumentSymbolSupport: true, // bool
						// Corrected: SymbolKind field type is *protocol.ClientSymbolKindOptions
						SymbolKind: &protocol.ClientSymbolKindOptions{
							ValueSet: symbolKinds,
						},
					},
					CodeLens: &protocol.CodeLensClientCapabilities{
						// DynamicRegistration: Ptr(true), // Check protocol.go
					},
					// Corrected: PublishDiagnostics is protocol.PublishDiagnosticsClientCapabilities (not pointer)
					PublishDiagnostics: protocol.PublishDiagnosticsClientCapabilities{
						// VersionSupport is bool
						VersionSupport: false,
						// Initialize embedded DiagnosticsCapabilities fields
						DiagnosticsCapabilities: protocol.DiagnosticsCapabilities{
							RelatedInformation: false, // bool
							// TagSupport: Ptr(...), // *ClientDiagnosticsTagOptions
							// CodeDescriptionSupport: false, // bool
							// DataSupport: false, // bool
						},
					},
				},
			},
			InitializationOptions: map[string]any{ // Use any instead of interface{}
				"codelenses": map[string]bool{
					"generate":           true,
					"regenerate_cgo":     true,
					"test":               true,
					"tidy":               true,
					"upgrade_dependency": true,
					"vendor":             true,
					"vulncheck":          false,
				},
			},
			// Corrected: Trace field is *protocol.TraceValue
			Trace: tracePtr,
		},
	}


	var result protocol.InitializeResult
	// Call is defined in transport.go
	if err := c.Call(ctx, "initialize", initParams, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	// Initialized is defined in methods.go
	if err := c.Initialized(ctx, protocol.InitializedParams{}); err != nil {
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}

	// Register handlers AFTER initialized notification
	// Handlers are defined in server-request-handlers.go
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics",
		func(params json.RawMessage) { HandleDiagnostics(c, params) }) // Pass client 'c'

	return &result, nil
}


// Close sends shutdown and exit messages to the LSP server and waits for it to terminate.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.CloseAllFiles(ctx) // Close tracked files first

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	// Call is defined in transport.go
	if err := c.Call(shutdownCtx, "shutdown", nil, nil); err != nil {
		if c.debug {
			log.Printf("Shutdown request failed (continuing): %v", err)
		}
	}

	exitCtx, exitCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer exitCancel()
	// Notify is defined in transport.go
	if err := c.Notify(exitCtx, "exit", nil); err != nil {
		if c.debug {
			log.Printf("Exit notification failed (continuing): %v", err)
		}
	}

	// Close stdin pipe after sending exit
	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			if c.debug {
				log.Printf("Failed to close stdin: %v", err)
			}
		}
		c.stdin = nil
	}

	// Wait for the process to exit
	done := make(chan error, 1)
	go func() {
		if c.Cmd != nil && c.Cmd.Process != nil {
			done <- c.Cmd.Wait()
		} else {
			done <- fmt.Errorf("process already exited or not started")
		}
	}()

	select {
	case err := <-done:
		if exitErr, ok := err.(*exec.ExitError); ok {
			if c.debug {
				log.Printf("LSP process exited with status: %s", exitErr.Error())
			}
			return nil // Expected exit after shutdown/exit
		}
		return err // Other wait error
	case <-time.After(2 * time.Second):
		// Timeout waiting for exit, kill the process
		if c.Cmd != nil && c.Cmd.Process != nil {
			if err := c.Cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process after timeout: %w", err)
			}
			return fmt.Errorf("process killed after timeout")
		}
		return fmt.Errorf("process wait timed out, but process was nil")
	}
}

// ServerState represents the readiness state of the LSP server.
type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
)

// WaitForServerReady waits until the LSP server is likely ready by sending a simple request.
func (c *Client) WaitForServerReady(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // Adjust timeout as needed
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timed out waiting for server readiness: %w", timeoutCtx.Err())
		case <-ticker.C:
			// Use RequestWorkspaceSymbols defined below
			_, err := c.RequestWorkspaceSymbols(timeoutCtx, protocol.WorkspaceSymbolParams{Query: ""})
			if err == nil {
				if c.debug {
					log.Println("LSP server reported ready.")
				}
				return nil // Success!
			}
			if c.debug {
				log.Printf("Readiness check failed (will retry): %v", err)
			}
			// Continue loop on error
		}
	}
}

// OpenFile notifies the LSP server that a file has been opened.
func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	uri := "file://" + filepath

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; exists {
		c.openFilesMu.Unlock()
		if c.debug {
			log.Printf("File already open: %s", filepath)
		}
		return nil // Already open
	}
	c.openFilesMu.Unlock()

	content, err := os.ReadFile(filepath)
	if err != nil {
		if c.debug {
			log.Printf("Skipping open for non-existent/unreadable file %s: %v", filepath, err)
		}
		return nil // Treat as non-fatal
	}

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentUri(uri),
			LanguageID: DetectLanguageID(uri), // Assign protocol.LanguageKind directly
			Version:    1,
			Text:       string(content),
		},
	}

	// Notify is defined in transport.go
	if err := c.Notify(ctx, "textDocument/didOpen", params); err != nil {
		if c.debug {
			log.Printf("Error sending didOpen notification for %s: %v", filepath, err)
		}
		return fmt.Errorf("didOpen notification failed for %s: %w", filepath, err)
	}

	c.openFilesMu.Lock()
	c.openFiles[uri] = &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentUri(uri),
	}
	c.openFilesMu.Unlock()

	if c.debug {
		log.Printf("Opened file: %s (Version 1)", filepath)
	}

	return nil
}

// NotifyChange notifies the LSP server that a file has changed.
func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	uri := "file://" + filepath

	content, err := os.ReadFile(filepath)
	if err != nil {
		if c.debug {
			log.Printf("Skipping change notification for unreadable file %s: %v", filepath, err)
		}
		return nil // Don't error out if file disappeared
	}

	c.openFilesMu.Lock()
	fileInfo, isOpen := c.openFiles[uri]
	if !isOpen {
		c.openFilesMu.Unlock()
		if c.debug {
			log.Printf("File %s changed but wasn't tracked as open, attempting implicit open.", filepath)
		}
		openErr := c.OpenFile(ctx, filepath) // Try to open it first
		if openErr != nil {
			return fmt.Errorf("failed to implicitly open changed file %s: %w", filepath, openErr)
		}
		c.openFilesMu.Lock() // Re-acquire lock
		fileInfo, isOpen = c.openFiles[uri]
		if !isOpen {
			c.openFilesMu.Unlock() // Should not happen, but handle defensively
			return fmt.Errorf("failed to track file %s even after implicit open", filepath)
		}
	}

	fileInfo.Version++
	version := fileInfo.Version
	c.openFilesMu.Unlock() // Unlock after getting version

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentUri(uri),
			},
			Version: int32(version), // protocol uses int32
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		},
	}

	// Notify is defined in transport.go
	if err := c.Notify(ctx, "textDocument/didChange", params); err != nil {
		if c.debug {
			log.Printf("Error sending didChange notification for %s (Version %d): %v", filepath, version, err)
		}
		// Consider reverting version increment on failure? For now, just return error.
		return fmt.Errorf("didChange notification failed for %s: %w", filepath, err)
	}

	if c.debug {
		log.Printf("Notified change for file: %s (Version %d)", filepath, version)
	}
	return nil
}

// CloseFile notifies the LSP server that a file has been closed.
func (c *Client) CloseFile(ctx context.Context, filepath string) error {
	uri := "file://" + filepath

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; !exists {
		c.openFilesMu.Unlock()
		if c.debug {
			log.Printf("File already closed or never opened: %s", filepath)
		}
		return nil // Already closed
	}
	// Keep lock until delete
	defer c.openFilesMu.Unlock()

	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri(uri),
		},
	}
	if c.debug {
		log.Printf("Closing file: %s", filepath)
	}
	// Notify is defined in transport.go
	if err := c.Notify(ctx, "textDocument/didClose", params); err != nil {
		if c.debug {
			log.Printf("Error sending didClose notification for %s: %v", filepath, err)
		}
		// Continue to remove from tracking even if notify fails
	}

	delete(c.openFiles, uri)

	// Also clear diagnostics for the closed file
	c.diagnosticsMu.Lock()
	delete(c.diagnostics, protocol.DocumentUri(uri))
	c.diagnosticsMu.Unlock()

	return nil
}

// IsFileOpen checks if the client is currently tracking the file as open.
func (c *Client) IsFileOpen(filepath string) bool {
	uri := "file://" + filepath
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// CloseAllFiles attempts to close all files currently tracked as open.
func (c *Client) CloseAllFiles(ctx context.Context) {
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))
	for uri := range c.openFiles {
		filePath := strings.TrimPrefix(uri, "file://")
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock() // Unlock before starting to close

	closedCount := 0
	for _, filePath := range filesToClose {
		// Use a timeout for each close operation within the overall context
		closeCtx, closeCancel := context.WithTimeout(ctx, 1*time.Second)
		err := c.CloseFile(closeCtx, filePath)
		closeCancel() // Release resources promptly
		if err == nil {
			closedCount++
		} else if c.debug {
			log.Printf("Error closing file %s during CloseAllFiles: %v", filePath, err)
		}
	}

	if c.debug {
		log.Printf("Attempted to close %d files, successfully closed %d", len(filesToClose), closedCount)
	}
}

// GetFileDiagnostics returns the cached diagnostics for a given file URI.
func (c *Client) GetFileDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	diags, ok := c.diagnostics[uri]
	if !ok || diags == nil {
		return nil // Return nil explicitly if no diagnostics exist
	}
	// Return a copy to prevent external modification of the cache
	diagsCopy := make([]protocol.Diagnostic, len(diags))
	copy(diagsCopy, diags)
	return diagsCopy
}

// --- NEW LSP Request Methods ---
// These methods use Call/Notify which are assumed to be defined in transport.go

// RequestRename sends a textDocument/rename request.
func (c *Client) RequestRename(ctx context.Context, params protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	var result protocol.WorkspaceEdit
	if err := c.Call(ctx, "textDocument/rename", params, &result); err != nil {
		return nil, fmt.Errorf("textDocument/rename failed: %w", err)
	}
	return &result, nil
}

// RequestWorkspaceSymbols sends a workspace/symbol request.
func (c *Client) RequestWorkspaceSymbols(ctx context.Context, params protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	var result []protocol.SymbolInformation
	rawResult := json.RawMessage{}
	if err := c.Call(ctx, "workspace/symbol", params, &rawResult); err != nil {
		return nil, fmt.Errorf("workspace/symbol request failed: %w", err)
	}

	if string(rawResult) == "null" {
		return []protocol.SymbolInformation{}, nil // Return empty slice, not nil
	}

	if err := json.Unmarshal(rawResult, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace/symbol result: %w", err)
	}
	return result, nil
}

// RequestDocumentSymbols sends a textDocument/documentSymbol request.
func (c *Client) RequestDocumentSymbols(ctx context.Context, params protocol.DocumentSymbolParams) (any, error) { // Use any instead of interface{}
	rawResult := json.RawMessage{}
	if err := c.Call(ctx, "textDocument/documentSymbol", params, &rawResult); err != nil {
		return nil, fmt.Errorf("textDocument/documentSymbol request failed: %w", err)
	}

	if string(rawResult) == "null" {
		return []protocol.DocumentSymbol{}, nil // Return empty slice of preferred type
	}

	// Try hierarchical structure first
	var docSymbols []protocol.DocumentSymbol
	if err := json.Unmarshal(rawResult, &docSymbols); err == nil {
		return docSymbols, nil
	}

	// If hierarchical fails, try flat structure
	var symbolInfo []protocol.SymbolInformation
	if err := json.Unmarshal(rawResult, &symbolInfo); err == nil {
		return symbolInfo, nil
	}

	return nil, fmt.Errorf("failed to unmarshal textDocument/documentSymbol result into known structures")
}


// --- Assumed functions/types (defined elsewhere) ---

// transport.go:
// func (c *Client) Call(ctx context.Context, method string, params interface{}, result interface{}) error
// func (c *Client) Notify(ctx context.Context, method string, params interface{}) error
// func (c *Client) handleMessages()
// type Message struct { ... }
// type NotificationHandler func(params json.RawMessage)
// type ServerRequestHandler func(params json.RawMessage) (interface{}, error)

// methods.go:
// func (c *Client) Initialized(ctx context.Context, params protocol.InitializedParams) error

// server-request-handlers.go:
// func HandleApplyEdit(params json.RawMessage) (interface{}, error)
// func HandleWorkspaceConfiguration(params json.RawMessage) (interface{}, error)
// func HandleRegisterCapability(params json.RawMessage) (interface{}, error)
// func HandleServerMessage(params json.RawMessage)
// func HandleDiagnostics(c *Client, params json.RawMessage)

// detect-language.go:
// func DetectLanguageID(uri string) protocol.LanguageKind

// protocol.go:
// (Contains various LSP type definitions like InitializeParams, ClientCapabilities, etc.)
