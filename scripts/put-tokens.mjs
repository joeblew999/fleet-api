// Sets the Worker's two secrets, READ_TOKEN and WRITE_TOKEN, from FLEET_API_READ_TOKEN and
// FLEET_API_WRITE_TOKEN in the environment (fnox). The values
// go in a request body or through a pipe, never on a command line or in the output.
// - The Worker exists: through Cloudflare's API.
// - It does not yet (a first deploy refuses to run without its secrets): deploys the build in
//   .cloudflare/output (mise run build) with them, cf deploy --secrets-file reading a pipe.
// Usage: fnox exec -- node scripts/put-tokens.mjs <the Worker's URL>   (mise run tokens:put)
import { spawnSync } from "node:child_process";

const worker = new URL(process.argv[2]).hostname.split(".")[0];
const { CLOUDFLARE_API_TOKEN: token, CLOUDFLARE_ACCOUNT_ID: account } = process.env;
// The Worker's secret, and the variable (the fnox name) its value comes from.
const tokens = { READ_TOKEN: "FLEET_API_READ_TOKEN", WRITE_TOKEN: "FLEET_API_WRITE_TOKEN" };
const names = Object.keys(tokens);
const value = name => process.env[tokens[name]];
const missing = ["CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", ...Object.values(tokens)].filter(name => !process.env[name]);
if (missing.length) {
  console.log(`FAIL  not set: ${missing.join(", ")} (run it under fnox exec)`);
  process.exit(1);
}

const api = `https://api.cloudflare.com/client/v4/accounts/${account}/workers/scripts/${worker}`;
const headers = { authorization: `Bearer ${token}` };
const exists = (await fetch(`${api}/settings`, { headers })).ok;
if (!exists) {
  console.log(`${worker} does not exist yet: deploying the build with its secrets`);
  const secrets = JSON.stringify(Object.fromEntries(names.map(name => [name, value(name)])));
  const deployed = spawnSync("npx", ["cf", "deploy", "--prebuilt", "--secrets-file", "/dev/stdin"], { input: secrets, stdio: ["pipe", "inherit", "inherit"], shell: process.platform === "win32" });
  console.log(`${deployed.status === 0 ? "PASS" : "FAIL"}  ${worker}: deployed with READ_TOKEN and WRITE_TOKEN`);
  process.exit(deployed.status === 0 ? 0 : 1);
}

let failed = 0;
for (const name of names) {
  const res = await fetch(`${api}/secrets`, {
    method: "PUT",
    headers: { ...headers, "content-type": "application/json" },
    body: JSON.stringify({ name, text: value(name), type: "secret_text" }),
  });
  const answer = await res.json().catch(() => ({}));
  const ok = res.ok && answer.success;
  console.log(`${ok ? "PASS" : "FAIL"}  ${worker}: ${name} ${ok ? "set" : `not set: HTTP ${res.status} ${JSON.stringify(answer.errors ?? [])}`}`);
  if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
