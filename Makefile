SHELL := /bin/sh

UNAME_M := $(shell uname -m 2>/dev/null || echo unknown)
ARCH ?= $(if $(filter arm64 aarch64,$(UNAME_M)),arm64,$(if $(filter x86_64 amd64,$(UNAME_M)),amd64,unknown))
VERSION ?=
WINDOWS_FORMAT ?= INSTALLER
FRONTEND_PORT ?= 5218

UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)
IS_WINDOWS := $(if $(filter Windows_NT,$(OS)),1,0)

.DEFAULT_GOAL := help
.PHONY: help deps dev dev-stable frontend-dev frontend-build bindings test test-go test-frontend check build build-mac build-linux build-windows assemble-runtime package-mac package-linux package-windows clean

help:
	@printf '%s\n' \
		'Hi Browser development commands:' \
		'  make deps              Install Go and frontend dependencies' \
		'  make dev               Start Wails + Vite hot reload' \
		'  make dev-stable        Start Wails with built frontend assets' \
		'  make frontend-dev      Start only the Vite development server' \
		'  make frontend-build    Type-check and build frontend assets' \
		'  make bindings          Regenerate Wails bindings' \
		'  make test              Run Go tests and frontend production build' \
		'  make check             Run tests plus shell and Makefile checks' \
		'  make build             Build a release package for the current host' \
		'  make package-mac       Package macOS (ARCH=arm64|amd64)' \
		'  make package-linux     Package Linux (ARCH=arm64|amd64)' \
		'  make package-windows   Package Windows (WINDOWS_FORMAT=INSTALLER|PORTABLE|BOTH)' \
		'  make clean             Remove repository build artifacts'

deps:
	go mod download
	cd frontend && npm ci --prefer-offline --no-audit --no-fund

dev:
ifeq ($(IS_WINDOWS),1)
	cmd /C "bat\dev.bat live --no-pause"
else
	FRONTEND_PORT=$(FRONTEND_PORT) bash ./dev.sh live
endif

dev-stable:
ifeq ($(IS_WINDOWS),1)
	cmd /C "bat\dev.bat stable --no-pause"
else
	bash ./dev.sh stable
endif

frontend-dev:
	cd frontend && npm run dev:raw -- --host 127.0.0.1 --port $(FRONTEND_PORT)

frontend-build:
	cd frontend && npm run build

bindings: frontend-build
ifeq ($(IS_WINDOWS),1)
	cmd /C "bat\generate-bindings.bat --no-pause"
else
	wails generate module
endif

test-go:
	go test ./...

test-frontend:
	cd frontend && npm run build

test: test-go test-frontend

check: test
	bash -n dev.sh publish/mac/publish-mac.sh publish/linux/publish-linux.sh
	$(MAKE) --no-print-directory help >/dev/null

build:
ifeq ($(IS_WINDOWS),1)
	powershell -NoProfile -ExecutionPolicy Bypass -File bat/publish.ps1 -Target WINDOWS -WindowsFormat $(WINDOWS_FORMAT) $(if $(VERSION),-Version $(VERSION),)
else ifeq ($(UNAME_S),Darwin)
	bash publish/mac/publish-mac.sh --arch $(ARCH) $(if $(VERSION),--version $(VERSION),)
else ifeq ($(UNAME_S),Linux)
	bash publish/linux/publish-linux.sh --arch $(ARCH) $(if $(VERSION),--version $(VERSION),)
else
	@echo '[ERROR] unsupported build host: $(UNAME_S)' >&2; exit 1
endif

assemble-runtime:
ifeq ($(IS_WINDOWS),1)
	@echo '[INFO] Windows local runtime assembly is handled by the Windows packaging flow.'
else
	bash tools/build/assemble-local-runtime.sh
endif

build-mac:
	@test "$(UNAME_S)" = "Darwin" || { echo '[ERROR] build-mac must run on macOS.' >&2; exit 1; }
	@test "$(ARCH)" != "unknown" || { echo '[ERROR] unsupported host architecture.' >&2; exit 1; }
	wails build -s -platform darwin/$(ARCH) -o ant-chrome
	bash tools/build/assemble-local-runtime.sh darwin-$(ARCH)

build-linux:
	@test "$(UNAME_S)" = "Linux" || { echo '[ERROR] build-linux must run on Linux.' >&2; exit 1; }
	@test "$(ARCH)" != "unknown" || { echo '[ERROR] unsupported host architecture.' >&2; exit 1; }
	wails build -s -platform linux/$(ARCH) -o ant-chrome
	bash tools/build/assemble-local-runtime.sh linux-$(ARCH)

build-windows:
ifeq ($(IS_WINDOWS),1)
	wails build -platform windows/amd64
else
	@echo '[ERROR] build-windows must run on Windows.' >&2; exit 1
endif

package-mac:
	@test "$(UNAME_S)" = "Darwin" || { echo '[ERROR] package-mac must run on macOS.' >&2; exit 1; }
	bash publish/mac/publish-mac.sh --arch $(ARCH) $(if $(VERSION),--version $(VERSION),)

package-linux:
	@test "$(UNAME_S)" = "Linux" || { echo '[ERROR] package-linux must run on Linux.' >&2; exit 1; }
	bash publish/linux/publish-linux.sh --arch $(ARCH) $(if $(VERSION),--version $(VERSION),)

package-windows:
ifeq ($(IS_WINDOWS),1)
	powershell -NoProfile -ExecutionPolicy Bypass -File bat/publish.ps1 -Target WINDOWS -WindowsFormat $(WINDOWS_FORMAT) $(if $(VERSION),-Version $(VERSION),)
else
	@echo '[ERROR] package-windows must run on Windows with GNU Make installed.' >&2; exit 1
endif

clean:
	rm -rf build/bin frontend/dist publish/staging/mac publish/staging/linux
