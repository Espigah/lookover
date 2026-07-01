package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lookover/internal/store"
)

// setup cria um CLAUDE_CONFIG_DIR temporário com sessões vivas (usa o pid do
// próprio processo de teste, que está vivo) e seus metas.
func setup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	sdir := filepath.Join(dir, "sessions")
	os.MkdirAll(sdir, 0o755)
	pid := os.Getpid()

	writeSess := func(file, id, cwd, name string) {
		s := map[string]any{"pid": pid, "sessionId": id, "cwd": cwd, "name": name, "status": "busy", "version": "2.1.195"}
		b, _ := json.Marshal(s)
		os.WriteFile(filepath.Join(sdir, file+".json"), b, 0o644)
	}
	writeSess("a", "sess-A", "/home/dev/repoA", "slack-bridge-on-2")
	writeSess("b", "sess-B", "/home/dev/oncall-work", "oncall-terminal")

	store.WriteMeta(store.Meta{SessionID: "sess-A", Name: "slack-bridge-on-2", Cwd: "/home/dev/repoA",
		Topics: []string{"revisa", "995", "unique", "index"}, LastFact: "PR #995 mencionado"})
	store.WriteMeta(store.Meta{SessionID: "sess-B", Name: "oncall-terminal", Cwd: "/home/dev/oncall-work",
		Topics: []string{"alerta", "chargeback", "oncall"}, CurrentSkill: "fs:oncall-patrolman"})
}

func TestHasIntent(t *testing.T) {
	if !HasIntent("com base no terminal slack-bridge-on-2, ...") {
		t.Fatal("devia detectar intenção")
	}
	if HasIntent("adiciona um teste unitário nessa função") {
		t.Fatal("falso positivo de intenção")
	}
}

func TestResolveExplicitName(t *testing.T) {
	setup(t)
	out := Resolve("com base no terminal slack-bridge-on-2, qual o ultimo PR?", "sess-B")
	if out.Kind != "winner" || out.Winner.Sess.SessionID != "sess-A" {
		t.Fatalf("esperava winner sess-A, veio %s / %+v", out.Kind, out.Winner.Sess.SessionID)
	}
}

func TestResolveFuzzyByTopicsAndSkill(t *testing.T) {
	setup(t)
	out := Resolve("com base no terminal onde trato o oncall, por que o alerta de chargeback nao foi tratado?", "sess-A")
	if out.Kind != "winner" || out.Winner.Sess.SessionID != "sess-B" {
		t.Fatalf("esperava winner sess-B, veio %s / %+v", out.Kind, out.Winner.Sess.SessionID)
	}
}

func TestResolveNoneOnUnknownReference(t *testing.T) {
	setup(t)
	out := Resolve("com base no terminal do financeiro-xyz-inexistente, faz algo", "sess-A")
	if out.Kind != "none" {
		t.Fatalf("esperava none (a palavra 'terminal' não pode casar com -terminal), veio %s -> %v",
			out.Kind, out.Winner.Sess.Name)
	}
}

func TestWeakGenericMatchIsNone(t *testing.T) {
	setup(t)
	// "index" é um único tópico de A; match fraco (score 2) não deve injetar.
	out := Resolve("mexi no index hoje, e no terminal?", "sess-B")
	if out.Kind != "none" {
		t.Fatalf("match fraco de 1 tópico deveria ser none, veio %s", out.Kind)
	}
}

func TestSelfExcluded(t *testing.T) {
	setup(t)
	// A referenciando o próprio nome não deve resolver A
	out := Resolve("com base no terminal slack-bridge-on-2, ...", "sess-A")
	if out.Kind == "winner" && out.Winner.Sess.SessionID == "sess-A" {
		t.Fatal("não devia resolver a própria sessão")
	}
}
