MODULE   := github.com/therealbill/typesafe-go
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)

.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.PHONY: help test lint vuln build integration selfreview docs clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

test: ## Run unit tests with the race detector
	go test -race -cover ./...

lint: ## Run gofmt check, go vet, and golangci-lint
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: files need formatting" && exit 1)
	go vet ./...
	golangci-lint run ./...

vuln: ## Run govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

build: ## Build bin/jev
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/jev ./cmd/jev

integration: ## Run the live API test (needs TYPESAFE_API_KEY)
	go test -run Integration -v .

selfreview: build ## Run the Jev-driven self-review (needs TYPESAFE_API_KEY)
	go run ./tools/selfreview -jev ./bin/jev

docs: ## Check that every docs/ page is reachable from README.md and links resolve
	@./tools/checkdocs.sh

clean: ## Remove build outputs
	rm -rf bin dist coverage.out selfreview-report.json
