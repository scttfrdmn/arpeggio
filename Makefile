BINARY  := arpd
PKG     := ./cmd/arpd
GOFLAGS := -trimpath

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Resolve and pin dependencies
	go mod tidy

.PHONY: build
build: ## Build the API binary for Lambda (arm64)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/bootstrap $(PKG)

.PHONY: test
test: ## Run unit tests
	go test ./... -race -count=1

.PHONY: lint
lint: ## Vet and format check
	go vet ./...
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "gofmt needed"; exit 1)

.PHONY: check
check: lint test ## Everything CI runs

.PHONY: package
package: build ## Zip the Lambda bundle
	cd bin && zip -q -j ../dist/arpd.zip bootstrap

.PHONY: deploy
deploy: ## Deploy the control plane (see deploy/README.md)
	sam deploy --template-file deploy/template.yaml --guided

.PHONY: gh-bootstrap
gh-bootstrap: ## Create GitHub labels, milestones, and P0 issues
	./scripts/gh-bootstrap.sh
