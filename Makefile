MPRLAB_GATEWAY_EXECUTABLE ?= mprlab-gateway
GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "*/.git/*")
UNIT_PACKAGES ?= ./...
INTEGRATION_PACKAGES ?= ./...
GO_TEST_FLAGS ?=
MODULE_DIRS := .
BINARY_NAME := pingwin
COMPOSE_PROFILE ?= dev
DOCKER_COMPOSE ?= docker compose
STATICCHECK_MODULE := honnef.co/go/tools/cmd/staticcheck@master
INEFFASSIGN_MODULE := github.com/gordonklaus/ineffassign@latest
SHORT_TIMEOUT := timeout -k 30s -s SIGKILL 30s
LONG_TIMEOUT := timeout -k 350s -s SIGKILL 350s
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REQUIRED_TOTAL ?= 100.0%

.PHONY: test-shared-ui format check-format lint test test-unit test-integration test-coverage test-frontend test-frontend-update build up down ci deploy

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
		( cd $$dir && $(LONG_TIMEOUT) go vet -tags=integration ./... ); \
		echo "Running staticcheck in $$dir"; \
		( cd $$dir && $(LONG_TIMEOUT) go run $(STATICCHECK_MODULE) -tags=integration ./... ); \
		echo "Running ineffassign in $$dir"; \
		( cd $$dir && $(LONG_TIMEOUT) go run $(INEFFASSIGN_MODULE) ./... ); \
	done

test-unit:
	$(LONG_TIMEOUT) go test $(GO_TEST_FLAGS) $(UNIT_PACKAGES)

test-integration:
	$(LONG_TIMEOUT) go test -tags=integration -run '^TestIntegration' $(GO_TEST_FLAGS) $(INTEGRATION_PACKAGES)

test: test-unit

test-coverage:
	$(LONG_TIMEOUT) go test -coverprofile=$(COVERAGE_PROFILE) -covermode=count $(GO_TEST_FLAGS) $(UNIT_PACKAGES)
	@coverage_total="$$(go tool cover -func=$(COVERAGE_PROFILE) | awk '/^total:/ {print $$3}')"; \
	if [ "$$coverage_total" != "$(COVERAGE_REQUIRED_TOTAL)" ]; then \
		echo "Expected total Go statement coverage $(COVERAGE_REQUIRED_TOTAL), got $$coverage_total"; \
		exit 1; \
	fi; \
	echo "Total Go statement coverage $$coverage_total"

test-frontend:
	CI=1 $(LONG_TIMEOUT) npm test

test-frontend-update:
	CI=1 $(LONG_TIMEOUT) npm test -- --update-snapshots=all

build:
	mkdir -p bin
	$(LONG_TIMEOUT) go build -o bin/$(BINARY_NAME) ./cmd/server

.PHONY: generate-proto
generate-proto:
	$(SHORT_TIMEOUT) protoc --go_out=. --go_opt=module=github.com/tyemirov/pingwin --go-grpc_out=. --go-grpc_opt=module=github.com/tyemirov/pingwin pkg/proto/pingwin.proto

up:
	$(LONG_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) up -d --build

down:
	$(SHORT_TIMEOUT) $(DOCKER_COMPOSE) --profile $(COMPOSE_PROFILE) down

ci: check-format lint test-coverage

.PHONY: deploy

deploy:
	@set -eu; \
	export CI=1; \
	application_root="$$(git rev-parse --show-toplevel)"; \
	if ! command -v "$(MPRLAB_GATEWAY_EXECUTABLE)" >/dev/null 2>&1; then \
		printf 'Gateway runtime is unavailable: %s. Install a released runtime and add its command directory to PATH.\n' "$(MPRLAB_GATEWAY_EXECUTABLE)" >&2; \
		exit 2; \
	fi; \
	exec "$(MPRLAB_GATEWAY_EXECUTABLE)" deploy --app-root "$${application_root}" </dev/null

test-shared-ui:
	CI=1 $(LONG_TIMEOUT) npm test -- shared-ui-candidate.spec.ts

.PHONY: test-installed-gateway test-deployment-contract test-installed-plan
test-installed-gateway:
	bash tests/installed-gateway.sh

test-deployment-contract:
	$(LONG_TIMEOUT) go test ./tests/config -run '^(TestProductionLifecycleUsesInstalledGateway|TestRepositoryOwnsCompleteVersionlessDeployment)$$' -count=1

test-installed-plan:
	$(LONG_TIMEOUT) go test -tags=integration ./tests/config -run '^TestIntegrationInstalledGatewayPlansCurrentTrackedManifest$$' -count=1

TEST_IMAGE ?= pingwin-test:local
.PHONY: test-image
test-image:
	$(LONG_TIMEOUT) docker build -t $(TEST_IMAGE) .
	$(SHORT_TIMEOUT) docker run --rm --entrypoint pingwin $(TEST_IMAGE) --help
	$(SHORT_TIMEOUT) docker run --rm --entrypoint pingwin-doctor $(TEST_IMAGE) --help
	$(SHORT_TIMEOUT) docker run --rm --entrypoint /bin/sh $(TEST_IMAGE) -c 'test ! -e /usr/local/bin/pingwin-convert-managed-tenants'

.PHONY: test-delivery test-delivery-contract test-delivery-coverage test-delivery-frontend
test-delivery:
	$(LONG_TIMEOUT) go test ./internal/delivery ./internal/httpapi ./internal/config ./cmd/server -run 'Delivery|CloudWatch' -count=1

test-delivery-contract:
	$(LONG_TIMEOUT) go test ./tests/config -run '^(TestGORMQueriesUseStructuredClauses|TestServiceEnvExamplesCoverConfigTemplates|TestServerRuntimeEnvironmentAccessIsCentralized|TestRepositoryOwnsCompleteVersionlessDeployment|TestProductionLifecycleUsesInstalledGateway)$$' -count=1

test-delivery-coverage:
	$(LONG_TIMEOUT) go test ./internal/delivery ./internal/httpapi -coverprofile=$(COVERAGE_PROFILE) -run 'Delivery|CloudWatch' -count=1

test-delivery-frontend:
	CI=1 $(LONG_TIMEOUT) npm test -- email-delivery.spec.ts dashboard.spec.ts ux.spec.ts
