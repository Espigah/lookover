// Package transcript lê, sob demanda, o conteúdo verbatim de outra sessão a
// partir do JSONL nativo (~/.claude/projects/<enc>/<sessionId>.jsonl). Lê só a
// CAUDA do arquivo (seek), então é seguro mesmo com transcripts de centenas de MB.
package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"lookover/internal/paths"
)

// ruído injetado que não faz parte da conversa real (não deve poluir o fetch).
var (
	reSysReminder = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)
	reLookover    = regexp.MustCompile(`(?s)<<<LOOKOVER.*?<<<FIM LOOKOVER>>>`)
	reCmdWrap     = regexp.MustCompile(`(?s)<(?:local-)?command-[a-z-]+>.*?</(?:local-)?command-[a-z-]+>`)
	reCmdStdout   = regexp.MustCompile(`(?s)<local-command-stdout>.*?</local-command-stdout>`)
)

func clean(s string) string {
	s = reSysReminder.ReplaceAllString(s, "")
	s = reLookover.ReplaceAllString(s, "")
	s = reCmdStdout.ReplaceAllString(s, "")
	s = reCmdWrap.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// tailWindow é quanto lemos do fim do arquivo (bytes). Cobre várias mensagens
// recentes sem carregar o transcript inteiro.
const tailWindow = 512 * 1024

// Msg é uma mensagem extraída (papel + texto verbatim, sem thinking/tool).
type Msg struct {
	Role string
	Text string
}

// Find localiza o arquivo de transcript de uma sessão por sessionId,
// varrendo os projetos (robusto a como o cwd é codificado no nome do dir).
func Find(sessionID string) string {
	root := filepath.Join(paths.ClaudeDir(), "projects")
	matches, _ := filepath.Glob(filepath.Join(root, "*", sessionID+".jsonl"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// Tail lê as últimas mensagens (user/assistant text) do transcript, limitadas
// a maxBytes de texto agregado (as mais recentes primeiro a serem garantidas).
func Tail(path string, maxBytes int) ([]Msg, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	seeked := false
	if fi.Size() > tailWindow {
		start = fi.Size() - tailWindow
		seeked = true
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil, err
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.Read(buf); err != nil && len(buf) > 0 {
		// leitura parcial ainda é útil; segue
	}
	lines := strings.Split(string(buf), "\n")
	if seeked && len(lines) > 0 {
		lines = lines[1:] // descarta a 1ª linha (provavelmente parcial)
	}

	var msgs []Msg
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var d map[string]any
		if json.Unmarshal([]byte(ln), &d) != nil {
			continue
		}
		t, _ := d["type"].(string)
		if t != "user" && t != "assistant" {
			continue
		}
		msg, _ := d["message"].(map[string]any)
		if msg == nil {
			continue
		}
		role, _ := msg["role"].(string)
		text := clean(extractText(msg["content"]))
		if text == "" {
			continue
		}
		msgs = append(msgs, Msg{Role: role, Text: text})
	}

	// mantém as mais recentes dentro do orçamento de bytes
	return trimByBytes(msgs, maxBytes), nil
}

// extractText pega o texto verbatim de content (string ou blocos), ignorando
// thinking e tool_use/tool_result (ruído + superfície de injeção).
func extractText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, b := range c {
			bm, ok := b.(map[string]any)
			if !ok {
				continue
			}
			if bt, _ := bm["type"].(string); bt == "text" {
				if s, ok := bm["text"].(string); ok {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func trimByBytes(msgs []Msg, budget int) []Msg {
	total := 0
	// percorre do fim pro começo, acumulando até o orçamento
	var kept []Msg
	for i := len(msgs) - 1; i >= 0; i-- {
		n := len(msgs[i].Text) + len(msgs[i].Role) + 2
		if total+n > budget && len(kept) > 0 {
			break
		}
		total += n
		kept = append([]Msg{msgs[i]}, kept...)
	}
	return kept
}
