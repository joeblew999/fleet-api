// Live test of the device API: a report posted with the write token is read back, as posted,
// through the list, the device's view and its history; the tokens, the rules and the limits hold.
// It reports as the test machine 0000000000000001 (host live-test), which then stays in the list.
// Usage, from the project's folder, with READ_TOKEN and WRITE_TOKEN set: node test/live-test.mjs <origin>
import { readFileSync } from "node:fs";

const origin = process.argv[2];
const { READ_TOKEN: read, WRITE_TOKEN: write } = process.env;
if (!read || !write) {
  console.log("FAIL  READ_TOKEN and WRITE_TOKEN must be set (on Cloudflare: fnox exec -- ...)");
  process.exit(1);
}
const id = "0000000000000001";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const ts = Date.now();
const report = { ...example, id, ts, reason: "once", next_s: 0, host: { ...example.host, name: "live-test" }, thermal: { status: "ok", pressure: "nominal" } };

let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
async function call(method, path, { token, body } = {}) {
  const headers = { ...(token ? { authorization: `Bearer ${token}` } : {}), ...(body ? { "content-type": "application/json" } : {}) };
  const res = await fetch(`${origin}${path}`, { method, headers, body: body === undefined ? undefined : typeof body === "string" ? body : JSON.stringify(body) });
  const text = await res.text();
  let json = null;
  try { json = JSON.parse(text); } catch {}
  return { status: res.status, json, text };
}
const reports = `/api/devices/${id}/reports`;

const hello = await call("GET", "/api/hello");
check("hello needs no token", hello.status === 200 && hello.json?.message?.startsWith("Hello from"), hello);

for (const [name, token] of [["no token", undefined], ["the read token", read], ["a wrong token", "nope"]]) {
  const refused = await call("POST", reports, { token, body: report });
  check(`posting with ${name} is 401`, refused.status === 401, refused.status);
}
const refusedRead = await call("GET", "/api/devices");
check("reading with no token is 401", refusedRead.status === 401, refusedRead.status);

const posted = await call("POST", reports, { token: write, body: report });
check("a report posted with the write token is 201", posted.status === 201 && posted.json?.duplicate === false && posted.json?.id === id, posted.json ?? posted.text);
const again = await call("POST", reports, { token: write, body: report });
check("the same report again is a duplicate", again.status === 201 && again.json?.duplicate === true, again.json ?? again.text);

const one = await call("GET", `/api/devices/${id}`, { token: read });
check("the device's view has the report as posted, a newer tool's field too", one.status === 200 && one.json?.report?.ts === ts && one.json.report.thermal?.pressure === "nominal" && one.json.report.rig?.status === "ok" && one.json.report.claims?.held?.length === 1, one.json ?? one.text);
check("a report that promises nothing has no due and no conditions", one.json && one.json.due === undefined && one.json.conditions?.length === 0, one.json);

const list = await call("GET", "/api/devices", { token: read });
check("the list has it, with the Worker's clock", list.status === 200 && list.json?.devices?.some(d => d.report.id === id && d.report.ts === ts) && list.json.now >= posted.json?.received, list.json ?? list.text);

const history = await call("GET", `${reports}?limit=5`, { token: read });
check("the history has it, newest first", history.status === 200 && history.json?.reports?.[0]?.report?.ts === ts && history.json.reports.every((r, i, all) => i === 0 || all[i - 1].report.ts >= r.report.ts), history.json ?? history.text);

const writerReads = await call("GET", `/api/devices/${id}`, { token: write });
check("the write token reads too", writerReads.status === 200, writerReads.status);

for (const [name, change, location] of [
  ["percent 101 (a tag)", r => { r.battery = { ...r.battery, percent: 101 }; }, "body.battery.percent"],
  ["unknown with no reason (a rule)", r => { r.lid = { status: "unknown" }; }, "body.lid.why"],
  ["a home directory (a rule)", r => { r.rig = { ...r.rig, work_dir: "/Users/someone/work" }; }, "body.rig.work_dir"],
  ["another machine's id", r => { r.id = "00aa11bb22cc33dd"; }, "body.id"],
]) {
  const bad = structuredClone(report);
  change(bad);
  const refused = await call("POST", reports, { token: write, body: bad });
  check(`${name} is 422 at ${location}`, refused.status === 422 && refused.json?.errors?.some(e => e.location === location), refused.json ?? refused.text);
}
const big = await call("POST", reports, { token: write, body: { ...report, ts: ts + 1, pad: "x".repeat(17000) } });
check("a report over 16 KiB is 413", big.status === 413, big.status);
const unknown = await call("GET", "/api/devices/ffffffffffffffff", { token: read });
check("a machine that never reported is 404", unknown.status === 404, unknown.status);

process.exit(failed ? 1 : 0);
