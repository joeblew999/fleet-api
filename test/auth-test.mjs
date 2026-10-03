// Who may do what, against a running server that trusts a test issuer this script serves: as
// Cloudflare Access (ACCESS_TEAM_DOMAIN=http://localhost:<keys port>, ACCESS_AUD=test-aud) and as
// an OpenID Connect provider (OIDC_ISSUER the same origin, OIDC_JWKS <origin>/jwks,
// OIDC_AUDIENCE=https://fleet-api.test). Under workerd this is what proves the TinyGo build
// verifies a JWT. Never point a deployed Worker at it: the keys are made here, for this run.
// Usage: node test/auth-test.mjs <origin> <keys port>   (mise run test:native, test:workerd)
import { createServer } from "node:http";
import { generateKeyPairSync, createSign } from "node:crypto";
import { readFileSync } from "node:fs";

const [origin, keysPort] = [process.argv[2], Number(process.argv[3])];
const issuer = `http://localhost:${keysPort}`;
const { publicKey, privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
const jwks = JSON.stringify({ keys: [{ ...publicKey.export({ format: "jwk" }), kid: "test", alg: "RS256", use: "sig" }] });
const keys = createServer((_, res) => res.writeHead(200, { "content-type": "application/json" }).end(jwks));
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
const id = "0000000000000001";
const report = { ...JSON.parse(readFileSync("api/example_report.json", "utf8")), id, ts: Date.now(), reason: "once", next_s: 0, host: { name: "auth-test", os: "linux", arch: "arm64" } };
const post = headers => call("POST", `/api/devices/${id}/reports`, headers, report);

try {
  for (const [name, headers, status, says] of [
    ["a person through Access reads", access({ email: "dev@example.com", sub: "u1" }), 200],
    ["a service token that was not enrolled reads", access({ common_name: "ci.access", sub: "" }), 200],
    ["an OIDC token with devices:read reads", oidc({ sub: "user-1", scope: "openid devices:read" }), 200],
    ["an OIDC token with no scope of ours is 403", oidc({ sub: "user-1", scope: "openid" }), 403, "devices:read"],
    ["an OIDC token for another API is 401", oidc({ sub: "user-1", scope: "devices:read", aud: "https://elsewhere.test" }), 401, "refused"],
    ["an Access JWT for another application is 401", access({ email: "dev@example.com", aud: ["another"] }), 401, "refused"],
    ["an expired one is 401", access({ email: "dev@example.com", exp: Math.floor(Date.now() / 1000) - 600 }), 401, "refused"],
    ["nothing is 401, naming where to find the provider", {}, 401, "credentials are required"],
  ]) {
    const got = await call("GET", "/api/devices", headers);
    check(name, got.status === status && (!says || got.text.includes(says)), got);
  }
  const person = await post(access({ email: "dev@example.com", sub: "u1" }));
  check("a person through Access may not post: 403", person.status === 403 && person.text.includes("devices:write"), person);
  const unenrolled = await post(access({ common_name: "ci.access", sub: "" }));
  check("a service token that was not enrolled may not post: 403", unenrolled.status === 403, unenrolled);
  const writer = await post(oidc({ sub: "app-1", scope: "devices:write" }));
  check("an OIDC token with devices:write posts: 201", writer.status === 201, writer);
  const metadata = await call("GET", "/.well-known/oauth-protected-resource");
  check("the protected resource metadata names the issuer", metadata.status === 200 && JSON.parse(metadata.text).authorization_servers?.[0] === issuer, metadata);
} finally {
  keys.close();
}
process.exit(failed ? 1 : 0);
