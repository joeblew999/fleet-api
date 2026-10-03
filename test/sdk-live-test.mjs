// The generated TypeScript SDK (sdk/out/typescript-dist) against the live API: a machine posts its
// report with the write token, a reader gets it back with the read token, typed errors included.
// Usage, from the project's folder (the SDK is the one in its sdk/out), with FLEET_API_READ_TOKEN
// and FLEET_API_WRITE_TOKEN set: node test/sdk-live-test.mjs <origin>
import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";
const origin = process.argv[2];
const { FleetClient, Fleet, serialization } = await import(pathToFileURL(`${process.cwd()}/sdk/out/typescript-dist/esm/index.mjs`));
const { FLEET_API_READ_TOKEN: read, FLEET_API_WRITE_TOKEN: write } = process.env;

let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  SDK ${name}${ok ? "" : `: ${detail instanceof Error ? detail.message : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
const attempt = promise => promise.then(value => ({ value }), error => ({ error }));

const machine = new FleetClient({ baseUrl: origin, token: write, maxRetries: 0 });
const reader = new FleetClient({ baseUrl: origin, token: read, maxRetries: 0 });
const id = "0000000000000001";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const ts = Date.now();
// The report as JSON, checked and typed by the SDK's own schema (the serde layer), as a machine does.
const body = serialization.DeviceReport.parseOrThrow({ ...example, id, ts, reason: "once", next_s: 0, host: { ...example.host, name: "live-test" } });
const invalid = serialization.DeviceReport.parse({ ...example, reason: "whenever", host: undefined });
check("the SDK's schema refuses a bad report before it is sent", !invalid.ok && invalid.errors.some(e => e.path.join(".") === "reason"), invalid);

const posted = await attempt(machine.devices.report({ id, body }));
check("devices.report() with the write token", posted.value?.id === id && posted.value.duplicate === false, posted.error ?? posted.value);
const view = await attempt(reader.devices.get({ id }));
check("devices.get() with the read token gives it back typed", view.value?.report?.ts === ts && view.value.report.rig?.status === "ok" && view.value.report.claims?.held?.[0]?.caller === "lead-agent", view.error ?? view.value);
const list = await attempt(reader.devices.list());
check("devices.list()", list.value?.devices?.some(d => d.report.id === id), list.error ?? list.value);
const history = await attempt(reader.devices.history({ id, limit: 3 }));
check("devices.history()", history.value?.reports?.[0]?.report?.ts === ts, history.error ?? history.value);

const denied = await attempt(reader.devices.report({ id, body: { ...body, ts: ts + 1 } }));
check("devices.report() with the read token is an UnauthorizedError", denied.error instanceof Fleet.UnauthorizedError, denied.error ?? denied.value);
const bad = await attempt(machine.devices.report({ id, body: { ...body, ts: ts + 2, battery: { ...body.battery, percent: 101 } } }));
check("a refused report is an UnprocessableEntityError with the location", bad.error instanceof Fleet.UnprocessableEntityError && bad.error.body?.errors?.some(e => e.location === "body.battery.percent"), bad.error ?? bad.value);
const missing = await attempt(reader.devices.get({ id: "ffffffffffffffff" }));
check("a machine that never reported is a NotFoundError", missing.error instanceof Fleet.NotFoundError, missing.error ?? missing.value);

const gone = "0000000000000003";
const second = await attempt(machine.devices.report({ id: gone, body: { ...body, id: gone } }));
check("devices.report() for a second test machine", second.value?.id === gone, second.error ?? second.value);
const deleted = await attempt(machine.devices.delete({ id: gone }));
check("devices.delete() with the write token", deleted.value?.id === gone && deleted.value.reports >= 1, deleted.error ?? deleted.value);
const deletedAgain = await attempt(machine.devices.delete({ id: gone }));
check("devices.delete() again is a NotFoundError", deletedAgain.error instanceof Fleet.NotFoundError, deletedAgain.error ?? deletedAgain.value);

process.exit(failed ? 1 : 0);
