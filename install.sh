#!/bin/sh

set -eu

PROGRAM="kvantumci"
REPOSITORY="${KVANTUMCI_REPOSITORY:-HackiHub/kvantumcli}"
VERSION="${KVANTUMCI_VERSION:-latest}"
INSTALL_DIR="${KVANTUMCI_INSTALL_DIR:-${HOME}/.local/bin}"

usage() {
    cat <<EOF
Install kvantumci for macOS or Linux.

Usage: install.sh [--version VERSION] [--install-dir DIRECTORY]

Options:
  --version VERSION       Release to install, for example v1.2.3 (default: latest)
  --install-dir DIRECTORY Destination directory (default: \$HOME/.local/bin)
  -h, --help              Show this help

The same settings can be provided with KVANTUMCI_VERSION and
KVANTUMCI_INSTALL_DIR.
EOF
}

fail() {
    echo "kvantumci installer: $*" >&2
    exit 1
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            [ "$#" -ge 2 ] || fail "--version requires a value"
            VERSION=$2
            shift 2
            ;;
        --install-dir)
            [ "$#" -ge 2 ] || fail "--install-dir requires a value"
            INSTALL_DIR=$2
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            fail "unknown argument: $1"
            ;;
    esac
done

case "$VERSION" in
    latest) RELEASE_PATH="latest/download" ;;
    ""|*[!A-Za-z0-9._+-]*) fail "invalid version: $VERSION" ;;
    v*) RELEASE_PATH="download/$VERSION" ;;
    *) RELEASE_PATH="download/v$VERSION" ;;
esac

case "$(uname -s)" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

ASSET="${PROGRAM}-${OS}-${ARCH}"
if [ -n "${KVANTUMCI_DOWNLOAD_BASE_URL:-}" ]; then
    URL="${KVANTUMCI_DOWNLOAD_BASE_URL%/}/${ASSET}"
else
    URL="https://github.com/${REPOSITORY}/releases/${RELEASE_PATH}/${ASSET}"
fi

TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t kvantumci)
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM
TMP_BINARY="${TMP_DIR}/${PROGRAM}"

echo "Downloading ${PROGRAM} ${VERSION} for ${OS}/${ARCH}..."
if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error "$URL" --output "$TMP_BINARY"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$URL" -O "$TMP_BINARY"
else
    fail "curl or wget is required"
fi

[ -s "$TMP_BINARY" ] || fail "downloaded file is empty"
chmod 755 "$TMP_BINARY"
mkdir -p "$INSTALL_DIR" || fail "cannot create install directory: $INSTALL_DIR"
mv "$TMP_BINARY" "${INSTALL_DIR}/${PROGRAM}" || fail "cannot install into: $INSTALL_DIR"

echo "Installed ${PROGRAM} to ${INSTALL_DIR}/${PROGRAM}"
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Add ${INSTALL_DIR} to PATH to run ${PROGRAM} from any directory." ;;
esac
