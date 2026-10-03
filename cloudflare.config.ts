import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker: the fleet's device API, written in Go on workers-go and built with TinyGo.
// D1 keeps each machine's newest report and 7 days of history. READ_TOKEN and WRITE_TOKEN are
// secrets: set on Cloudflare (docs/guides/tokens.md), from the environment under cf dev.
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
			},
		},
	};
});
