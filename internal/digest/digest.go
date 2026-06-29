// Package digest gera o campo "digest" de uma sessão. Determinístico por
// default (zero token); compactação por LLM é opcional, destacada e degradável.
package digest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"lookover/internal/config"
	"lookover/internal/store"
)

// Deterministic monta um resumo curto sem LLM a partir dos eventos.
func Deterministic(sessionID string) string {
	d := store.ReadDigest(sessionID)
	m, _ := store.ReadMeta(sessionID)
	var b strings.Builder
	if len(m.Topics) > 0 {
		fmt.Fprintf(&b, "tópicos: %s\n", strings.Join(m.Topics, ", "))
	}
	if m.CurrentSkill != "" {
		fmt.Fprintf(&b, "skill atual: %s\n", m.CurrentSkill)
	}
	evs := d.Events
	if len(evs) > 15 {
		evs = evs[len(evs)-15:]
	}
	for _, e := range evs {
		switch e.Kind {
		case "prompt":
			fmt.Fprintf(&b, "- pediu: %s\n", e.Text)
		case "tool":
			fmt.Fprintf(&b, "- %s\n", e.Summary)
		case "skill":
			fmt.Fprintf(&b, "- skill: %s\n", e.Name)
		case "fact":
			fmt.Fprintf(&b, "- fato: %s\n", e.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

// Compact regrava o digest da sessão. Se o LLM estiver ligado e disponível,
// usa-o sobre o delta; senão cai pro determinístico (marcando stale).
func Compact(sessionID string) error {
	cfg := config.Load()
	lk, _ := store.Acquire(sessionID)
	defer lk.Release()

	d := store.ReadDigest(sessionID)
	m, ok := store.ReadMeta(sessionID)
	if !ok {
		return nil
	}

	if cfg.LLMEnabled && cfg.ClaudePath != "" {
		if out, err := runLLM(cfg, buildPayload(d)); err == nil && strings.TrimSpace(out) != "" {
			d.Digest = strings.TrimSpace(out)
			m.DigestStale = false
			m.DigestGeneratedAt = time.Now().UTC().Format(time.RFC3339)
			m.EventsSinceDigest = 0
			_ = store.WriteDigest(sessionID, d)
			return store.WriteMeta(m)
		}
		// LLM falhou: degrada, mantém digest anterior, marca stale.
		m.DigestStale = true
		return store.WriteMeta(m)
	}

	// Sem LLM: digest determinístico (não é "stale" pq é o piso confiável).
	d.Digest = Deterministic(sessionID)
	m.DigestStale = false
	m.DigestGeneratedAt = time.Now().UTC().Format(time.RFC3339)
	m.EventsSinceDigest = 0
	_ = store.WriteDigest(sessionID, d)
	return store.WriteMeta(m)
}

// buildPayload monta o delta enviado ao LLM: resumo anterior + eventos recentes.
func buildPayload(d store.Digest) string {
	var b strings.Builder
	if strings.TrimSpace(d.Digest) != "" {
		b.WriteString("Resumo anterior:\n")
		b.WriteString(d.Digest)
		b.WriteString("\n\nEventos novos:\n")
	} else {
		b.WriteString("Eventos:\n")
	}
	evs := d.Events
	if len(evs) > 40 {
		evs = evs[len(evs)-40:]
	}
	for _, e := range evs {
		switch e.Kind {
		case "prompt":
			fmt.Fprintf(&b, "- pediu: %s\n", e.Text)
		case "tool":
			fmt.Fprintf(&b, "- %s\n", e.Summary)
		case "skill":
			fmt.Fprintf(&b, "- skill: %s\n", e.Name)
		case "fact":
			fmt.Fprintf(&b, "- fato: %s\n", e.Text)
		}
	}
	return b.String()
}

// runLLM chama `claude -p` com timeout, passando só o delta (digest+eventos).
func runLLM(cfg config.Config, payload string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prompt := "Resuma o estado desta sessão de terminal em ~10 bullets curtos " +
		"(o que está sendo feito, decisões, últimos resultados como PRs/alertas/IDs). " +
		"Responda só os bullets, sem preâmbulo.\n\n" + payload
	cmd := exec.CommandContext(ctx, cfg.ClaudePath, "-p", "--model", cfg.LLMModel, prompt)
	out, err := cmd.Output()
	return string(out), err
}

// SpawnDetached dispara `lookover compact <id>` num processo destacado e NÃO espera
// (não bloqueia o fechamento do turno).
func SpawnDetached(sessionID string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "compact", sessionID)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}
