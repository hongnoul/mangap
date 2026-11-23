MAKEFLAGS += --silent

version := $(or $(patsubst v%,%,$(shell git describe --tags --abbrev=0 2>/dev/null)),0.1.0)

ldflags := -X 'github.com/hongnoul/mangap/constant.BuiltAt=$(shell date -u)'
ldflags += -X 'github.com/hongnoul/mangap/constant.Version=$(version)'
ldflags += -X 'github.com/hongnoul/mangap/constant.BuiltBy=$(shell whoami)'
ldflags += -X 'github.com/hongnoul/mangap/constant.Revision=$(shell git rev-parse --short HEAD)'
ldflags += -s
ldflags += -w

build_flags := -ldflags=${ldflags}

all: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the mangap binary"
	@echo "  install      Install the mangap binary"
	@echo "  uninstall    Uninstall the mangap binary"
	@echo "  completions  Regenerate shell completions"
	@echo "  test         Run the tests"
	@echo "  help         Show this help message"
	@echo ""

install:
	@go install "$(build_flags)"

build:
	@go build "$(build_flags)"

completions: build
	@./mangap completion bash > completions/mangap.bash
	@./mangap completion zsh > completions/mangap.zsh
	@./mangap completion fish > completions/mangap.fish
	@./mangap completion powershell > completions/mangap.ps1

test:
	@go test ./...

uninstall:
	@rm -f "$(shell go env GOPATH)/bin/mangap"
