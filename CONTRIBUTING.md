# Contributing

## Setup

```sh
mise install
go mod tidy
```

## Local dev loop

```sh
make build         # compile
make generate      # tfplugindocs → docs/
make test          # unit tests
make lint          # golangci-lint
make testacc       # acceptance tests (requires LARM_API_KEY + a reachable backend)
```

Run `make build test lint` before pushing. Run `make generate` if you've changed the resource schema, and commit the regenerated `docs/`.

## End-to-end with a local Larm backend

```sh
# Install provider locally for terraform to pick up via dev_overrides:
cat > ~/.terraformrc <<EOF
provider_installation {
  dev_overrides { "larmhq/larm" = "$(go env GOPATH)/bin" }
  direct {}
}
EOF
make install

# Try it:
cd $(mktemp -d)
cat > main.tf <<EOF
terraform { required_providers { larm = { source = "larmhq/larm" } } }
provider "larm" {
  endpoint = "http://localhost:4000/api/v1"
  api_key  = "<dev key>"
}
EOF
terraform plan
```

## Releases

Releases are cut by tagging a commit on `main`:

```sh
git tag v0.x.y
git push --tags
```

The release workflow imports the GPG signing key, verifies the build via `make build test lint`, runs `goreleaser release --clean`, and publishes signed artifacts that the Terraform Registry picks up automatically via its repo webhook.
