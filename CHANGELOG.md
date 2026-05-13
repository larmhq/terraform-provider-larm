# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - TBD

### Added

- Initial release: provider `larmhq/larm` for the [Larm](https://larm.dev) uptime monitoring platform.
- Resource `larm_monitor` — manages monitors of any check type (http, tcp, dns, heartbeat, synthetic). Polymorphic `config` field uses `jsonencode({...})` and is compared semantically (no spurious diffs on formatting). Importable by ID.
- Provider config: `endpoint` (optional, defaults to `https://app.larm.dev/api/v1`) and `api_key` (required, also resolved from `LARM_API_KEY`).
