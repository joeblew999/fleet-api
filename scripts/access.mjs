// Cloudflare Access for the Worker: the application in front of its hostname, who may pass, and a
// service token per machine. Every step is idempotent and no secret is printed.
//
//   setup                               the Access application for the Worker's hostname (API_URL):
//                                       people log in with GitHub (FLEET_API_ACCESS_EMAILS may), MCP clients
//                                       through Access's OAuth, machines pass with their service token;
//                                       then the Worker's ACCESS_TEAM_DOMAIN
//                                       and ACCESS_AUD secrets, and FLEET_API_ACCESS_APP_ID and
//                                       FLEET_API_ACCESS_AUD in fnox
//   token create <machine> <device id> <file | fnox>
//                                       a service token fleet-api-<machine>, posting for that device only;
//                                       its Client ID and Secret go into <file> (made readable by its owner
//                                       alone) or, with fnox, into FLEET_API_ACCESS_CLIENT_ID and _SECRET
//   token list                          the machines' tokens: name, device, expiry (no secret)
//   token revoke <machine>              deletes the token: Access refuses it from then on
//
// Usage: fnox exec -- node scripts/access.mjs <the Worker's URL> <command> [args]   (mise run access:...)
import { spawnSync } from "node:child_process";
import { writeFileSync, chmodSync } from "node:fs";

const [url, command, ...args] = process.argv.slice(2);
const host = new URL(url).hostname;
const worker = host.split(".")[0];
const env = process.env;
const need = names => {
  const missing = names.filter(name => !env[name]);
  if (missing.length) fail(`not set: ${missing.join(", ")} (run it under fnox exec)`);
};
function fail(message) {
  console.log(`FAIL  ${message}`);
  process.exit(1);
}
need(["CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID"]);
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

// fnoxSet stores a value in the keychain under a name fnox.toml declares, through a pipe.
function fnoxSet(name, value) {
  const done = spawnSync("fnox", ["set", name, "-p", "keychain"], { input: value, stdio: ["pipe", "ignore", "inherit"], shell: process.platform === "win32" });
  if (done.status !== 0) fail(`fnox set ${name}`);
}

const prefix = `${worker}-`; // the service tokens of this Worker's machines
const app = async () => (await cf("GET", "/access/apps")).find(a => a.domain === host);
const tokens = async () => (await cf("GET", "/access/service_tokens?per_page=1000")).filter(t => t.name.startsWith(prefix));

// D1: the machines table, which device each token posts for.
async function d1(sql, params) {
  const db = (await cf("GET", `/d1/database?name=${worker}-db`)).find(d => d.name === `${worker}-db`);
  if (!db) fail(`no D1 database ${worker}-db: deploy the Worker first (mise run deploy)`);
  const [result] = await cf("POST", `/d1/database/${db.uuid}/query`, { sql, params });
  return result.results;
}

// policies is what the application lets in, in this order: the machines' service tokens (Service
// Auth), then the people who may log in with GitHub.
async function policies(machineTokens) {
  need(["CF_ACCESS_GITHUB_IDP_ID", "FLEET_API_ACCESS_EMAILS"]);
  const emails = env.FLEET_API_ACCESS_EMAILS.split(/[\s,]+/).filter(Boolean);
  const list = [];
  if (machineTokens.length) {
    list.push({ name: `${worker} machines`, decision: "non_identity", include: machineTokens.map(t => ({ service_token: { token_id: t.id } })) });
  }
  list.push({
    name: `${worker} developers`, decision: "allow",
    include: emails.map(email => ({ email: { email } })),
    require: [{ login_method: { id: env.CF_ACCESS_GITHUB_IDP_ID } }],
  });
  return list.map((p, i) => ({ ...p, precedence: i + 1 }));
}

