// Command lookover — compartilhamento de contexto entre terminais do Claude Code.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"lookover/internal/capture"
	"lookover/internal/config"
	"lookover/internal/digest"
	"lookover/internal/hookio"
	"lookover/internal/paths"
	"lookover/internal/render"
	"lookover/internal/resolve"
	"lookover/internal/sessions"
	"lookover/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "hook":
		cmdHook() // sempre sai 0 (ver dentro)
	case "init":
		os.Exit(cmdInit(os.Args[2:]))
	case "list":
		os.Exit(cmdList())
	case "show":
		os.Exit(cmdShow(os.Args[2:]))
	case "compact":
		os.Exit(cmdCompact(os.Args[2:]))
	case "doctor":
		os.Exit(cmdDoctor())
	case "status":
		os.Exit(cmdStatus())
	case "uninstall":
		os.Exit(cmdUninstall())
	case "version", "-v", "--version":
		fmt.Println("lookover 0.1.0")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `lookover — contexto entre terminais do Claude Code
uso:
  lookover init [--llm] [--shadow] [--yes] [-g]   configura hooks (dry-run + confirma)
  lookover list                                   lista sessões vivas
  lookover show <name|sessionId>                  mostra o digest de uma sessão
  lookover doctor                                 diagnóstico de saúde
  lookover status                                 uso/adoção local
  lookover uninstall                              remove os hooks (restaura backup)
  lookover hook                                   (interno) ponto de entrada dos hooks`)
}

// logf grava no log de diagnóstico (best-effort).
func logf(format string, a ...any) {
	_ = paths.EnsureStore()
	f, err := os.OpenFile(paths.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, a...)...)
}

// ---------------------------------------------------------------------------
// hook: caminho quente. NUNCA falha fatal — recover + exit 0 sempre.
// ---------------------------------------------------------------------------

func cmdHook() {
	defer func() {
		if r := recover(); r != nil {
			logf("panic no hook: %v", r)
		}
		os.Exit(0) // invariante: hook nunca bloqueia o Claude Code
	}()

	// 1) kill switch instantâneo
	if _, err := os.Stat(paths.DisabledFlag()); err == nil {
		return
	}
	if os.Getenv("LOOKOVER_DISABLED") == "1" {
		return
	}

	// 2) watchdog anti-hang (alvo de captura é sub-50ms; isto é só rede de segurança)
	time.AfterFunc(5*time.Second, func() { os.Exit(0) })

	in := hookio.Read()
	cfg := config.Load()

	// 3) resolução multi-fonte do sessionId/cwd (resiliência)
	sess, ok := resolveSelf(in)
	if !ok {
		logf("não resolveu a própria sessão (parseOk=%v missing=%v)", in.ParseOk, in.MissingFields)
		return
	}

	// 4) opt-out por diretório
	for _, d := range cfg.OptOutDirs {
		if d != "" && strings.HasPrefix(sess.Cwd, d) {
			return
		}
	}

	// 5) invariante ensure-meta (cobre sessões pré-install)
	lk, _ := store.Acquire(sess.SessionID)
	defer lk.Release()
	m := ensureMeta(sess, in)

	switch in.HookEventName {
	case "SessionStart":
		_ = store.WriteMeta(m)

	case "UserPromptSubmit":
		// captura — não polui os topics da própria sessão quando o prompt é uma
		// query cross-terminal (o texto fala da OUTRA sessão, não desta).
		if !resolve.HasIntent(in.Prompt) {
			if topics := capture.Topics(in.Prompt, store.MaxTopics); len(topics) > 0 {
				m.Topics = capture.MergeTopics(m.Topics, topics, store.MaxTopics)
			}
		}
		if f := capture.Fact(in.Prompt); f != "" {
			m.LastFact = f
		}
		m.EventsSinceDigest++
		_ = store.WriteMeta(m)
		_ = store.AppendEvent(sess.SessionID, store.Event{
			Kind: "prompt", Text: capture.Truncate(in.Prompt, store.MaxPromptChars),
		})
		// query: só se houver intenção cross-terminal (gate de custo zero)
		if !cfg.Shadow {
			maybeInject(in, sess.SessionID, cfg)
		} else if resolve.HasIntent(in.Prompt) {
			logf("[shadow] intenção detectada no prompt; injeção suprimida")
		}

	case "PostToolUse":
		if sk := capture.SkillName(in.ToolName, in.ToolInput); sk != "" {
			m.CurrentSkill = sk
			_ = store.AppendEvent(sess.SessionID, store.Event{Kind: "skill", Name: sk})
		} else {
			sum := capture.ToolSummary(in.ToolName, in.ToolInput)
			_ = store.AppendEvent(sess.SessionID, store.Event{Kind: "tool", Tool: in.ToolName, Summary: sum})
			if f := capture.Fact(sum); f != "" {
				m.LastFact = f
			}
		}
		m.EventsSinceDigest++
		_ = store.WriteMeta(m)

	case "Stop", "PreCompact":
		// throttle: só compacta no PreCompact ou ao acumular o suficiente
		due := in.HookEventName == "PreCompact" || m.EventsSinceDigest >= cfg.ThrottleEvents
		if due {
			lk.Release() // solta antes de destacar o worker
			digest.SpawnDetached(sess.SessionID)
		}
	}
}

