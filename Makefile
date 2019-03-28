# Copyright (2018) Cobalt Speech and Language Inc.

# Needed tools are installed to BINDIR.
#
# TODO: We share some tools across repos, so having this be local is not ideal.
# We should figure out a strategy for this BINDIR, but until then having a local
# folder helps us not depend on GOPATH at all (beyond what the go toolchain
# needs/does) and allows our CI builds to be ephemeral.
BINDIR := ./bin

LINTER := $(BINDIR)/golangci-lint
LINTER_VERSION := 1.15.0

# Linux vs Darwin detection for the machine on which the build is taking place (not to be used for the build target)
DEV_OS := $(shell uname -s | tr A-Z a-z)

all: build

$(LINTER):
	mkdir -p $(BINDIR)
	wget "https://github.com/golangci/golangci-lint/releases/download/v$(LINTER_VERSION)/golangci-lint-$(LINTER_VERSION)-$(DEV_OS)-amd64.tar.gz" -O - | tar -xz -C $(BINDIR) --strip-components=1 --exclude=README.md --exclude=LICENSE

# Get vendored dependencies
.PHONY: mod
mod:
	go mod tidy

# Run go-fmt on all go files.  We list all go files in the repository, run
# gofmt.  gofmt produces output with a list of files that have fmt errors.  If
# we have an empty output, we exit with 0 status, otherwise we exit with nonzero
# status.
.PHONY: fmt
fmt:
	BADFILES=$$(gofmt -l -d $$(find . -type f -name '*.go')) && [ -z "$$BADFILES" ] && exit 0

# Run lint checks
.PHONY: lint

# TODO: Some linters are disabled (via .golangci.yml) since they have trouble
# with the new go modules. This should be fixed once the linters support
# modules.
lint: $(LINTER)
	$(LINTER) run --deadline=2m

# Run tests
.PHONY: test
test: mod
	go test ./...

# Building debug and release versions
CLI_BINARY:=cubic-cli

# Set variables for version reporting.
VERSION:=$(shell git describe --tags --dirty --always)
COMMIT_HASH:=$(shell git rev-parse --short HEAD 2>/dev/null)
PKG:=github.com/cobaltspeech/cubic-cli/cmd/cubic-cli/cmd
LDFLAGSVERSION:=-X $(PKG).commitHash=$(COMMIT_HASH) -X $(PKG).version=$(VERSION)

PLATFORMS := linux darwin windows
goos = $(patsubst release-%,%, $(patsubst debug-%,%, $(word 1, $@)))
ext = $(patsubst windows,.exe,$(filter windows,$(goos)))

DEBUG_PLATFORMS := $(addprefix debug-,$(PLATFORMS))
RELEASE_PLATFORMS := $(addprefix release-,$(PLATFORMS))

$(DEBUG_PLATFORMS): mod
	mkdir -p $(BINDIR)/debug
	GOOS=$(goos) GOARCH=amd64 go build -o $(BINDIR)/debug/$(CLI_BINARY)-$(goos)-amd64$(ext) -ldflags "$(LDFLAGSVERSION)" ./cmd/cubic-cli


$(RELEASE_PLATFORMS): mod
	mkdir -p $(BINDIR)/release
	GOOS=$(goos) GOARCH=amd64 go build -o $(BINDIR)/release/$(CLI_BINARY)-$(goos)-amd64$(ext) -ldflags "-s -w $(LDFLAGSVERSION)" ./cmd/cubic-cli

.PHONY: debug release
debug: $(DEBUG_PLATFORMS)
release: $(RELEASE_PLATFORMS)

.PHONY: $(PLATFORMS) $(DEBUG_PLATFORMS) $(RELEASE_PLATFORMS)

# General current build
.PHONY: build
build: mod
	go build -ldflags "-s -w $(LDFLAGSVERSION)" -o $(BINDIR)/$(CLI_BINARY) ./cmd/cubic-cli

# Clean
.PHONY: clean
clean:
	rm -rf ./bin
