package lsp

import (
	"encoding/json"
)

// Message represents a JSON-RPC 2.0 message
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int32           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError represents a JSON-RPC 2.0 error
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewRequest(id int32, method string, params interface{}) (*Message, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

func NewNotification(method string, params interface{}) (*Message, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// --- Common LSP Types ---

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// Position represents a position in a text document.
type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

// Range represents a range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource, such as a line inside a text file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextEdit represents a textual change applicable to a text document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit represents changes to many resources managed in the workspace.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []TextDocumentEdit    `json:"documentChanges,omitempty"` // Use TextDocumentEdit for versioned changes
}

// TextDocumentEdit represents edits to a specific text document.
type TextDocumentEdit struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                      `json:"edits"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"` // Use integer for version
}

// --- Initialize ---

// InitializeParams parameters for the initialize request.
type InitializeParams struct {
	ProcessID             int                `json:"processId,omitempty"`
	RootURI               string             `json:"rootUri,omitempty"` // Use string for URI
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
	Trace                 string             `json:"trace,omitempty"` // off | messages | verbose
}

// ClientCapabilities capabilities provided by the client.
type ClientCapabilities struct {
	// Define necessary client capabilities if needed, otherwise keep it minimal or empty.
	Workspace *WorkspaceClientCapabilities `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type WorkspaceClientCapabilities struct {
	ApplyEdit              *bool                               `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditClientCapabilities    `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCapabilities `json:"didChangeConfiguration,omitempty"`
	Symbol                 *WorkspaceSymbolClientCapabilities  `json:"symbol,omitempty"`
	// Add other workspace capabilities as needed
}

type WorkspaceEditClientCapabilities struct {
	DocumentChanges *bool `json:"documentChanges,omitempty"`
	// Add other workspace edit capabilities as needed
}

type DidChangeConfigurationCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Completion *CompletionClientCapabilities `json:"completion,omitempty"`
	Hover      *HoverClientCapabilities      `json:"hover,omitempty"`
	SignatureHelp *SignatureHelpClientCapabilities `json:"signatureHelp,omitempty"`
	References *ReferencesClientCapabilities `json:"references,omitempty"`
	DocumentHighlight *DocumentHighlightClientCapabilities `json:"documentHighlight,omitempty"`
	DocumentSymbol *DocumentSymbolClientCapabilities `json:"documentSymbol,omitempty"`
	Formatting *FormattingClientCapabilities `json:"formatting,omitempty"`
	RangeFormatting *RangeFormattingClientCapabilities `json:"rangeFormatting,omitempty"`
	OnTypeFormatting *OnTypeFormattingClientCapabilities `json:"onTypeFormatting,omitempty"`
	Definition *DefinitionClientCapabilities `json:"definition,omitempty"`
	CodeAction *CodeActionClientCapabilities `json:"codeAction,omitempty"`
	CodeLens   *CodeLensClientCapabilities   `json:"codeLens,omitempty"`
	DocumentLink *DocumentLinkClientCapabilities `json:"documentLink,omitempty"`
	Rename     *RenameClientCapabilities     `json:"rename,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`
	// Add other text document capabilities as needed
}

// Define specific capability structures if needed, e.g.:
type CompletionClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
	CompletionItem *struct {
		SnippetSupport *bool `json:"snippetSupport,omitempty"`
		// ... other CompletionItem capabilities
	} `json:"completionItem,omitempty"`
	// ... other completion capabilities
}

type HoverClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
	ContentFormat []string `json:"contentFormat,omitempty"` // e.g., ["markdown", "plaintext"]
}

type SignatureHelpClientCapabilities struct {
	// ...
}

type ReferencesClientCapabilities struct {
	// ...
}

type DocumentHighlightClientCapabilities struct {
	// ...
}

type FormattingClientCapabilities struct {
	// ...
}

type RangeFormattingClientCapabilities struct {
	// ...
}

type OnTypeFormattingClientCapabilities struct {
	// ...
}

type DefinitionClientCapabilities struct {
	// ...
}

type CodeActionClientCapabilities struct {
	// ...
}

type CodeLensClientCapabilities struct {
	// ...
}

type DocumentLinkClientCapabilities struct {
	// ...
}

type PublishDiagnosticsClientCapabilities struct {
	RelatedInformation *bool `json:"relatedInformation,omitempty"`
	// ... other diagnostic capabilities
}


