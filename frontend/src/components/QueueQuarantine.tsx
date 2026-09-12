import { useEffect, useState } from 'react';
import { Archive, ClipboardCheck, RefreshCw } from 'lucide-react';
import { useAuth } from '../auth';
import { useI18n } from '../i18n';
import { applyQueueEventQuarantine, getConfig, getQueueEventAudit, getSyncRules, planQueueEventQuarantine, type QueueAuditReceipt, type SyncRule } from '../services/wails';
import { ConfirmDialog } from './ConfirmDialog';

export function QueueQuarantine() {
  const { t } = useI18n();
  const { ensureUnlocked } = useAuth();
  const [rules, setRules] = useState<SyncRule[]>([]);
  const [queues, setQueues] = useState<string[]>([]);
  const [queue, setQueue] = useState('');
  const [rule, setRule] = useState('');
  const [event, setEvent] = useState('');
  const [auditID, setAuditID] = useState('');
  const [receipt, setReceipt] = useState<QueueAuditReceipt | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    void Promise.all([getConfig(), getSyncRules()]).then(([config, saved]) => {
      if (!active) return;
      const available = config.mode === 'server' ? ['server.cdc.ingress.q', 'server.dead.q'] : ['edge.upload.cdc.q', 'edge.downlink.q', 'edge.dead.q'];
      setQueues(available); setQueue(available[0]); setRules(saved); setRule(saved[0]?.id || '');
    }).catch((err: unknown) => { if (active) setError(String(err)); });
    return () => { active = false; };
  }, []);

  async function run(action: 'plan' | 'apply' | 'audit') {
    if (busy) return;
    setConfirm(false); setBusy(true); setError('');
    try {
      if (action !== 'audit' && !(await ensureUnlocked())) return;
      const result = action === 'plan' ? await planQueueEventQuarantine(queue, rule, event.trim()) : action === 'apply' ? await applyQueueEventQuarantine(receipt!.plan.plan_id) : await getQueueEventAudit(auditID.trim());
      setReceipt(result); setAuditID(result.plan.plan_id);
    } catch (err) { setError(err instanceof Error ? err.message : String(err)); }
    finally { setBusy(false); }
  }

  return <section className="queue-quarantine">
    <h3>{t('queueQuarantine')}</h3>
    {error ? <div className="result-line warn" role="alert">{error}</div> : null}
    <div className="toolbar-row">
      <select className="toolbar-input" aria-label={t('queue')} disabled={busy} value={queue} onChange={(e) => { setQueue(e.target.value); setReceipt(null); }}>{queues.map((name) => <option key={name}>{name}</option>)}</select>
      <select className="toolbar-input" aria-label="rule_id" disabled={busy} value={rule} onChange={(e) => { setRule(e.target.value); setReceipt(null); }}>{rules.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}</select>
      <input className="toolbar-input" aria-label={t('quarantineEventId')} placeholder="event_id" disabled={busy} value={event} onChange={(e) => { setEvent(e.target.value); setReceipt(null); }} />
      <button className="button-secondary" disabled={busy || !event.trim() || !rule || !queue} onClick={() => void run('plan')}><ClipboardCheck size={13} /> {t('quarantinePlan')}</button>
    </div>
    <div className="toolbar-row">
      <input className="toolbar-input" aria-label={t('quarantinePlanId')} placeholder="plan_id" disabled={busy} value={auditID} onChange={(e) => setAuditID(e.target.value)} />
      <button className="button-secondary" disabled={busy || !auditID.trim()} onClick={() => void run('audit')}><RefreshCw size={13} /> {t('quarantineAudit')}</button>
    </div>
    {receipt ? <>
      <dl className="queue-audit-details">
        <dt>{t('status')}</dt><dd>{t(`quarantineState_${receipt.state}`)}</dd>
        <dt>event_id / rule_id</dt><dd>{receipt.plan.event_id} / {receipt.plan.rule_id}</dd>
        <dt>{t('queue')}</dt><dd>{receipt.plan.source_queue} → {receipt.plan.target_queue}</dd>
        <dt>{t('quarantinePlanId')}</dt><dd>{receipt.plan.plan_id}</dd>
        <dt>{t('quarantineFingerprint')}</dt><dd>{receipt.plan.fingerprint}</dd>
        <dt>{t('quarantineExpires')}</dt><dd>{new Date(receipt.plan.expires_at).toLocaleString()}</dd>
      </dl>
      {receipt.state !== 'acked' ? <button className="button-danger" disabled={busy || Date.parse(receipt.plan.expires_at) <= Date.now()} onClick={() => setConfirm(true)}><Archive size={13} /> {t('quarantineConfirm')}</button> : null}
    </> : null}
    {confirm && receipt ? <ConfirmDialog title={t('quarantineConfirm')} detail={`${receipt.plan.event_id}: ${receipt.plan.source_queue} → ${receipt.plan.target_queue}. ${t('quarantineWarning')}`} confirmLabel={t('quarantineConfirm')} cancelLabel={t('cancel')} onConfirm={() => void run('apply')} onCancel={() => setConfirm(false)} /> : null}
  </section>;
}
