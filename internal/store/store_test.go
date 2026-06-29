package store

import (
	"encoding/json"
	"testing"
)

func TestTrimByBytesEvictsOldest(t *testing.T) {
	var evs []Event
	for i := 0; i < 100; i++ {
		evs = append(evs, Event{Kind: "tool", Summary: "comando bem comprido pra ocupar bytes " + string(rune('A'+i%26))})
	}
	trimmed := trimByBytes(evs, 1024)
	b, _ := json.Marshal(trimmed)
	if len(b) > 1024 {
		t.Fatalf("não respeitou o orçamento: %d bytes", len(b))
	}
	// deve manter os MAIS RECENTES (último evento preservado)
	if trimmed[len(trimmed)-1].Summary != evs[len(evs)-1].Summary {
		t.Fatal("evicção tirou o evento mais recente em vez do mais antigo")
	}
}

func TestRingBufferRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	id := "sess-test"
	if err := AppendEvent(id, Event{Kind: "prompt", Text: "oi"}); err != nil {
		t.Fatal(err)
	}
	d := ReadDigest(id)
	if len(d.Events) != 1 || d.Events[0].Text != "oi" {
		t.Fatalf("roundtrip falhou: %+v", d)
	}
}
