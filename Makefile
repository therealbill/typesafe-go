MODULE   := github.com/therealbill/typesafe-go
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)

.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.PHONY: help test lint vuln build release integration review docs site site-serve clean

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

release: ## Tag and push a release: make release VERSION=vX.Y.Z (runs lint and test first)
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || (echo "release: VERSION must look like v1.2.3, got '$(VERSION)'" && exit 1)
	@test -z "$$(git status --porcelain)" || (echo "release: working tree is not clean" && exit 1)
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "main" || (echo "release: not on main" && exit 1)
	@git fetch -q origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "release: main is not up to date with origin/main" && exit 1)
	@! git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null || (echo "release: tag $(VERSION) already exists locally" && exit 1)
	@! git ls-remote --exit-code --tags origin "$(VERSION)" >/dev/null 2>&1 || (echo "release: tag $(VERSION) already exists on origin" && exit 1)
	$(MAKE) lint test
	git tag -a "$(VERSION)" -m "typesafe-go $(VERSION)"
	git push origin "$(VERSION)"
	@echo "release: pushed $(VERSION); the release workflow builds and publishes the binaries"

integration: ## Run the live API test (needs TYPESAFE_API_KEY)
	go test -run Integration -v .

review: build ## Run jev review against this repository (needs TYPESAFE_API_KEY)
	./bin/jev review

docs: site ## Build the site and check that every docs/ page is reachable and links resolve
	@./tools/checkdocs.sh

site: ## Build the documentation site into site/public
	cd site && hugo --gc --minify

site-serve: ## Serve the documentation site locally with live reload
	cd site && hugo server --buildDrafts --navigateToChanged

clean: ## Remove build outputs
	rm -rf bin dist coverage.out jev-review-report.json site/public site/resources