// maybeInject roda o resolvedor e injeta o additionalContext, se for o caso.
func maybeInject(in hookio.Input, selfID string, cfg config.Config) {
	if !resolve.HasIntent(in.Prompt) {
		return // 99% dos prompts: custo zero de token
	}
	out := resolve.Resolve(in.Prompt, selfID)
	switch out.Kind {
	case "winner":
		if resolve.IsDeep(in.Prompt) {
			// fetch profundo on-demand: conteúdo verbatim, orçamento maior
			hookio.EmitContext(render.DeepWinner(out.Winner, cfg.DeepBudgetBytes))
		} else {
			hookio.EmitContext(render.Winner(out.Winner, cfg.InjectBudgetBytes))
		}
	case "shortlist":
		hookio.EmitContext(render.Shortlist(out.Shortlist, cfg.InjectBudgetBytes))
	default:
		hookio.EmitContext(render.None(out.Live, cfg.InjectBudgetBytes))
	}
}

// resolveSelf descobre a sessão do hook: stdin → ppid → cwd único.
func resolveSelf(in hookio.Input) (sessions.Session, bool) {
	if in.SessionID != "" {
		if s, ok := sessions.ByID(in.SessionID); ok {
			return s, true
		}
		// stdin trouxe id mas o arquivo nativo ainda não tem: sintetiza mínimo
		return sessions.Session{SessionID: in.SessionID, Cwd: cwdOf(in)}, true
	}
	if s, ok := sessions.FindByPidCwd(os.Getppid(), cwdOf(in)); ok {
		return s, true
	}
	return sessions.Session{}, false
}

func cwdOf(in hookio.Input) string {
	if in.Cwd != "" {
		return in.Cwd
	}
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	wd, _ := os.Getwd()
	return wd
}

// ensureMeta cria/atualiza o meta a partir do arquivo nativo + diagnóstico.
func ensureMeta(sess sessions.Session, in hookio.Input) store.Meta {
	m, _ := store.ReadMeta(sess.SessionID)
	m.SessionID = sess.SessionID
	if sess.Name != "" {
		m.Name = sess.Name
	}
	if sess.Cwd != "" {
		m.Cwd = sess.Cwd
	} else if m.Cwd == "" {
		m.Cwd = cwdOf(in)
	}
	if sess.Pid != 0 {
		m.Pid = sess.Pid
	}
	if sess.Status != "" {
		m.Status = sess.Status
	}
	if sess.Version != "" {
		m.ClaudeVersion = sess.Version
	}
	m.LastParseOk = in.ParseOk
	m.MissingFields = in.MissingFields
	return m
}

// ---------------------------------------------------------------------------
// subcomandos de CLI
// ---------------------------------------------------------------------------

