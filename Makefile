GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "*/.git/*")
ROOT_FAST_PACKAGES := $(shell go list ./... | grep -v '/tests/integration$$')
ROOT_SLOW_PACKAGES := $(shell go list ./... | grep '/tests/integration$$')
MODULE_DIRS := .
BINARY_NAME := pinguin
COMPOSE_PROFILE ?= dev
DOCKER_COMPOSE ?= docker compose
STATICCHECK_MODULE := honnef.co/go/tools/cmd/staticcheck@master
INEFFASSIGN_MODULE := github.com/gordonklaus/ineffassign@latest
SHORT_TIMEOUT := timeout -k 30s -s SIGKILL 30s
LONG_TIMEOUT := timeout -k 350s -s SIGKILL 350s
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REQUIRED_TOTAL ?= 100.0%

.PHONY: format check-format lint test test-unit test-integration test-fast test-slow test-coverage test-frontend build up down ci release publish deploy

format:
	$(SHORT_TIMEOUT) gofmt -w $(GO_SOURCES)

check-format:
	@formatted_files="$$( $(SHORT_TIMEOUT) gofmt -l $(GO_SOURCES) )"; \
	if [ -n "$$formatted_files" ]; then \
		echo "Go files require formatting:"; \
		echo "$$formatted_files"; \
		exit 1; \
	fi

lint:
	@set -e; \
	for dir in $(MODULE_DIRS); do \
		echo "Running go vet in $$dir"; \
		( cd $$dir && $(LONG_TIMEOUT) go vet ./... ); \
		echo "Running staticcheck in $$dir"; \
		( cd $$dir && $(LONG_TIMEOUT) go run $(STATICCHECK_MODULE) ./... ); \
		echo "Running ineffassign in $$dir"; \
		( cd $$dir && $(LONG_TIMEOUT) go run $(INEFFASSIGN_MODULE) ./... ); \
	done

test-fast:
	$(LONG_TIMEOUT) go test $(ROOT_FAST_PACKAGES)

test-slow:
ifneq ($(strip $(ROOT_SLOW_PACKAGES)),)
	$(LONG_TIMEOUT) go test $(ROOT_SLOW_PACKAGES)
else
	@echo "No slow test packages detected"
endif

test-unit: test-fast

test-integration: test-slow

test: test-fast test-slow

test-coverage:
	$(LONG_TIMEOUT) go test ./... -coverprofile=$(COVERAGE_PROFILE) -covermode=count
	@coverage_total="$$(go tool cover -func=$(COVERAGE_PROFILE) | awk '/^total:/ {print $$3}')"; \
	if [ "$$coverage_total" != "$(COVERAGE_REQUIRED_TOTAL)" ]; then \
		echo "Expected total Go statement coverage $(COVERAGE_REQUIRED_TOTAL), got $$coverage_total"; \
		exit 1; \
	fi; \
	echo "Total Go statement coverage $$coverage_total"

test-frontend:
	CI=1 $(LONG_TIMEOUT) npm test

build:
	mkdir -p bin
	$(LONG_TIMEOUT) go build -o bin/$(BINARY_NAME) ./cmd/server

up:
	$(LONG_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) up -d --build

down:
	$(SHORT_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) down

ci: check-format lint test test-coverage test-frontend

.PHONY: release publish deploy

release publish deploy:
	@application_root="$$(git rev-parse --show-toplevel)"; \
	gateway_root="$$(dirname "$${application_root}")/mprlab-gateway"; \
	if [ ! -d "$${gateway_root}" ]; then \
		printf "required sibling gateway is missing: %s; clone mprlab-gateway at exactly %s\n" \
			"$${gateway_root}" "$${gateway_root}" >&2; \
		exit 2; \
	fi; \
	$(MAKE) --no-print-directory -C "$${gateway_root}" "app-$@" \
		MPRLAB_APP_ROOT="$${application_root}"
