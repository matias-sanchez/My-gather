#!/usr/bin/env sh
# my-gather installer. Detects OS+arch, downloads the matching raw
# binary from the latest GitHub Release, verifies its SHA-256
# against the published SHA256SUMS, installs to $PREFIX (default
# $HOME/.local/bin) and prints the installed version.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | sh
#   curl -fsSL ... | VERSION=v0.3.0 sh          # pin to a specific tag
#   curl -fsSL ... | PREFIX=/usr/local/bin sh   # install to /usr/local/bin (sudo may be needed)
#
# No telemetry, no background work, no automatic updates.

set -eu

REPO="matias-sanchez/My-gather"
BIN="my-gather"

# Resolve OS.
case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) echo "my-gather: unsupported OS '$(uname -s)'. Supported: darwin, linux." >&2; exit 1 ;;
esac

# Resolve arch.
case "$(uname -m)" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64)  arch="amd64" ;;
  *) echo "my-gather: unsupported arch '$(uname -m)'. Supported: arm64, amd64." >&2; exit 1 ;;
esac

asset="${BIN}-${os}-${arch}"

# Resolve version: default to latest release, honour $VERSION override.
version="${VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n1)
  if [ -z "$version" ]; then
    echo "my-gather: could not resolve latest release tag from GitHub API." >&2
    exit 1
  fi
fi

prefix="${PREFIX:-$HOME/.local/bin}"
mkdir -p "$prefix"

url="https://github.com/${REPO}/releases/download/${version}/${asset}"
sums_url="https://github.com/${REPO}/releases/download/${version}/SHA256SUMS"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "my-gather: downloading ${asset} (${version})"
curl -fsSL "$url" -o "${tmp}/${asset}"

# Verify SHA-256 against the published SHA256SUMS.
if curl -fsSL "$sums_url" -o "${tmp}/SHA256SUMS" 2>/dev/null; then
  expected=$(grep "  ${asset}\$" "${tmp}/SHA256SUMS" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "${tmp}/${asset}" | awk '{print $1}')
    else
      actual=$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')
    fi
    if [ "$expected" != "$actual" ]; then
      echo "my-gather: checksum mismatch for ${asset}" >&2
      echo "  expected: ${expected}" >&2
      echo "  actual:   ${actual}" >&2
      exit 1
    fi
    echo "my-gather: checksum verified"
  else
    echo "my-gather: ${asset} not listed in SHA256SUMS; skipping verify" >&2
  fi
else
  echo "my-gather: SHA256SUMS not found for ${version}; skipping verify" >&2
fi

chmod +x "${tmp}/${asset}"
mv "${tmp}/${asset}" "${prefix}/${BIN}"

echo "my-gather: installed to ${prefix}/${BIN}"
"${prefix}/${BIN}" --version || true

case ":${PATH}:" in
  *":${prefix}:"*) ;;
  *) echo "my-gather: note — ${prefix} is not on your \$PATH. Add it with:" >&2
     echo "    export PATH=\"${prefix}:\$PATH\"" >&2 ;;
esac
