# terraform-provider-larm

Official Terraform provider for [Larm](https://larm.dev) — uptime monitoring and status pages.

## Status

Pre-1.0. The schema may change in minor releases until 1.0. Pin to a specific version.

## Installation

```hcl
terraform {
  required_providers {
    larm = {
      source  = "larmhq/larm"
      version = "~> 0.1"
    }
  }
}

provider "larm" {
  # api_key can also be set via the LARM_API_KEY environment variable
  api_key = var.larm_api_key
}
```

## Usage

```hcl
resource "larm_monitor" "homepage" {
  name       = "Homepage"
  check_type = "http"
  config = jsonencode({
    url                   = "https://larm.dev"
    method                = "GET"
    expected_status_codes = [200]
    follow_redirects      = true
  })
}
```

See the [documentation on the Terraform Registry](https://registry.terraform.io/providers/larmhq/larm/latest/docs) for the full reference.

## Reporting security issues

See [SECURITY.md](SECURITY.md).

## License

[MPL-2.0](LICENSE).
