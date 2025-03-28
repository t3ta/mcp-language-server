package lsp

import (
	"encoding/json"
	"log"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

// Requests

func HandleWorkspaceConfiguration(params json.RawMessage) (interface{}, error) {
	return []map[string]interface{}{{}}, nil
}

func HandleRegisterCapability(params json.RawMessage) (interface{}, error) {
	var registerParams protocol.RegistrationParams
	if err := json.Unmarshal(params, &registerParams); err != nil {
		log.Printf("Error unmarshaling registration params: %v", err)
		return nil, err
	}

	for _, reg := range registerParams.Registrations {
		log.Printf("Registration received for method: %s, id: %s", reg.Method, reg.ID)

		switch reg.Method {
		case "workspace/didChangeWatchedFiles":
			// Parse the registration options
			optionsJSON, err := json.Marshal(reg.RegisterOptions)
			if err != nil {
				log.Printf("Error marshaling registration options: %v", err)
				continue
			}

			var options protocol.DidChangeWatchedFilesRegistrationOptions
			if err := json.Unmarshal(optionsJSON, &options); err != nil {
				log.Printf("Error unmarshaling registration options: %v", err)
				continue
			}

			// Store the file watchers registrations
			notifyFileWatchRegistration(reg.ID, options.Watchers)
		}
	}

	return nil, nil
}

func HandleApplyEdit(params json.RawMessage) (interface{}, error) {
	var edit protocol.ApplyWorkspaceEditParams
	if err := json.Unmarshal(params, &edit); err != nil {
		return nil, err
	}

	err := utilities.ApplyWorkspaceEdit(edit.Edit)
	if err != nil {
		log.Printf("Error applying workspace edit: %v", err)
		return protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: err.Error()}, nil
	}

	return protocol.ApplyWorkspaceEditResult{Applied: true}, nil
}

// FileWatchRegistrationHandler is a function that will be called when file watch registrations are received
type FileWatchRegistrationHandler func(id string, watchers []protocol.FileSystemWatcher)

// fileWatchHandler holds the current handler for file watch registrations
var fileWatchHandler FileWatchRegistrationHandler

// RegisterFileWatchHandler sets the handler for file watch registrations
func RegisterFileWatchHandler(handler FileWatchRegistrationHandler) {
	fileWatchHandler = handler
}

// notifyFileWatchRegistration notifies the handler about new file watch registrations
func notifyFileWatchRegistration(id string, watchers []protocol.FileSystemWatcher) {
	if fileWatchHandler != nil {
		fileWatchHandler(id, watchers)
	}
}

// Notifications

func HandleServerMessage(params json.RawMessage) {
	var msg struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &msg); err == nil {
		log.Printf("Server message: %s\n", msg.Message)
	}
}

func HandleDiagnostics(client *Client, params json.RawMessage) {
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		log.Printf("Error unmarshaling diagnostic params: %v", err)
		return
	}

	client.diagnosticsMu.Lock()
	defer client.diagnosticsMu.Unlock()

	client.diagnostics[diagParams.URI] = diagParams.Diagnostics

	log.Printf("Received diagnostics for %s: %d items", diagParams.URI, len(diagParams.Diagnostics))
}