async function putApp() {
  need(["CF_ACCESS_TEAM_DOMAIN"]);
  const existing = await app();
  const body = {
    name: worker, type: "self_hosted", domain: host, destinations: [{ type: "public", uri: host }],
    allowed_idps: [env.CF_ACCESS_GITHUB_IDP_ID], auto_redirect_to_identity: true, app_launcher_visible: false,
    session_duration: "24h", http_only_cookie_attribute: true,
    // A request that fails a Service Auth policy gets 401, not the login page: what a program expects.
    service_auth_401_redirect: true,
    // Access as the OAuth server for MCP clients (/api/mcp): a client that is refused gets 401 and
    // the metadata to log in with, through GitHub, as a person. Cloudflare calls it beta.
    oauth_configuration: { enabled: true, dynamic_client_registration: { enabled: true, allowed_uris: ["https://claude.ai/api/mcp/auth_callback"] } },
    policies: await policies(await tokens()),
  };
  const result = existing ? await cf("PUT", `/access/apps/${existing.id}`, body) : await cf("POST", "/access/apps", body);
  console.log(`PASS  Access application ${worker} for ${host}: ${existing ? "updated" : "created"}, ${result.policies.length} policies`);
  return result;
}

async function setup() {
  const result = await putApp();
  fnoxSet("FLEET_API_ACCESS_APP_ID", result.id);
  fnoxSet("FLEET_API_ACCESS_AUD", result.aud);
  console.log("PASS  fnox: FLEET_API_ACCESS_APP_ID, FLEET_API_ACCESS_AUD");
  for (const [name, value] of [["ACCESS_TEAM_DOMAIN", env.CF_ACCESS_TEAM_DOMAIN], ["ACCESS_AUD", result.aud]]) {
    await cf("PUT", `/workers/scripts/${worker}/secrets`, { name, text: value, type: "secret_text" });
    console.log(`PASS  ${worker}: the secret ${name} set`);
  }
}

async function create(machine, device, into) {
  if (!machine || !/^[a-z0-9-]{1,40}$/.test(machine)) fail("a machine name: lower-case letters, digits and dashes");
  if (!/^[0-9a-f]{16}$/.test(device ?? "")) fail("a device id: 16 lower-case hex digits (claude-rig keeps it in ~/.config/claude-rig/device-id)");
  if (!into) fail("where the credentials go: a file, or fnox");
  const name = prefix + machine;
  if ((await tokens()).some(t => t.name === name)) fail(`${name} exists: revoke it first, or pick another name`);
  const token = await cf("POST", "/access/service_tokens", { name, duration: "8760h" });
  await d1("INSERT INTO machines (token, device, name, created) VALUES (?, ?, ?, ?) ON CONFLICT (token) DO UPDATE SET device = excluded.device", [token.client_id, device, name, Date.now()]);
  const credentials = { client_id: token.client_id, client_secret: token.client_secret };
  if (into === "fnox") {
    fnoxSet("FLEET_API_ACCESS_CLIENT_ID", credentials.client_id);
    fnoxSet("FLEET_API_ACCESS_CLIENT_SECRET", credentials.client_secret);
  } else {
    writeFileSync(into, JSON.stringify(credentials) + "\n", { mode: 0o600 });
    chmodSync(into, 0o600);
  }
  await putApp();
  console.log(`PASS  ${name}: posts for ${device} only, expires ${token.expires_at}; its Client ID and Secret are in ${into === "fnox" ? "fnox (FLEET_API_ACCESS_CLIENT_ID, FLEET_API_ACCESS_CLIENT_SECRET)" : into}`);
}

async function list() {
  const rows = await d1("SELECT token, device FROM machines", []);
  const device = Object.fromEntries(rows.map(r => [r.token, r.device]));
  for (const t of await tokens()) console.log(`${t.name}\tdevice ${device[t.client_id] ?? "(none: reads only)"}\texpires ${t.expires_at}`);
}

async function revoke(machine) {
  const name = prefix + machine;
  const token = (await tokens()).find(t => t.name === name);
  if (!token) fail(`no service token ${name}`);
  await cf("DELETE", `/access/service_tokens/${token.id}`);
  await d1("DELETE FROM machines WHERE token = ?", [token.client_id]);
  await putApp();
  console.log(`PASS  ${name} revoked: Access refuses it, and the Worker no longer ties it to a device`);
}

switch (`${command} ${args[0] ?? ""}`.trim()) {
  case "setup": await setup(); break;
  case "token create": await create(args[1], args[2], args[3]); break;
  case "token list": await list(); break;
  case "token revoke": await revoke(args[1]); break;
  default: fail("commands: setup | token create <machine> <device id> <file | fnox> | token list | token revoke <machine>");
}
