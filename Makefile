# Binary name
BIN := kvantumci
CMD := ./cmd/kvantumci
RELEASE_SIGNING_CERT_FILE ?= release-signing-cert.pem

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/$(BIN) $(CMD)

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: dist force-dist
force-dist:

# Rebuild every asset for release checks; never reuse a stale dist binary.
dist: \
	dist/$(BIN)-darwin-amd64 \
	dist/$(BIN)-darwin-arm64 \
	dist/$(BIN)-linux-amd64 \
	dist/$(BIN)-linux-arm64 \
	dist/$(BIN)-windows-amd64.exe \
	dist/$(BIN)-windows-arm64.exe

dist/$(BIN)-darwin-amd64: force-dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

dist/$(BIN)-darwin-arm64: force-dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

dist/$(BIN)-linux-amd64: force-dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

dist/$(BIN)-linux-arm64: force-dist
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

dist/$(BIN)-windows-amd64.exe: force-dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

dist/$(BIN)-windows-arm64.exe: force-dist
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags=-buildid= -o $@ $(CMD)

# RELEASE_TAG is the exact tag used in the signed manifest and download URLs.
.PHONY: release-manifest release-sign
release-manifest: dist
	@test -n "$(RELEASE_TAG)" && printf '%s' "$(RELEASE_TAG)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || { echo 'Set RELEASE_TAG to a vX.Y.Z release tag' >&2; exit 1; }
	@{ printf 'kvantumci-release-v1\nversion %s\n' "$(RELEASE_TAG)"; \
	  for asset in $(BIN)-darwin-amd64 $(BIN)-darwin-arm64 $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-windows-amd64.exe $(BIN)-windows-arm64.exe; do \
	    test -s "dist/$$asset" || { echo "Missing release asset: $$asset" >&2; exit 1; }; \
	    hash=$$(openssl dgst -sha256 "dist/$$asset" | sed 's/^.*= //'); \
	    printf 'sha256 %s %s\n' "$$hash" "$$asset"; \
	  done; } > dist/release-manifest.txt

release-sign: release-manifest
	@test -n "$(RELEASE_SIGNING_KEY_FILE)" && test -f "$(RELEASE_SIGNING_KEY_FILE)" || { echo 'Set RELEASE_SIGNING_KEY_FILE to an RSA-3072 private key PEM file' >&2; exit 1; }
	@test -n "$(RELEASE_SIGNING_CERT_FILE)" && test -f "$(RELEASE_SIGNING_CERT_FILE)" || { echo 'Set RELEASE_SIGNING_CERT_FILE to the pinned certificate PEM file' >&2; exit 1; }
	@tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT; \
	  openssl pkey -in "$(RELEASE_SIGNING_KEY_FILE)" -pubout -out "$$tmp" 2>/dev/null && \
	  openssl rsa -pubin -in "$$tmp" -noout -text 2>/dev/null | grep -Eq '^(RSA )?Public-Key: \(3072 bit\)$$' || \
	  { echo 'Signing key must be RSA-3072' >&2; exit 1; }
	@key_pub=$$(openssl pkey -in "$(RELEASE_SIGNING_KEY_FILE)" -pubout); cert_pub=$$(openssl x509 -in "$(RELEASE_SIGNING_CERT_FILE)" -pubkey -noout); \
	  test -n "$$key_pub" && test "$$key_pub" = "$$cert_pub" || { echo 'Signing key and certificate do not match' >&2; exit 1; }
	@fingerprint=$$(openssl x509 -in "$(RELEASE_SIGNING_CERT_FILE)" -outform DER | openssl dgst -sha256 | sed 's/^.*= //'); \
	  grep -Fqx "PINNED_CERT_SHA256=$$fingerprint" install.sh && \
	  grep -Fqx "\$$PinnedCertificateSha256 = '$$fingerprint'" install.ps1 || { echo 'Certificate fingerprint is not pinned in both installers' >&2; exit 1; }
	@openssl dgst -sha256 -sign "$(RELEASE_SIGNING_KEY_FILE)" -out dist/release-manifest.sig dist/release-manifest.txt

.PHONY: clean
clean:
	rm -rf bin dist

.PHONY: run
run: build
	./bin/$(BIN) $(ARGS)
