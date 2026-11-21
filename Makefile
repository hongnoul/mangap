MAKEFLAGS += --silent

ldflags := -X 'github.com/hongnoul/mangaz/constant.BuiltAt=$(shell date -u)'
ldflags += -X 'github.com/hongnoul/mangaz/constant.BuiltBy=$(shell whoami)'
ldflags += -X 'github.com/hongnoul/mangaz/constant.Revision=$(shell git rev-parse --short HEAD)'
ldflags += -s
ldflags += -w

build_flags := -ldflags=${ldflags}

all: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the mangaz binary"
	@echo "  install      Install the mangaz binary"
	@echo "  uninstall    Uninstall the mangaz binary"
	@echo "  completions  Regenerate shell completions"
	@echo "  test         Run the tests"
	@echo "  help         Show this help message"
	@echo ""

install:
	@go install "$(build_flags)"

build:
	@go build "$(build_flags)"

completions: build
	@./mangaz completion bash > completions/mangaz.bash
	@./mangaz completion zsh > completions/mangaz.zsh
	@./mangaz completion fish > completions/mangaz.fish
	@./mangaz completion powershell > completions/mangaz.ps1

test:
	@go test ./...

uninstall:
	@rm -f "$(shell go env GOPATH)/bin/mangaz"
