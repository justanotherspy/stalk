#!/usr/bin/env bash
# go-template installer — download a prebuilt release binary instead of compiling.
#
# Fetches the release archive that matches the host OS/arch from GitHub
# Releases, verifies it against checksums.txt (fail closed on mismatch), and
# installs the binary into an on-PATH directory. This is the fast alternative to
# `go install github.com/justanotherspy/go-template/cmd/go-template@latest`.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/justanotherspy/go-template/main/install.sh | bash
#
# Environment overrides (PREFIX is the binary name uppercased, e.g. GO_TEMPLATE):
#   <PREFIX>_VERSION       release to install, e.g. v0.2.0 (default: latest release)
#   <PREFIX>_INSTALL_DIR   target directory (default: /usr/local/bin, else ~/.local/bin)
#   GITHUB_TOKEN/GH_TOKEN  used (if set) to authenticate GitHub API calls
set -euo pipefail

REPO="justanotherspy/go-template"
BINARY="go-template"

# Override env vars track the binary name so repos generated from this template
# work without edits: PREFIX is BINARY uppercased, non-alphanumerics → '_'.
PREFIX="$(printf '%s' "$BINARY" | tr '[:lower:]' '[:upper:]' | tr -c 'A-Z0-9' '_')"
PREFIX="${PREFIX%_}"
_version_var="${PREFIX}_VERSION"
_dir_var="${PREFIX}_INSTALL_DIR"
WANT_VERSION="${!_version_var:-}"
WANT_DIR="${!_dir_var:-}"

# Temp dir for downloads; cleaned up by the EXIT trap. Declared at file scope so
# it stays in scope when the trap fires (after main returns) under `set -u`.
tmpd=""

log() { echo "${BINARY}-install: $*" >&2; }
die() { log "$*"; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1; }

fetch() { # url dest
  if need_cmd curl; then
    curl -fsSL --connect-timeout 10 --max-time 120 --retry 3 -o "$2" "$1"
  elif need_cmd wget; then
    wget -q --timeout=120 -O "$2" "$1"
  else
    die "need curl or wget to download"
  fi
}

# fetch_stdout url -> prints body (used for the GitHub API, with optional auth)
fetch_stdout() {
  local url="$1" auth=()
  local token="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
  if [ -n "$token" ]; then auth=(-H "Authorization: Bearer $token"); fi
  if need_cmd curl; then
    curl -fsSL --connect-timeout 10 --max-time 60 --retry 3 "${auth[@]}" "$url"
  elif need_cmd wget; then
    local hdr=()
    if [ -n "$token" ]; then hdr=(--header="Authorization: Bearer $token"); fi
    wget -q --timeout=60 "${hdr[@]}" -O - "$url"
  else
    die "need curl or wget to download"
  fi
}

sha256_of() { # file -> prints hash
  if need_cmd sha256sum; then sha256sum "$1" | awk '{print $1}'
  elif need_cmd shasum; then shasum -a 256 "$1" | awk '{print $1}'
  else return 1; fi
}

# Resolve the release tag (e.g. v0.2.0). Honor <PREFIX>_VERSION, else ask the API
# for the latest non-draft, non-prerelease release.
resolve_tag() {
  if [ -n "$WANT_VERSION" ]; then
    case "$WANT_VERSION" in v*) echo "$WANT_VERSION" ;; *) echo "v$WANT_VERSION" ;; esac
    return 0
  fi
  local api="https://api.github.com/repos/${REPO}/releases/latest"
  local tag
  tag="$(fetch_stdout "$api" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  [ -n "$tag" ] || die "could not resolve latest release tag (set ${PREFIX}_VERSION)"
  echo "$tag"
}

# Map uname -> the goreleaser os/arch used in the asset names.
detect_os() {
  case "$(uname -s)" in
    Linux)                 echo linux ;;
    Darwin)                echo darwin ;;
    MINGW*|MSYS*|CYGWIN*)  echo windows ;;
    *) die "unsupported OS $(uname -s)" ;;
  esac
}
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) die "unsupported arch $(uname -m)" ;;
  esac
}

