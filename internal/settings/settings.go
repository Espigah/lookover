// Package settings faz o patch idempotente do ~/.claude/settings.json,
// registrando os 4 hooks do lookover sem duplicar nem apagar hooks existentes.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"lookover/internal/paths"
)

// hookEvents são os eventos que o lookover registra.
var hookEvents = []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"}

func load() (map[string]any, error) {
	b, err := os.ReadFile(paths.SettingsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("settings.json inválido: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func entryFor(binPath, event string) map[string]any {
	hook := map[string]any{"type": "command", "command": binPath, "args": []any{"hook"}}
	entry := map[string]any{"hooks": []any{hook}}
	if event == "PostToolUse" {
		entry["matcher"] = ""
	}
	return entry
}

// hasLookover diz se o array de um evento já tem uma entrada apontando pro binário.
func hasLookover(arr []any, binPath string) bool {
	for _, e := range arr {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		hs, ok := em["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == binPath {
				return true
			}
			// também detecta pelo basename "lookover" pra robustez
			if cmd, _ := hm["command"].(string); strings.HasSuffix(cmd, "/lookover") {
				return true
			}
		}
	}
	return false
}

// build aplica o merge e devolve o mapa resultante + se houve mudança.
func build(m map[string]any, binPath string) (map[string]any, bool) {
	changed := false
	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, ev := range hookEvents {
		arr, _ := hooks[ev].([]any)
		if !hasLookover(arr, binPath) {
			arr = append(arr, entryFor(binPath, ev))
			hooks[ev] = arr
			changed = true
		}
	}
	m["hooks"] = hooks
	return m, changed
}

// Preview devolve o JSON resultante do patch sem gravar (dry-run).
func Preview(binPath string) (string, bool, error) {
	m, err := load()
	if err != nil {
		return "", false, err
	}
	res, changed := build(m, binPath)
	b, _ := json.MarshalIndent(res, "", "  ")
	return string(b), changed, nil
}

// Apply grava o patch com backup. Idempotente.
func Apply(binPath string) (bool, error) {
	m, err := load()
	if err != nil {
		return false, err
	}
	res, changed := build(m, binPath)
	if !changed {
		return false, nil
	}
	// backup do original (se existir)
	if orig, err := os.ReadFile(paths.SettingsFile()); err == nil {
		_ = os.WriteFile(paths.SettingsFile()+".bak", orig, 0o644)
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	tmp := paths.SettingsFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, paths.SettingsFile())
}

// Remove tira as entradas do lookover (uninstall). Devolve true se mudou.
func Remove(binPath string) (bool, error) {
	m, err := load()
	if err != nil {
		return false, err
	}
	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		return false, nil
	}
	changed := false
	for _, ev := range hookEvents {
		arr, _ := hooks[ev].([]any)
		var kept []any
		for _, e := range arr {
			if entryIsLookover(e, binPath) {
				changed = true
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			delete(hooks, ev)
		} else {
			hooks[ev] = kept
		}
	}
	if !changed {
		return false, nil
	}
	m["hooks"] = hooks
	b, _ := json.MarshalIndent(m, "", "  ")
	tmp := paths.SettingsFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, paths.SettingsFile())
}

func entryIsLookover(e any, binPath string) bool {
	em, ok := e.(map[string]any)
	if !ok {
		return false
	}
	hs, _ := em["hooks"].([]any)
	return hasLookover([]any{map[string]any{"hooks": hs}}, binPath)
}
