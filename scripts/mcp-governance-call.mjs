import { spawn } from 'node:child_process';

const [config, rules, tool, encodedArguments = 'base64:e30='] = process.argv.slice(2);
if (!config || !rules || !tool) {
  throw new Error('usage: node scripts/mcp-governance-call.mjs <config> <rules> <tool> [arguments-json]');
}

const agent = process.env.NODEBRIDGE_AGENT_PATH;
const child = spawn(agent || 'go', [
  ...(agent ? [] : ['run', './cmd/sync-agent']), 'mcp-stdio', '-lab-full-access',
  '-config', config, '-rules', rules,
], { cwd: process.cwd(), windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });

const rawArguments = encodedArguments.startsWith('base64:')
  ? Buffer.from(encodedArguments.slice(7), 'base64').toString('utf8')
  : encodedArguments;

let stdout = '';
let stderr = '';
child.stdout.setEncoding('utf8').on('data', chunk => { stdout += chunk; });
child.stderr.setEncoding('utf8').on('data', chunk => { stderr += chunk; });

child.stdin.write(JSON.stringify({
  jsonrpc: '2.0', id: 1, method: 'initialize',
  params: { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'nodebridge-governance-e2e', version: '1' } },
}) + '\n');
child.stdin.write(JSON.stringify({
  jsonrpc: '2.0', id: 2, method: 'tools/call',
  params: { name: tool, arguments: JSON.parse(rawArguments) },
}) + '\n');
child.stdin.end();

const exitCode = await new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    child.kill();
    reject(new Error(`MCP call timed out: ${tool}`));
  }, 90000);
  child.once('error', error => {
    clearTimeout(timer);
    reject(error);
  });
  child.once('close', code => {
    clearTimeout(timer);
    resolve(code);
  });
});

if (exitCode !== 0) {
  throw new Error(`MCP process failed (${exitCode}): ${stderr.trim()}`);
}
const messages = stdout.split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line.replace(/^\uFEFF/, '')));
const response = messages.find(message => message.id === 2);
if (!response || response.error) {
  throw new Error(`MCP protocol error: ${JSON.stringify(response)}`);
}
if (response.result?.isError) {
  throw new Error(response.result.content?.[0]?.text || `MCP tool failed: ${tool}`);
}
const text = response.result?.content?.[0]?.text;
if (typeof text !== 'string') {
  throw new Error(`MCP tool returned no JSON text: ${tool}`);
}
process.stdout.write(JSON.stringify(JSON.parse(text)));
