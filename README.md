<p align="center">
  <img src="site/logo.png" alt="lookover" width="128">
</p>

<h1 align="center">lookover</h1>

<p align="center">
  Pergunte de um terminal o que <b>outra sessão do Claude Code</b> está fazendo.<br>
  O contexto da outra sessão é injetado automaticamente, basta mencioná-la.
</p>

<p align="center">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-3b82f6">
  <img alt="go" src="https://img.shields.io/badge/Go-1.21%2B-22d3ee">
  <img alt="claude code" src="https://img.shields.io/badge/Claude%20Code-hooks-6e9bff">
  <a href="https://espigah.github.io/lookover/"><img alt="site" src="https://img.shields.io/badge/site-espigah.github.io%2Flookover-0a0f1e"></a>
</p>

---

Cada sessão do Claude Code roda isolada: o Claude do terminal A não enxerga o terminal B. O
**lookover** fecha esse buraco. Você referencia outro terminal, por nome ou por descrição, e o
contexto dele entra no seu prompt antes do Claude responder.

```
você:  com base no terminal slack-bridge-on-2, qual o último PR analisado?
você:  no terminal onde trato o oncall, por que o alerta xpto não foi tratado?
```

## Como funciona

- **Captura (100% Go, zero token):** um hook leve grava um resumo enxuto de cada sessão
  (tópicos extraídos do prompt, skill em uso, fatos) em `~/.claude/lookover/<sessionId>.*.json`.
  Nunca guarda conteúdo verbatim.
- **Resolve:** ao detectar que você mencionou outro terminal, varre `~/.claude/sessions/` (nativo)
  + os metas e ranqueia por score lexical (nome > tópicos > repo > fato > skill). Acha a sessão
  certa, devolve uma shortlist se houver empate, ou diz quais estão abertas.
- **Injeta:** o contexto entra como `additionalContext`, dentro de um envelope anti-injection
  (marcado como dado, não instrução) e sanitizado.

## Dois níveis de contexto

| Você escreve | O que entra |
|---|---|
| *"o que o terminal X fez?"* | **resumo**: tópicos + pedidos (barato, identificação) |
| *"me mostra o **texto completo** do terminal X"* | **conteúdo verbatim** lido do transcript real da sessão |

O modo profundo é disparado por cues como *"texto completo", "na íntegra", "conteúdo",
"o que foi escrito", "verbatim", "o parágrafo", "mostra tudo"*.

## Instalar

Requer Go 1.21+ e o Claude Code.

```sh
git clone https://github.com/Espigah/lookover.git
cd lookover
go build -o ~/.local/bin/lookover ./cmd/lookover
lookover init        # mostra o diff do settings.json e só aplica após confirmar
```

O `init` registra 4 hooks (SessionStart, UserPromptSubmit, PostToolUse, Stop), **preserva** hooks
existentes, faz backup (`settings.json.bak`), registra as sessões já abertas (backfill) e oferece
instalar um primer no `CLAUDE.md`. Reverter: `lookover uninstall`.

Flags: `--llm` (liga a compactação opcional por LLM), `--shadow` (captura sem injetar),
`--yes` (sem prompt), `-g` (global).

## Comandos

| comando | o quê |
|---|---|
| `lookover list` | sessões vivas + skill/tópicos |
| `lookover show <nome\|id>` | digest de uma sessão |
| `lookover doctor` | diagnóstico de saúde |
| `lookover status` | uso/adoção local |
| `lookover uninstall` | remove os hooks |

## Robustez e segurança

- **Custo zero no uso normal:** sem referência a outro terminal, o hook não injeta nada.
- **Nunca quebra o Claude:** recover de panic, watchdog anti-hang, sempre exit 0.
- **Anti prompt-injection:** o resumo nunca guarda output verbatim, só fatos derivados; injeção
  sempre envelopada e sanitizada.
- **Kill switch:** `touch ~/.claude/lookover/disabled` (ou `LOOKOVER_DISABLED=1`) desliga na hora.
- **Sem servidor:** um arquivo por sessão, escrita atômica, resolução por varredura local.

## Desenvolvimento

```sh
go build ./...
go test ./...
```

Pacotes em `internal/`: `paths`, `config`, `sessions`, `store`, `hookio`, `capture`, `resolve`,
`render`, `digest`, `transcript`, `settings`.

## Privacidade

Tudo é local, em `~/.claude/lookover/`. Nada sai pra rede, com a única exceção da compactação
opcional por LLM (off por padrão), que usa o `claude` que você já tem instalado.

## Licença

MIT.
