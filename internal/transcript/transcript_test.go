package transcript

import (
	"strings"
	"testing"
)

func TestExtractTextFromBlocks(t *testing.T) {
	content := []any{
		map[string]any{"type": "thinking", "thinking": "segredo interno"},
		map[string]any{"type": "text", "text": "a história de verdade"},
		map[string]any{"type": "tool_use", "name": "Bash"},
	}
	got := extractText(content)
	if got != "a história de verdade" {
		t.Fatalf("deveria pegar só o bloco text, veio %q", got)
	}
	if strings.Contains(got, "segredo") {
		t.Fatal("thinking não pode vazar")
	}
}

func TestCleanStripsInjectedNoise(t *testing.T) {
	in := "pergunta real\n<system-reminder>ignore isso</system-reminder>\n" +
		"<<<LOOKOVER:x>>>\ncontexto de outra sessão\n<<<FIM LOOKOVER>>>\nfim"
	got := clean(in)
	for _, bad := range []string{"system-reminder", "LOOKOVER", "ignore isso", "outra sessão"} {
		if strings.Contains(got, bad) {
			t.Fatalf("ruído %q não foi removido: %q", bad, got)
		}
	}
	if !strings.Contains(got, "pergunta real") || !strings.Contains(got, "fim") {
		t.Fatalf("conteúdo real foi perdido: %q", got)
	}
}
