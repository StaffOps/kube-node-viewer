.PHONY: help clean hooks check verify lint prices catalog boilerplate licenses download coverage generate test

NO_COLOR=\033[0m
GREEN=\033[32;01m
YELLOW=\033[33;01m
RED=\033[31;01m
TEST_PKGS=./pkg/... ./cmd/...
# keep in step with .github/workflows/test.yaml
GOLANGCI_LINT_VERSION=v2.14.0

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[33m%-20s\033[0m %s\n", $$1, $$2}'

build: generate ## Build
	go build -ldflags="-s -w -X main.version=local -X main.builtBy=Makefile" ./cmd/kube-node-viewer

goreleaser: ## Release snapshot
	goreleaser build --snapshot --clean

download: ## Download dependencies
	go mod download
	go mod tidy

licenses: download ## Check licenses
	go-licenses check ./... --allowed_licenses=MIT,Apache-2.0,BSD-3-Clause,ISC \
	--ignore github.com/mattn/go-localereader # MIT

boilerplate: ## Add license headers
	go run hack/boilerplate.go ./

hooks: ## Use the repo git hooks (.githooks: pre-commit, commit-msg, pre-push)
	git config core.hooksPath .githooks
	@echo "git hooks: .githooks (pre-commit, commit-msg, pre-push)"

check: ## Before a push: what CI checks (main merged in, verify, secrets, guardrails, lint, build) and the next release version
	git fetch -q origin main
	@git merge-base --is-ancestor origin/main HEAD || { echo "main has moved: run 'git merge origin/main' first (CHANGELOG lines can land in a released section)"; exit 1; }
	scripts/verify.sh
	sh scripts/checks/content.sh --commits origin/main..HEAD
	@if command -v gitleaks >/dev/null; then gitleaks git --redact --no-banner --log-level warn --exit-code 1 --log-opts=origin/main..HEAD .; else echo "gitleaks not installed: secrets not checked (CI checks them): brew install gitleaks"; fi
	@t=$$(command -v dash || echo sh); echo "$$t scripts/checks/test-checks.sh"; $$t scripts/checks/test-checks.sh >/dev/null || $$t scripts/checks/test-checks.sh
	$(MAKE) lint
	scripts/build.sh
	@v=$$(python3 scripts/release-prepare.py); if [ -n "$$v" ] && ! git rev-parse -q --verify "refs/tags/v$$v" >/dev/null; then echo "check: ok; the next release (Monday, or on demand) is v$$v"; else echo "check: ok; no [Unreleased] entries: nothing to release"; fi

lint: ## Lint with the pinned golangci-lint (built on first use)
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

verify: boilerplate licenses download ## Format and Lint
	gofmt -w -s ./.
	$(MAKE) lint

coverage: ## Run tests w/ coverage
	go test -coverprofile=coverage.out $(TEST_PKGS)
	go tool cover -html=coverage.out

generate: ## Generate attribution
	# run generate twice, gen_licenses needs the ATTRIBUTION file or it fails.  The second run
	# ensures that the latest copy is embedded when we build.
	go generate ./...
	./hack/gen_licenses.sh
	go generate ./...
	$(MAKE) prices

KARPENTER_PRICING=https://raw.githubusercontent.com/aws/karpenter-provider-aws/main/pkg/providers/pricing

prices: ## Refresh the static on-demand price tables (from Karpenter; no credentials)
	for p in aws aws_cn aws_us_gov; do \
	  curl -fsSL "$(KARPENTER_PRICING)/zz_generated.pricing_$$p.go" -o "pkg/aws/zz_generated_$$p.pricing.go" || exit 1; \
	done
	sed -i'.bkup' 's/package pricing/package aws/' pkg/aws/zz_generated_*.pricing.go
	rm -f pkg/aws/*.bkup

catalog: ## Refresh the instance catalog from the AWS Price List (needs pricing:GetProducts)
	go run ./hack/catalog

clean: ## Clean artifacts
	rm -rf kube-node-viewer
	rm -rf dist/

test:
	go test -v $(TEST_PKGS)

