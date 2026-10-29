# Changelog

All notable changes to this package. Versions follow [Semantic Versioning](https://semver.org).

## 1.2.0 — 2026-10-29

### Added
- `client.Domains`: `List`, `Create`, `Get`, `Records`, `Verify`, `Remove`. Add a sending domain, get its DNS records, check DNS on demand, and see verification and warm-up state.
- `client.APIKeys`: `List`, `Create`, `Remove`. Create keys (full access or sending-only, optionally limited to some domains, optionally expiring) and revoke them.
- `client.Webhooks` manages endpoints as well as verifying deliveries: `List`, `Create`, `Get`, `Update`, `Remove`, `Test`, `Deliveries`, `Redeliver`, `RotateSecret`, `Events`.

These need a full-access API key; a sending-only key gets `restricted_api_key`. Requires Quolle API support for API-key access to `/v1/keys` and `/v1/webhooks` (live from 29 Oct 2026).

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
