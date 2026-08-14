GO      ?= go
NPM     ?= npm
WAILS   ?= wails

.PHONY: build test vet test-ui e2e desktop build-desktop install clean

## Build the CLI + backend
build:
	$(GO) build ./...

## Backend tests + vet
test:
	$(GO) build ./...
	$(GO) vet ./...
	$(GO) test ./...

## Frontend type check + e2e (Playwright, vite dev + mock backend)
test-ui:
	cd desktop/frontend && $(NPM) run test && $(NPM) run test:e2e

## e2e only
e2e:
	cd desktop/frontend && $(NPM) run test:e2e

## Build the native desktop app (macOS .app)
desktop: build-desktop

build-desktop:
	cd desktop && $(WAILS) build

## Build + install to /Applications
install: build-desktop
	rm -rf /Applications/GizNews.app
	cp -R desktop/build/bin/GizNews.app /Applications/

## Remove build artifacts
clean:
	rm -rf desktop/build/bin desktop/frontend/dist desktop/frontend/test-results
