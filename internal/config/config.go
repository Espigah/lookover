// Package config carrega/grava as opções do lookover escolhidas no init.
package config

import (
	"encoding/json"
	"os"

	"lookover/internal/paths"
)

// Config são as opções persistidas em ~/.claude/lookover/config.json.
type Config struct {
	// LLMEnabled liga a compactação por LLM (off por default — economia de tokens).
	LLMEnabled bool `json:"llmEnabled"`
	// ClaudePath é o caminho absoluto do binário claude resolvido no init.
	ClaudePath string `json:"claudePath"`
	// LLMModel é o modelo usado na compactação (default haiku, o mais barato).
	LLMModel string `json:"llmModel"`
	// ThrottleEvents / ThrottleMinutes controlam o debounce da compactação.
	ThrottleEvents  int `json:"throttleEvents"`
	ThrottleMinutes int `json:"throttleMinutes"`
	// InjectBudgetBytes limita o tamanho do additionalContext injetado.
	InjectBudgetBytes int `json:"injectBudgetBytes"`
	// DeepBudgetBytes limita o fetch profundo (conteúdo verbatim on-demand).
	DeepBudgetBytes int `json:"deepBudgetBytes"`
	// Shadow captura e loga sem injetar (validação pós-install).
	Shadow bool `json:"shadow"`
	// OptOutDirs: cwds (prefixos) onde a captura é desligada.
	OptOutDirs []string `json:"optOutDirs"`
	// TestedClaudeVersion é a versão do CC validada por último. A compatibilidade
	// é avaliada só por feature (major.minor), então patch (2.1.n) não conta.
	TestedClaudeVersion string `json:"testedClaudeVersion"`
}

// Default devolve a config padrão (LLM off, budgets sensatos).
func Default() Config {
	return Config{
		LLMEnabled:        false,
		LLMModel:          "claude-haiku-4-5-20251001",
		ThrottleEvents:    20,
		ThrottleMinutes:   5,
		InjectBudgetBytes: 6 * 1024,
		DeepBudgetBytes:   24 * 1024,
		Shadow:            false,
	}
}

// Load lê a config; se não existir, devolve Default (sem erro).
func Load() Config {
	c := Default()
	b, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c) // tolerante: campos faltando ficam no default
	return c
}

// Save grava a config de forma atômica.
func Save(c Config) error {
	if err := paths.EnsureStore(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := paths.ConfigFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, paths.ConfigFile())
}
