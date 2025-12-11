GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "*/.git/*")
ROOT_FAST_PACKAGES := $(shell go list ./... | grep -v '/tests/integration$$')
ROOT_SLOW_PACKAGES := $(shell go list ./... | grep '/tests/integration$$')
MODULE_DIRS := .
RELEASE_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
RELEASE_DIRECTORY := dist
RELEASE_BINARY_NAME := pinguin
STATICCHECK_MODULE := honnef.co/go/tools/cmd/staticcheck@master
INEFFASSIGN_MODULE := github.com/gordonklaus/ineffassign@latest
SHORT_TIMEOUT := timeout -k 30s -s SIGKILL 30s
LONG_TIMEOUT := timeout -k 350s -s SIGKILL 350s

.PHONY: format check-format lint test test-unit test-integration test-fast test-slow test-frontend build release ci

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

test-frontend:
	CI=1 $(LONG_TIMEOUT) npm test

build:
	mkdir -p bin
	$(LONG_TIMEOUT) go build -o bin/$(RELEASE_BINARY_NAME) ./cmd/server

release:
	rm -rf $(RELEASE_DIRECTORY)
	mkdir -p $(RELEASE_DIRECTORY)
	for target in $(RELEASE_TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		output_path=$(RELEASE_DIRECTORY)/$(RELEASE_BINARY_NAME)-$$os-$$arch; \
		echo "Building $$output_path"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(LONG_TIMEOUT) go build -o $$output_path ./cmd/server; \
	done

ci: check-format lint test test-frontend
