VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X 'main.version=$(VERSION)'

.PHONY: build dev test clean

build:
	wails build -ldflags "$(LDFLAGS)" -tags no_duckdb_arrow
	mkdir -p dist
	cp -r build/bin/slack-personal-agent.app dist/

dev:
	wails dev -tags no_duckdb_arrow

test:
	go test -tags no_duckdb_arrow ./...

clean:
	rm -rf build/bin dist
