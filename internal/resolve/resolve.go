// Package resolve decide, a partir do prompt, qual sessão (se alguma) o usuário
// está referenciando. Tudo puro-Go: gate de intenção + score lexical + decisão
// por confiança. Sem LLM no caminho quente.
package resolve

import (
	"regexp"
	"sort"
	"strings"

	"lookover/internal/capture"
	"lookover/internal/sessions"
	"lookover/internal/store"
)

// intentRe é o gate barato: só segue se o prompt parece referenciar outro
// terminal/sessão. Sem match → custo zero de token (não injeta nada).
var intentRe = regexp.MustCompile(`(?i)\b(termin\w+|sess(?:ã|a)o|na aba|no terminal|outra janela|other (?:terminal|session)|com base n[oa]\b)`)

// HasIntent diz se o prompt referencia outro terminal.
func HasIntent(prompt string) bool { return intentRe.MatchString(prompt) }

// deepRe detecta o pedido de CONTEÚDO COMPLETO (fetch profundo on-demand),
// vs. só o resumo de identificação.
var deepRe = regexp.MustCompile(`(?i)(complet[oa]|í?ntegra|inteir[oa]|verbatim|na ?í?ntegra|conte[úu]do|o texto|texto (?:da|do|completo)|exatament\w*|o que (?:foi )?(?:escrit|dit|fal)\w*|par[áa]grafo|ipsis|mostr\w+ tudo|literal\w*)`)

// IsDeep diz se o usuário quer o conteúdo verbatim da outra sessão.
func IsDeep(prompt string) bool { return deepRe.MatchString(prompt) }

// Candidate é uma sessão pontuada.
type Candidate struct {
	Sess  sessions.Session
	Meta  store.Meta
	Score float64
}

// Outcome é o resultado da decisão por confiança.
type Outcome struct {
	Kind     string      // "winner" | "shortlist" | "none"
	Winner   Candidate   // se Kind == winner
	Shortlist []Candidate // se Kind == shortlist
	Live     []Candidate // todas as vivas (pra mensagem mínima)
}

var nameRe = regexp.MustCompile(`[a-z0-9][a-z0-9_\-]{2,}`)

// Resolve ranqueia as sessões vivas (exceto a própria) e decide.
func Resolve(prompt, selfSessionID string) Outcome {
	live, _ := sessions.Scan(true)
	var cands []Candidate
	pl := strings.ToLower(prompt)
	promptTokens := tokenSet(pl)

	for _, s := range live {
		if s.SessionID == selfSessionID {
			continue
		}
		m, _ := store.ReadMeta(s.SessionID)
		c := Candidate{Sess: s, Meta: m}
		c.Score = score(pl, promptTokens, s, m)
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })

	out := Outcome{Kind: "none", Live: cands}
	if len(cands) == 0 {
		return out
	}
	top := cands[0]
	second := 0.0
	if len(cands) > 1 {
		second = cands[1].Score
	}
	switch {
	case top.Score < minScore:
		// nada suficientemente forte: não injeta (evita falso positivo de
		// match genérico/fraco, ex: um único tópico banal em comum).
		out.Kind = "none"
	case second < minScore || top.Score >= second*1.5:
		// um candidato claramente à frente: LOCALIZADO -> dispara o dump profundo.
		out.Kind = "winner"
		out.Winner = top
	default:
		// dois+ candidatos fortes e próximos: ambíguo de verdade, pede desambiguação.
		for _, c := range cands {
			if c.Score < minScore || len(out.Shortlist) >= 3 {
				break
			}
			out.Shortlist = append(out.Shortlist, c)
		}
		out.Kind = "shortlist"
	}
	return out
}

// minScore é o piso pra considerar que o prompt referencia mesmo uma sessão.
// Match por nome vale ~10, então referências reais passam folgado; matches
// genéricos (1 tópico banal = 2) ficam abaixo e não injetam nada.
const minScore = 3.0

func tokenSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, t := range nameRe.FindAllString(s, -1) {
		set[t] = true
	}
	return set
}

// score: nome (peso alto) > topics > repo > lastFact > skill, + bônus recência.
func score(pl string, ptoks map[string]bool, s sessions.Session, m store.Meta) float64 {
	var sc float64
	// match de nome explícito da sessão (forte)
	if s.Name != "" {
		nl := strings.ToLower(s.Name)
		if strings.Contains(pl, nl) {
			sc += 10
		} else if ptoks[nl] {
			sc += 8
		} else if overlapName(nl, ptoks) {
			sc += 4
		}
	}
	// topics (sinal primário fuzzy)
	for _, t := range m.Topics {
		if ptoks[strings.ToLower(t)] {
			sc += 2
		}
	}
	// nome do repo
	if r := strings.ToLower(capture.RepoName(s.Cwd)); r != "" {
		if ptoks[r] {
			sc += 1.5
		} else if overlapName(r, ptoks) {
			sc += 0.75
		}
	}
	// lastFact
	for _, t := range capture.Topics(m.LastFact, 8) {
		if ptoks[t] {
			sc += 1
		}
	}
	// currentSkill (bônus quando existe)
	if m.CurrentSkill != "" {
		for _, t := range capture.Topics(m.CurrentSkill, 6) {
			if ptoks[t] {
				sc += 1.5
			}
		}
	}
	return sc
}

// nameStop são sub-tokens genéricos demais pra valerem como match de nome
// (senão "oncall-terminal" casa com a palavra "terminal" de toda referência).
var nameStop = map[string]bool{
	"terminal": true, "term": true, "session": true, "sessao": true, "sessão": true,
	"claude": true, "shell": true, "console": true, "tab": true, "window": true,
	"janela": true, "aba": true, "the": true, "dev": true, "main": true, "work": true,
}

// overlapName checa se algum sub-token distintivo do nome (separado por -/_/.)
// bate com o prompt, ignorando tokens genéricos.
func overlapName(name string, ptoks map[string]bool) bool {
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	}) {
		if len(part) >= 4 && !nameStop[part] && ptoks[part] {
			return true
		}
	}
	return false
}
