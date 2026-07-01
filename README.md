<p align="center">
  <img src="site/logo.png" alt="lookover" width="128">
</p>

<h1 align="center">lookover</h1>

<p align="center"><b><i>look over your shoulder</i></b> — into your other Claude Code terminals.</p>

<p align="center">
  Ask one terminal what <b>another Claude Code session</b> is doing.<br>
  Its context is injected automatically, just mention it.
</p>

<p align="center">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-3b82f6">
  <img alt="go" src="https://img.shields.io/badge/Go-1.21%2B-22d3ee">
  <img alt="claude code" src="https://img.shields.io/badge/Claude%20Code-hooks-6e9bff">
  <a href="https://espigah.github.io/lookover/"><img alt="site" src="https://img.shields.io/badge/site-espigah.github.io%2Flookover-0a0f1e"></a>
</p>

---

When you have Claude open in several terminals, each one only knows about itself, the Claude in
terminal A can't see terminal B. **lookover** closes that gap. Just mention another terminal, by its
name or by what it's doing, and it brings over what's happening there before Claude answers, like
glancing over your own shoulder at the session next to you.

```
you:  based on the slack-bridge-on-2 terminal, what was the last PR we reviewed?
you:  in the terminal where I handle on-call, why wasn't the xpto alert handled?
```

## How it works

- **It takes notes:** each terminal quietly keeps short notes on what it's doing, what you asked and
  what it's working on. Nothing heavy, and no full text is kept.
- **It finds the right one:** when you mention another terminal, it picks the one you mean, by its
  name or by what it's working on. If it's not sure, it offers you a short list to choose from.
- **It brings the context over:** that goes to Claude as background info (kept clearly separate, so
  it's never read as commands), and Claude answers already knowing what the other terminal did.

## Recent context, or the whole thing

| You write | What you get |
|---|---|
| *"what did terminal X do?"* | its **recent context**: what you asked there and what came back |
| *"show me the **full context** from terminal X"* | more of it, reaching further back, word for word |

Naming a terminal already brings its real context. Asking for more, with words like *"full context",
"what was written", "the whole thing", "in full"*, reaches further back.

## Install

**One line** (Linux, amd64/arm64):

```sh
curl -fsSL https://espigah.github.io/lookover/install.sh | bash
```

It grabs the latest release, drops the binary in `~/.local/bin`, and offers to turn it on.

Prefer a package or the raw binary? Download a `.deb`, `.rpm`, `.tar.gz` or the executable from the
[**releases page**](https://github.com/Espigah/lookover/releases/latest):

```sh
# Debian/Ubuntu
sudo dpkg -i lookover_*_linux_amd64.deb
# Fedora/RHEL
sudo rpm -i lookover_*_linux_amd64.rpm
```

From source (needs Go 1.21+):

```sh
git clone https://github.com/Espigah/lookover.git
cd lookover
go build -o ~/.local/bin/lookover ./cmd/lookover
```

Then turn it on:

```sh
lookover init        # shows the settings.json diff and only applies after you confirm
```

`init` registers 4 hooks (SessionStart, UserPromptSubmit, PostToolUse, Stop), **preserves** existing
hooks, makes a backup (`settings.json.bak`), registers already-open sessions (backfill) and offers
to install a primer in `CLAUDE.md`. Roll back: `lookover uninstall`.

Flags: `--llm` (enable the optional LLM compaction), `--shadow` (capture without injecting),
`--yes` (no prompt), `-g` (global).

## Commands

| command | what |
|---|---|
| `lookover list` | live sessions + skill/topics |
| `lookover show <name\|id>` | a session's digest |
| `lookover doctor` | health diagnostics |
| `lookover status` | local usage/adoption |
| `lookover uninstall` | remove the hooks |

## Stays out of your way

- **Invisible when unused:** if you're not asking about another terminal, it does nothing.
- **Never gets in the way:** it can't slow Claude down or break your session.
- **Safe by design:** whatever comes from another terminal is treated as background info, never as
  instructions.
- **Off whenever you want:** `touch ~/.claude/lookover/disabled` (or `LOOKOVER_DISABLED=1`) pauses it
  on the spot.
- **Nothing to manage:** no extra app or service running in the background.

## Development

```sh
go build ./...
go test ./...
```

Packages under `internal/`: `paths`, `config`, `sessions`, `store`, `hookio`, `capture`, `resolve`,
`render`, `digest`, `transcript`, `settings`.

## Privacy

Everything is local, in `~/.claude/lookover/`. Nothing leaves your machine, with the single
exception of the optional LLM compaction (off by default), which uses the `claude` binary you
already have installed.

## License

MIT.
