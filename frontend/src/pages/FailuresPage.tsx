import { useEffect, useState } from 'react';
import { EmptyState, ErrorState, LoadingState } from '../components/PageState';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { QueueQuarantine } from '../components/QueueQuarantine';
import { SectionHeader } from '../components/SectionHeader';
import { useAuth } from '../auth';
import { translateStatus, useI18n } from '../i18n';
import {
  getDeadLetters,
  getConfig,
  getFailedEvents,
  getEventStatus,
  type EventStatusResponse,
  isConfigMissing,
  retryFailedEvents,
  retryFailedEvent,
  type DeadLetterMessageDTO,
  type FailedEventDTO,
  type OperationResult,
} from '../services/wails';

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export function FailuresPage() {
  const { t } = useI18n();
  const { ensureUnlocked } = useAuth();
  const [items, setItems] = useState<FailedEventDTO[]>([]);
  const [configMissing, setConfigMissing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [operation, setOperation] = useState<OperationResult | null>(null);
  const [deadLetters, setDeadLetters] = useState<DeadLetterMessageDTO[]>([]);
  const [deadLetterQueue, setDeadLetterQueue] = useState('');
  const [deadLettersVisible, setDeadLettersVisible] = useState(false);
  const [confirmBatchRetry, setConfirmBatchRetry] = useState(false);
  const [eventFilter, setEventFilter] = useState('');
  const [eventStatus, setEventStatus] = useState<EventStatusResponse | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const config = await getConfig();
      const missing = isConfigMissing(config);
      setConfigMissing(missing);
      setItems(missing ? [] : await getFailedEvents(50));
      if (!missing) await inspectEvent();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('failuresError'));
    } finally {
      setLoading(false);
    }
  }

  async function inspectEvent() {
    setStatusLoading(true);
    try {
      setEventStatus(await getEventStatus(eventFilter.trim()));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('failuresError'));
    } finally {
      setStatusLoading(false);
    }
  }

  async function retry(item: FailedEventDTO) {
    setOperation(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      setOperation(await retryFailedEvent(item.event_id, item.target_node_id));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('failuresError'));
    }
  }

  async function retryBatch() {
    setOperation(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    setConfirmBatchRetry(true);
  }

  async function runBatchRetry() {
    try {
      setOperation(await retryFailedEvents(100));
      setConfirmBatchRetry(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('failuresError'));
    }
  }

  async function previewDeadLetters() {
    setOperation(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      setDeadLetters(await getDeadLetters({ queue: deadLetterQueue.trim(), limit: 10 }));
      setDeadLettersVisible(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('failuresError'));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <section className="page-panel">
      <SectionHeader title={t('failures')} tone="db" />
      {loading ? <LoadingState title={t('loadingFailures')} /> : null}
      {error ? <ErrorState title={t('failuresError')} detail={error} /> : null}
      <h3>{t('runtimeEventStatus')}</h3>
      <div className="toolbar-row">
        <input className="toolbar-input" aria-label={t('eventId')} value={eventFilter} onChange={(e) => setEventFilter(e.target.value)} placeholder="event_id" />
        <button className="button-secondary" type="button" disabled={statusLoading || configMissing} onClick={() => void inspectEvent()}>{statusLoading ? t('checking') : t('queryEventStatus')}</button>
      </div>
      {eventStatus ? <>
        <div className="notice-line">{t('eventStatusScope')}</div>
        {eventStatus.items.length ? <div className="table-scroll"><table className="terminal-table wide-content-table"><thead><tr><th>event_id / rule_id</th><th>{t('status')}</th><th>{t('targetTable')}</th><th>{t('nextRetry')}</th><th>{t('error')}</th></tr></thead><tbody>
          {eventStatus.items.map((item) => <tr key={item.event_id}><td>{item.event_id}<br />{item.rule_id || '-'}</td><td>{t(`eventState_${item.state}`)}<br />{t(`eventApply_${item.apply_status}`)}</td><td>{item.target_database}.{item.target_table}</td><td>{formatTime(item.next_retry_at)}</td><td>{item.mysql_error_code || ''} {item.last_error || '-'}</td></tr>)}
        </tbody></table></div> : <EmptyState title={t('noRuntimeObservations')} detail={t('eventStatusScope')} />}
        {eventStatus.warnings.filter((warning) => !warning.startsWith('Empty results')).map((warning) => <div className="notice-line" key={warning}>{warning}</div>)}
      </> : null}
      {!configMissing && !loading ? <QueueQuarantine /> : null}
      <h3>{t('ackFailures')}</h3>
      {operation ? (
        <div className={operation.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{translateStatus(t, operation.status)}</span>
          <strong>{operation.message || t('operationCompleted')}</strong>
        </div>
      ) : null}

      <div className="toolbar-row">
        <button className="button-secondary" type="button" onClick={() => void load()}>
          {t('refresh')}
        </button>
        <button className="button-danger" type="button" onClick={() => void retryBatch()} disabled={configMissing || items.length === 0}>
          {t('retryBatch')}
        </button>
        <input
          className="toolbar-input"
          value={deadLetterQueue}
          onChange={(event) => setDeadLetterQueue(event.target.value)}
          placeholder={t('deadLetterQueuePlaceholder')}
        />
        <button className="button-secondary" type="button" onClick={() => void previewDeadLetters()} disabled={configMissing}>
          {t('previewDeadLetters')}
        </button>
      </div>

      {!loading && items.length === 0 ? (
        <section className="operational-empty">
          <EmptyState
            title={configMissing ? t('configMissing') : t('noAckFailures')}
            detail={configMissing ? t('failuresConfigMissing') : t('ackFailureScope')}
          />
          <div className="operational-empty-grid">
            <div className="readonly-item">
              <span>{t('status')}</span>
              <strong>{configMissing ? t('notConfigured') : t('noAckFailures')}</strong>
            </div>
            <div className="readonly-item">
              <span>{t('retryBatch')}</span>
              <strong>{items.length}</strong>
            </div>
            <div className="readonly-item">
              <span>{t('deadLettersPreview')}</span>
              <strong>{configMissing ? t('notConfigured') : t('previewDeadLetters')}</strong>
            </div>
          </div>
        </section>
      ) : null}

      {items.length > 0 ? (
        <table className="terminal-table">
          <thead>
            <tr>
              <th>{t('eventId')}</th>
              <th>{t('targetNode')}</th>
              <th>{t('status')}</th>
              <th>{t('error')}</th>
              <th>{t('created')}</th>
              <th>{t('action')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={`${item.event_id}-${item.target_node_id}`}>
                <td>{item.event_id}</td>
                <td>{item.target_node_id}</td>
                <td>{translateStatus(t, item.status)}</td>
                <td>{item.error_message}</td>
                <td>{formatTime(item.created_at)}</td>
                <td>
                  <button className="button-tool compact" type="button" onClick={() => void retry(item)}>
                    {t('retry')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}

      {deadLettersVisible ? (
        <section className="dead-letter-panel">
          <div className="agent-process-head">
            <strong>{t('deadLettersPreview')}</strong>
            <span className="status-chip">{deadLetters.length}</span>
          </div>
          <div className="notice-line">{t('deadLettersSensitive')}</div>
          {deadLetters.length === 0 ? <EmptyState title={t('noDeadLetters')} detail={t('noDeadLettersDetail')} /> : null}
          {deadLetters.length > 0 ? (
            <div className="table-scroll dead-letter-scroll">
            <table className="terminal-table wide-content-table">
              <thead>
                <tr>
                  <th>{t('queue')}</th>
                  <th>{t('type')}</th>
                  <th>{t('size')}</th>
                  <th>{t('preview')}</th>
                  <th>{t('headers')}</th>
                </tr>
              </thead>
              <tbody>
                {deadLetters.map((item, index) => (
                  <tr key={`${item.queue}-${index}`}>
                    <td>{item.queue || '-'}</td>
                    <td>{item.content_type || '-'}</td>
                    <td>{item.body_size}</td>
                    <td className="log-message">{item.body_preview || '-'}</td>
                    <td className="log-message">{item.headers ? JSON.stringify(item.headers) : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            </div>
          ) : null}
        </section>
      ) : null}
      {confirmBatchRetry ? (
        <ConfirmDialog
          title={t('confirmBatchRetryTitle')}
          detail={t('confirmBatchRetryDetail').replace('{count}', String(items.length)).replace('{limit}', '100')}
          confirmLabel={t('retryBatch')}
          cancelLabel={t('cancel')}
          onCancel={() => setConfirmBatchRetry(false)}
          onConfirm={() => void runBatchRetry()}
        />
      ) : null}
    </section>
  );
}
