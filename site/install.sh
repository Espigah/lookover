#!/usr/bin/env bash
# lookover installer.
#   curl -fsSL https://espigah.github.io/lookover/install.sh | bash
#
# Downloads the latest lookover release, installs it to ~/.local/bin (no root),
# and offers to turn it on (lookover init).
set -euo pipefail

REPO="Espigah/lookover"
BIN="$HOME/.local/bin/lookover"

[ "$(uname -s)" = "Linux" ] || { echo "lookover currently supports Linux only."; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m) (linux amd64/arm64 for now)."; exit 1 ;;
esac
for t in curl tar; do command -v "$t" >/dev/null 2>&1 || { echo "missing required tool: $t"; exit 1; }; done

echo ">> finding the latest release"
ver="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -nE 's/.*"tag_name"[: ]+"v?([^"]+)".*/\1/p' | head -1)"
[ -n "$ver" ] || { echo "could not determine the latest version"; exit 1; }
echo ">> lookover v$ver (linux/$ARCH)"

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
base="https://github.com/$REPO/releases/download/v$ver"
curl -fsSL "$base/lookover_${ver}_linux_${ARCH}.tar.gz" -o "$tmp/lookover.tgz"
tar xzf "$tmp/lookover.tgz" -C "$tmp"
install -Dm755 "$tmp/lookover" "$BIN"
echo ">> installed to $BIN"

# make sure ~/.local/bin is on PATH for future shells
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;
  *)
    for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
      [ -f "$rc" ] || continue
      grep -q '.local/bin' "$rc" 2>/dev/null && continue
      printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
    done
    export PATH="$HOME/.local/bin:$PATH"
    echo ">> added ~/.local/bin to your PATH (open a new terminal if 'lookover' isn't found)"
    ;;
esac

echo
echo "lookover v$ver is installed."
# Turning it on edits ~/.claude/settings.json, so it asks first. Read answers
# from the terminal even when this script is piped from curl.
if [ -e /dev/tty ]; then
  printf "Turn it on now (runs 'lookover init')? [y/N] "
  read -r ans < /dev/tty || ans=""
  case "$ans" in
    y|Y) "$BIN" init < /dev/tty ;;
    *)   echo "ok, run it yourself anytime:  lookover init" ;;
  esac
else
  echo "next step:  lookover init"
fi
