// Which device each machine's Access service token posts for: the Worker's machines table (D1),
// keyed by the token's Client ID. The tokens themselves are charter's (mise run access:token: named
// <worker>:<machine>); this is the one thing about them that is fleet-api's own. No secret is
// printed: a Client ID is shown only by its last characters.
//
//   enrol <machine> <device id>   the token <worker>:<machine> posts for that device, and no other
//   list                          every token of the Worker's: the device it posts for, its expiry
//   forget <machine>              the token no longer posts for any device (it still reads)
//
// Usage: fnox exec -- node scripts/machines.mjs <the Worker's URL> <command> [args]   (mise run machine:...)
const [url, command, ...args] = process.argv.slice(2);
const worker = new URL(url).hostname.split(".")[0];
const env = process.env;
function fail(message) {
  console.log(`FAIL  ${message}`);
  process.exit(1);
}
if (!env.CLOUDFLARE_API_TOKEN || !env.CLOUDFLARE_ACCOUNT_ID) fail("CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are not set (run it under fnox exec)");
const account = `https://api.cloudflare.com/client/v4/accounts/${env.CLOUDFLARE_ACCOUNT_ID}`;

async function cf(method, path, body) {
  const res = await fetch(`${account}${path}`, {
    method,
    headers: { authorization: `Bearer ${env.CLOUDFLARE_API_TOKEN}`, ...(body ? { "content-type": "application/json" } : {}) },
    body: body && JSON.stringify(body),
  });
  const answer = await res.json().catch(() => ({}));
  if (!res.ok || !answer.success) fail(`${method} ${path.replace(/[0-9a-f-]{32,}/g, "<id>")}: HTTP ${res.status} ${JSON.stringify(answer.errors ?? [])}`);
  return answer.result;
}

async function d1(sql, params) {
  const db = (await cf("GET", `/d1/database?name=${worker}-db`)).find(d => d.name === `${worker}-db`);
  if (!db) fail(`no D1 database ${worker}-db: deploy the Worker first (mise run deploy)`);
  const [result] = await cf("POST", `/d1/database/${db.uuid}/query`, { sql, params });
  return result.results;
}

const prefix = `${worker}:`; // charter's name for this Worker's machines' tokens
const tokens = async () => (await cf("GET", "/access/service_tokens?per_page=1000")).filter(t => t.name.startsWith(prefix));
const shown = clientId => `…${clientId.slice(-12)}`;
const machineName = machine => {
  if (!/^[a-z0-9-]{1,40}$/.test(machine ?? "")) fail("a machine's name: lower-case letters, digits and dashes, as access:token create was given");
  return prefix + machine;
};

async function enrol(machine, device) {
  const name = machineName(machine);
  if (!/^[0-9a-f]{16}$/.test(device ?? "")) fail("a device id: 16 lower-case hex digits (claude-rig keeps it in ~/.config/claude-rig/device-id)");
  const token = (await tokens()).find(t => t.name === name);
  if (!token) fail(`no service token ${name}: mise run access:token -- create ${machine} <file|fnox> first`);
  await d1("INSERT INTO machines (token, device, name, created) VALUES (?, ?, ?, ?) ON CONFLICT (token) DO UPDATE SET device = excluded.device, name = excluded.name", [token.client_id, device, name, Date.now()]);
  console.log(`PASS  ${name} (${shown(token.client_id)}) posts for ${device} only`);
}

async function list() {
  const device = Object.fromEntries((await d1("SELECT token, device FROM machines", [])).map(r => [r.token, r.device]));
  for (const t of await tokens()) console.log(`${t.name.slice(prefix.length)}\t${shown(t.client_id)}\tdevice ${device[t.client_id] ?? "(none: reads only)"}\texpires ${t.expires_at}`);
}

async function forget(machine) {
  const name = machineName(machine);
  const token = (await tokens()).find(t => t.name === name);
  const gone = await d1("DELETE FROM machines WHERE name = ? OR token = ? RETURNING device", [name, token?.client_id ?? ""]);
  console.log(`PASS  ${name}: posts for no device now (${gone.length} removed)`);
}

switch (command) {
  case "enrol": await enrol(args[0], args[1]); break;
  case "list": await list(); break;
  case "forget": await forget(args[0]); break;
  default: fail("commands: enrol <machine> <device id> | list | forget <machine>");
}
