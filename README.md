# MCP Language Server

**Note:** This repository is a fork of [isaacphi/mcp-language-server](https://github.com/isaacphi/mcp-language-server). It has undergone significant architectural changes (supporting multiple language servers in a single process via a configuration file) and is not intended for merging back into the original repository.

A Model Context Protocol (MCP) server that manages multiple language servers for different programming languages within a single workspace. It provides tools for communicating with the appropriate language server based on file context or explicit language specification.

## Motivation

Language servers excel at tasks that LLMs often struggle with, such as precisely understanding types, navigating complex codebases, and providing accurate symbol references across large projects. This project aims to bring the power of multiple language servers to LLMs through a unified MCP interface, enabling more sophisticated code understanding and manipulation capabilities.

## Status

⚠️ Pre-beta Quality ⚠️

I have tested this server with the following language servers

- pyright (Python)
- tsserver (TypeScript)
- gopls (Go)
- rust-analyzer (Rust)

But it should be compatible with many more.

## Tools

This server provides the following tools, automatically routing requests to the appropriate language server based on file extensions (for file-based tools) or an explicit `language` argument.

- `read_definition`: Retrieves the complete source code definition of a symbol. **Requires a `language` argument** (e.g., `"typescript"`, `"go"`) to specify which language server to query.
- `find_references`: Locates all usages and references of a symbol. **Requires a `language` argument** (e.g., `"typescript"`, `"go"`) to specify which language server to query.
- `get_diagnostics`: Provides diagnostic information for a specific file (language determined by file extension).
- `get_codelens`: Retrieves code lens hints for a specific file (language determined by file extension).
- `execute_codelens`: Runs a code lens action for a specific file (language determined by file extension).
- `apply_text_edit`: Allows making multiple text edits to a file programmatically (language determined by file extension).

Behind the scenes, this MCP server can act on `workspace/applyEdit` requests from the language servers, enabling features like refactoring, adding imports, and code formatting.

Most tools support options like `showLineNumbers`. Refer to the tool schemas for detailed usage.

## About

This codebase makes use of edited code from [gopls](https://go.googlesource.com/tools/+/refs/heads/master/gopls/internal/protocol) to handle LSP communication. See ATTRIBUTION for details.

[mcp-golang](https://github.com/metoro-io/mcp-golang) is used for MCP communication.

## Prerequisites

1.  **Install Go:** Follow instructions at <https://golang.org/doc/install>
2.  **Install Language Servers:** Install the language servers for the languages you want to use in your project. Examples:
    - Python (pyright): `npm install -g pyright`
- TypeScript (tsserver): `npm install -g typescript typescript-language-server`
- Go (gopls): `go install golang.org/x/tools/gopls@latest`
- Rust (rust-analyzer): `rustup component add rust-analyzer`
- Or use any language server

## Setup

1.  **Build the Server:**
    Clone the repository and build the executable:
    ```bash
    git clone https://github.com/isaacphi/mcp-language-server.git
    cd mcp-language-server
    go build -o mcp-language-server .
    ```

2.  **Create Configuration File (`config.json`):**
    Create a JSON configuration file (e.g., `config.json` in the project root or another location) to define the language servers you want to manage.

    **`config.json` Example:**
    ```json
    {
      "workspaceDir": "/Users/you/dev/yourcodebase", // Absolute path to your project root
      "languageServers": [
        {
          "language": "typescript", // Unique name for the language
          "command": "typescript-language-server", // Command to run the LSP server
          "args": ["--stdio"], // Arguments for the LSP command
          "extensions": [".ts", ".tsx", ".js", ".jsx"] // File extensions for this language
        },
        {
          "language": "go",
          "command": "/path/to/your/gopls", // Absolute path or command name for gopls
          "args": [],
          "extensions": [".go"]
        },
        {
          "language": "python",
          "command": "pyright-langserver",
          "args": ["--stdio"],
          "extensions": [".py"]
        }
        // Add entries for other languages as needed
      ]
    }
    ```
    - Replace `/Users/you/dev/yourcodebase` with the absolute path to your project.
    - Replace `/path/to/your/gopls` etc. with the correct command or absolute path for each language server.

3.  **Configure MCP Client:**
    Add the following configuration to your Claude Desktop settings (or similar MCP-enabled client), adjusting paths as necessary:

    ```json
    {
      "mcpServers": {
        "mcp-language-server": { // You can choose any name here
          "command": "/full/path/to/your/clone/mcp-language-server/mcp-language-server", // Absolute path to the built executable
          "args": [
            "--config", "/full/path/to/your/config.json" // Absolute path to your config.json
          ],
          "cwd": "/full/path/to/your/clone/mcp-language-server", // Optional: Set working directory to project root
          "env": {
            // Add any necessary environment variables for LSP servers (e.g., PATH)
            // "PATH": "...",
            "DEBUG": "1" // Optional: Enable debug logging
          }
        }
        // Add other MCP servers here if needed
      }
    }
    ```
    - Ensure the `command` path points to the `mcp-language-server` executable you built.
    - Ensure the `--config` argument points to the `config.json` file you created.
    - Set `cwd` if necessary (usually the directory containing the executable).
    - Add required environment variables (like `PATH` if using shims like `asdf`) to the `env` object.

## Development

Clone the repository:

```bash
git clone https://github.com/isaacphi/mcp-language-server.git
forcd mcp-language-server
```

Install dev dependencies:

```bash
go mod download
```

Build:

```bash
go build
```

Configure your Claude Desktop (or similar) to use the local binary, similar to the Setup section, ensuring the `command` points to your locally built executable and the `--config` argument points to your development `config.json`:

```json
{
  "mcpServers": {
    "mcp-language-dev": { // Example name for development server
      "command": "/full/path/to/your/clone/mcp-language-server/mcp-language-server", // Path to your built binary
      "args": [
        "--config", "/full/path/to/your/clone/mcp-language-server/config.dev.json" // Path to your development config file
      ],
      "cwd": "/full/path/to/your/clone/mcp-language-server", // Working directory
      "env": {
        "DEBUG": "1" // Enable debug logging for development
      }
    }
  }
}
```
Remember to create a `config.dev.json` (or similar) for your development environment.

Rebuild (`go build -o mcp-language-server .`) after making code changes.

## Feedback

Include

```
env: {
  "DEBUG": 1
}
```

To get detailed LSP and application logs. Please include as much information as possible when opening issues.

The following features are on my radar:

- [x] Read definition
- [x] Get references
- [x] Apply edit
- [x] Get diagnostics
- [x] Code lens
- [ ] Hover info
- [ ] Code actions
- [ ] Better handling of context and cancellation
- [ ] Add LSP server configuration options and presets for common languages
- [ ] Make a more consistent and scalable API for tools (pagination, etc.)
- [ ] Create tools at a higher level of abstraction, combining diagnostics, code lens, hover, and code actions when reading definitions or references.
