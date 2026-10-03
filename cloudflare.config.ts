import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker: the fleet's device API, written in Go on workers-go and built with TinyGo.
// D1 keeps each machine's newest report and 7 days of history, and which device each machine's
// service token posts for. A mode that starts with "perf-" (charter perf) deploys a scratch Worker
// of that name, with a database of its own.

// Who else may call (go/auth): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD, kept in fnox) and
// an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE). They are the Worker's optional secrets
// (WORKER_OPTIONAL_SECRETS in mise.toml): declared when the environment that deploys or runs the
// Worker has them, and then set by charter deploy, or taken from the environment by cf dev. Unset,
// the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries((process.env.WORKER_OPTIONAL_SECRETS ?? "").split(" ").filter(name => process.env[name]).map(name => [name, bindings.secret()]));

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
				// The bearer tokens (api/auth.go): set by every mise run deploy, from the environment under cf dev.
				READ_TOKEN: bindings.secret(),
				WRITE_TOKEN: bindings.secret(),
				...optional,
			},
		},
	};
});
