// Test of the API's MCP endpoint (/api/mcp) with a real MCP client, the official TypeScript one, over
// Streamable HTTP. Everything runs twice, once per protocol era the server speaks: the stateless
// 2026-07-28 (every request names its version; server/discover) and the handshake one (initialize,
// 2025-11-25). The tools must be the read operations, a tool call must answer what the REST route
// answers, and the caller's token must be what decides. It posts the test machine's report first.
// Usage, from the project's folder (@modelcontextprotocol/client comes from its node_modules), with
// FLEET_API_READ_TOKEN and FLEET_API_WRITE_TOKEN set: node test/mcp-test.mjs <origin>
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
const { Client, StreamableHTTPClientTransport } = createRequire(`${process.cwd()}/`)("@modelcontextprotocol/client");

const origin = process.argv[2];
const endpoint = `${origin}/api/mcp`;
const { FLEET_API_READ_TOKEN: read, FLEET_API_WRITE_TOKEN: write } = process.env;
let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
if (!read || !write) {
  check("FLEET_API_READ_TOKEN and FLEET_API_WRITE_TOKEN are set", false, "on Cloudflare: fnox exec -- ...");
  process.exit(1);
}
const text = result => result.content.map(block => block.text).join("");
const auth = token => ({ requestInit: { headers: { authorization: `Bearer ${token}` } } });

const id = "0000000000000001";
const example = JSON.parse(readFileSync("api/example_report.json", "utf8"));
const posted = await fetch(`${origin}/api/devices/${id}/reports`, {
  method: "POST", headers: { authorization: `Bearer ${write}`, "content-type": "application/json" },
  body: JSON.stringify({ ...example, id, ts: Date.now(), reason: "once", next_s: 0, host: { ...example.host, name: "live-test" } }),
});
check("the test machine's report is posted", posted.status === 201, posted.status);

for (const [era, mode, version] of [["stateless", { pin: "2026-07-28" }, "2026-07-28"], ["handshake", "legacy", "2025-11-25"]]) {
  const client = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode } });
  await client.connect(new StreamableHTTPClientTransport(new URL(endpoint), auth(read)));
  const label = name => `${era} (${client.getNegotiatedProtocolVersion()}): ${name}`;
  check(label("connects"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === "fleet-api", [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

  const { tools } = await client.listTools();
  const names = tools.map(tool => tool.name);
  check(label("tools/list is hello and the three reads; posting is not a tool"), names.join() === "hello,listDevices,getDevice,listDeviceReports", names);
  const byName = Object.fromEntries(tools.map(tool => [tool.name, tool]));
  check(label("getDevice takes the id with its pattern"), byName.getDevice?.inputSchema?.properties?.id?.pattern === "^[0-9a-f]{16}$", byName.getDevice?.inputSchema);

  const got = await client.callTool({ name: "getDevice", arguments: { id } });
  const rest = await (await fetch(`${origin}/api/devices/${id}`, { headers: { authorization: `Bearer ${read}` } })).json();
  check(label("getDevice answers what GET /api/devices/{id} answers"), !got.isError && got.structuredContent?.report?.id === id && got.structuredContent.received === rest.received, { got, rest });
  const history = await client.callTool({ name: "listDeviceReports", arguments: { id, limit: 2 } });
  check(label("listDeviceReports"), !history.isError && history.structuredContent?.reports?.length >= 1, history);

  const refused = await client.callTool({ name: "listDeviceReports", arguments: { id, limit: 1000 } });
  const problem = refused.isError ? JSON.parse(text(refused)) : null;
  check(label("validation failure is isError with the location"), problem?.status === 422 && problem.errors?.[0]?.location === "query.limit", refused);

  const error = await client.callTool({ name: "postDeviceReport", arguments: {} }).then(result => result, error => error);
  check(label("postDeviceReport is no tool (error -32602)"), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
  await client.close();
}

// Without a token, a call is refused as the REST route refuses it.
const anonymous = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode: "legacy" } });
await anonymous.connect(new StreamableHTTPClientTransport(new URL(endpoint)));
const denied = await anonymous.callTool({ name: "listDevices", arguments: {} });
check("a call without a token is isError 401", denied.isError && JSON.parse(text(denied)).status === 401, denied);
await anonymous.close();

// The transport's edges, raw.
const get = await fetch(endpoint, { headers: { accept: "text/event-stream" } });
check("GET is 405 (no stream)", get.status === 405 && get.headers.get("allow") === "POST", get.status);

process.exit(failed ? 1 : 0);
