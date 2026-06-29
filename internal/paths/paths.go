// Package paths centraliza a descoberta dos diretórios do Claude Code e do lookover.
package paths

import (
	"os"
	"path/filepath"
)

// ClaudeDir devolve o diretório de config do Claude Code, respeitando
// CLAUDE_CONFIG_DIR quando definido (default ~/.claude).
func ClaudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// SessionsDir é a pasta nativa do Claude Code com um JSON por sessão (nome = pid).
func SessionsDir() string { return filepath.Join(ClaudeDir(), "sessions") }

// SettingsFile é o settings.json global.
func SettingsFile() string { return filepath.Join(ClaudeDir(), "settings.json") }

// StoreDir é onde o lookover guarda os meta/digest por sessão.
func StoreDir() string { return filepath.Join(ClaudeDir(), "lookover") }

// MetaFile / DigestFile resolvem os dois arquivos de uma sessão.
func MetaFile(sessionID string) string {
	return filepath.Join(StoreDir(), sessionID+".meta.json")
}
func DigestFile(sessionID string) string {
	return filepath.Join(StoreDir(), sessionID+".digest.json")
}

// LogFile é o log de diagnóstico do lookover.
func LogFile() string { return filepath.Join(StoreDir(), "lookover.log") }

// DisabledFlag é o kill switch por arquivo.
func DisabledFlag() string { return filepath.Join(StoreDir(), "disabled") }

// ConfigFile guarda as opções escolhidas no init.
func ConfigFile() string { return filepath.Join(StoreDir(), "config.json") }

// EnsureStore cria o diretório do store se faltar.
func EnsureStore() error { return os.MkdirAll(StoreDir(), 0o755) }
