package sessions

import "testing"

func TestFeatureVersion(t *testing.T) {
	cases := map[string]string{
		"2.1.195": "2.1",
		"2.1.200": "2.1",
		"2.1":     "2.1",
		"2.2.0":   "2.2",
		"3.0.1":   "3.0",
		"2":       "2",
		"":        "",
	}
	for in, want := range cases {
		if got := FeatureVersion(in); got != want {
			t.Errorf("FeatureVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPatchIsCompatible(t *testing.T) {
	// mesmo feature (patch diferente) => compatível
	if FeatureVersion("2.1.195") != FeatureVersion("2.1.999") {
		t.Fatal("patch diferente dentro de 2.1 deveria ser compatível")
	}
	// feature diferente => incompatível (merece atenção)
	if FeatureVersion("2.1.195") == FeatureVersion("2.2.0") {
		t.Fatal("mudança de feature (2.1 -> 2.2) não deveria ser compatível")
	}
}
