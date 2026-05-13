default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

generate:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name larm

# CI check: docs/ matches what tfplugindocs would generate from the current schema/examples.
check-generate: generate
	@if [ -n "$$(git diff --name-only docs/)" ]; then \
		echo "Generated docs are out of date. Run 'make generate' and commit."; \
		git diff docs/; \
		exit 1; \
	fi

fmt:
	go fmt ./...
	goimports -local github.com/larmhq/terraform-provider-larm -w .

test:
	go test -v -cover -race -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -race -timeout 30m ./...

lint:
	golangci-lint run

vuln:
	govulncheck ./...

verify:
	go mod verify

.PHONY: build install generate check-generate fmt test testacc lint vuln verify
