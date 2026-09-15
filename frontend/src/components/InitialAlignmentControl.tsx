import { useEffect, useRef, useState } from 'react';
import { Play, RefreshCw, Square } from 'lucide-react';
import { useAuth } from '../auth';
import { useI18n } from '../i18n';
import { getInitialAlignmentStatus, interruptInitialAlignment, startInitialAlignment, type InitialAlignmentStatus, type SyncRule } from '../services/wails';

export function InitialAlignmentControl({ rules, dirty, mode, nodeIDs, onRunning, onCompleted }: {
  rules: SyncRule[];
  dirty: boolean;
  mode: string;
  nodeIDs: string[];
  onRunning: (running: boolean) => void;
  onCompleted: () => void;
}) {
  const { t } = useI18n();
  const { authState, ensureUnlocked } = useAuth();
  const [ruleID, setRuleID] = useState('');
  const [peer, setPeer] = useState('');
  const [confirm, setConfirm] = useState(false);
  const [status, setStatus] = useState<InitialAlignmentStatus | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const callbacks = useRef({ onRunning, onCompleted });
  callbacks.current = { onRunning, onCompleted };
  const lastRunning = useRef(false);
  const operation = useRef(0);
  const busyRef = useRef(false);
  const restored = useRef(false);
  const eligible = rules.filter((rule) => rule.initial_alignment?.policy === 'MANUAL');
  const selected = eligible.some((rule) => rule.id === ruleID) ? ruleID : eligible[0]?.id || '';
  const group = mode.toUpperCase() === 'SERVER' && eligible.find((rule) => rule.id === selected)?.direction === 'BIDIRECTIONAL';
  const peerLabel = t(group ? 'alignmentMembers' : 'alignmentPeer');
  const peers = peer.split(/[,;\s]+/).filter(Boolean);
  const candidates = [...new Set([...nodeIDs, ...rules.flatMap((rule) => rule.source_node_ids || [])])].sort();

  function accept(next: InitialAlignmentStatus) {
    setStatus(next);
    callbacks.current.onRunning(next.running);
    if (lastRunning.current && !next.running) callbacks.current.onCompleted();
    lastRunning.current = next.running;
  }

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      try {
        if (busyRef.current) { timer = setTimeout(() => void poll(), 1500); return; }
        const generation = operation.current;
        const next = await getInitialAlignmentStatus();
        if (!active) return;
        if (generation !== operation.current) { timer = setTimeout(() => void poll(), 1500); return; }
        if (!restored.current) {
          setRuleID(next.rule_id || '');
          setPeer(next.peer_node_id || '');
          restored.current = true;
        }
        accept(next);
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : String(err));
      }
      if (active) timer = setTimeout(() => void poll(), 1500);
    }
    void poll();
    return () => { active = false; clearTimeout(timer); };
  }, []);

  async function start() {
    if (!(await ensureUnlocked())) return;
    operation.current++;
    busyRef.current = true;
    callbacks.current.onRunning(true);
    setBusy(true);
    setError('');
    try {
      accept(await startInitialAlignment({ rule_id: selected, peer_node_id: peer.trim(), confirm }));
      setConfirm(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      callbacks.current.onRunning(false);
    } finally { setBusy(false); busyRef.current = false; }
  }

  async function interrupt() {
    if (!(await ensureUnlocked())) return;
    operation.current++;
    busyRef.current = true;
    setBusy(true);
    setError('');
    try { accept(await interruptInitialAlignment()); }
    catch (err) { setError(err instanceof Error ? err.message : String(err)); }
    finally { setBusy(false); busyRef.current = false; }
  }

  return <section className="initial-alignment-control" aria-label={t('initialAlignment')}>
    <div className="alignment-status" aria-live="polite">
      <strong>{t('initialAlignment')}</strong>
      <span className={`status-chip ${status?.stage === 'completed' ? 'status-ok' : status?.stage === 'failed' ? 'status-error' : 'status-unknown'}`}>{status ? t(`alignmentStage_${status.stage}`) : t('loading')}</span>
      {status?.rule_id ? <code>{status.rule_id}</code> : null}
      {status?.peer_node_id ? <span>{t('alignmentPeer')}: {status.peer_node_id}</span> : null}
      {status?.stage === 'completed' ? <span>{t('alignmentRows')}: {status.rows}</span> : null}
    </div>
    {authState.unlocked ? <>
      <div className="alignment-fields">
        <label><span>{t('savedRule')}</span><select aria-label={t('alignmentSavedRule')} value={selected} disabled={busy || status?.running} onChange={(event) => { setRuleID(event.target.value); setConfirm(false); }}>
          {!eligible.length ? <option value="">{t('alignmentNoManualRule')}</option> : null}
          {eligible.map((rule) => <option key={rule.id} value={rule.id}>{rule.id}</option>)}
        </select></label>
        <label><span>{peerLabel}</span><input aria-label={peerLabel} value={peer} disabled={busy || status?.running} autoComplete="off" spellCheck={false} onChange={(event) => { setPeer(event.target.value); setConfirm(false); }} /></label>
        <button className="button-primary" type="button" disabled={busy || !status || status.running || dirty || !selected || !peer.trim() || !confirm} onClick={() => void start()}>
          {status?.stage === 'failed' || status?.stage === 'unknown' ? <RefreshCw size={14} /> : <Play size={14} />}{t(status?.stage === 'failed' || status?.stage === 'unknown' ? 'alignmentRetry' : 'alignmentStart')}
        </button>
        {status?.running ? <button className="button-danger" type="button" disabled={busy || status.stage === 'stopping'} onClick={() => void interrupt()}><Square size={14} />{t('alignmentInterrupt')}</button> : null}
      </div>
      {group && candidates.length > 0 ? <div className="alignment-member-options" role="group" aria-label={t('alignmentMembers')}>
        {candidates.map((id) => <label className="alignment-confirm" key={id}><input type="checkbox" checked={peers.includes(id)} disabled={busy || status?.running} onChange={(event) => { setPeer((event.target.checked ? [...new Set([...peers, id])] : peers.filter((member) => member !== id)).join(',')); setConfirm(false); }} /><span>{id}</span></label>)}
      </div> : null}
      <label className="alignment-confirm"><input type="checkbox" checked={confirm} disabled={busy || status?.running || dirty} onChange={(event) => setConfirm(event.target.checked)} /><span>{t('alignmentConfirm')}</span></label>
      {dirty ? <div className="result-line">{t('alignmentUnsaved')}</div> : null}
    </> : null}
    {status?.message ? <div className="result-line error" role="status">{status.message}</div> : null}
    {error ? <div className="result-line error" role="alert">{error}</div> : null}
  </section>;
}
