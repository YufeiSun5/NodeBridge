import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { writeFile } from 'node:fs/promises';

assert.equal(process.env.NODEBRIDGE_OWNED_CDC_FIXTURE, '1', 'owned fixture required');
const [agent, config, rules, expected, output] = process.argv.slice(2);
assert(['running', 'blocked'].includes(expected));
const child = spawn(agent, ['mcp-stdio', '-config', config, '-rules', rules, '-lab-full-access'], { windowsHide: true });
const pending = new Map();
let sequence = 0, stderr = '';
child.stderr.on('data', b => { stderr += b; });
const closed = new Promise((resolve, reject) => {
  child.on('error', reject);
  child.on('close', code => { for (const p of pending.values()) p.reject(new Error(stderr)); resolve(code); });
});
createInterface({ input: child.stdout }).on('line', line => {
  const reply = JSON.parse(line.replace(/^\uFEFF/, ''));
  const p = pending.get(reply.id);
  if (p) { pending.delete(reply.id); reply.error ? p.reject(new Error(JSON.stringify(reply.error))) : p.resolve(reply.result); }
});
function call(method, params = {}) {
  const id = ++sequence;
  const result = new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  return result;
}
async function tool(name, allowError = false) {
  const result = await call('tools/call', { name, arguments: {} });
  if (!allowError) assert(!result.isError, JSON.stringify(result));
  return JSON.parse(result.content[0].text);
}
const timer = setTimeout(() => child.kill(), 60000);
try {
  await call('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'owned-start-test', version: '1' } });
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
  const start = await tool('nodebridge_start_agent', expected === 'blocked');
  if (expected === 'running') {
    assert.equal(start.ok, true, JSON.stringify(start));
    assert.equal(start.status, 'running');
    const state = await tool('nodebridge_agent_status');
    assert.equal(state.status, 'running', JSON.stringify(state));
    assert(state.pid > 0);
  } else {
    assert.equal(start.ok, false, JSON.stringify(start));
    assert.equal(start.status, 'error');
    assert.match(start.message, /alignment_topology_pending/);
  }
  await writeFile(output, JSON.stringify({ passed: true, expected, start }, null, 2));
  console.log(`PASS MCP startup handshake: ${expected}`);
} finally {
  try { await tool('nodebridge_stop_agent', true); } finally { child.stdin.end(); await closed; clearTimeout(timer); }
}
