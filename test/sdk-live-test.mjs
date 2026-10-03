// The generated TypeScript SDK (sdk/out/typescript-dist) against the live API: a machine posts its
// report, a reader gets it back, typed errors included.
// Usage, from the project's folder (the SDK is the one in its sdk/out): node test/sdk-live-test.mjs <origin>,
// with FLEET_API_ACCESS_CLIENT_ID and FLEET_API_ACCESS_CLIENT_SECRET set (the test machine's Access
// service token: the deployed Worker), or READ_TOKEN and WRITE_TOKEN.
import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";
const origin = process.argv[2];
const { FleetClient, Fleet, serialization } = await import(pathToFileURL(`${process.cwd()}/sdk/out/typescript-dist/esm/index.mjs`));
const { READ_TOKEN: read, WRITE_TOKEN: write, FLEET_API_ACCESS_CLIENT_ID: clientId, FLEET_API_ACCESS_CLIENT_SECRET: clientSecret } = process.env;
const viaAccess = Boolean(clientId && clientSecret);

let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  SDK ${name}${ok ? "" : `: ${detail instanceof Error ? detail.message : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
const attempt = promise => promise.then(value => ({ value }), error => ({ error }));

// Behind Access both are the test machine's service token. The SDK should send its two headers from
// FLEET_API_ACCESS_CLIENT_ID and _SECRET by itself, but its generated auth knows only one of two
// header schemes, so it is given them as headers with its own auth off.
// Upstream: fern-api/fern#17775 (when fixed: new FleetClient({ baseUrl, maxRetries: 0 }), the variables doing the rest)
const accessClient = () => new FleetClient({ baseUrl: origin, maxRetries: 0, auth: false, headers: { "CF-Access-Client-Id": clientId, "CF-Access-Client-Secret": clientSecret } });
const machine = viaAccess ? accessClient() : new FleetClient({ baseUrl: origin, bearer: { token: write }, maxRetries: 0 });
const reader = viaAccess ? accessClient() : new FleetClient({ baseUrl: origin, bearer: { token: read }, maxRetries: 0 });
const id = "0000000000000001";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const ts = Date.now();
// The report as JSON, checked and typed by the SDK's own schema (the serde layer), as a machine does.
const body = serialization.DeviceReport.parseOrThrow({ ...example, id, ts, reason: "once", next_s: 0, host: { ...example.host, name: "live-test" } });
const invalid = serialization.DeviceReport.parse({ ...example, reason: "whenever", host: undefined });
check("the SDK's schema refuses a bad report before it is sent", !invalid.ok && invalid.errors.some(e => e.path.join(".") === "reason"), invalid);

const posted = await attempt(machine.devices.report({ id, body }));
check("devices.report() by the test machine", posted.value?.id === id && posted.value.duplicate === false, posted.error ?? posted.value);
const view = await attempt(reader.devices.get({ id }));
check("devices.get() gives it back typed", view.value?.report?.ts === ts && view.value.report.rig?.status === "ok" && view.value.report.claims?.held?.[0]?.caller === "lead-agent", view.error ?? view.value);
const list = await attempt(reader.devices.list());
check("devices.list()", list.value?.devices?.some(d => d.report.id === id), list.error ?? list.value);
const history = await attempt(reader.devices.history({ id, limit: 3 }));
check("devices.history()", history.value?.reports?.[0]?.report?.ts === ts, history.error ?? history.value);

if (viaAccess) {
  const other = "00aa11bb22cc33dd";
  const notMine = await attempt(machine.devices.report({ id: other, body: { ...body, id: other } }));
  check("devices.report() for another machine is a ForbiddenError", notMine.error instanceof Fleet.ForbiddenError, notMine.error ?? notMine.value);
  const forget = await attempt(machine.devices.delete({ id }));
  check("devices.delete() by a machine is a ForbiddenError", forget.error instanceof Fleet.ForbiddenError, forget.error ?? forget.value);
} else {
  const denied = await attempt(reader.devices.report({ id, body: { ...body, ts: ts + 1 } }));
  check("devices.report() with the read token is a ForbiddenError", denied.error instanceof Fleet.ForbiddenError, denied.error ?? denied.value);
}
const bad = await attempt(machine.devices.report({ id, body: { ...body, ts: ts + 2, battery: { ...body.battery, percent: 101 } } }));
check("a refused report is an UnprocessableEntityError with the location", bad.error instanceof Fleet.UnprocessableEntityError && bad.error.body?.errors?.some(e => e.location === "body.battery.percent"), bad.error ?? bad.value);
const missing = await attempt(reader.devices.get({ id: "ffffffffffffffff" }));
check("a machine that never reported is a NotFoundError", missing.error instanceof Fleet.NotFoundError, missing.error ?? missing.value);

if (!viaAccess) {
  const gone = "0000000000000003";
  const second = await attempt(machine.devices.report({ id: gone, body: { ...body, id: gone } }));
  check("devices.report() for a second test machine", second.value?.id === gone, second.error ?? second.value);
  const deleted = await attempt(machine.devices.delete({ id: gone }));
  check("devices.delete() with the write token", deleted.value?.id === gone && deleted.value.reports >= 1, deleted.error ?? deleted.value);
  const deletedAgain = await attempt(machine.devices.delete({ id: gone }));
  check("devices.delete() again is a NotFoundError", deletedAgain.error instanceof Fleet.NotFoundError, deletedAgain.error ?? deletedAgain.value);
}

process.exit(failed ? 1 : 0);
