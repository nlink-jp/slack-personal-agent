VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X 'main.version=$(VERSION)'

.PHONY: build build-notifier dev test clean

build: build-notifier
	wails build -ldflags "$(LDFLAGS)" -tags no_duckdb_arrow
	mkdir -p dist
	cp -r build/bin/slack-personal-agent.app dist/
	# Bundle notifier helper inside .app
	mkdir -p dist/slack-personal-agent.app/Contents/Resources
	cp notifier/.build/release/spa-notify dist/slack-personal-agent.app/Contents/Resources/

build-notifier:
	cd notifier && swift build -c release

dev:
	wails dev -tags no_duckdb_arrow

test:
	go test -tags no_duckdb_arrow ./...

clean:
	rm -rf build/bin dist
	cd notifier && swift package clean
