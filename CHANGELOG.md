# Changelog

All notable changes to this package. Versions follow [Semantic Versioning](https://semver.org).

## 1.1.0 — 2026-10-29

### Added
- `Error.Code`: the API's machine-readable error code (e.g. `monthly_quota_exceeded`). Branch on it, not on the message.
- `SendParams.Type` with `TypeTransactional` / `TypeMarketing`.

### Changed
- Quota limits are no longer retried. `daily_quota_exceeded` and `warmup_limit_exceeded` (and any 429 whose `Retry-After` is over 60 seconds) are returned at once with their code, instead of the client waiting, possibly for hours, before failing. Ordinary rate limits are still retried.

### Unchanged
- Minimum Go version stays 1.21.

## 1.0.0

Attachments; module path `github.com/Quolle-main/quolle-go`.