func cmdList() int {
	live, err := sessions.Scan(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	if len(live) == 0 {
		fmt.Println("nenhuma sessão viva.")
		return 0
	}
	for _, s := range live {
		m, _ := store.ReadMeta(s.SessionID)
		name := s.Name
		if name == "" {
			name = "(sem nome)"
		}
		fmt.Printf("%-28s %-8s %s\n", name, s.Status, s.Cwd)
		if m.CurrentSkill != "" || m.LastFact != "" {
			fmt.Printf("    skill=%s  fato=%s\n", m.CurrentSkill, m.LastFact)
		}
		if len(m.Topics) > 0 {
			fmt.Printf("    topics: %s\n", strings.Join(m.Topics, ", "))
		}
	}
	return 0
}

func cmdShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "uso: lookover show <name|sessionId>")
		return 2
	}
	id := lookupID(args[0])
	if id == "" {
		fmt.Fprintln(os.Stderr, "sessão não encontrada:", args[0])
		return 1
	}
	m, _ := store.ReadMeta(id)
	d := store.ReadDigest(id)
	fmt.Printf("# %s  (%s)\n", m.Name, m.Cwd)
	fmt.Printf("status=%s skill=%s stale=%v\n", m.Status, m.CurrentSkill, m.DigestStale)
	if len(m.Topics) > 0 {
		fmt.Println("topics:", strings.Join(m.Topics, ", "))
	}
	if d.Digest != "" {
		fmt.Println("\n## digest\n" + d.Digest)
	}
	fmt.Printf("\n## eventos (%d)\n", len(d.Events))
	for _, e := range d.Events {
		fmt.Printf("- [%s] %s %s %s\n", e.Kind, e.Text, e.Name, e.Summary)
	}
	return 0
}

// lookupID resolve um name ou sessionId para o sessionId canônico.
func lookupID(ref string) string {
	if s, ok := sessions.ByID(ref); ok {
		return s.SessionID
	}
	all, _ := sessions.Scan(false)
	for _, s := range all {
		if s.Name == ref {
			return s.SessionID
		}
	}
	return ""
}

func cmdCompact(args []string) int {
	if len(args) < 1 {
		return 2
	}
	if err := digest.Compact(args[0]); err != nil {
		logf("compact erro: %v", err)
		return 1
	}
	return 0
}

func cmdDoctor() int {
	cfg := config.Load()
	fmt.Println("lookover doctor")
	fmt.Printf("  store:        %s\n", paths.StoreDir())
	fmt.Printf("  settings:     %s\n", paths.SettingsFile())
	fmt.Printf("  LLM:          enabled=%v model=%s claude=%s\n", cfg.LLMEnabled, cfg.LLMModel, cfg.ClaudePath)
	fmt.Printf("  shadow:       %v\n", cfg.Shadow)
	if _, err := os.Stat(paths.DisabledFlag()); err == nil {
		fmt.Println("  KILL SWITCH:  ATIVO (remova", paths.DisabledFlag(), "pra religar)")
	}
	live, _ := sessions.Scan(true)
	fmt.Printf("  sessões vivas: %d\n", len(live))
	var stale, noParse int
	for _, s := range live {
		if m, ok := store.ReadMeta(s.SessionID); ok {
			if m.DigestStale {
				stale++
			}
			if !m.LastParseOk {
				noParse++
			}
			if cfg.TestedClaudeVersion != "" && m.ClaudeVersion != "" && m.ClaudeVersion != cfg.TestedClaudeVersion {
				fmt.Printf("  AVISO: sessão %s na versão %s (testada %s) — modo observa\n",
					short(s.SessionID), m.ClaudeVersion, cfg.TestedClaudeVersion)
			}
		}
	}
	fmt.Printf("  digests stale: %d  parse-falho: %d\n", stale, noParse)
	return 0
}

func cmdStatus() int {
	live, _ := sessions.Scan(true)
	covered := 0
	for _, s := range live {
		if _, ok := store.ReadMeta(s.SessionID); ok {
			covered++
		}
	}
	fmt.Printf("sessões vivas: %d   cobertas pelo lookover: %d\n", len(live), covered)
	return 0
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
