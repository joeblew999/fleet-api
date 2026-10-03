import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker: the fleet's device API, written in Go on workers-go and built with TinyGo.
// D1 keeps each machine's newest report and 7 days of history, and which device each machine's
// service token posts for. The secrets are set on Cloudflare (docs/guides/tokens.md,
// docs/guides/access.md), from the environment under cf dev.
// A mode that starts with "perf-" (charter perf) deploys a scratch Worker of that name, with a
// database of its own.
export default defineConfig(ctx => {
	const name = ctx.mode?.startsWith("perf-") ? `fleet-api-${ctx.mode}` : "fleet-api";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			entrypoint,
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text(name),
				DB: bindings.d1(),
				READ_TOKEN: bindings.secret(),
				WRITE_TOKEN: bindings.secret(),
				// Cloudflare Access in front of the Worker (docs/concepts/auth.md): set by mise run access:setup.
				ACCESS_TEAM_DOMAIN: bindings.secret(),
				ACCESS_AUD: bindings.secret(),
				// An OpenID Connect provider for people and apps, when there is one: from the environment
				// as the Worker is built or run, so only a configuration that sets them trusts one (the
				// local checks set a test issuer's).
				...(process.env.OIDC_ISSUER ? {
					OIDC_ISSUER: bindings.text(process.env.OIDC_ISSUER),
					OIDC_JWKS: bindings.text(process.env.OIDC_JWKS ?? ""),
					OIDC_AUDIENCE: bindings.text(process.env.OIDC_AUDIENCE ?? ""),
				} : {}),
			},
		},
	};
});
