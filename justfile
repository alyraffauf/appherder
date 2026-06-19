set dotenv-load

GOFLAGS := "-trimpath"
GOCACHE := `printf '%s/appherder/go-build' "${XDG_CACHE_HOME:-$HOME/.cache}"`

default:
    just --list

build: build-cli build-ui

build-cli:
    mkdir -p '{{GOCACHE}}'
    nix develop -c env GOCACHE='{{GOCACHE}}' GOSUMDB=off go build {{GOFLAGS}} -o ./appherder ./cmd/appherder

build-ui:
    mkdir -p '{{GOCACHE}}'
    nix develop -c env GOCACHE='{{GOCACHE}}' GOSUMDB=off go build {{GOFLAGS}} -tags gtk -o ./appherder-gui ./cmd/appherder-gui

test:
    mkdir -p '{{GOCACHE}}'
    nix develop -c env GOCACHE='{{GOCACHE}}' GOSUMDB=off go test ./...

test-ui:
    mkdir -p '{{GOCACHE}}'
    nix develop -c env GOCACHE='{{GOCACHE}}' GOSUMDB=off go test -tags gtk ./cmd/appherder-gui
