# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0]

### Added

- Resource `larm_status_page` — manages a status page and its full structure (groups, components, monitor links) as one resource. The `components` attribute is the ordered tree shown on the page; top-level entries are either groups (which contain components) or ungrouped components, and they interleave in the order written. CRUD goes through the backend's atomic `PUT /status-pages/:id/structure` endpoint so reorder, move-between-groups, and add/remove all apply as a single transaction. Importable by ID.

### Notes

- `slug` is mutable in-place to match the API. Replacement would cascade-destroy components, subscribers, and any linked custom domain, so an in-place rename is the correct (less destructive) behavior. Consumers of the public URL should still treat slug changes as a contract break.
- `down_status` on monitor links defaults to `major_outage` and is validated against the closed set `degraded_performance | partial_outage | major_outage` to fail at `terraform plan` instead of on apply.

## [0.1.0]

### Added

- Initial release: provider `larmhq/larm` for the [Larm](https://larm.dev) uptime monitoring platform.
- Resource `larm_monitor` — manages monitors of any check type (http, tcp, dns, heartbeat, synthetic). Polymorphic `config` field uses `jsonencode({...})` and is compared semantically (no spurious diffs on formatting). Importable by ID.
- Provider config: `endpoint` (optional, defaults to `https://app.larm.dev/api/v1`) and `api_key` (required, also resolved from `LARM_API_KEY`).
