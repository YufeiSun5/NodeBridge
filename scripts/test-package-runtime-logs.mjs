import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { createHash } from 'node:crypto';

const [agentArg, version, directoryArg] = process.argv.slice(2);
assert(agentArg && version && directoryArg, 'agent, version, isolated output directory required');
const agent = resolve(agentArg), directory = resolve(directoryArg);
await mkdir(directory, { recursive: true });
await assert.rejects(readFile(join(directory,'logs','sync-runtime.jsonl')), {code:'ENOENT'}, 'use a fresh fixture directory; stale logs must not satisfy this test');
const config = join(directory, 'config.yaml'), rules = join(directory, 'rules.yaml');
await writeFile(config, 'mode: server\nnode:\n  id: package-log-test\nmysql:\n  database: isolated_log_fixture\nrabbitmq: {}\n');
await writeFile(rules, 'rules: []\n');
async function run(args, input = '') {
  const child = spawn(agent, args, { windowsHide: true, cwd: directory });
  let stdout = '', stderr = '';
  child.stdout.setEncoding('utf8').on('data', data => { stdout += data; });
  child.stderr.setEncoding('utf8').on('data', data => { stderr += data; });
  const timer = setTimeout(() => child.kill(), 20000);
  const code = await new Promise((done, fail) => {
    child.once('error', fail);
    child.once('close', done);
    child.stdin.end(input);
  }).finally(() => clearTimeout(timer));
  return { code, stdout, stderr };
}
const failure = await run(['run','-config',config,'-rules',rules,'-max-steps','1']);
assert.equal(failure.code, 1, 'missing broker config must fail');
const path = join(directory, 'logs', 'sync-runtime.jsonl');
const persisted = (await readFile(path, 'utf8')).trim().split('\n').map(JSON.parse);
const error = persisted.find(row => row.level === 'ERROR' && row.msg === 'agent run failed');
assert(error && error.error && error.node_id === 'package-log-test');
assert.equal(error.version, version);
assert(Number.isFinite(Date.parse(error.time)) && error.pid > 0);
// A numeric test password must not corrupt the PID when MCP redacts the log.
await writeFile(config, `mode: server\nnode:\n  id: package-log-test\nmysql:\n  database: isolated_log_fixture\n  password: "${error.pid}"\nrabbitmq: {}\n`);
const requests = [
  {jsonrpc:'2.0',id:1,method:'initialize',params:{protocolVersion:'2025-11-25',capabilities:{},clientInfo:{name:'runtime-log-gate',version:'1'}}},
  {jsonrpc:'2.0',id:2,method:'tools/call',params:{name:'nodebridge_logs',arguments:{limit:100}}},
  {jsonrpc:'2.0',id:3,method:'resources/read',params:{uri:'nodebridge://logs'}},
];
const mcp = await run(['mcp-stdio','-lab-full-access','-config',config,'-rules',rules],requests.map(x=>JSON.stringify(x)).join('\n')+'\n');
assert.equal(mcp.code,0,mcp.stderr);
const replies = mcp.stdout.trim().split(/\r?\n/).map(JSON.parse);
const tool = replies.find(x=>x.id===2);
assert(!tool.error && !tool.result.isError, JSON.stringify(tool));
const logs = JSON.parse(tool.result.content[0].text);
assert.equal(resolve(logs.log_path),resolve(path));
assert(logs.items.some(line=>JSON.parse(line).msg === 'agent run failed'));
assert.equal(JSON.parse(logs.items.find(line=>JSON.parse(line).msg === 'agent run failed')).pid,error.pid);
const resource = replies.find(x=>x.id===3);
assert(!resource.error,JSON.stringify(resource));
const resourceLogs = JSON.parse(resource.result.contents[0].text);
assert.deepEqual(resourceLogs.items,logs.items);
const report = {passed:true,at:new Date().toISOString(),version,agent,agent_sha256:createHash('sha256').update(await readFile(agent)).digest('hex').toUpperCase(),run_exit_code:failure.code,persisted_error:error,mcp_tool:true,mcp_resource:true,log_path:path};
await writeFile(join(directory,'evidence.json'),JSON.stringify(report,null,2));
console.log(`PASS: packaged agent startup ERROR persisted and readable through MCP tool/resource (${version})`);
