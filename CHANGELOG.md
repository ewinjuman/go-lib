# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v2.2.0] - 2026-07-15

### Added

- **`oauth` package** — reusable multi-provider OAuth/OIDC toolkit:
  - `Provider` interface (`AuthCodeURL`, `Exchange`, `FetchUserInfo`, `VerifyIDToken`, `SupportsIDToken`) with `Registry` lookup by provider name; unregistered names return a 404 `*ApplicationError`.
  - PKCE helper (`NewPKCE`, RFC 7636 S256 challenge) — nil-safe for providers that don't support it.
  - **Generic OIDC provider** (`NewOIDCProvider`) via issuer discovery — covers Google (`NewGoogleProvider` convenience wrapper) and any OIDC-compliant issuer (Microsoft Entra, Okta, Keycloak, ...). id_token verification includes signature, audience, and optional nonce checks.
  - **GitHub provider** (`NewGitHubProvider`) — REST-based (`/user` with `/user/emails` fallback to the verified primary email); GitHub is not OIDC-compliant, so `VerifyIDToken` returns an error and `SupportsIDToken()` is `false`. Outbound requests use a bounded HTTP timeout.
  - **Facebook provider** (`NewFacebookProvider`) — Graph API `/me` (id, name, email, picture); email presence implies verified (Facebook only returns confirmed addresses).
- **`cache` package** — Redis toolkit: `RedisClient` wrapping `redis.UniversalClient` (standalone / Sentinel / Cluster), typed `Store[T]`, stampede-protected `ObjectCache[T]` (in-process singleflight + cross-pod distributed lock), `Lock` (SETNX with token-checked Lua unlock), and cacheability `Policy` (`PolicyAll`/`PolicyNone`/`PolicyKeys`/`PolicyFunc`). Returns plain `error` — no `apperror`/`logger` coupling.
- **`database` package** — GORM connection factory (`NewConnection`) for MySQL/PostgreSQL with ping-on-connect and go-lib logger integration for query logging.

### Security

- Upgraded `github.com/jackc/pgx/v5` v5.6.0 → **v5.9.2** — fixes [GO-2026-5004](https://pkg.go.dev/vuln/GO-2026-5004) (SQL injection via placeholder confusion with dollar-quoted string literals).
- Bumped Go toolchain to **go1.26.5** — picks up the `crypto/tls` fix from the Go standard library.

### Dependencies

- New: `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3` (and `github.com/go-jose/go-jose/v4`, test-only, for minting signed test JWTs).

[v2.2.0]: https://github.com/ewinjuman/go-lib/compare/v2.1.0...v2.2.0
