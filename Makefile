PROJECT         := cnab-azure
ORG             := deislabs
BINDIR          := $(CURDIR)/bin
GOFLAGS         :=
LDFLAGS         := -w -s
TESTFLAGS       := -v
INSTALL_DIR     := /usr/local/bin

ifeq ($(OS),Windows_NT)
	TARGET = $(PROJECT).exe
	SHELL  = cmd /c
	CHECK  = where
else
	TARGET = $(PROJECT)
	SHELL  ?= bash
	CHECK  ?= which
endif

GIT_TAG   := $(shell git describe --tags --always)
VERSION   ?= ${GIT_TAG}
# Replace + with -, for Docker image tag compliance
IMAGE_TAG ?= $(subst +,-,$(VERSION))
LDFLAGS   += -X main.Version=$(VERSION)

.PHONY: default
default: build

.PHONY: build
build:

	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(TARGET) github.com/$(ORG)/$(PROJECT)/cmd/...

.PHONY: install
install:
	install $(BINDIR)/$(TARGET) $(INSTALL_DIR)

CX_OSES  = linux windows darwin
CX_ARCHS = amd64

.PHONY: build-release
build-release:
	@for os in $(CX_OSES); do \
		echo "building $$os"; \
		for arch in $(CX_ARCHS); do \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(PROJECT)-$$os-$$arch github.com/$(ORG)/$(PROJECT)/cmd/...; \
		done; \
		if [ $$os = 'windows' ]; then \
			mv $(BINDIR)/$(PROJECT)-$$os-$$arch $(BINDIR)/$(PROJECT)-$$os-$$arch.exe; \
		fi; \
	done

.PHONY: debug
debug:
	go build $(GOFLAGS) -o $(BINDIR)/$(TARGET) github.com/$(ORG)/$(PROJECT)/cmd/...

.PHONY: test
test:
	go test $(TESTFLAGS) ./...

.PHONY: test-in-azure
test-in-azure:
ifeq ($(OS),Windows_NT)
	powershell -executionPolicy bypass -noexit -NoProfile -file ./test/run_azure_test.local.ps1
else
	./test/run_azure_test.local.sh
endif

.PHONY: lint
lint:
	golangci-lint run --config ./golangci.yml

HAS_DEP          := $(shell $(CHECK) dep)
HAS_GOLANGCI     := $(shell $(CHECK) golangci-lint)
HAS_GOIMPORTS    := $(shell $(CHECK) goimports)
GOLANGCI_VERSION := v1.16.0

.PHONY: bootstrap
bootstrap:
ifndef HAS_DEP
ifeq ($(OS),Windows_NT)
	choco install dep -y
else
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
endif

ifndef HAS_GOLANGCI
ifeq ($(OS),Windows_NT)
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
else
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_VERSION)
endif
endif

ifndef HAS_GOIMPORTS
	go get -u golang.org/x/tools/cmd/goimports
endif
	dep ensure -vendor-only -v

.PHONY: goimports
goimports:
	find . -name "*.go" | fgrep -v vendor/ | xargs goimports -w -local github.com/$(ORG)/$(PROJECT)