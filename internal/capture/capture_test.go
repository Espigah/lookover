package capture

import (
	"testing"

	"lookover/internal/store"
)

func TestTopicsSkipsStopwords(t *testing.T) {
	got := Topics("com base no terminal, revisa o PR 995 do unique index", 12)
	want := map[string]bool{"revisa": true, "995": true, "unique": true, "index": true}
	for _, g := range got {
		delete(want, g)
	}
	if len(want) != 0 {
		t.Fatalf("faltaram topics distintivos: %v (got=%v)", want, got)
	}
	for _, g := range got {
		if g == "terminal" || g == "com" || g == "base" {
			t.Fatalf("stopword vazou em topics: %q", g)
		}
	}
}

func TestFactDetectsPR(t *testing.T) {
	if f := Fact("revisa o PR 995"); f == "" {
		t.Fatal("não detectou nº de PR")
	}
	if f := Fact("sem numero aqui"); f != "" {
		t.Fatalf("falso positivo de fato: %q", f)
	}
}

func TestToolSummaryCapped(t *testing.T) {
	long := ""
	for i := 0; i < 1000; i++ {
		long += "x"
	}
	s := ToolSummary("Bash", map[string]any{"command": long})
	if len([]rune(s)) > store.MaxToolChars+1 {
		t.Fatalf("resumo não foi capado: %d", len([]rune(s)))
	}
}

func TestMergeTopicsRecencyAndCap(t *testing.T) {
	got := MergeTopics([]string{"a", "b", "c"}, []string{"x", "b"}, 3)
	if len(got) != 3 || got[0] != "x" || got[1] != "b" {
		t.Fatalf("merge errado: %v", got)
	}
}
