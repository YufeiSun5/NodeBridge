import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';

const root = process.cwd();
const evidence = path.resolve(process.argv[3] || '.cache/mcp-v046-smoke');
await fs.mkdir(evidence, { recursive: true });
const exe = path.resolve(process.argv[2] || 'build/bin/SyncAgent.exe');
const work = await fs.mkdtemp(path.join(evidence, 'run-'));
const config = path.join(work, 'config.yaml');
const checks = [];

async function connect(command, args) {
  const child = spawn(command, args, { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
  let buffer = '', stderr = '', sequence = 0;
  const pending = new Map();
  child.stderr.setEncoding('utf8').on('data', chunk => { stderr += chunk; });
  child.stdout.setEncoding('utf8').on('data', chunk => {
    buffer += chunk;
    for (;;) {
      const index = buffer.indexOf('\n');
      if (index < 0) break;
      const line = buffer.slice(0, index).replace(/^\uFEFF/, '').trim();
      buffer = buffer.slice(index + 1);
      if (!line) continue;
      let message;
      try { message = JSON.parse(line); } catch { for (const item of pending.values()) item.reject(new Error('Non-JSON stdout: ' + line)); continue; }
      const item = pending.get(message.id);
      if (item) { clearTimeout(item.timer); pending.delete(message.id); item.resolve(message); }
    }
  });
  const ended = new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', code => {
      for (const item of pending.values()) { clearTimeout(item.timer); item.reject(new Error('MCP closed: ' + code + ' ' + stderr)); }
      resolve(code);
    });
  });
  return {
    async request(method, params) {
      const id = ++sequence;
      const reply = new Promise((resolve, reject) => {
        const timer = setTimeout(() => { pending.delete(id); child.kill(); reject(new Error('MCP request timed out: ' + method)); }, 45000);
        pending.set(id, { resolve, reject, timer });
      });
      child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
      return reply;
    },
    async tool(name, args = {}) {
      const response = await this.request('tools/call', { name, arguments: args });
      assert.equal(response.error, undefined, JSON.stringify(response));
      return response.result;
    },
    async close() {
      child.stdin.end();
      const timer = setTimeout(() => child.kill(), 5000);
      try { assert.equal(await ended, 0, stderr); } finally { clearTimeout(timer); }
    }
  };
}

const client = await connect(exe, ['mcp-stdio', '-lab-full-access', '-config', config]);
try {
  const initialized = await client.request('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'nodebridge-release-smoke', version: '1' } });
  assert.equal(initialized.result.protocolVersion, '2025-11-25');
  const catalog = await client.request('tools/list', {});
  assert(catalog.result.tools.length >= 26);
  checks.push({ name: 'initialize_and_tools', passed: true, count: catalog.result.tools.length });
  const patch = {
    mode: 'edge', node: { id: 'lab-edge', name: '\u5185\u7f51\u6d4b\u8bd5', location: 'lab' },
    mysql: { host: '127.0.0.1', port: 3306, username: 'lab', password: 'smoke-mysql-secret', database: 'lab' },
    rabbitmq: { mode: 'external', install: false, local_url: 'amqp://lab:smoke-amqp-secret@127.0.0.1:5672/lab', server_url: 'amqp://lab:smoke-amqp-secret@127.0.0.1:5672/lab', management_url: 'http://127.0.0.1:15672', username: 'lab', password: 'smoke-rabbit-secret', vhost: 'lab' },
    cdc: { type: 'canal', mode: 'external', install: false, reader_name: 'lab', canal_addr: '127.0.0.1:11111', config_dir: work, service_name: 'NodeBridgeCanal', destination: 'lab', username: 'lab', password: 'smoke-canal-secret', filter: 'lab.*', batch_size: 100, use_gtid: false },
    sync: { upload_batch_size: 100, dispatch_batch_size: 100, apply_lanes: 2, enable_crud_compact: true, flush_interval_millis: 500, retry_interval_seconds: 10, heartbeat_interval_seconds: 15, node_timeout_seconds: 60 },
    log_web: { enable: false, bind: '127.0.0.1', port: 18180, token: 'smoke-log-secret' },
    mcp_server: { enable: false }, security: { admin_password: 'smoke-admin-secret', exit_password: 'smoke-exit-secret' }
  };
  const preview = await client.tool('nodebridge_validate_config_patch', { patch });
  assert.equal(preview.isError, false);
  await assert.rejects(fs.stat(config), { code: 'ENOENT' });
  const saved = await client.tool('nodebridge_save_config_patch', { patch });
  assert.equal(saved.isError, false);
  assert(!JSON.stringify(saved).includes('smoke-mysql-secret'));
  const disk = await fs.readFile(config, 'utf8');
  for (const secret of ['smoke-mysql-secret', 'smoke-amqp-secret', 'smoke-rabbit-secret', 'smoke-canal-secret', 'smoke-log-secret', 'smoke-admin-secret', 'smoke-exit-secret']) assert(!disk.includes(secret));
  assert(disk.includes('dpapi:'));
  checks.push({ name: 'all_config_sections_and_secrets_encrypted', passed: true });
  const patchSchema = catalog.result.tools.find(t => t.name === 'nodebridge_save_config_patch').inputSchema.properties.patch;
  for (const [section, value] of Object.entries(patch)) {
    const schema = patchSchema.properties[section];
    assert(schema, section);
    if (typeof value === 'object') for (const field of Object.keys(schema.properties)) assert(Object.hasOwn(value, field), `smoke is missing ${section}.${field}`);
  }
  assert.equal((await client.tool('nodebridge_save_sync_rules', {})).isError, true);
  assert.equal((await client.tool('nodebridge_save_sync_rules', { rules: [] })).isError, false);
  assert.equal((await client.tool('nodebridge_save_config_patch', { patch: { mysql: { pasword: 'typo' } } })).isError, true);
  checks.push({ name: 'validation_and_missing_rules_errors', passed: true });
} finally { await client.close(); }

// Execute the exact encoded Windows command generated for SSH, with redirected stdio.
const generated = await new Promise((resolve, reject) => {
  const child = spawn(exe, ['mcp-client-config', '-lab-full-access', '-ssh-host', 'lab@192.0.2.1', '-config', config], { windowsHide: true });
  let data = ''; child.stdout.on('data', chunk => { data += chunk; });
  child.once('error', reject); child.once('close', code => code === 0 ? resolve(JSON.parse(data)) : reject(new Error('client config failed')));
});
const remote = generated.mcpServers.nodebridge.args.at(-1);
const encoded = remote.slice(remote.lastIndexOf(' ') + 1);
const reconnect = await connect('powershell.exe', ['-NoLogo', '-NoProfile', '-NonInteractive', '-EncodedCommand', encoded]);
try {
  const configResult = await reconnect.tool('nodebridge_config_summary');
  assert.equal(configResult.isError, false);
  assert(configResult.content[0].text.includes('\u5185\u7f51\u6d4b\u8bd5'));
  assert(!configResult.content[0].text.includes('smoke-mysql-secret'));
  assert.equal((await reconnect.tool('nodebridge_save_config_patch', { patch: { node: { name: 'reconnected' }, mysql: { password: '******' } } })).isError, false);
  checks.push({ name: 'generated_ssh_powershell_stdio_unicode_and_reconnect', passed: true, actual_ssh_network_test: false });
} finally { await reconnect.close(); }

const report = { passed: true, binary: exe, work, checks, actual_remote_machine_test: false, created_at: new Date().toISOString() };
await fs.writeFile(path.join(evidence, 'report.json'), JSON.stringify(report, null, 2));
console.log(JSON.stringify(report, null, 2));
