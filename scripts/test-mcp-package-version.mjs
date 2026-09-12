import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';

const [agent, version, output, config, rules] = process.argv.slice(2);
assert(agent && version && output && config && rules, 'agent, expected version, report and isolated config/rules paths are required');
const child = spawn(agent, ['mcp-stdio', '-config', config, '-rules', rules], { windowsHide: true });
let stderr = '', sequence = 0;
const pending = new Map();
child.stderr.setEncoding('utf8').on('data', chunk => { stderr += chunk; });
const closed = new Promise((resolve, reject) => {
  child.once('error', reject);
  child.once('close', code => { for (const request of pending.values()) request.reject(new Error(`MCP closed ${code}: ${stderr}`)); resolve(code); });
});
createInterface({ input: child.stdout }).on('line', line => {
  const response = JSON.parse(line.replace(/^\uFEFF/, ''));
  const request = pending.get(response.id);
  if (request) { pending.delete(response.id); request.resolve(response); }
});
async function call(method, params = {}) {
  const id = ++sequence;
  const reply = new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  const result = await reply;
  assert.equal(result.error, undefined, JSON.stringify(result));
  return result.result;
}
const timeout = setTimeout(() => child.kill(), 30000);
const report = { agent, agent_sha256: createHash('sha256').update(await readFile(agent)).digest('hex').toUpperCase(), at: new Date().toISOString(), passed: false };
let validated = false;
try {
  report.initialize = await call('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'package-version-gate', version: '1' } });
  assert.equal(report.initialize.serverInfo.version, version);
  assert.equal(report.initialize.protocolVersion, '2025-11-25');
  assert(report.initialize.capabilities.tools);
  assert(report.initialize.capabilities.resources);
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
  await call('ping');
  const resources = (await call('resources/list')).resources;
  report.resources = resources.map(resource => resource.uri);
  assert.equal(resources.length, 5);
  const resource = await call('resources/read', { uri: 'nodebridge://diagnostic-summary' });
  assert.equal(resource.contents[0].uri, 'nodebridge://diagnostic-summary');
  JSON.parse(resource.contents[0].text);
  const catalog = (await call('tools/list')).tools;
  report.tools = catalog.length;
  assert.equal(report.tools, 39);
  for (const name of ['capabilities', 'rule_preflight', 'event_status', 'queue_event_plan', 'queue_event_apply', 'queue_event_audit']) {
    assert(catalog.some(tool => tool.name === `nodebridge_${name}`), name);
  }
  for (const tool of catalog) {
    assert.equal(tool.inputSchema.type, 'object', tool.name);
    assert(tool.description, tool.name);
  }
  assert(catalog.some(tool => tool.name === 'nodebridge_mysql_diagnostics' && tool.annotations.readOnlyHint));
  const capabilitiesResult = await call('tools/call', { name: 'nodebridge_capabilities', arguments: {} });
  assert(!capabilitiesResult.isError, JSON.stringify(capabilitiesResult));
  report.capabilities = JSON.parse(capabilitiesResult.content[0].text);
  assert.equal(report.capabilities.transport, 'stdio');
  assert(!report.capabilities.features.some(feature => feature.id === 'mcp.remote_vpn'));
  assert(report.capabilities.features.some(feature => feature.id === 'sync.binary_values' && feature.supported));
  assert(report.capabilities.features.some(feature => feature.id === 'initial_alignment.execute' && !feature.supported));
  const result = await call('tools/call', { name: 'nodebridge_overview', arguments: {} });
  assert(!result.isError, JSON.stringify(result));
  const overview = JSON.parse(result.content[0].text);
  report.overview_version = overview.version;
  assert.equal(overview.version, version);
  const diagnosticResult = await call('tools/call', { name: 'nodebridge_mysql_diagnostics', arguments: {} });
  assert(!diagnosticResult.isError, JSON.stringify(diagnosticResult));
  report.diagnostics = JSON.parse(diagnosticResult.content[0].text);
  assert.equal(report.diagnostics.status, 'ok');
  assert.equal(report.diagnostics.steps.length, 4);
  validated = true;
} finally {
  child.stdin.end();
  report.exit_code = await closed;
  report.passed = validated && report.exit_code === 0;
  clearTimeout(timeout);
  await writeFile(output, JSON.stringify(report, null, 2));
}
assert.equal(report.exit_code, 0);
console.log(`MCP/UI overview version ${version}, ${report.tools} tools verified`);
