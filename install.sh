#!/bin/sh
set -eu

PROGRAM=kvantumci
REPOSITORY=${KVANTUMCI_REPOSITORY:-HackiHub/kvantumcli}
VERSION=${KVANTUMCI_VERSION:-latest}
INSTALL_DIR=${KVANTUMCI_INSTALL_DIR:-"$HOME/.local/bin"}
PUBLIC_KEY=${KVANTUMCI_PUBLIC_KEY_FILE:-}
MIRROR=${KVANTUMCI_DOWNLOAD_BASE_URL:-}
PINNED_CERT_SHA256=7f200aeb7faf7e5158caa354d7a0c72bfcbca09ab024b8d557016b6a10aa197b

fail() { printf 'kvantumci installer: %s\n' "$*" >&2; exit 1; }
usage() {
    cat <<'USAGE'
Install kvantumci for macOS or Linux.
Usage: install.sh [--version VERSION] [--install-dir DIRECTORY]
A trusted X.509 certificate PEM file is required via KVANTUMCI_PUBLIC_KEY_FILE.
KVANTUMCI_VERSION, KVANTUMCI_INSTALL_DIR, KVANTUMCI_REPOSITORY and
KVANTUMCI_DOWNLOAD_BASE_URL can also be set in the environment.
USAGE
}
while [ "$#" -gt 0 ]; do
    case "$1" in
        --version) [ "$#" -ge 2 ] || fail '--version requires a value'; VERSION=$2; shift 2 ;;
        --install-dir) [ "$#" -ge 2 ] || fail '--install-dir requires a value'; INSTALL_DIR=$2; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) fail "unknown argument: $1" ;;
    esac
done

no_space_or_control() {
    [ "$1" = "$(printf '%s' "$1" | LC_ALL=C tr -d '[:space:][:cntrl:]')" ]
}
no_space_or_control "$REPOSITORY" || fail 'invalid GitHub repository'
printf '%s' "$REPOSITORY" | grep -Eq '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$' || fail 'invalid GitHub repository'
case "$REPOSITORY" in *..*|*./*|*/.*|*/-*) fail 'invalid GitHub repository' ;; esac
printf '%s' "$PINNED_CERT_SHA256" | grep -Eq '^[0-9a-f]{64}$' || fail 'installer trust root has not been provisioned'
[ -n "$PUBLIC_KEY" ] && [ -f "$PUBLIC_KEY" ] || fail 'KVANTUMCI_PUBLIC_KEY_FILE must name a trusted X.509 certificate PEM file'
command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v openssl >/dev/null 2>&1 || fail 'OpenSSL is required'
TMP_DIR=$(mktemp -d) || fail 'cannot create temporary directory'
STAGED=
cleanup() { [ -z "$STAGED" ] || rm -f "$STAGED"; rm -rf "$TMP_DIR"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
CERT="$TMP_DIR/trusted-cert.pem"
cp "$PUBLIC_KEY" "$CERT" || fail 'cannot snapshot trusted certificate'
openssl x509 -in "$CERT" -outform DER -out "$TMP_DIR/trusted-cert.der" 2>/dev/null || fail 'invalid trusted certificate'
openssl dgst -sha256 "$TMP_DIR/trusted-cert.der" > "$TMP_DIR/cert-hash.txt" || fail 'cannot hash trusted certificate'
CERT_SHA256=$(sed 's/^.*= //' "$TMP_DIR/cert-hash.txt")
[ "$CERT_SHA256" = "$PINNED_CERT_SHA256" ] || fail 'trusted certificate fingerprint mismatch'
openssl x509 -in "$CERT" -pubkey -noout > "$TMP_DIR/public.pem" 2>/dev/null || fail 'invalid trusted certificate'
openssl rsa -pubin -in "$TMP_DIR/public.pem" -noout -text > "$TMP_DIR/rsa-public.txt" 2>/dev/null || fail 'trusted certificate must contain an RSA-3072 key'
grep -Eq '^(RSA )?Public-Key: \(3072 bit\)$' "$TMP_DIR/rsa-public.txt" || fail 'trusted certificate must contain an RSA-3072 key'

validate_https() {
    no_space_or_control "$1" || fail 'invalid download URL'
    case "$1" in https://*) ;; *) fail 'download URL must use HTTPS' ;; esac
    case "$1" in *'@'*|*'?'*|*'#'*) fail 'invalid download URL' ;; esac
    printf '%s' "$1" | grep -Eq '^https://[A-Za-z0-9.-]+(:[0-9]+)?(/[^[:space:]]*)?$' || fail 'invalid download URL'
}
download() {
    validate_https "$1"
    curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' --output "$2" "$1" || fail "download failed: $1"
}

