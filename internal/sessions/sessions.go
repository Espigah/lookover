// Package sessions lê a pasta nativa ~/.claude/sessions/ (um JSON por sessão,
// nomeado pelo pid) — a fonte mais estável de descoberta/liveness de sessões.
package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"lookover/internal/paths"
)

// Session espelha os campos do arquivo nativo do Claude Code. `name` é opcional.
type Session struct {
	Pid       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Entrypoint string `json:"entrypoint"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	StartedAt int64  `json:"startedAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Alive devolve true se o processo pid ainda existe (sinal 0).
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Em linux, Kill com sinal 0 não envia nada: só testa existência/permissão.
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// FeatureVersion reduz uma versão do Claude Code (ex: "2.1.195") à sua versão
// de feature "2.1". A compatibilidade do lookover é avaliada SÓ por major.minor:
// mudanças de patch (o terceiro número) não contam, "2.1.n" sempre funciona
// igual e o contexto continua sendo injetado normalmente.
func FeatureVersion(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// Scan lê todas as sessões. onlyLive filtra processos mortos.
func Scan(onlyLive bool) ([]Session, error) {
	dir := paths.SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(b, &s) != nil || s.SessionID == "" {
			continue
		}
		if onlyLive && !Alive(s.Pid) {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// ByID procura uma sessão pelo sessionId.
func ByID(id string) (Session, bool) {
	all, err := Scan(false)
	if err != nil {
		return Session{}, false
	}
	for _, s := range all {
		if s.SessionID == id {
			return s, true
		}
	}
	return Session{}, false
}

// FindByPidCwd é o fallback de resolução de sessionId quando o stdin do hook
// não traz session_id (resiliência à instabilidade da interface).
func FindByPidCwd(pid int, cwd string) (Session, bool) {
	all, err := Scan(false)
	if err != nil {
		return Session{}, false
	}
	for _, s := range all {
		if pid != 0 && s.Pid == pid {
			return s, true
		}
	}
	// fallback mais fraco: única sessão viva naquele cwd
	var match []Session
	for _, s := range all {
		if cwd != "" && s.Cwd == cwd && Alive(s.Pid) {
			match = append(match, s)
		}
	}
	if len(match) == 1 {
		return match[0], true
	}
	return Session{}, false
}
