// Live test of the device API: a report posted by the test machine is read back, as posted,
// through the list, the device's view and its history; who may do what, the rules and the limits hold.
// It reports as the test machine 0000000000000001 (host live-test), which then stays in the list, and
// with the bearer tokens as a second one, 0000000000000002, which it then deletes.
// Usage, from the project's folder: node test/live-test.mjs <origin>, with either
//   - FLEET_API_ACCESS_CLIENT_ID and FLEET_API_ACCESS_CLIENT_SECRET: the test machine's Access service
//     token (enrolled for 0000000000000001), for the deployed Worker behind Access; or
//   - FLEET_API_READ_TOKEN and FLEET_API_WRITE_TOKEN: the bearer tokens (the local checks).
import { readFileSync } from "node:fs";

const origin = process.argv[2];
const { FLEET_API_READ_TOKEN: read, FLEET_API_WRITE_TOKEN: write, FLEET_API_ACCESS_CLIENT_ID: clientId, FLEET_API_ACCESS_CLIENT_SECRET: clientSecret } = process.env;
const viaAccess = Boolean(clientId && clientSecret);
if (!viaAccess && (!read || !write)) {
  console.log("FAIL  set FLEET_API_ACCESS_CLIENT_ID and FLEET_API_ACCESS_CLIENT_SECRET, or FLEET_API_READ_TOKEN and FLEET_API_WRITE_TOKEN (on Cloudflare: fnox exec -- ...)");
  process.exit(1);
}
// Who calls: the test machine posts, a reader reads. Behind Access both are the test machine's token.
const serviceToken = { "cf-access-client-id": clientId, "cf-access-client-secret": clientSecret };
const machine = viaAccess ? serviceToken : { authorization: `Bearer ${write}` };
const reader = viaAccess ? serviceToken : { authorization: `Bearer ${read}` };
const id = "0000000000000001";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const ts = Date.now();
const report = { ...example, id, ts, reason: "once", next_s: 0, host: { ...example.host, name: "live-test" }, thermal: { status: "ok", pressure: "nominal" } };

let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
// as is the caller's headers, or a bare token for Authorization: Bearer. Redirects are not
// followed: Access answers a request it refuses with one, to its login page.
async function call(method, path, { as, body } = {}) {
  const auth = typeof as === "string" ? { authorization: `Bearer ${as}` } : as ?? {};
  const headers = { ...auth, ...(body ? { "content-type": "application/json" } : {}) };
  const res = await fetch(`${origin}${path}`, { method, headers, redirect: "manual", body: body === undefined ? undefined : typeof body === "string" ? body : JSON.stringify(body) });
  const text = await res.text();
  let json = null;
  try { json = JSON.parse(text); } catch {}
  return { status: res.status, json, text, location: res.headers.get("location") };
}
const reports = `/api/devices/${id}/reports`;

if (viaAccess) {
  // Cloudflare Access refuses at the edge: the Worker never sees these.
  for (const [name, as] of [["no credentials", undefined], ["a wrong service token", { "cf-access-client-id": "nope.access", "cf-access-client-secret": "nope" }], ["the old bearer token alone", "nope"]]) {
    const refused = await call("GET", "/api/hello", { as });
    check(`${name}: refused at the edge (Access)`, [302, 401, 403].includes(refused.status) && !refused.json?.message, { status: refused.status, location: refused.location?.replace(/\?.*/, "") });
  }
  const other = "00aa11bb22cc33dd";
  const notMine = await call("POST", `/api/devices/${other}/reports`, { as: machine, body: { ...report, id: other } });
  check("the test machine's token may not post another machine's report: 403", notMine.status === 403 && notMine.json?.detail?.includes("may not post for"), notMine.json ?? notMine.text);
} else {
  for (const [name, as, status] of [["no token", undefined, 401], ["the read token", read, 403], ["a wrong token", "nope", 401]]) {
    const refused = await call("POST", reports, { as, body: report });
    check(`posting with ${name} is ${status}`, refused.status === status, refused.status);
  }
  const refusedRead = await call("GET", "/api/devices");
  check("reading with no token is 401", refusedRead.status === 401, refusedRead.status);
}
const hello = await call("GET", "/api/hello", { as: reader });
check("hello", hello.status === 200 && hello.json?.message?.startsWith("Hello from"), hello);