// InitializeResult result of the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities capabilities provided by the server.
type ServerCapabilities struct {
	TextDocumentSync                 *TextDocumentSyncOptions    `json:"textDocumentSync,omitempty"`
	CompletionProvider               *CompletionOptions          `json:"completionProvider,omitempty"`
	HoverProvider                    *bool                       `json:"hoverProvider,omitempty"` // Or HoverOptions
	SignatureHelpProvider            *SignatureHelpOptions       `json:"signatureHelpProvider,omitempty"`
	DefinitionProvider               *bool                       `json:"definitionProvider,omitempty"` // Or DefinitionOptions
	ReferencesProvider               *bool                       `json:"referencesProvider,omitempty"` // Or ReferencesOptions
	DocumentHighlightProvider        *bool                       `json:"documentHighlightProvider,omitempty"` // Or DocumentHighlightOptions
	DocumentSymbolProvider           *bool                       `json:"documentSymbolProvider,omitempty"` // Or DocumentSymbolOptions
	WorkspaceSymbolProvider          *bool                       `json:"workspaceSymbolProvider,omitempty"` // Or WorkspaceSymbolOptions
	CodeActionProvider               *bool                       `json:"codeActionProvider,omitempty"` // Or CodeActionOptions
	CodeLensProvider                 *CodeLensOptions            `json:"codeLensProvider,omitempty"`
	DocumentFormattingProvider       *bool                       `json:"documentFormattingProvider,omitempty"` // Or DocumentFormattingOptions
	DocumentRangeFormattingProvider  *bool                       `json:"documentRangeFormattingProvider,omitempty"` // Or DocumentRangeFormattingOptions
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   *bool                       `json:"renameProvider,omitempty"` // Or RenameOptions
	DocumentLinkProvider             *DocumentLinkOptions        `json:"documentLinkProvider,omitempty"`
	ExecuteCommandProvider           *ExecuteCommandOptions      `json:"executeCommandProvider,omitempty"`
	Workspace                        *ServerWorkspaceCapabilities `json:"workspace,omitempty"`
	// Add other server capabilities as needed
}

type TextDocumentSyncOptions struct {
	OpenClose *bool             `json:"openClose,omitempty"`
	Change    *TextDocumentSyncKind `json:"change,omitempty"` // Use pointer to enum
	// Add other sync options if needed
}

type TextDocumentSyncKind int

const (
	// TextDocumentSyncKindNone Documents should not be synced at all.
	TextDocumentSyncKindNone TextDocumentSyncKind = 0
	// TextDocumentSyncKindFull Documents are synced by sending the full content of the document.
	TextDocumentSyncKindFull TextDocumentSyncKind = 1
	// TextDocumentSyncKindIncremental Documents are synced by sending incremental updates.
	TextDocumentSyncKindIncremental TextDocumentSyncKind = 2
)

type CompletionOptions struct {
	ResolveProvider   *bool    `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	// Add other completion options if needed
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	// Add other signature help options if needed
}

type CodeLensOptions struct {
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

type DocumentLinkOptions struct {
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

type ServerWorkspaceCapabilities struct {
	WorkspaceFolders *WorkspaceFoldersServerCapabilities `json:"workspaceFolders,omitempty"`
	// Add other server workspace capabilities if needed
}

type WorkspaceFoldersServerCapabilities struct {
	Supported           *bool `json:"supported,omitempty"`
	ChangeNotifications *bool `json:"changeNotifications,omitempty"` // Can be string ID or bool
}

// --- Shutdown ---
// No specific params/result types needed for shutdown

// --- Exit ---
// No specific params/result types needed for exit

// --- Text Document Synchronization ---

// DidOpenTextDocumentParams parameters for textDocument/didOpen notification.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentItem describes a text document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidChangeTextDocumentParams parameters for textDocument/didChange notification.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// TextDocumentContentChangeEvent an event describing a change to a text document.
type TextDocumentContentChangeEvent struct {
	// Range is the range of the document that changed.
	// Optional: If not provided, the whole document content is replaced.
	Range *Range `json:"range,omitempty"`
	// RangeLength is the length of the range that got replaced.
	// Optional: Only used if Range is provided.
	RangeLength *int `json:"rangeLength,omitempty"`
	// Text is the new text for the provided range or the whole document.
	Text string `json:"text"`
}


// DidCloseTextDocumentParams parameters for textDocument/didClose notification.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidSaveTextDocumentParams parameters for textDocument/didSave notification.
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"` // Optional content on save
}

// --- Diagnostics ---

// PublishDiagnosticsParams parameters for textDocument/publishDiagnostics notification.
type PublishDiagnosticsParams struct {
	URI         string        `json:"uri"`
	Diagnostics []Diagnostic  `json:"diagnostics"`
	Version     *int          `json:"version,omitempty"` // Optional document version
}

// Diagnostic represents a diagnostic, like a compiler error or warning.
type Diagnostic struct {
	Range              Range                  `json:"range"`
	Severity           *DiagnosticSeverity    `json:"severity,omitempty"` // Use pointer
	Code               *DiagnosticCode        `json:"code,omitempty"`     // Use pointer or interface{}
	Source             *string                `json:"source,omitempty"`
	Message            string                 `json:"message"`
	Tags               []DiagnosticTag        `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               interface{}            `json:"data,omitempty"` // Language server specific data
}

// DiagnosticCode can be a number or string. Use interface{} or a custom type.
type DiagnosticCode interface{} // Or define a struct if structure is known

// DiagnosticSeverity severity of a diagnostic.
type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 1
	SeverityWarning DiagnosticSeverity = 2
	SeverityInfo    DiagnosticSeverity = 3
	SeverityHint    DiagnosticSeverity = 4
)

// DiagnosticTag additional metadata about the diagnostic.
type DiagnosticTag int

const (
	TagUnnecessary DiagnosticTag = 1
	TagDeprecated  DiagnosticTag = 2
)

// DiagnosticRelatedInformation represents related diagnostic information.
type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// --- Code Lens ---

// CodeLensParams parameters for textDocument/codeLens request.
type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// CodeLens represents a command that should be shown along with source text.
type CodeLens struct {
	Range   Range        `json:"range"`
	Command *Command     `json:"command,omitempty"` // Use pointer
	Data    interface{}  `json:"data,omitempty"`  // Optional data field
}

// Command represents a command like 'run test' or 'apply fix'.
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"` // Identifier of the command handler
	Arguments []interface{} `json:"arguments,omitempty"`
}

