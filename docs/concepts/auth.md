---
title: Auth and authz
nav_order: 1
parent: Concepts
---

# Auth and authz: who calls, how they prove it, what they may do

The contract says it once; the edge, the Worker and the SDKs follow it. Setting it up: [Access](../guides/access.md).

## Who calls, and how they prove it

| Caller | Proves who it is with | Where it is checked | Code in this repo |
|---|---|---|---|
| A person in a browser or on a phone | Cloudflare Access: GitHub login, an address in `FLEET_API_ACCESS_EMAILS` | Cloudflare's edge, before the Worker | none |
| An MCP client (Claude's connectors) | Access as its OAuth server: the person logs in with GitHub | the edge | none |
| A machine: a report, CI, a job | its own Access service token (`CF-Access-Client-Id`, `CF-Access-Client-Secret`), made per machine, revoked alone | the edge, then the Worker ties it to its device | the `machines` table |
| An app or a person of a product | an access token from an OpenID Connect provider (`Authorization: Bearer`), with scopes | the Worker, against the provider's keys | `authn/` |
| Anything, for one release | the read or write token ([Tokens](../guides/tokens.md)) | the Worker; behind Access only with Access credentials too | `api/auth.go` |

What reaches the Worker from Access is a JWT in `Cf-Access-Jwt-Assertion`, signed by the team's keys (`https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`, RS256). For a person it holds `email`; for a service token `common_name` is the token's Client ID and `sub` is empty ([Cloudflare: application token](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/application-token/)). The Worker checks it anyway: the signature, the issuer, the application's AUD tag and the times.

## Scopes, in the contract

Each operation's `Security` in `api/contract.go` names the scope it needs; `api/auth.go` holds the schemes and the scopes. The spec shows both, so the SDKs know what to send.

| Scope | Operations | Granted to |
|---|---|---|
| `devices:read` | `listDevices`, `getDevice`, `listDeviceReports` | a person (Access), any service token, the read and write tokens, an OIDC token with the scope |
| `devices:write` | `postDeviceReport` | a machine's service token, for its own device only; the write token; an OIDC token with the scope |
| `devices:forget` | `deleteDevice` | a person (Access), the write token, an OIDC token with the scope |
| none | `hello` | anyone who gets past the edge |

## Enforced from the contract

1. `authn` verifies what the request carries, locally (the issuer's keys are fetched once and kept): the Access JWT, or a bearer JWT from the OIDC provider (`OIDC_ISSUER`, `OIDC_JWKS`, `OIDC_AUDIENCE`), or the read or write token.
2. One Huma middleware reads the operation's `Security`: any requirement whose scheme vouched for the caller, with its scope. No credentials or bad ones: 401 with `WWW-Authenticate` naming `/.well-known/oauth-protected-resource`. Known but not allowed: 403 with `error="insufficient_scope"`.
3. Then the rules a scope cannot say (`rules` in `api/auth.go`): a machine's token posts only for the device it was enrolled for. A relation check against a Zanzibar engine (the owner's `authz-core`: may this caller do this to that object?) goes in the same place when there are more such rules.

An MCP tool call (`/api/mcp`) runs the same operation as its REST route, as the same caller.

## People of a product: an OpenID Connect provider

The contract declares an `openIdConnect` scheme at `/.well-known/openid-configuration` on the API; the Worker sends a client on to the provider it trusts and serves `/.well-known/oauth-protected-resource` (RFC 9728) naming it. Which provider is the deployment's choice: none is set on the deployed Worker, and the local checks trust a test issuer they make (`test/auth-test.mjs`). A provider must offer:

- a JWKS with RS256 or Ed25519 (EdDSA) keys (`authn` leaves ES256 out: it would add more Wasm than both of those, [Benchmarks](../benchmarks.md));
- access tokens as JWTs whose `aud` is this API and whose `scope` holds the scopes above;
- discovery (`/.well-known/openid-configuration`), the authorization code flow with PKCE, and for CLIs the device flow.

## Limits

- Behind Access, a bearer token alone never reaches the Worker: the edge refuses it. The bearer tokens work only locally, or with Access credentials beside them.
- Fern's TypeScript SDK sends only one of the two service-token headers by itself (fern-api/fern#17775): give it both as `headers` with `auth: false` ([Access](../guides/access.md#from-the-sdks)). The Go SDK sends both from `FLEET_API_ACCESS_CLIENT_ID` and `FLEET_API_ACCESS_CLIENT_SECRET`.
- Access's OAuth for MCP clients is beta at Cloudflare; a client must support RFC 8707.
