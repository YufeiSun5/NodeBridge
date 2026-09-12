import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import { createInterface } from 'node:readline';
import { promisify } from 'node:util';

const [agent, version, host, key, output] = process.argv.slice(2);
assert(agent && version && host && key && output, 'agent, version, SSH host, key path and report required');
const exec = promisify(execFile);
const ssh = ['-i', key, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'ConnectTimeout=8'];
const ps = text => `'${text.replaceAll("'", "''")}'`;
async function remote(script) {
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  return (await exec('ssh', [...ssh, host, `powershell.exe -NoProfile -NonInteractive -EncodedCommand ${encoded}`], { windowsHide: true, timeout: 30000 })).stdout.trim();
}
const name = `NodeBridge-package-${randomUUID()}`;
const directory = await remote(`$ErrorActionPreference='Stop'; $p=Join-Path ([IO.Path]::GetTempPath()) ${ps(name)}; if(Test-Path -LiteralPath $p){throw 'fixture already exists'}; New-Item -ItemType Directory -Path $p | Out-Null; [IO.Path]::GetFullPath($p)`);
assert(directory.endsWith(name) && !/[\r\n]/.test(directory), 'unexpected remote fixture path');
const binary = `${directory}\\SyncAgent.exe`;
const report = { passed: false, version, host, scope: 'temporary remote binary, no installation or business configuration', agent_sha256: createHash('sha256').update(await readFile(agent)).digest('hex').toUpperCase() };
try {
  await exec('scp', [...ssh, agent, `${host}:${binary.replaceAll('\\', '/')}`], { windowsHide: true, timeout: 60000 });
  const hash = await remote(`(Get-FileHash -LiteralPath ${ps(binary)} -Algorithm SHA256).Hash`);
  assert.equal(hash, report.agent_sha256);
  const generated = await exec(agent, ['mcp-client-config', '-name', 'owned-ssh', '-lab-full-access', '-ssh-host', host, '-ssh-key', key, '-exe', binary, '-config', `${directory}\\config.yaml`, '-rules', `${directory}\\rules.yaml`], { windowsHide: true, timeout: 10000 });
  const definition = JSON.parse(generated.stdout).mcpServers['owned-ssh'];
  assert.equal(definition.command, 'ssh');
  const child = spawn(definition.command, ['-o', 'StrictHostKeyChecking=yes', ...definition.args], { windowsHide: true });
  let sequence = 0, stderr = '';
  const pending = new Map();
  child.stderr.setEncoding('utf8').on('data', data => { stderr += data; });
  const closed = new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', code => { for (const request of pending.values()) request.reject(new Error(`SSH MCP closed ${code}: ${stderr}`)); resolve(code); });
  });
  createInterface({ input: child.stdout }).on('line', line => {
    const value = JSON.parse(line.replace(/^\uFEFF/, ''));
    const request = pending.get(value.id);
    if (request) { pending.delete(value.id); request.resolve(value); }
  });
  async function call(method, params = {}) {
    const id = ++sequence;
    const reply = new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
    const response = await reply;
    assert.equal(response.error, undefined, JSON.stringify(response));
    return response.result;
  }
  const timer = setTimeout(() => child.kill(), 20000);
  try {
    report.initialize = await call('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'owned-ssh-package-gate', version: '1' } });
    assert.equal(report.initialize.serverInfo.version, version);
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
    await call('ping');
    report.tools = (await call('tools/list')).tools.length;
    report.resources = (await call('resources/list')).resources.length;
    assert.equal(report.tools, 39);
    assert.equal(report.resources, 5);
    const result = await call('tools/call', { name: 'nodebridge_capabilities', arguments: {} });
    assert(!result.isError, JSON.stringify(result));
    const capabilities = JSON.parse(result.content[0].text);
    assert.equal(capabilities.transport, 'stdio');
    assert(!capabilities.features.some(feature => feature.id === 'mcp.remote_vpn'));
    report.passed = true;
  } finally {
    child.stdin.end();
    report.exit_code = await closed;
    clearTimeout(timer);
    assert.equal(report.exit_code, 0, stderr);
  }
} finally {
  await remote(`$ErrorActionPreference='Stop'; $expected=[IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) ${ps(name)})); $p=[IO.Path]::GetFullPath(${ps(directory)}); if($p -ne $expected){throw 'unsafe cleanup path'}; if(Test-Path -LiteralPath $p){Remove-Item -LiteralPath $p -Recurse -Force}; if(Test-Path -LiteralPath $p){throw 'cleanup incomplete'}`);
  report.remote_fixture_removed = true;
  await writeFile(output, JSON.stringify(report, null, 2));
}
console.log(`PASS: SSH candidate MCP ${version}, ${report.tools} tools, ${report.resources} resources, fixture removed`);
