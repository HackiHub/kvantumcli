#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/mirror/v1.2.3" "$TMP/bin" "$TMP/install"
openssl req -x509 -newkey rsa:3072 -nodes -days 1 -subj '/CN=kvantumci-installer-test' -keyout "$TMP/key.pem" -out "$TMP/cert.pem" >/dev/null 2>&1
cp "$TMP/cert.pem" "$TMP/original-cert.pem"
openssl req -x509 -newkey rsa:3072 -nodes -days 1 -subj '/CN=kvantumci-other-test' -keyout "$TMP/other-key.pem" -out "$TMP/other-cert.pem" >/dev/null 2>&1
fingerprint=$(openssl x509 -in "$TMP/cert.pem" -outform DER | openssl dgst -sha256 | sed 's/^.*= //')
sed "s/7f200aeb7faf7e5158caa354d7a0c72bfcbca09ab024b8d557016b6a10aa197b/$fingerprint/g" "$ROOT/install.sh" > "$TMP/install-test.sh"
for asset in kvantumci-darwin-amd64 kvantumci-darwin-arm64 kvantumci-linux-amd64 kvantumci-linux-arm64 kvantumci-windows-amd64.exe kvantumci-windows-arm64.exe; do
    printf 'new binary: %s\n' "$asset" > "$TMP/mirror/v1.2.3/$asset"
done
{
    printf 'kvantumci-release-v1\nversion v1.2.3\n'
    for asset in kvantumci-darwin-amd64 kvantumci-darwin-arm64 kvantumci-linux-amd64 kvantumci-linux-arm64 kvantumci-windows-amd64.exe kvantumci-windows-arm64.exe; do
        hash=$(openssl dgst -sha256 "$TMP/mirror/v1.2.3/$asset" | sed 's/^.*= //')
        printf 'sha256 %s %s\n' "$hash" "$asset"
    done
} > "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/mirror/v1.2.3/release-manifest.txt"
cat > "$TMP/bin/curl" <<'CURL'
#!/bin/sh
set -eu
out=
proto=
redirect_proto=
last=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output) out=$2; shift 2 ;;
        --write-out) shift 2 ;;
        --proto) proto=$2; shift 2 ;;
        --proto-redir) redirect_proto=$2; shift 2 ;;
        *) last=$1; shift ;;
    esac
done
[ "$proto" = '=https' ] && [ "$redirect_proto" = '=https' ] || exit 1
if [ -n "${FIXTURE_CURL_LOG:-}" ]; then printf '%s\n' "$last" >> "$FIXTURE_CURL_LOG"; fi
if [ "${FIXTURE_SWAP_CERT:-}" = 1 ]; then cp "$FIXTURE_OTHER_CERT" "$FIXTURE_ORIGINAL_CERT"; fi
if [ "$last" = 'https://github.com/HackiHub/kvantumcli/releases/latest' ]; then
    [ "$out" = /dev/null ] || exit 1
    printf '%s' 'https://github.com/HackiHub/kvantumcli/releases/tag/v1.2.3'
    exit 0
fi
case "$last" in https://fixture.invalid/v1.2.3/*) asset=${last##*/} ;; *) exit 1 ;; esac
[ "${FIXTURE_CURL_FAIL_ASSET:-}" != "$asset" ] || exit 22
if [ "${FIXTURE_CURL_SIGNAL:-}" = 1 ]; then kill -TERM "$PPID"; exit 0; fi
cp "$FIXTURE_MIRROR/v1.2.3/$asset" "$out"
CURL
chmod +x "$TMP/bin/curl"
export PATH="$TMP/bin:$PATH" FIXTURE_MIRROR="$TMP/mirror" KVANTUMCI_VERSION=v1.2.3
export FIXTURE_OTHER_CERT="$TMP/other-cert.pem" FIXTURE_ORIGINAL_CERT="$TMP/cert.pem" FIXTURE_CURL_LOG="$TMP/curl.log"
export KVANTUMCI_DOWNLOAD_BASE_URL=https://fixture.invalid KVANTUMCI_INSTALL_DIR="$TMP/install"
export KVANTUMCI_PUBLIC_KEY_FILE="$TMP/cert.pem"
case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) echo 'unsupported test OS' >&2; exit 1 ;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo 'unsupported test arch' >&2; exit 1 ;; esac
asset=kvantumci-$os-$arch
assert_refused() {
    expected=$1
    shift
    if "$@" > "$TMP/stdout" 2> "$TMP/stderr"; then echo "installer accepted $expected" >&2; exit 1; fi
    grep -Fq "$expected" "$TMP/stderr" || { cat "$TMP/stderr" >&2; echo "expected rejection: $expected" >&2; exit 1; }
    [ "$(cat "$TMP/install/kvantumci")" = 'old binary' ] || { echo 'refusal changed installed binary' >&2; exit 1; }
    [ -z "$(find "$TMP/install" -name '.kvantumci.*' -print)" ] || { echo 'refusal left staged binary' >&2; exit 1; }
}
printf 'old binary\n' > "$TMP/install/kvantumci"
sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"
cmp "$TMP/install/kvantumci" "$TMP/mirror/v1.2.3/$asset"
KVANTUMCI_VERSION=latest sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"
cmp "$TMP/install/kvantumci" "$TMP/mirror/v1.2.3/$asset"
printf 'old binary\n' > "$TMP/install/kvantumci"
# The source certificate changes on the first download; verification must use the pinned snapshot.
FIXTURE_SWAP_CERT=1 sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"
cmp "$TMP/install/kvantumci" "$TMP/mirror/v1.2.3/$asset"
cp "$TMP/other-cert.pem" "$TMP/cert.pem"
printf 'old binary\n' > "$TMP/install/kvantumci"
assert_refused 'trusted certificate fingerprint mismatch' sh "$TMP/install-test.sh"
openssl x509 -in "$TMP/cert.pem" -outform DER | openssl dgst -sha256 | sed 's/^.*= //' | grep -Fqx "$fingerprint" && { echo 'certificate swap did not occur' >&2; exit 1; }
cp "$TMP/original-cert.pem" "$TMP/cert.pem"
cp "$TMP/mirror/v1.2.3/release-manifest.txt" "$TMP/good-manifest.txt"
openssl dgst -sha256 -sign "$TMP/other-key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/good-manifest.txt"
assert_refused 'release manifest signature is invalid' env FIXTURE_SWAP_CERT=1 sh "$TMP/install-test.sh"
cp "$TMP/original-cert.pem" "$TMP/cert.pem"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/good-manifest.txt"
sed '2s/v1.2.3/v1.2.2/' "$TMP/good-manifest.txt" > "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/mirror/v1.2.3/release-manifest.txt"
assert_refused 'release manifest version mismatch' sh "$TMP/install-test.sh"
cp "$TMP/good-manifest.txt" "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/good-manifest.txt"
assert_no_fetch() {
    expected=$1
    shift
    : > "$TMP/curl.log"
    assert_refused "$expected" "$@"
    [ ! -s "$TMP/curl.log" ] || { echo 'malformed input caused fetch' >&2; exit 1; }
}
for bad in "$(printf 'https://fixture.invalid/a\nb')" "$(printf 'https://fixture.invalid/a\rb')" "$(printf 'https://fixture.invalid/a\tb')" "$(printf 'https://fixture.invalid\nhttps://fixture.invalid')" ' https://fixture.invalid' 'https://fixture.invalid '; do
    assert_no_fetch 'invalid download URL' env KVANTUMCI_DOWNLOAD_BASE_URL="$bad" KVANTUMCI_VERSION=latest sh "$TMP/install-test.sh"