// --- References ---

// ReferenceParams parameters for textDocument/references request.
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

// ReferenceContext context for reference request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// --- Definition ---

// DefinitionParams parameters for textDocument/definition request.
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Definition result can be Location or []Location or LocationLink[]
// Using []Location for simplicity here, adjust if LocationLink is needed.
type Definition = []Location // Type alias for definition result


// --- NEW TYPES for Rename and Symbols ---

// RenameParams parameters for textDocument/rename request.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// RenameClientCapabilities capabilities specific to the rename request.
type RenameClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
	PrepareSupport      *bool `json:"prepareSupport,omitempty"` // Support for prepareRename request
}

// RenameOptions server capabilities for rename requests.
type RenameOptions struct {
	PrepareProvider *bool `json:"prepareProvider,omitempty"`
}

// WorkspaceSymbolParams parameters for workspace/symbol request.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// DocumentSymbolParams parameters for textDocument/documentSymbol request.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SymbolKind kind of symbol.
type SymbolKind int

const (
	File          SymbolKind = 1
	Module        SymbolKind = 2
	Namespace     SymbolKind = 3
	Package       SymbolKind = 4
	Class         SymbolKind = 5
	Method        SymbolKind = 6
	Property      SymbolKind = 7
	Field         SymbolKind = 8
	Constructor   SymbolKind = 9
	Enum          SymbolKind = 10
	Interface     SymbolKind = 11
	Function      SymbolKind = 12
	Variable      SymbolKind = 13
	Constant      SymbolKind = 14
	String        SymbolKind = 15
	Number        SymbolKind = 16
	Boolean       SymbolKind = 17
	Array         SymbolKind = 18
	Object        SymbolKind = 19
	Key           SymbolKind = 20
	Null          SymbolKind = 21
	EnumMember    SymbolKind = 22
	Struct        SymbolKind = 23
	Event         SymbolKind = 24
	Operator      SymbolKind = 25
	TypeParameter SymbolKind = 26
)

// SymbolTag additional metadata about a symbol.
type SymbolTag int

const (
	// SymbolTagDeprecated Render a symbol as obsolete, usually using a strike-out.
	SymbolTagDeprecated SymbolTag = 1
)


// SymbolInformation represents information about programming constructs like variables, classes, etc.
type SymbolInformation struct {
	Name           string     `json:"name"`
	Kind           SymbolKind `json:"kind"`
	Tags           []SymbolTag `json:"tags,omitempty"`
	Deprecated     *bool      `json:"deprecated,omitempty"` // Deprecated: Use tags instead
	Location       Location   `json:"location"`
	ContainerName *string    `json:"containerName,omitempty"` // Name of the symbol containing this symbol.
}

// DocumentSymbol represents programming constructs like variables, classes, interfaces etc.
// specific to a document. DocumentSymbols can be hierarchical.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         *string          `json:"detail,omitempty"` // More detail for this symbol, e.g., function signature.
	Kind           SymbolKind       `json:"kind"`
	Tags           []SymbolTag      `json:"tags,omitempty"`
	Deprecated     *bool            `json:"deprecated,omitempty"` // Deprecated: Use tags instead
	Range          Range            `json:"range"` // Range encompassing this symbol.
	SelectionRange Range            `json:"selectionRange"` // Range that should be selected when revealing this symbol.
	Children       []DocumentSymbol `json:"children,omitempty"` // Children of this symbol, e.g., methods of a class.
}

// WorkspaceSymbolClientCapabilities capabilities specific to workspace/symbol request.
type WorkspaceSymbolClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
	SymbolKind *struct {
		ValueSet []SymbolKind `json:"valueSet,omitempty"` // Supported symbol kinds.
	} `json:"symbolKind,omitempty"`
}

// DocumentSymbolClientCapabilities capabilities specific to textDocument/documentSymbol request.
type DocumentSymbolClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
	SymbolKind *struct {
		ValueSet []SymbolKind `json:"valueSet,omitempty"` // Supported symbol kinds.
	} `json:"symbolKind,omitempty"`
	HierarchicalDocumentSymbolSupport *bool `json:"hierarchicalDocumentSymbolSupport,omitempty"` // Supports hierarchical document symbols.
}

// DocumentSymbolOptions server capabilities for document symbol requests.
type DocumentSymbolOptions struct {
	// Server supports document symbol requests.
}

// WorkspaceSymbolOptions server capabilities for workspace symbol requests.
type WorkspaceSymbolOptions struct {
	// Server supports workspace symbol requests.
}