# Pick an install dir: explicit override, then a writable /usr/local/bin, else
# ~/.local/bin (created if needed).
pick_install_dir() {
  if [ -n "$WANT_DIR" ]; then
    echo "$WANT_DIR"; return 0
  fi
  if [ -w /usr/local/bin ] 2>/dev/null; then
    echo /usr/local/bin; return 0
  fi
  echo "$HOME/.local/bin"
}

main() {
  local tag version os arch ext binname archive base
  tag="$(resolve_tag)"
  version="${tag#v}"
  os="$(detect_os)"
  arch="$(detect_arch)"

  if [ "$os" = "windows" ]; then ext="zip"; binname="${BINARY}.exe"; else ext="tar.gz"; binname="$BINARY"; fi

  # Asset names follow .goreleaser.yaml: <binary>_<version>_<os>_<arch>.<ext>
  archive="${BINARY}_${version}_${os}_${arch}.${ext}"
  base="https://github.com/${REPO}/releases/download/${tag}"

  tmpd="$(mktemp -d)"
  trap 'rm -rf "$tmpd"' EXIT

  log "downloading $archive ($tag) ..."
  fetch "$base/$archive" "$tmpd/$archive" || die "download failed ($base/$archive)"

  # Verify against checksums.txt (fail closed on mismatch).
  fetch "$base/checksums.txt" "$tmpd/checksums.txt" || die "could not download checksums.txt"
  local expected got
  expected="$(awk -v f="$archive" '$2==f {print $1}' "$tmpd/checksums.txt")"
  [ -n "$expected" ] || die "$archive not listed in checksums.txt"
  got="$(sha256_of "$tmpd/$archive")" || die "no sha256 tool (sha256sum/shasum) available to verify download"
  [ "$got" = "$expected" ] || die "checksum mismatch for $archive (expected $expected, got $got)"
  log "checksum verified"

  # Extract the archive (binary plus bundled man page / completions).
  case "$ext" in
    tar.gz) tar -xzf "$tmpd/$archive" -C "$tmpd" ;;
    zip)
      need_cmd unzip || die "need 'unzip' to extract $archive"
      unzip -oq "$tmpd/$archive" -d "$tmpd"
      ;;
  esac
  [ -f "$tmpd/$binname" ] || die "$binname not found inside $archive"

  # Install atomically into the chosen dir.
  local dir
  dir="$(pick_install_dir)"
  mkdir -p "$dir" || die "could not create install dir $dir"
  if [ ! -w "$dir" ]; then
    die "install dir $dir is not writable (set ${PREFIX}_INSTALL_DIR or re-run with sufficient permissions)"
  fi
  mv "$tmpd/$binname" "$dir/$binname.partial"
  chmod +x "$dir/$binname.partial"
  mv "$dir/$binname.partial" "$dir/$binname"

  log "installed $BINARY $tag to $dir/$binname ($os/$arch)"
  case ":$PATH:" in
    *":$dir:"*) ;;
    *) log "note: $dir is not on your PATH; add it to use '$BINARY' directly" ;;
  esac

  # Best-effort extras: the release archive also bundles a man page and shell
  # completions. Installing them is a nice-to-have, so never fail on them.
  local man_dir
  if [ -f "$tmpd/man/${BINARY}.1" ]; then
    man_dir="$(dirname "$dir")/share/man/man1"
    if mkdir -p "$man_dir" 2>/dev/null && cp "$tmpd/man/${BINARY}.1" "$man_dir/" 2>/dev/null; then
      log "installed man page to $man_dir/${BINARY}.1"
    else
      man_dir="$HOME/.local/share/man/man1"
      mkdir -p "$man_dir" 2>/dev/null && cp "$tmpd/man/${BINARY}.1" "$man_dir/" 2>/dev/null \
        && log "installed man page to $man_dir/${BINARY}.1" || true
    fi
  fi
  if [ -d "$tmpd/completions" ]; then
    log "shell completions are bundled in the archive (completions/); or run '$BINARY completion <shell>'"
  fi
}

main "$@"