case "$(uname -s)" in Linux) OS=linux ;; Darwin) OS=darwin ;; *) fail 'unsupported operating system' ;; esac
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; arm64|aarch64) ARCH=arm64 ;; *) fail 'unsupported architecture' ;; esac
ASSET="$PROGRAM-$OS-$ARCH"
if [ "$VERSION" != latest ]; then
    case "$VERSION" in v*) TAG=$VERSION ;; *) TAG=v$VERSION ;; esac
    no_space_or_control "$TAG" || fail 'invalid release version'
    printf '%s' "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' || fail 'invalid release version'
fi
[ -z "$MIRROR" ] || validate_https "$MIRROR"

if [ "$VERSION" = latest ]; then
    LATEST_URL="https://github.com/$REPOSITORY/releases/latest"
    EFFECTIVE=$(curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' --output /dev/null --write-out '%{url_effective}' "$LATEST_URL") || fail 'could not resolve latest release'
    case "$EFFECTIVE" in "https://github.com/$REPOSITORY/releases/tag/"*) TAG=${EFFECTIVE##*/} ;; *) fail 'unexpected latest release URL' ;; esac
fi
no_space_or_control "$TAG" || fail 'invalid release version'
printf '%s' "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' || fail 'invalid release version'

if [ -n "$MIRROR" ]; then
    BASE="${MIRROR%/}/$TAG"
else
    BASE="https://github.com/$REPOSITORY/releases/download/$TAG"
fi

MANIFEST="$TMP_DIR/release-manifest.txt"
SIGNATURE="$TMP_DIR/release-manifest.sig"
BINARY="$TMP_DIR/$ASSET"
download "$BASE/release-manifest.txt" "$MANIFEST"
download "$BASE/release-manifest.sig" "$SIGNATURE"
[ -s "$MANIFEST" ] && [ -s "$SIGNATURE" ] || fail 'empty manifest or signature'
openssl dgst -sha256 -verify "$TMP_DIR/public.pem" -signature "$SIGNATURE" "$MANIFEST" >/dev/null 2>&1 || fail 'release manifest signature is invalid'

[ "$(wc -l < "$MANIFEST" | tr -d ' ')" = 8 ] || fail 'release manifest must contain exactly eight lines'
[ "$(sed -n '1p' "$MANIFEST")" = 'kvantumci-release-v1' ] || fail 'invalid release manifest marker'
[ "$(sed -n '2p' "$MANIFEST")" = "version $TAG" ] || fail 'release manifest version mismatch'
line_number=3
EXPECTED_HASH=
set -f
for expected in kvantumci-darwin-amd64 kvantumci-darwin-arm64 kvantumci-linux-amd64 kvantumci-linux-arm64 kvantumci-windows-amd64.exe kvantumci-windows-arm64.exe; do
    line=$(sed -n "${line_number}p" "$MANIFEST")
    set -- $line
    [ "$#" -eq 3 ] && [ "$line" = "sha256 $2 $expected" ] || fail 'invalid or unsorted release manifest entry'
    [ "${#2}" -eq 64 ] && printf '%s' "$2" | grep -Eq '^[0-9a-f]{64}$' || fail 'invalid release manifest hash'
    [ "$expected" = "$ASSET" ] && EXPECTED_HASH=$2
    line_number=$((line_number + 1))
done
[ -n "$EXPECTED_HASH" ] || fail 'asset not present in release manifest'
download "$BASE/$ASSET" "$BINARY"
[ -s "$BINARY" ] || fail 'downloaded binary is empty'
ACTUAL_HASH=$(openssl dgst -sha256 "$BINARY" | sed 's/^.*= //')
[ "$ACTUAL_HASH" = "$EXPECTED_HASH" ] || fail 'downloaded binary hash mismatch'

mkdir -p "$INSTALL_DIR" || fail 'cannot create install directory'
STAGED=$(mktemp "$INSTALL_DIR/.kvantumci.XXXXXX") || fail 'cannot stage binary in install directory'
cp "$BINARY" "$STAGED" || fail 'cannot stage binary'
chmod 755 "$STAGED" || fail 'cannot make binary executable'
mv -f "$STAGED" "$INSTALL_DIR/$PROGRAM" || fail 'cannot install binary'
STAGED=
printf 'Installed %s to %s\n' "$PROGRAM" "$INSTALL_DIR/$PROGRAM"
case ":$PATH:" in *":$INSTALL_DIR:"*) ;; *) printf 'Add %s to PATH to run %s from any directory.\n' "$INSTALL_DIR" "$PROGRAM" ;; esac
