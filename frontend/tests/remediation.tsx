import { createRoot } from 'react-dom/client';
import { RulesPage } from '../src/pages/RulesPage';
import { FailuresPage } from '../src/pages/FailuresPage';
import { AuthProvider } from '../src/auth';
import { I18nProvider } from '../src/i18n';
import { ThemeProvider } from '../src/theme';
import { emptyConfig, type SyncRule, type QueueAuditReceipt } from '../src/services/wails';
import '../src/styles/global.css';

const params = new URLSearchParams(location.search);
localStorage.setItem('NodeBridge.language', params.get('language') || 'zh');
localStorage.setItem('nodebridge.theme', params.get('theme') || 'dark');
let revision = 'saved-000000000001';
let data: SyncRule[] = [{ id: 'owned-business-rule-with-a-long-identifier', database_name: 'owned_source', table_name: 'items', target_database_name: 'owned_target', target_table_name: 'items', primary_keys: ['id'], direction: 'EDGE_TO_SERVER', conflict_policy: 'NONE', delete_mode: 'HARD', enable: true, initial_alignment: { policy: 'DISABLED' } }];
const snapshot = () => ({ rules: structuredClone(data), saved_revision: revision, active_revision: 'old-000000000000', activation: 'restart_required' });
let quarantine: QueueAuditReceipt | null = null;
window.go = { datasyncui: { App: {
  GetAuthState: async () => ({ unlocked: true, status: 'unlocked', timeout_seconds: 300 }),
  GetConfig: async () => ({ ...emptyConfig, mode: 'server', node: { id: 'owned-ui', name: 'isolated', location: '' }, mysql: { host: '127.0.0.1', port: 13367, username: 'fixture', password: '', database: 'owned_meta' } }),
  GetNodeOptions: async () => ({ items: [], status: 'configured' }),
  GetSyncRules: async () => snapshot(),
  SaveSyncRules: async (req) => {
    if (params.get('conflict') === '1' || req.expected_revision !== revision) throw new Error('revision_conflict: reload before saving');
    data = structuredClone(req.rules); revision = 'saved-000000000002'; return snapshot();
  },
  PreflightSyncRule: async (req) => ({ rule_id: req.rule_id, side: req.side, ok: false, schema: { database: 'owned_target', table: 'items', engine: 'InnoDB', primary_keys: ['id'] }, findings: [{ code: 'permission_delete', severity: 'error', message: 'Error 1142: DELETE command denied on owned_target.items' }], checked_permissions: ['SELECT', 'INSERT', 'UPDATE'], unverified: ['remote_schema_compatibility', 'existing_data_conflicts', 'cdc_actual_coverage'] }),
  GetFailedEvents: async () => ({ items: [] }),
  PlanQueueEventQuarantine: async (req) => {
    quarantine = { plan: { plan_id: 'a'.repeat(64), node_id: 'owned-ui', source_queue: req.queue, target_queue: `${req.queue}.quarantine`, rule_id: req.rule_id, rule_revision: revision, config_revision: 'c'.repeat(64), event_id: req.event_id, fingerprint: 'f'.repeat(64), expires_at: new Date(Date.now() + 600000).toISOString() }, state: 'planned', updated_at: new Date().toISOString() };
    return structuredClone(quarantine);
  },
  ApplyQueueEventQuarantine: async (req) => {
    if (!req.confirm || req.plan_id !== quarantine?.plan.plan_id) throw new Error('queue_plan_stale');
    quarantine.state = 'acked'; return structuredClone(quarantine);
  },
  GetQueueEventAudit: async () => { if (!quarantine) throw new Error('missing plan'); return structuredClone(quarantine); },
  GetEventStatus: async () => ({ scope: 'bounded_runtime_errors_and_apply_receipts', complete: false, warnings: [], items: [
    { event_id: 'owned-event-9007199254740993', rule_id: data[0].id, state: 'retry_pending', apply_status: 'not_recorded', target_database: 'owned_target', target_table: 'items', last_error: 'target_row_missing: queued event retained', next_retry_at: '2026-09-11T14:00:10+08:00' },
    { event_id: 'owned-superseded-event', rule_id: data[0].id, state: 'superseded', apply_status: 'superseded', target_database: 'owned_target', target_table: 'items' },
    { event_id: 'owned-unverified-event', rule_id: data[0].id, state: 'recorded_outcome_unknown', apply_status: 'recorded_outcome_unknown', target_database: 'owned_target', target_table: 'items' },
  ] }),
} } };

createRoot(document.getElementById('root')!).render(<ThemeProvider><I18nProvider><AuthProvider><main className="content-area">{params.get('page') === 'failures' ? <FailuresPage /> : <RulesPage />}</main></AuthProvider></I18nProvider></ThemeProvider>);
