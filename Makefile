GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "*/.git/*")
ROOT_FAST_PACKAGES := $(shell go list ./... | grep -v '/tests/integration$$')
ROOT_SLOW_PACKAGES := $(shell go list ./... | grep '/tests/integration$$')
MODULE_DIRS := .
RELEASE_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
RELEASE_DIRECTORY := dist
RELEASE_BINARY_NAME := pinguin
DOCKER_IMAGE ?= ghcr.io/tyemirov/pinguin
DOCKER_TAG ?=
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64
DOCKER_BUILDX_BUILDER ?= pinguin-builder
DOCKERFILE ?= Dockerfile
DOCKER_BUILD_CONTEXT ?= .
RELEASE_ARGS ?=
RELEASE_HELPER := $(abspath $(CURDIR)/scripts/release/release_helper.py)
PUBLISH_RELEASE_ARGS ?=
DEPLOY_ARGS ?=
RELEASE_ARTIFACT_TARGETS ?= release-artifacts container-artifacts pages-artifact
RELEASE_TOOL_DIR := $(abspath $(CURDIR)/scripts/release)
PUBLISH_PLATFORMS ?= $(DOCKER_PLATFORMS)
PUBLISH_BRANCH ?= master
PUBLISH_REMOTE ?= origin
GATEWAY_DIR ?=
PAGES_URL ?= https://pinguin.mprlab.com/
PAGES_DIST_DIR ?= $(CURDIR)/.pages-dist
PAGES_PUBLISH_REMOTE ?= origin
PAGES_PUBLISH_BRANCH ?= gh-pages
PAGES_VERSION ?=
PAGES_DEPLOY_ARGS ?=
COMPOSE_PROFILE ?= dev
DOCKER_COMPOSE ?= docker compose
STATICCHECK_MODULE := honnef.co/go/tools/cmd/staticcheck@master
INEFFASSIGN_MODULE := github.com/gordonklaus/ineffassign@latest
SHORT_TIMEOUT := timeout -k 30s -s SIGKILL 30s
LONG_TIMEOUT := timeout -k 350s -s SIGKILL 350s
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REQUIRED_TOTAL ?= 100.0%

.PHONY: format check-format lint test test-release-pages test-unit test-integration test-fast test-slow test-coverage test-frontend build release release-artifacts container-artifacts pages-artifact publish-release publish deploy pages-build pages-deploy up down ci

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

test-release-pages:
	@python3 tests/release_pages_contract_test.py

test: test-release-pages test-fast test-slow

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
	$(LONG_TIMEOUT) go build -o bin/$(RELEASE_BINARY_NAME) ./cmd/server

release: ## Prepare a local repository release without publishing or deploying
	RELEASE_HELPER="$(RELEASE_HELPER)" RELEASE_ARTIFACT_TARGETS="$(RELEASE_ARTIFACT_TARGETS)" bash scripts/release.sh $(RELEASE_ARGS)

release-artifacts: ## Build local release binaries into the prepared artifact
	@test -n "$(RELEASE_ARTIFACT_DIR)" || { echo "error: RELEASE_ARTIFACT_DIR is required" >&2; exit 1; }
	rm -rf "$(RELEASE_ARTIFACT_DIR)/payloads/release-assets/bin"
	mkdir -p "$(RELEASE_ARTIFACT_DIR)/payloads/release-assets/bin"
	for target in $(RELEASE_TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		output_path="$(RELEASE_ARTIFACT_DIR)/payloads/release-assets/bin/$(RELEASE_BINARY_NAME)-$$os-$$arch"; \
		echo "Building $$output_path"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(LONG_TIMEOUT) go build -o $$output_path ./cmd/server; \
	done

container-artifacts:
	@"$(RELEASE_TOOL_DIR)/prepare_container_artifact.sh" --name pinguin --image "$(DOCKER_IMAGE)" --file "$(DOCKERFILE)" --context "$(DOCKER_BUILD_CONTEXT)" --platforms "$(PUBLISH_PLATFORMS)"

pages-artifact: pages-build
	@"$(RELEASE_TOOL_DIR)/prepare_pages_artifact.sh" --source "$(PAGES_DIST_DIR)" --domain pinguin.mprlab.com

publish-release:
	@RELEASE_HELPER="$(RELEASE_HELPER)" bash scripts/publish-release.sh $(PUBLISH_RELEASE_ARGS)

publish: publish-release
	@"$(RELEASE_TOOL_DIR)/publish_container_artifacts.sh"

deploy:
	@GATEWAY_DIR="$(GATEWAY_DIR)" DOCKER_IMAGE="$(DOCKER_IMAGE)" PAGES_URL="$(PAGES_URL)" PAGES_PUBLISH_REMOTE="$(PAGES_PUBLISH_REMOTE)" PAGES_PUBLISH_BRANCH="$(PAGES_PUBLISH_BRANCH)" ./scripts/deploy.sh $(DEPLOY_ARGS)

pages-build:
	@./scripts/build_pages_artifact.sh "$(PAGES_DIST_DIR)"

pages-deploy:
	@"$(RELEASE_TOOL_DIR)/deploy_pages_artifact.sh" --remote "$(PAGES_PUBLISH_REMOTE)" --branch "$(PAGES_PUBLISH_BRANCH)" --url "$(PAGES_URL)" $(if $(PAGES_VERSION),--version "$(PAGES_VERSION)") $(PAGES_DEPLOY_ARGS)

up:
	$(LONG_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) up -d --build

down:
	$(SHORT_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) down

ci: check-format lint test test-coverage test-frontend
