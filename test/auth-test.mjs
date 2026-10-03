// Who may do what, against a running server that trusts a test issuer this script serves: as
// Cloudflare Access (ACCESS_TEAM_DOMAIN=http://localhost:<keys port>, ACCESS_AUD=test-aud) and as
// an OpenID Connect issuer (OIDC_ISSUER the same origin, found by its discovery document;
// OIDC_AUDIENCE=https://fleet-api.test). The machine whose service token's Client ID is
// machine.access is enrolled for 0000000000000001: by MACHINES natively, here in the local D1 under
// workerd. Under workerd this is what proves the TinyGo build verifies a JWT. Never point a deployed
// Worker at it: the keys are made here, for this run.
// Usage: node test/auth-test.mjs <origin> <keys port>   (mise run test:native, test:workerd)
import { createServer } from "node:http";
import { generateKeyPairSync, createSign } from "node:crypto";
import { readFileSync } from "node:fs";

const [origin, keysPort] = [process.argv[2], Number(process.argv[3])];
const issuer = `http://localhost:${keysPort}`;
const { publicKey, privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
const jwks = JSON.stringify({ keys: [{ ...publicKey.export({ format: "jwk" }), kid: "test", alg: "RS256", use: "sig" }] });
const keys = createServer((req, res) => res.writeHead(200, { "content-type": "application/json" })
  .end(req.url === "/.well-known/openid-configuration" ? JSON.stringify({ issuer, jwks_uri: `${issuer}/jwks` }) : jwks));
await new Promise(resolve => keys.listen(keysPort, "localhost", resolve));

const b64 = value => Buffer.from(typeof value === "string" ? value : JSON.stringify(value)).toString("base64url");
function sign(claims) {
  const now = Math.floor(Date.now() / 1000);
  const signed = `${b64({ alg: "RS256", kid: "test", typ: "JWT" })}.${b64({ iss: issuer, iat: now, exp: now + 300, ...claims })}`;
  return `${signed}.${createSign("RSA-SHA256").update(signed).sign(privateKey).toString("base64url")}`;
}
const access = claims => ({ "cf-access-jwt-assertion": sign({ aud: ["test-aud"], type: "app", ...claims }) });
const oidc = claims => ({ authorization: `Bearer ${sign({ aud: "https://fleet-api.test", ...claims })}` });

let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
async function call(method, path, headers = {}, body) {
  const res = await fetch(`${origin}${path}`, { method, headers: { ...headers, ...(body ? { "content-type": "application/json" } : {}) }, body: body && JSON.stringify(body) });
  return { status: res.status, text: await res.text(), header: res.headers.get("www-authenticate") };
}
const id = "0000000000000001", other = "00aa11bb22cc33dd";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const report = device => ({ ...example, id: device, ts: Date.now(), reason: "once", next_s: 0, host: { name: "auth-test", os: "linux", arch: "arm64" } });
const post = (headers, device = id) => call("POST", `/api/devices/${device}/reports`, headers, report(device));
const person = { email: "dev@example.com", sub: "u1" }, machine = { common_name: "machine.access", sub: "" }, ci = { common_name: "ci.access", sub: "" };

// Under workerd (cf dev), enrol the machine in the local D1, as mise run machine:enrol does in the
// deployed one. Natively MACHINES did it; there this path is not the D1 explorer's, and nothing happens.
await fetch(`${origin}/cdn-cgi/local/explorer/api/d1/database/DB-fleet-api/raw`, {
  method: "POST", headers: { "content-type": "application/json" },
  body: JSON.stringify({ sql: `INSERT INTO machines (token, device, name, created) VALUES ('machine.access', '${id}', 'fleet-api:auth-test', 0) ON CONFLICT (token) DO NOTHING` }),
}).catch(() => {});

try {
  for (const [name, headers, status, says] of [
    ["a person through Access reads", access(person), 200],
    ["a machine's service token reads", access(machine), 200],
    ["a service token that was not enrolled reads", access(ci), 200],
    ["an OIDC token with devices:read reads", oidc({ sub: "user-1", scope: "openid devices:read" }), 200],
    ["an OIDC token with no scope of ours is 403", oidc({ sub: "user-1", scope: "openid" }), 403, "devices:read"],
    ["an OIDC token for another API is 401", oidc({ sub: "user-1", scope: "devices:read", aud: "https://elsewhere.test" }), 401, "refused"],
    ["an Access JWT for another application is 401", access({ ...person, aud: ["another"] }), 401, "refused"],
    ["an expired one is 401", access({ ...person, exp: Math.floor(Date.now() / 1000) - 600 }), 401, "refused"],
    ["the read token reads", { authorization: `Bearer ${process.env.READ_TOKEN}` }, 200],
    ["a token this API does not know is 401", { authorization: "Bearer nope" }, 401, "refused"],
    ["nothing is 401", {}, 401, "credentials are required"],
  ]) {
    const got = await call("GET", "/api/devices", headers);
    check(name, got.status === status && (!says || got.text.includes(says)), got);
  }
  for (const [name, headers, device, status, says] of [
    ["a machine's service token posts for its own device: 201", access(machine), id, 201],
    ["a machine's service token may not post for another device: 403", access(machine), other, 403, `for device ${id}: it may not post for ${other}`],
    ["a service token that was not enrolled may not post: 403", access(ci), id, 403, "was not enrolled"],
    ["a person through Access may not post (wrong scope): 403", access(person), id, 403, "devices:write"],
    ["the read token may not post (wrong scope): 403", { authorization: `Bearer ${process.env.READ_TOKEN}` }, id, 403, "devices:write"],
    ["an OIDC token with devices:read may not post (wrong scope): 403", oidc({ sub: "user-1", scope: "devices:read" }), id, 403, "devices:write"],
    ["an OIDC token with devices:write posts: 201", oidc({ sub: "app-1", scope: "devices:write" }), other, 201],
    ["nothing may not post: 401", {}, id, 401, "credentials are required"],
  ]) {
    const got = await post(headers, device);
    check(name, got.status === status && (!says || got.text.includes(says)), got);
  }
  const forget = await call("DELETE", `/api/devices/${other}`, access(machine));
  check("a machine's service token may not forget a machine: 403", forget.status === 403 && forget.text.includes("devices:forget"), forget);
  const discovery = await fetch(`${origin}/.well-known/openid-configuration`, { redirect: "manual" });
  check("the spec's openIdConnectUrl sends a client on to the issuer", discovery.status === 302 && discovery.headers.get("location") === `${issuer}/.well-known/openid-configuration`, { status: discovery.status, location: discovery.headers.get("location") });
} finally {
  keys.close();
}
process.exit(failed ? 1 : 0);
