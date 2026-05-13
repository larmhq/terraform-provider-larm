# Security Policy

## Reporting a vulnerability

Email **security@larm.dev** with reproduction steps and affected versions. Please do not file public GitHub issues for security reports.

## Supported versions

During the `0.x` series, only the latest minor release receives security fixes. After `1.0`, the latest two minor releases will receive fixes.

## Scope

This provider talks to the Larm API on behalf of Terraform users. Vulnerabilities of interest include:

- Token leakage (logging credentials, sending to wrong endpoints)
- TLS / transport issues introduced by the provider
- Privilege escalation via provider-managed state

For vulnerabilities in the Larm backend or the `larm-go` SDK, see the security policies of those projects.
