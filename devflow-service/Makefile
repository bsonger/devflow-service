APP ?= meta-service
DOCKER_IMAGE ?= $(APP):local
GOFMT_DIRS := cmd internal
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
GO_RUN_ENV := GOCACHE=$(GOCACHE)

.PHONY: fmt
fmt:
	@files="$$(find $(GOFMT_DIRS) -type f -name '*.go' 2>/dev/null)"; \
	if [ -n "$$files" ]; then gofmt -w $$files; fi

.PHONY: fmt-check
fmt-check:
	@files="$$(find $(GOFMT_DIRS) -type f -name '*.go' 2>/dev/null)"; \
	if [ -z "$$files" ]; then exit 0; fi; \
	out="$$(gofmt -l $$files)"; \
	if [ -n "$$out" ]; then printf '%s\n' "$$out"; exit 1; fi

.PHONY: vet
vet:
	mkdir -p $(GOCACHE)
	$(GO_RUN_ENV) go vet ./...

.PHONY: lint
lint:
	mkdir -p $(GOCACHE)
	mkdir -p $(GOLANGCI_LINT_CACHE)
	$(GO_RUN_ENV) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run ./...

.PHONY: test
test:
	mkdir -p $(GOCACHE)
	$(GO_RUN_ENV) go test ./...

.PHONY: build
build:
	mkdir -p bin
	mkdir -p $(GOCACHE)
	$(GO_RUN_ENV) go build -o bin/$(APP) ./cmd/$(APP)

.PHONY: package
package:
	mkdir -p bin
	mkdir -p $(GOCACHE)
	$(GO_RUN_ENV) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(APP) ./cmd/$(APP)

.PHONY: docker-build
docker-build:
	bash scripts/docker-build.sh $(DOCKER_IMAGE) Dockerfile

.PHONY: verify
verify:
	bash scripts/verify.sh

.PHONY: ci
ci: fmt-check vet lint test build docker-build verify

.PHONY: run
run:
	go run ./cmd/$(APP)

.PHONY: tidy
tidy:
	mkdir -p $(GOCACHE)
	$(GO_RUN_ENV) go mod tidy

.PHONY: clean
clean:
	rm -rf ./bin ./.cache
