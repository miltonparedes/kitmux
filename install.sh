#!/bin/sh
set -eu

repo="miltonparedes/kitmux"
version="${KITMUX_VERSION:-}"
install_dir="${INSTALL_DIR:-${KITMUX_INSTALL_DIR:-$HOME/.local/bin}}"

os="${KITMUX_OS:-$(uname -s)}"
arch="${KITMUX_ARCH:-$(uname -m)}"
os="$(printf '%s' "$os" | tr '[:upper:]' '[:lower:]')"
arch="$(printf '%s' "$arch" | tr '[:upper:]' '[:lower:]')"

case "$os" in
  darwin|mac|macos) os="darwin" ;;
  linux) os="linux" ;;
  *)
    echo "kitmux: unsupported OS: $os (supported: macOS, Linux)" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "kitmux: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ -z "$version" ]; then
  if ! command -v curl >/dev/null 2>&1; then
    echo "kitmux: required command not found: curl" >&2
    exit 1
  fi

  version="$(
    curl -fsSL "https://api.github.com/repos/${repo}/releases?per_page=1" |
      sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1
  )"

  if [ -z "$version" ]; then
    echo "kitmux: could not resolve latest release for ${repo}" >&2
    exit 1
  fi
fi

archive="kitmux_${version#v}_${os}_${arch}.tar.gz"
url="https://github.com/${repo}/releases/download/${version}/${archive}"
tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/kitmux.XXXXXX")"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

if [ "${KITMUX_INSTALL_DRY_RUN:-}" = "1" ]; then
  echo "kitmux: target ${os}/${arch}"
  echo "kitmux: archive ${archive}"
  echo "kitmux: url ${url}"
  echo "kitmux: install_dir ${install_dir}"
  exit 0
fi

for cmd in curl tar install; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "kitmux: required command not found: $cmd" >&2
    exit 1
  fi
done

mkdir -p "$install_dir"

echo "Downloading kitmux ${version} for ${os}/${arch}..."
curl -fsSL "$url" -o "$tmpdir/$archive"
tar -xzf "$tmpdir/$archive" -C "$tmpdir" kitmux
install -m 0755 "$tmpdir/kitmux" "$install_dir/kitmux"

echo "kitmux installed to ${install_dir}/kitmux"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    echo "Add ${install_dir} to PATH to run kitmux from any shell."
    ;;
esac
