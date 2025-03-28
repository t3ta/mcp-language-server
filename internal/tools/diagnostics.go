package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath" // Added for absolute path conversion
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDiagnostics retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string, includeContext bool, showLineNumbers bool) (string, error) {
	// Ensure filePath is absolute
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path for '%s': %w", filePath, err)
	}
	filePath = absFilePath // Use absolute path from now on

	err = client.OpenFile(ctx, filePath) // Use absolute path
	if err != nil {
		// Don't return error if file is already open, just log it maybe?
		// For now, let's proceed, assuming the file might be open from a previous call.
		log.Printf("Warning: OpenFile failed (file might already be open): %v", err)
		// return "", fmt.Errorf("could not open file '%s': %w", filePath, err)
	}

	uri := protocol.DocumentUri("file://" + filePath) // URIはここで定義

	// ★追加: まずキャッシュを確認
	cachedDiagnostics := client.GetFileDiagnostics(uri)
	if len(cachedDiagnostics) > 0 {
		// キャッシュに既に情報があれば、それをフォーマットして返す
		return formatDiagnosticsOutput(ctx, client, filePath, uri, cachedDiagnostics, includeContext, showLineNumbers)
	}
	// ★ここまで追加

	// Wait for diagnostics to appear in the cache with a timeout
	timeout := time.After(30 * time.Second)          // 最大30秒待つ
	ticker := time.NewTicker(200 * time.Millisecond) // 200ミリ秒ごとにチェック
	defer ticker.Stop()

	// ★変更なし: ポーリング開始前の最終診断時刻を取得 (存在しない場合はゼロ値)
	initialLastTime, _ := client.GetLastDiagnosticsTime(uri)

	// ★変更なし: for ループ以降
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for diagnostics for %s", filePath)
		case <-timeout:
			// タイムアウトした場合、キャッシュを最終確認
			diagnostics := client.GetFileDiagnostics(uri)
			if len(diagnostics) == 0 {
				log.Printf("Timeout waiting for diagnostics for %s", filePath)
				return "No diagnostics found for " + filePath + " (timeout waiting)", nil
			}
			// タイムアウトしてもキャッシュに何かあればそれをフォーマットして返す
			return formatDiagnosticsOutput(ctx, client, filePath, uri, diagnostics, includeContext, showLineNumbers)
		case <-ticker.C:
			// ★変更: キャッシュではなく最終更新時刻をチェック
			lastTime, ok := client.GetLastDiagnosticsTime(uri)
			// 診断情報が来ていて、かつポーリング開始後ならループを抜ける
			if ok && lastTime.After(initialLastTime) {
				goto format_diagnostics // ループを抜けてフォーマット処理へ
			}
			// まだ来てなければポーリング続行
		}
	}

format_diagnostics: // ★ラベル名を変更
	// ループを抜けたら、キャッシュから最新の診断情報を取得
	diagnostics := client.GetFileDiagnostics(uri)

	// フォーマット処理を呼び出す
	return formatDiagnosticsOutput(ctx, client, filePath, uri, diagnostics, includeContext, showLineNumbers)
}

// formatDiagnosticsOutput formats a list of diagnostics into a user-readable string.
func formatDiagnosticsOutput(ctx context.Context, client *lsp.Client, filePath string, uri protocol.DocumentUri, diagnostics []protocol.Diagnostic, includeContext bool, showLineNumbers bool) (string, error) {
	if len(diagnostics) == 0 {
		return "No diagnostics found for " + filePath, nil
	}

	var formattedDiagnostics []string
	for _, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("Line %d, Column %d", diag.Range.Start.Line+1, diag.Range.Start.Character+1)
		var codeContext string
		startLine := diag.Range.Start.Line + 1
		if includeContext {
			// Use a timeout for potentially slow GetFullDefinition
			ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second) // 5秒タイムアウト
			content, loc, err := GetFullDefinition(ctxTimeout, client, protocol.Location{URI: uri, Range: diag.Range})
			cancel() // Release context resources promptly
			if err != nil {
				log.Printf("failed to get file content for context: %v", err)
				// Fallback to reading just the line if GetFullDefinition fails
				contentBytes, readErr := os.ReadFile(filePath)
				if readErr == nil {
					lines := strings.Split(string(contentBytes), "\n")
					if int(diag.Range.Start.Line) < len(lines) {
						codeContext = lines[diag.Range.Start.Line]
					}
				}
			} else {
				startLine = loc.Range.Start.Line + 1 // Update startLine based on GetFullDefinition result
				codeContext = content
			}
		} else {
			content, err := os.ReadFile(filePath)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				if int(diag.Range.Start.Line) < len(lines) {
					codeContext = lines[diag.Range.Start.Line]
				}
			}
		}
		formattedDiag := fmt.Sprintf("%s\n[%s] %s\nLocation: %s\nMessage: %s\n", strings.Repeat("=", 60), severity, filePath, location, diag.Message)
		if diag.Source != "" {
			formattedDiag += fmt.Sprintf("Source: %s\n", diag.Source)
		}
		if diag.Code != nil {
			formattedDiag += fmt.Sprintf("Code: %v\n", diag.Code)
		}
		formattedDiag += strings.Repeat("=", 60)
		if codeContext != "" {
			if showLineNumbers {
				codeContext = addLineNumbers(codeContext, int(startLine))
			}
			formattedDiag += fmt.Sprintf("\n%s\n", codeContext)
		}
		formattedDiagnostics = append(formattedDiagnostics, formattedDiag)
	}
	return strings.Join(formattedDiagnostics, "\n"), nil
}

func getSeverityString(severity protocol.DiagnosticSeverity) string {
	switch severity {
	case protocol.SeverityError:
		return "ERROR"
	case protocol.SeverityWarning:
		return "WARNING"
	case protocol.SeverityInformation:
		return "INFO"
	case protocol.SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}