const posted = await call("POST", reports, { as: machine, body: report });
check("the test machine's report is 201", posted.status === 201 && posted.json?.duplicate === false && posted.json?.id === id, posted.json ?? posted.text);
const again = await call("POST", reports, { as: machine, body: report });
check("the same report again is a duplicate", again.status === 201 && again.json?.duplicate === true, again.json ?? again.text);

const one = await call("GET", `/api/devices/${id}`, { as: reader });
check("the device's view has the report as posted, a newer tool's field too", one.status === 200 && one.json?.report?.ts === ts && one.json.report.thermal?.pressure === "nominal" && one.json.report.rig?.status === "ok" && one.json.report.claims?.held?.length === 1, one.json ?? one.text);
check("a report that promises nothing has no due and no conditions", one.json && one.json.due === undefined && one.json.conditions?.length === 0, one.json);

const list = await call("GET", "/api/devices", { as: reader });
check("the list has it, with the Worker's clock", list.status === 200 && list.json?.devices?.some(d => d.report.id === id && d.report.ts === ts) && list.json.now >= posted.json?.received, list.json ?? list.text);

const history = await call("GET", `${reports}?limit=5`, { as: reader });
check("the history has it, newest first", history.status === 200 && history.json?.reports?.[0]?.report?.ts === ts && history.json.reports.every((r, i, all) => i === 0 || all[i - 1].report.ts >= r.report.ts), history.json ?? history.text);

const writerReads = await call("GET", `/api/devices/${id}`, { as: machine });
check("the machine reads too", writerReads.status === 200, writerReads.status);

for (const [name, change, location] of [
  ["percent 101 (a tag)", r => { r.battery = { ...r.battery, percent: 101 }; }, "body.battery.percent"],
  ["unknown with no reason (a rule)", r => { r.lid = { status: "unknown" }; }, "body.lid.why"],
  ["a home directory (a rule)", r => { r.rig = { ...r.rig, work_dir: "/Users/someone/work" }; }, "body.rig.work_dir"],
  ["another machine's id", r => { r.id = "00aa11bb22cc33dd"; }, "body.id"],
]) {
  const bad = structuredClone(report);
  change(bad);
  const refused = await call("POST", reports, { as: machine, body: bad });
  check(`${name} is 422 at ${location}`, refused.status === 422 && refused.json?.errors?.some(e => e.location === location), refused.json ?? refused.text);
}
const big = await call("POST", reports, { as: machine, body: { ...report, ts: ts + 1, pad: "x".repeat(17000) } });
check("a report over 16 KiB is 413", big.status === 413, big.status);
const unknown = await call("GET", "/api/devices/ffffffffffffffff", { as: reader });
check("a machine that never reported is 404", unknown.status === 404, unknown.status);

if (viaAccess) {
  // Forgetting a machine is a person's (logged in through Access), never a machine's.
  const notByMachine = await call("DELETE", `/api/devices/${id}`, { as: machine });
  check("the test machine's token may not forget a machine: 403", notByMachine.status === 403 && notByMachine.json?.detail?.includes("devices:forget"), notByMachine.json ?? notByMachine.text);
} else {
  const gone = "0000000000000002";
  const second = await call("POST", `/api/devices/${gone}/reports`, { as: write, body: { ...report, id: gone } });
  check("a second test machine reports", second.status === 201, second.json ?? second.text);
  const notByReader = await call("DELETE", `/api/devices/${gone}`, { as: read });
  check("deleting with the read token is 403", notByReader.status === 403, notByReader.status);
  const deleted = await call("DELETE", `/api/devices/${gone}`, { as: write });
  check("deleting with the write token forgets it and its reports", deleted.status === 200 && deleted.json?.id === gone && deleted.json.reports >= 1, deleted.json ?? deleted.text);
  const afterDelete = await call("GET", `/api/devices/${gone}`, { as: read });
  check("a deleted machine is 404", afterDelete.status === 404, afterDelete.status);
  const deletedAgain = await call("DELETE", `/api/devices/${gone}`, { as: write });
  check("deleting it again is 404", deletedAgain.status === 404, deletedAgain.status);
}

process.exit(failed ? 1 : 0);