done
for bad in "$(printf 'HackiHub/kvantumcli\nHackiHub/kvantumcli')" "$(printf 'HackiHub/kvantumcli\r')" "$(printf 'HackiHub/kvantumcli\t')" ' HackiHub/kvantumcli'; do
    assert_no_fetch 'invalid GitHub repository' env KVANTUMCI_REPOSITORY="$bad" KVANTUMCI_VERSION=latest sh "$TMP/install-test.sh"
done
for bad in "$(printf 'v1.2.3\nv1.2.4')" "$(printf 'v1.2.3\r')" "$(printf 'v1.2.3\t')" 'v1.2.3 '; do
    assert_no_fetch 'invalid release version' env KVANTUMCI_VERSION="$bad" sh "$TMP/install-test.sh"
done
printf 'tampered\n' > "$TMP/mirror/v1.2.3/$asset"
if sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'tampered binary accepted' >&2; exit 1; fi
[ "$(cat "$TMP/install/kvantumci")" = 'old binary' ]
printf 'new binary: %s\n' "$asset" > "$TMP/mirror/v1.2.3/$asset"
cp "$TMP/mirror/v1.2.3/release-manifest.txt" "$TMP/valid-manifest.txt"
printf 'tampered\n' >> "$TMP/mirror/v1.2.3/release-manifest.txt"
if sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'tampered manifest accepted' >&2; exit 1; fi
[ "$(cat "$TMP/install/kvantumci")" = 'old binary' ]
awk 'NR < 8' "$TMP/valid-manifest.txt" > "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/mirror/v1.2.3/release-manifest.txt"
if sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'signed manifest with missing entry accepted' >&2; exit 1; fi
awk 'NR == 7 { previous = $0 } NR == 8 { $0 = previous } { print }' "$TMP/valid-manifest.txt" > "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/mirror/v1.2.3/release-manifest.txt"
if sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'signed manifest with duplicate entry accepted' >&2; exit 1; fi
cp "$TMP/valid-manifest.txt" "$TMP/mirror/v1.2.3/release-manifest.txt"
openssl dgst -sha256 -sign "$TMP/key.pem" -out "$TMP/mirror/v1.2.3/release-manifest.sig" "$TMP/mirror/v1.2.3/release-manifest.txt"
if FIXTURE_CURL_FAIL_ASSET="$asset" sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'failed asset download accepted' >&2; exit 1; fi
[ "$(cat "$TMP/install/kvantumci")" = 'old binary' ]
mkdir "$TMP/work"
set +e
TMPDIR="$TMP/work" FIXTURE_CURL_SIGNAL=1 sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"
status=$?
set -e
[ "$status" -eq 143 ] || { echo "interrupted installer exited with $status, expected 143" >&2; exit 1; }
[ -z "$(ls -A "$TMP/work")" ] || { echo 'interrupt left temporary files' >&2; exit 1; }
[ "$(cat "$TMP/install/kvantumci")" = 'old binary' ]
if KVANTUMCI_PUBLIC_KEY_FILE= sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'missing trust root accepted' >&2; exit 1; fi
if KVANTUMCI_DOWNLOAD_BASE_URL=http://fixture.invalid sh "$TMP/install-test.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'HTTP mirror accepted' >&2; exit 1; fi
if sh "$ROOT/install.sh" > "$TMP/stdout" 2> "$TMP/stderr"; then echo 'unprovisioned production installer accepted' >&2; exit 1; fi
[ "$(cat "$TMP/install/kvantumci")" = 'old binary' ]
echo 'Unix installer checks passed'
