# Binary name
BIN := kvantumci
CMD := ./cmd/kvantumci

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/$(BIN) $(CMD)

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: dist
dist: \
	dist/$(BIN)-darwin-amd64 \
	dist/$(BIN)-darwin-arm64 \
	dist/$(BIN)-linux-amd64 \
	dist/$(BIN)-linux-arm64 \
	dist/$(BIN)-windows-amd64.exe \
	dist/$(BIN)-windows-arm64.exe

dist/$(BIN)-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ $(CMD)

dist/$(BIN)-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ $(CMD)

dist/$(BIN)-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(CMD)

dist/$(BIN)-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $@ $(CMD)

dist/$(BIN)-windows-amd64.exe:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $@ $(CMD)

dist/$(BIN)-windows-arm64.exe:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o $@ $(CMD)

.PHONY: clean
clean:
	rm -rf bin dist

.PHONY: run
run: build
	./bin/$(BIN) $(ARGS)
