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
| A person in a browser or on a phone | Cloudflare Access: GitHub login, an address given to `access:setup` | Cloudflare's edge, before the Worker | none |
| An MCP client (Claude's connectors) | Access as its OAuth server: the person logs in with GitHub | the edge | none |
| A machine: a report, CI, a job | its own Access service token (`CF-Access-Client-Id`, `CF-Access-Client-Secret`), made per machine, revoked alone | the edge, then the Worker ties it to its device | `MayPostFor` in `api/auth.go`, the `machines` table |
| An app or a person of a product | an access token from an OpenID Connect issuer (`Authorization: Bearer`), with scopes | the Worker, against the issuer's keys | none: charter's `go/auth` |
| Anything, for one release | the read or write token ([Tokens](../guides/tokens.md)) | the Worker; behind Access only with Access credentials too | none: charter's `go/auth` |

What reaches the Worker from Access is a JWT in `Cf-Access-Jwt-Assertion`, signed by the team's keys (`https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`, RS256). For a person it holds `email`; for a service token `common_name` is the token's Client ID and `sub` is empty ([Cloudflare: application token](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/application-token/)). The Worker checks it anyway: the signature, the issuer, the application's AUD tag and the times.

## Scopes, in the contract

Each operation's `Security` in `api/contract.go` names the scope it needs (`auth.Needs`); `api/auth.go` holds the scopes and who is granted them (`Trusted`); `api/spec.go` declares the schemes (`auth.Scheme`, `auth.AccessScheme`, `auth.OIDCScheme`). The spec shows both, so the SDKs know what to send.

| Scope | Operations | Granted to |
|---|---|---|
| `devices:read` | `listDevices`, `getDevice`, `listDeviceReports` | a person (Access), any service token, the read and write tokens, an OIDC token with the scope |
| `devices:write` | `postDeviceReport` | a machine's service token, for its own device only; the write token; an OIDC token with the scope |
| `devices:forget` | `deleteDevice` | a person (Access), the write token, an OIDC token with the scope |
| none | `hello` | anyone who gets past the edge |

## Enforced from the contract

1. Charter's `go/auth` middleware (`auth.Middleware`) verifies what the request carries, locally (the issuers' keys are fetched once and kept): the Access JWT (`ACCESS_TEAM_DOMAIN`, `ACCESS_AUD`), a bearer JWT from the OpenID Connect issuer (`OIDC_ISSUER`, whose keys it finds by its discovery document, and `OIDC_AUDIENCE`), or the read or write token. Then it reads the operation's `Security`: a caller with the scope it needs is let in. No credentials or bad ones: 401 with `WWW-Authenticate: Bearer`. Known but without the scope: 403.
2. Then the rule a scope cannot say (`MayPostFor` in `api/auth.go`, called by the post handler): a machine's token posts only for the device it was enrolled for, found by its Client ID (`auth.CallerOf(ctx).Machine`). A relation check against a Zanzibar engine (the owner's `authz-core`: may this caller do this to that object?) goes in the same place when there are more such rules.

An MCP tool call (`/api/mcp`) runs the same operation as its REST route, with the caller's `Authorization` header.

## People of a product: an OpenID Connect issuer

The contract declares an `openIdConnect` scheme at `/.well-known/openid-configuration` on the API; the Worker sends a client on to the issuer it trusts (`auth.Discovery`). Which issuer is the deployment's choice: none is set on the deployed Worker, and the local checks trust a test issuer they make (`test/auth-test.mjs`). An issuer must offer:

- discovery (`/.well-known/openid-configuration`, with `jwks_uri`) and a JWKS with RS256, ES256 or Ed25519 (EdDSA) keys;
- access tokens as JWTs whose `aud` is this API and whose `scope` holds the scopes above;
- the authorization code flow with PKCE, and for CLIs the device flow.

## Limits

- Behind Access, a bearer token alone never reaches the Worker: the edge refuses it. The bearer tokens work only locally, or with Access credentials beside them.
- Fern's TypeScript SDK sends only one of the two service-token headers by itself (fern-api/fern#17775): give it both as `headers` with `auth: false` ([Access](../guides/access.md#from-the-sdks)). The Go SDK sends both from `FLEET_API_ACCESS_CLIENT_ID` and `FLEET_API_ACCESS_CLIENT_SECRET`.
- Access's OAuth for MCP clients is beta at Cloudflare; a client must support RFC 8707. Charter's `access` tasks do not keep it on the application ([Access](../guides/access.md#protect-the-worker)).
- An MCP tool call passes on only the `Authorization` header (charter's `go/humamcp`), so a caller that comes through Access (a person's login, a machine's service token) is 401 inside a tool call; a bearer token works.
