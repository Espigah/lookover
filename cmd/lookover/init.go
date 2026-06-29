package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lookover/internal/config"
	"lookover/internal/paths"
	"lookover/internal/sessions"
	"lookover/internal/settings"
	"lookover/internal/store"
)

const primerStart = "<!-- lookover:start -->"
const primerEnd = "<!-- lookover:end -->"
const primerBody = `## Contexto entre terminais (lookover)
Você pode referenciar outra sessão de terminal do Claude por **nome** ou **descrição**
(ex: "com base no terminal slack-bridge-on-2, ..." ou "o terminal onde trato o oncall").
O contexto da outra sessão é injetado automaticamente — basta mencioná-la naturalmente.`

func cmdInit(args []string) int {
	var llm, shadow, yes bool
	for _, a := range args {
		switch a {
		case "--llm":
			llm = true
		case "--shadow":
			shadow = true
		case "--yes", "-y":
			yes = true
		case "-g", "--global":
			// global é o único modo suportado hoje; aceito por compat com rtk
		}
	}

	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "não resolvi o caminho do binário:", err)
		return 1
	}
	bin, _ = filepath.Abs(bin)

	claudePath, _ := exec.LookPath("claude")
	ccVersion := detectCCVersion()

	cfg := config.Default()
	cfg.Shadow = shadow
	cfg.LLMEnabled = llm
	cfg.ClaudePath = claudePath
	cfg.TestedClaudeVersion = ccVersion

	// dry-run obrigatório
	preview, changed, err := settings.Preview(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro lendo settings.json:", err)
		return 1
	}

	fmt.Println("== lookover init (dry-run) ==")
	fmt.Println("binário:        ", bin)
	fmt.Println("settings.json:  ", paths.SettingsFile())
	fmt.Println("store:          ", paths.StoreDir())
	fmt.Printf("LLM:             enabled=%v model=%s claude=%q\n", cfg.LLMEnabled, cfg.LLMModel, cfg.ClaudePath)
	if cfg.LLMEnabled && cfg.ClaudePath == "" {
		fmt.Println("AVISO: --llm pedido mas 'claude' não está no PATH; cairá no determinístico.")
	}
	fmt.Println("shadow:         ", cfg.Shadow)
	fmt.Println("versão CC:      ", ccVersion)
	if !changed {
		fmt.Println("\nhooks já registrados — nada a alterar no settings.json.")
	} else {
		fmt.Println("\nsettings.json resultante:")
		fmt.Println(preview)
	}

	if !yes {
		fmt.Print("\naplicar? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "y" {
			fmt.Println("abortado.")
			return 0
		}
	}

	// aplica
	if _, err := settings.Apply(bin); err != nil {
		fmt.Fprintln(os.Stderr, "falha aplicando settings:", err)
		return 1
	}
	if err := config.Save(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "falha salvando config:", err)
		return 1
	}
	backfill()
	installPrimer(yes)

	fmt.Println("\nok. rodando doctor:")
	return cmdDoctor()
}

// detectCCVersion lê a versão do CC do primeiro arquivo de sessão.
func detectCCVersion() string {
	all, _ := sessions.Scan(false)
	for _, s := range all {
		if s.Version != "" {
			return s.Version
		}
	}
	return ""
}

// backfill gera meta pras sessões já vivas (bootstrapping retroativo).
func backfill() {
	live, _ := sessions.Scan(true)
	n := 0
	for _, s := range live {
		if _, ok := store.ReadMeta(s.SessionID); ok {
			continue
		}
		m := store.Meta{
			SessionID: s.SessionID, Name: s.Name, Cwd: s.Cwd, Pid: s.Pid,
			Status: s.Status, ClaudeVersion: s.Version, LastParseOk: true,
		}
		if store.WriteMeta(m) == nil {
			n++
		}
	}
	fmt.Printf("backfill: %d sessões pré-existentes registradas.\n", n)
}

// installPrimer instala o bloco gerenciado no ~/.claude/CLAUDE.md (disseminação).
func installPrimer(autoYes bool) {
	claudeMd := filepath.Join(paths.ClaudeDir(), "CLAUDE.md")
	existing, _ := os.ReadFile(claudeMd)
	if strings.Contains(string(existing), primerStart) {
		return // já instalado
	}
	if !autoYes {
		fmt.Print("instalar o primer do lookover no CLAUDE.md (recomendado p/ disseminação)? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "y" {
			return
		}
	}
	block := "\n" + primerStart + "\n" + primerBody + "\n" + primerEnd + "\n"
	f, err := os.OpenFile(claudeMd, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(block)
	fmt.Println("primer instalado em", claudeMd)
}

func cmdUninstall() int {
	bin, _ := os.Executable()
	bin, _ = filepath.Abs(bin)
	changed, err := settings.Remove(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	if changed {
		fmt.Println("hooks do lookover removidos do settings.json (backup em settings.json.bak).")
	} else {
		fmt.Println("nenhum hook do lookover encontrado no settings.json.")
	}
	fmt.Println("nota: os dados em", paths.StoreDir(), "foram mantidos. Remova manualmente se quiser.")
	return 0
}
