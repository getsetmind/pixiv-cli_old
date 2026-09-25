ifeq ($(OS),Windows_NT)
SHELL   := cmd.exe
EXE     := .exe
MKDIR   := if not exist bin mkdir bin
RM      := if exist bin rmdir /s /q bin & if exist dist rmdir /s /q dist
GO_ENV  := set CGO_ENABLED=0&&
NULL    := NUL
DATE    := $(shell powershell -NoProfile -Command "(Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')")
else
MKDIR   := mkdir -p bin
RM      := rm -rf bin dist
GO_ENV  := CGO_ENABLED=0
NULL    := /dev/null
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
endif

BINARY  := bin/pixiv$(EXE)
PKG     := ./cmd/pixiv
VERSION := $(shell git describe --tags --always --dirty 2>$(NULL))
COMMIT  := $(shell git rev-parse --short HEAD 2>$(NULL))
LDFLAGS := -s -w \
	-X github.com/tamnd/pixiv-cli/cli.Version=$(VERSION) \
	-X github.com/tamnd/pixiv-cli/cli.Commit=$(COMMIT) \
	-X github.com/tamnd/pixiv-cli/cli.Date=$(DATE)

.PHONY: build install test vet fmt clean run

build:
	@$(MKDIR)
	$(GO_ENV) go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install:
	$(GO_ENV) go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w -s .

clean:
	$(RM)

run: build
	$(BINARY) $(ARGS)
