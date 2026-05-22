import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';

const root = resolve(import.meta.dirname, '..');
const service = readFileSync(resolve(root, 'src/services/wails.ts'), 'utf8');
const generated = readFileSync(resolve(root, 'wailsjs/go/datasyncui/App.js'), 'utf8');
const configPage = readFileSync(resolve(root, 'src/pages/ConfigPage.tsx'), 'utf8');
const settingsPage = readFileSync(resolve(root, 'src/pages/SettingsPage.tsx'), 'utf8');

assert.match(generated, /window\['go'\]\['datasyncui'\]\['App'\]/, 'generated Wails binding must use datasyncui.App');
assert.match(service, /window\.go\?\.datasyncui\?\.App/, 'service must call datasyncui.App first');
assert.doesNotMatch(service, /return window\.go\?\.main\?\.App;/, 'service must not only call main.App');
assert.match(service, /admin_password\?: string/, 'SecurityConfig must include admin_password');
assert.match(service, /UnlockAdmin/, 'service must expose UnlockAdmin');
assert.match(service, /GetAuthState/, 'service must expose GetAuthState');
assert.match(service, /GetAgentProcessStatus/, 'service must expose GetAgentProcessStatus');
assert.match(service, /RetryFailedEvents/, 'service must expose RetryFailedEvents');
assert.match(service, /GetDeadLetters/, 'service must expose GetDeadLetters');
assert.match(service, /GetMCPServerStatus/, 'service must expose GetMCPServerStatus');
assert.match(service, /SetMCPServerEnabled/, 'service must expose SetMCPServerEnabled');
assert.match(service, /GetManagedInstallPlan/, 'service must expose GetManagedInstallPlan');
assert.match(service, /ApplyManagedInstall/, 'service must expose ApplyManagedInstall');
assert.match(settingsPage, /admin_password/, 'Settings page must expose admin_password input');
assert.doesNotMatch(configPage, /admin_password/, 'Config page must not mix app security settings into sync config');

console.log('wails contract checks passed');
