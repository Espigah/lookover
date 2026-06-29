// Package hookio faz o I/O do hook: parse tolerante do stdin e emissão do
// additionalContext. Nunca falha fatal — campos faltando degradam, não quebram.
package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Input são os campos do envelope do hook, extraídos de forma tolerante.
type Input struct {
	Raw            map[string]any
	HookEventName  string
	SessionID      string
	Cwd            string
	TranscriptPath string
	Prompt         string
	ToolName       string
	ToolInput      map[string]any
	Source         string // SessionStart: startup|resume|clear|compact
	MissingFields  []string
	ParseOk        bool
}

func str(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Read consome o stdin e devolve um Input. Erros de parse não são fatais:
// ParseOk vira false e os campos ausentes são listados.
func Read() Input {
	in := Input{ParseOk: true}
	b, err := io.ReadAll(os.Stdin)
	if err != nil || len(b) == 0 {
		in.ParseOk = false
		return in
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		in.ParseOk = false
		return in
	}
	in.Raw = m
	in.HookEventName = str(m, "hook_event_name")
	in.SessionID = str(m, "session_id")
	in.Cwd = str(m, "cwd")
	in.TranscriptPath = str(m, "transcript_path")
	in.Prompt = str(m, "prompt")
	in.ToolName = str(m, "tool_name")
	in.Source = str(m, "source")
	if ti, ok := m["tool_input"].(map[string]any); ok {
		in.ToolInput = ti
	}
	for _, f := range []string{"session_id", "cwd", "hook_event_name"} {
		if str(m, f) == "" {
			in.MissingFields = append(in.MissingFields, f)
		}
	}
	return in
}

// EmitContext escreve o additionalContext de UserPromptSubmit no stdout.
func EmitContext(text string) {
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": text,
		},
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(os.Stdout, string(b))
}

// EmitEmpty não injeta nada (caminho de custo zero de token).
func EmitEmpty() {}
