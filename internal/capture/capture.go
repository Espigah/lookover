// Package capture faz a extração determinística por evento (puro-Go, zero token):
// topics a partir do texto do prompt, resumo de tool, skill e "fatos".
package capture

import (
	"regexp"
	"sort"
	"strings"

	"lookover/internal/store"
)

// Truncate corta s em n runas, com reticências.
func Truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}_\-/.#]{2,}`)

// stopwords PT/EN genéricas (machine-agnostic; nada do domínio do time).
var stop = map[string]bool{
	"que": true, "com": true, "base": true, "para": true, "por": true, "uma": true, "uns": true,
	"the": true, "and": true, "for": true, "with": true, "from": true, "this": true, "that": true,
	"você": true, "voce": true, "terminal": true, "sessão": true, "sessao": true, "fazer": true,
	"está": true, "esta": true, "tem": true, "tá": true, "ta": true, "qual": true, "como": true,
	"meu": true, "minha": true, "isso": true, "essa": true, "esse": true, "onde": true, "ultimo": true,
	"último": true, "veja": true, "dme": true, "the_": true, "não": true, "nao": true, "sobre": true,
}

// Topics extrai tokens distintivos do texto (prompt), preservando ordem de
// primeira aparição, sem stopwords, capado.
func Topics(text string, max int) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range wordRe.FindAllString(strings.ToLower(text), -1) {
		if stop[m] || len(m) < 3 {
			continue
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
		if len(out) >= max {
			break
		}
	}
	return out
}

// MergeTopics funde topics novos com os existentes, mantendo recência (novos
// na frente) e limitando ao teto.
func MergeTopics(existing, fresh []string, max int) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range append(fresh, existing...) {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= max {
			break
		}
	}
	return out
}

// SkillName devolve o nome da skill se o tool for "Skill".
func SkillName(toolName string, toolInput map[string]any) string {
	if toolName != "Skill" {
		return ""
	}
	if v, ok := toolInput["skill"].(string); ok {
		return v
	}
	if v, ok := toolInput["command"].(string); ok {
		return v
	}
	return ""
}

// ToolSummary monta um resumo de 1 linha capado do tool call (nunca verbatim
// de output; só o nome + a 1ª linha relevante do input).
func ToolSummary(toolName string, toolInput map[string]any) string {
	var detail string
	switch toolName {
	case "Bash":
		detail = firstString(toolInput, "command", "description")
	case "Read", "Edit", "Write":
		detail = firstString(toolInput, "file_path")
	case "Grep", "Glob":
		detail = firstString(toolInput, "pattern", "query")
	case "Skill":
		detail = firstString(toolInput, "skill", "command")
	default:
		detail = firstString(toolInput, "description", "query", "prompt")
	}
	detail = strings.SplitN(strings.TrimSpace(detail), "\n", 2)[0]
	s := toolName
	if detail != "" {
		s += ": " + detail
	}
	return Truncate(s, store.MaxToolChars)
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

var prNumRe = regexp.MustCompile(`(?i)\b(?:PR|pull request|MR)[ #]*(\d{2,})`)

// Fact tenta derivar um "fato" curto e marcante do prompt/summary (ex: nº de PR).
// Heurística conservadora; vazio quando nada óbvio.
func Fact(text string) string {
	if m := prNumRe.FindStringSubmatch(text); m != nil {
		return Truncate("PR #"+m[1]+" mencionado", store.MaxFactChars)
	}
	return ""
}

// RepoName extrai o último componente do cwd (nome do repo) pra scoring.
func RepoName(cwd string) string {
	cwd = strings.TrimRight(cwd, "/")
	if i := strings.LastIndex(cwd, "/"); i >= 0 {
		return cwd[i+1:]
	}
	return cwd
}

// SortedTopicsForScore devolve topics ordenados (determinístico em testes).
func SortedTopicsForScore(t []string) []string {
	c := append([]string(nil), t...)
	sort.Strings(c)
	return c
}
