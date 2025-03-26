package main

import (
	"context"
	"encoding/json" // Added for config parsing
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec" // Re-enable for command validation
	"os/signal"
	"path/filepath" // Re-enable for path manipulation
	"strings"       // Added for extension checking
	"syscall"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

var debug = os.Getenv("DEBUG") != ""
var configPath string // Variable to hold the config file path

func init() {
	// Define command-line flags
	flag.StringVar(&configPath, "config", "config.json", "Path to the configuration JSON file")
	// Add other flags here if needed in the future
	// Need to parse flags early before loading config
	flag.Parse()
}

// Note: Old 'config' struct definition removed

type server struct {
	config              Config                 // Use new Config type
	lspClients          map[string]*lsp.Client // Map language name to LSP client
	extensionToLanguage map[string]string      // Map file extension to language name
	mcpServer           *mcp_golang.Server
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	workspaceWatcher    *watcher.WorkspaceWatcher
}

// LanguageServerConfig defines the configuration for a single language server
type LanguageServerConfig struct {
	Language   string   `json:"language"`   // e.g., "typescript", "go"
	Command    string   `json:"command"`    // e.g., "typescript-language-server", "gopls"
	Args       []string `json:"args"`       // Arguments for the LSP command
	Extensions []string `json:"extensions"` // File extensions associated with this language, e.g., [".ts", ".tsx"]
}

// Config holds the overall configuration for the mcp-language-server
type Config struct {
	WorkspaceDir  string                 `json:"workspaceDir"`
	LanguageServers []LanguageServerConfig `json:"languageServers"`
}

/* // Comment out the old parseConfig function
func parseConfig() (*config, error) {
	cfg := &config{}
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (args should be passed after --)")
	flag.StringVar(&cfg.Language, "language", "", "Target language for the LSP server (e.g., typescript, go)") // Added language flag
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	// Validate workspace directory
	if cfg.workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory is required")
	}

	workspaceDir, err := filepath.Abs(cfg.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	cfg.workspaceDir = workspaceDir

	if _, err := os.Stat(cfg.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", cfg.workspaceDir)
	}

	// Validate LSP command
	if cfg.lspCommand == "" {
		return nil, fmt.Errorf("LSP command is required")
	}

	if _, err := exec.LookPath(cfg.lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", cfg.lspCommand)
	}

	return cfg, nil
}
*/ // End comment for old parseConfig function

// loadConfig loads the server configuration from the specified JSON file path
func loadConfig(configPath string) (*Config, error) {
	log.Printf("Loading configuration from: %s", configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If the default config.json doesn't exist, maybe return a default config or a clearer error?
		// For now, return the error.
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
	}

	// --- Validation (Optional but recommended) ---
	if config.WorkspaceDir == "" {
		return nil, fmt.Errorf("config error: workspaceDir is required")
	}
	// Ensure WorkspaceDir is absolute?
	absWorkspaceDir, err := filepath.Abs(config.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("config error: failed to get absolute path for workspaceDir '%s': %w", config.WorkspaceDir, err)
	}
	config.WorkspaceDir = absWorkspaceDir
	if _, err := os.Stat(config.WorkspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("config error: workspaceDir '%s' does not exist", config.WorkspaceDir)
	}


	if len(config.LanguageServers) == 0 {
		log.Printf("Warning: No language servers defined in config file '%s'", configPath)
		// Return error or allow running without language servers? Allow for now.
	}

	// Validate each language server config (e.g., check command exists)
	for i := range config.LanguageServers {
		lsConfig := &config.LanguageServers[i] // Get pointer to modify original
		if lsConfig.Language == "" {
			return nil, fmt.Errorf("config error: language name is required for server at index %d", i)
		}
		if lsConfig.Command == "" {
			return nil, fmt.Errorf("config error: command is required for language '%s'", lsConfig.Language)
		}
		// Check if command exists in PATH or is an absolute path
		if _, err := exec.LookPath(lsConfig.Command); err != nil {
			// Check if it's an absolute path that exists
			if !filepath.IsAbs(lsConfig.Command) || os.IsNotExist(err) {
 				return nil, fmt.Errorf("config error: command '%s' for language '%s' not found in PATH or as absolute path: %w", lsConfig.Command, lsConfig.Language, err)
			}
			// If it's an absolute path that exists, LookPath might fail if it doesn't have execute permissions,
			// but we might allow it here and let the execution fail later. Or add an explicit check?
		}

		if len(lsConfig.Extensions) == 0 {
			log.Printf("Warning: No file extensions specified for language '%s'", lsConfig.Language)
		}
		// Ensure extensions start with '.'?
		for j, ext := range lsConfig.Extensions {
			if !strings.HasPrefix(ext, ".") {
				log.Printf("Warning: Extension '%s' for language '%s' does not start with '.'. Adding '.' automatically.", ext, lsConfig.Language)
				lsConfig.Extensions[j] = "." + ext
			}
		}
	}


	log.Printf("Configuration loaded successfully.")
	return &config, nil
}


func newServer(config *Config) (*server, error) { // Use new Config type
	ctx, cancel := context.WithCancel(context.Background())
	s := &server{
		config:              *config, // Assign the new Config
		lspClients:          make(map[string]*lsp.Client),
		extensionToLanguage: make(map[string]string),
		ctx:                 ctx,
		cancelFunc:          cancel,
	}

	// Build extension to language map
	for _, lsConfig := range config.LanguageServers {
		for _, ext := range lsConfig.Extensions {
			if _, exists := s.extensionToLanguage[ext]; exists {
				log.Printf("Warning: Extension %s is associated with multiple languages. Using %s.", ext, lsConfig.Language)
			}
			s.extensionToLanguage[ext] = lsConfig.Language
		}
	}

	return s, nil
}

// initializeLSP initializes LSP clients for all configured language servers
func (s *server) initializeLSP() error {
	if err := os.Chdir(s.config.WorkspaceDir); err != nil { // Use s.config.WorkspaceDir
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	// Initialize a single workspace watcher (shared by all clients for now)
	// TODO: Consider if watcher needs client-specific handling or if one is sufficient
	if len(s.config.LanguageServers) > 0 {
		// Create a temporary client just for the watcher? Or pick the first one?
		// Let's pick the first one for now, assuming watcher registration is similar.
		firstLangCfg := s.config.LanguageServers[0]
		tempClientForWatcher, err := lsp.NewClient(firstLangCfg.Command, firstLangCfg.Args...)
		if err != nil {
			log.Printf("Warning: Failed to create temporary LSP client for watcher: %v. File watching might not work.", err)
			// Continue without watcher if temp client fails? Or return error? For now, continue.
		} else {
			s.workspaceWatcher = watcher.NewWorkspaceWatcher(tempClientForWatcher)
			// We don't need to fully initialize this temp client, just use it for watcher registration.
			// Maybe there's a better way? Refactor watcher later if needed.
			// Close the temp client immediately? No, watcher needs it. Manage its lifecycle?
			log.Printf("Workspace watcher initialized using LSP client for %s", firstLangCfg.Language)
			go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.WorkspaceDir) // Use s.config.WorkspaceDir
		}
	}

	for _, langCfg := range s.config.LanguageServers {
		log.Printf("Initializing LSP client for %s: %s %v", langCfg.Language, langCfg.Command, langCfg.Args)
		client, err := lsp.NewClient(langCfg.Command, langCfg.Args...)
		if err != nil {
			// Log error but continue trying to initialize other servers
			log.Printf("Error creating LSP client for %s: %v", langCfg.Language, err)
			continue
		}

		// Store the client in the map
		s.lspClients[langCfg.Language] = client

		// Initialize the client (sends 'initialize' request)
		initResult, err := client.InitializeLSPClient(s.ctx, s.config.WorkspaceDir) // Use s.config.WorkspaceDir
		if err != nil {
			log.Printf("Error initializing LSP client for %s: %v", langCfg.Language, err)
			// Remove the client from the map if initialization fails?
			delete(s.lspClients, langCfg.Language)
			client.Close() // Attempt to clean up the failed client process
			continue
		}

		if debug {
			log.Printf("Initialized %s LSP server. Capabilities: %+v\n\n", langCfg.Language, initResult.Capabilities)
		}

		// Wait for server ready (optional, might need adjustment)
		// Doing this sequentially might slow down startup if many servers
		// Maybe wait in parallel? For now, sequential.
		if err := client.WaitForServerReady(s.ctx); err != nil {
			log.Printf("Error waiting for %s LSP server to be ready: %v", langCfg.Language, err)
			// Consider this non-fatal for now?
		}
		log.Printf("%s LSP client ready.", langCfg.Language)
	}

	if len(s.lspClients) == 0 {
		return fmt.Errorf("failed to initialize any LSP clients")
	}

	log.Printf("Finished initializing %d LSP client(s)", len(s.lspClients))
	return nil
}

func (s *server) start() error {
	if err := s.initializeLSP(); err != nil {
		return err
	}

	s.mcpServer = mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := s.registerTools()
	if err != nil {
		return fmt.Errorf("tool registration failed: %v", err)
	}

	return s.mcpServer.Serve()
}

func main() {
	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Load configuration from file (replace parseConfig)
	// TODO: Make config path configurable (e.g., via flag)
	configPath := "config.json"
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	server, err := newServer(config)
	if err != nil {
		log.Fatal(err)
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination
	// Claude desktop does not properly kill child processes for MCP servers
	go func() {
		ppid := os.Getppid()
		if debug {
			log.Printf("Monitoring parent process: %d", ppid)
		}

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentPpid := os.Getppid()
				if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
					log.Printf("Parent process %d terminated (current ppid: %d), initiating shutdown", ppid, currentPpid)
					close(parentDeath)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Handle shutdown triggers
	go func() {
		select {
		case sig := <-sigChan:
			log.Printf("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(server, done)
		case <-parentDeath:
			log.Printf("Parent death detected, initiating shutdown")
			cleanup(server, done)
		}
	}()

	if err := server.start(); err != nil {
		log.Printf("Server error: %v", err)
		cleanup(server, done)
		os.Exit(1)
	}

	<-done
	log.Printf("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(0)
}

func cleanup(s *server, done chan struct{}) {
	log.Printf("Cleanup initiated for PID: %d", os.Getpid())

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cleanup all LSP clients
	if s.lspClients != nil {
		log.Printf("Cleaning up %d LSP client(s)...", len(s.lspClients))
		for lang, client := range s.lspClients {
			log.Printf("Cleaning up %s LSP client...", lang)
			if client != nil {
				log.Printf("Closing open files for %s", lang)
				client.CloseAllFiles(ctx) // Close files first

				log.Printf("Sending shutdown request to %s", lang)
				if err := client.Shutdown(ctx); err != nil {
					log.Printf("Shutdown request failed for %s: %v", lang, err)
				}

				// Exit notification might not be strictly necessary after shutdown,
				// but let's keep it for now, similar to the original logic.
				log.Printf("Sending exit notification to %s", lang)
				if err := client.Exit(ctx); err != nil {
					log.Printf("Exit notification failed for %s: %v", lang, err)
				}

				log.Printf("Closing %s LSP client connection", lang)
				if err := client.Close(); err != nil {
					log.Printf("Failed to close %s LSP client: %v", lang, err)
				}
				log.Printf("Finished cleanup for %s LSP client.", lang)
			}
		}
		log.Printf("Finished cleaning up all LSP clients.")
	}

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	log.Printf("Cleanup completed for PID: %d", os.Getpid())
}
