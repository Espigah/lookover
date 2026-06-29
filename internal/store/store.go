// Package store lê/grava os arquivos por sessão (meta + digest). Single-writer
// por sessão: escrita atômica via tmp+rename, sem índice compartilhado.
package store

import (
	"encoding/json"
	"os"
	"time"

	"lookover/internal/paths"
)

// MaxEventsBytes é o teto do ring buffer de eventos (evicção do mais antigo).
const MaxEventsBytes = 64 * 1024

// Limites por evento, capados na origem (segurança + economia).
const (
	MaxPromptChars  = 500
	MaxToolChars    = 200
	MaxFactChars    = 200
	MaxTopics       = 12
)

// Meta é o arquivo pequeno varrido na resolução.
type Meta struct {
	SessionID         string   `json:"sessionId"`
	Name              string   `json:"name,omitempty"`
	Cwd               string   `json:"cwd"`
	Pid               int      `json:"pid"`
	Status            string   `json:"status,omitempty"`
	CurrentSkill      string   `json:"currentSkill,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	LastFact          string   `json:"lastFact,omitempty"`
	UpdatedAt         string   `json:"updatedAt"`
	DigestGeneratedAt string   `json:"digestGeneratedAt,omitempty"`
	DigestStale       bool     `json:"digestStale"`
	// diagnóstico (resiliência)
	LastParseOk   bool     `json:"lastParseOk"`
	ClaudeVersion string   `json:"claudeVersion,omitempty"`
	MissingFields []string `json:"missingFields,omitempty"`
	// throttle da compactação LLM
	EventsSinceDigest int    `json:"eventsSinceDigest"`
}

// Event é uma entrada do digest (sempre derivada/capada, nunca verbatim).
type Event struct {
	Ts      string `json:"ts"`
	Kind    string `json:"kind"` // prompt | tool | skill | fact
	Text    string `json:"text,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Name    string `json:"name,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// Digest é o corpo maior, lido só da sessão resolvida.
type Digest struct {
	Events []Event `json:"events"`
	Digest string  `json:"digest,omitempty"`
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339) }

func writeAtomic(path string, v any) error {
	if err := paths.EnsureStore(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadMeta lê o meta de uma sessão (ok=false se não existe).
func ReadMeta(sessionID string) (Meta, bool) {
	var m Meta
	b, err := os.ReadFile(paths.MetaFile(sessionID))
	if err != nil {
		return m, false
	}
	if json.Unmarshal(b, &m) != nil {
		return m, false
	}
	return m, true
}

// WriteMeta grava o meta atomicamente, carimbando updatedAt.
func WriteMeta(m Meta) error {
	m.UpdatedAt = nowRFC()
	return writeAtomic(paths.MetaFile(m.SessionID), m)
}

// ReadDigest lê o corpo de uma sessão (zero-value se não existe).
func ReadDigest(sessionID string) Digest {
	var d Digest
	b, err := os.ReadFile(paths.DigestFile(sessionID))
	if err != nil {
		return d
	}
	_ = json.Unmarshal(b, &d)
	return d
}

// WriteDigest grava o corpo atomicamente.
func WriteDigest(sessionID string, d Digest) error {
	return writeAtomic(paths.DigestFile(sessionID), d)
}

// AppendEvent adiciona um evento e aplica o ring buffer por bytes.
func AppendEvent(sessionID string, e Event) error {
	e.Ts = nowRFC()
	d := ReadDigest(sessionID)
	d.Events = append(d.Events, e)
	d.Events = trimByBytes(d.Events, MaxEventsBytes)
	return WriteDigest(sessionID, d)
}

// trimByBytes descarta os eventos mais antigos até caber no orçamento.
func trimByBytes(evs []Event, budget int) []Event {
	for len(evs) > 1 {
		b, _ := json.Marshal(evs)
		if len(b) <= budget {
			break
		}
		evs = evs[1:]
	}
	return evs
}
