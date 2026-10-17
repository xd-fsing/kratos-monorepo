GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
# Fall back to "dev" outside a git checkout (the template ships as a plain
# directory).
VERSION=$(shell git describe --tags --always 2>/dev/null || echo dev)
# Services are discovered from the apps directory: scaffolding a new service
# (make service app=<name>) is picked up automatically on the next build.
SERVICES := $(shell ls apps)
# app selects one service for the api/config/service targets; empty = all.
app ?=

.PHONY: init
# init env
init:
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/bufbuild/buf/cmd/buf@latest

.PHONY: config
# generate internal proto (app=<name> for one service)
config:
	@if [ -z "$(app)" ]; then \
		buf generate --template buf.gen.config.yaml; \
	else \
		buf generate --template buf.gen.config.yaml --path apps/$(app)/internal/conf; \
	fi

.PHONY: api
# generate api proto (app=<name> for one service)
api:
	@if [ -z "$(app)" ]; then \
		buf generate --template buf.gen.yaml; \
	else \
		buf generate --template buf.gen.yaml --path api/$(app); \
	fi

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# generate all
all:
	make api
	make config
	make generate

.PHONY: service
# scaffold a new service (app=<name>)
service:
	@test -n "$(app)" || (echo "usage: make service app=<name>"; exit 1)
	./scripts/gen-service.sh $(app)

.PHONY: build
# build all services
build: $(addprefix build-,$(SERVICES))

.PHONY: build-%
# build one service
build-%:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o bin/$* ./apps/$*/cmd

.PHONY: run-%
# run one service
run-%:
	go run ./apps/$*/cmd -conf ./apps/$*/configs

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
