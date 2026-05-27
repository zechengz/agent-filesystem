SHELL := /bin/sh

GO ?= go
BIN ?= liveskills
BUILD_DIR ?= bin
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
INSTALL ?= install
INSTALL_MODE ?= 0755

.PHONY: all build install uninstall test surface-test fmt clean help

all: build

build:
	@mkdir -p "$(BUILD_DIR)"
	$(GO) build -o "$(BUILD_DIR)/$(BIN)" .

install: build
	@mkdir -p "$(BINDIR)"
	$(INSTALL) -m "$(INSTALL_MODE)" "$(BUILD_DIR)/$(BIN)" "$(BINDIR)/$(BIN)"
	@printf 'Installed %s to %s\n' "$(BIN)" "$(BINDIR)/$(BIN)"
	@case ":$$PATH:" in *:"$(BINDIR)":*) ;; *) printf 'Note: %s is not on PATH yet.\n' "$(BINDIR)" ;; esac

uninstall:
	rm -f "$(BINDIR)/$(BIN)"

test:
	$(GO) test ./...

surface-test:
	python3 scripts/liveskills_surface_test.py

fmt:
	gofmt -w *.go

clean:
	rm -rf "$(BUILD_DIR)"

help:
	@printf 'LiveSkills Make targets:\n'
	@printf '  make          Build %s into %s/\n' "$(BIN)" "$(BUILD_DIR)"
	@printf '  make install  Install %s to %s\n' "$(BIN)" "$(BINDIR)"
	@printf '  make test     Run Go tests\n'
	@printf '  make surface-test  Run isolated CLI surface test\n'
	@printf '  make fmt      Format Go files\n'
	@printf '  make clean    Remove build output\n'
