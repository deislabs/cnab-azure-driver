BINDIR          := $(CURDIR)/bin
TESTFLAGS       := -v
INSTALL_DIR     := /usr/local/bin
FILENAME        := cnab-azure
PKG := github.com/deislabs/$(FILENAME)-driver
COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell git describe --tags 2> /dev/null || echo v0)

ifeq ($(OS),Windows_NT)
	TARGET = $(FILENAME).exe
	SHELL  = pwsh.exe
	CHECK  = where
else
	TARGET = $(FILENAME)
	SHELL  ?= bash
	CHECK  ?= which
endif

GO = GO111MODULE=on go
LDFLAGS   =  -w -s -X $(PKG)/pkg.Version=$(VERSION) -X $(PKG)/pkg.Commit=$(COMMIT)

default: build

.PHONY: build
build:
	mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(TARGET) ./cmd/...

.PHONY: install
install:
	install $(BINDIR)/$(TARGET) $(INSTALL_DIR)

CX_OSES  = linux windows darwin
CX_ARCHS = amd64

xbuild-all:
ifeq ($(OS),Windows_NT)
	powershell -executionPolicy bypass -NoLogo -NoProfile -File ./build/build-release.ps1 -oses '$(CX_OSES)' -arch  $(CX_ARCHS) -ldflags $(LDFLAGS) -filename $(FILENAME) -bindir $(BINDIR)
else
	@for os in $(CX_OSES); do \
		echo "building $$os"; \
		for arch in $(CX_ARCHS); do \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(TARGET)-$$os-$$arch ./cmd/...; \
		done; \
		if [ $$os = 'windows' ]; then \
			mv $(BINDIR)/$(TARGET)-$$os-$$arch $(BINDIR)/$(TARGET)-$$os-$$arch.exe; \
		fi; \
	done
endif

test-all-local: test test-in-azure-local

test-all: test test-in-azure

.PHONY: test
test:
	$(GO) test $(TESTFLAGS) ./...

test-in-azure:
	$(GO) test  $(TESTFLAGS) -timeout 12m ./pkg/... -args -runazuretest -verbosedriveroutput

test-in-azure-local:
ifeq ($(OS),Windows_NT)
	powershell -executionPolicy bypass -NoLogo -NoProfile -file ./test/run_azure_test.local.ps1
else
	./test/run_azure_test.local.sh
endif

lint: bootstrap
	golangci-lint run --config ./golangci.yml

HAS_GOLANGCI     := $(shell $(CHECK) golangci-lint)
HAS_GOIMPORTS    := $(shell $(CHECK) goimports)
GOLANGCI_VERSION := v1.29.0

bootstrap:
ifndef HAS_GOLANGCI
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
endif

ifndef HAS_GOIMPORTS
	go get -u golang.org/x/tools/cmd/goimports
endif

goimports: bootstrap
	find . -name "*.go" | fgrep -v vendor/ | xargs goimports -w -local $(PKG)