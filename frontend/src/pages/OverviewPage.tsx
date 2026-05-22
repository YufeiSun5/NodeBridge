import { useEffect, useState } from 'react';
import { MetricGrid } from '../components/MetricGrid';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { useAuth } from '../auth';
import { translateStatus, useI18n } from '../i18n';
import {
  fallbackOverview,
  getAgentProcessStatus,
  getOverview,
  restartAgent,
  startAgent,
  stopAgent,
  type AgentProcessStatus,
  type OperationResult,
} from '../services/wails';
import type { RuntimeSummary } from '../stores/uiStore';

function toneForStatus(status: string) {
  if (status === 'running' || status === 'stopped') return 'ok';
  if (status === 'error') return 'error';
  if (status === 'unsupported' || status === 'unknown') return 'muted';
  return 'warn';
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export function OverviewPage() {
  const { t } = useI18n();
  const { ensureUnlocked } = useAuth();
  const [summary, setSummary] = useState<RuntimeSummary>(fallbackOverview);
  const [configMissing, setConfigMissing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [operation, setOperation] = useState<OperationResult | null>(null);
  const [agentProcess, setAgentProcess] = useState<AgentProcessStatus | null>(null);
  const [pendingControl, setPendingControl] = useState<'stop' | 'restart' | null>(null);

  async function refresh() {
    setLoading(true);
    setError('');
    try {
      const [overview, processStatus] = await Promise.all([getOverview(), getAgentProcessStatus()]);
      setConfigMissing(!overview.configLoaded);
      setSummary(overview);
      setAgentProcess(processStatus);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('overviewError'));
      setSummary(fallbackOverview);
      setAgentProcess(null);
      setConfigMissing(false);
    } finally {
      setLoading(false);
    }
  }

  async function runControl(action: 'start' | 'stop' | 'restart') {
    setOperation(null);
    if (configMissing && action !== 'stop') {
      setOperation({ ok: false, status: 'config_missing', message: t('configMissingAction') });
      return;
    }
    if (!(await ensureUnlocked())) {
      return;
    }
    if (action === 'stop' || action === 'restart') {
      setPendingControl(action);
      return;
    }
    await executeControl(action);
  }

  async function executeControl(action: 'start' | 'stop' | 'restart') {
    const result =
      action === 'start' ? await startAgent() : action === 'stop' ? await stopAgent() : await restartAgent();
    setOperation(result);
    setPendingControl(null);
    await refresh();
  }

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <section className="page-panel">
      <SectionHeader title={t('overview')} />
      {loading ? <LoadingState title={t('loadingOverview')} /> : null}
      {error ? <ErrorState title={t('overviewError')} detail={error} /> : null}
      {operation ? (
        <div className={operation.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{translateStatus(t, operation.status)}</span>
          <strong>{operation.message || translateStatus(t, operation.status)}</strong>
        </div>
      ) : null}
      {configMissing ? (
        <div className="result-line warn">
          <span>{t('configMissing')}</span>
          <strong>{t('configMissingOverview')}</strong>
        </div>
      ) : null}
      {!configMissing && summary.cdcMessage ? (
        <div className={summary.cdcStatus === 'error' ? 'result-line warn' : 'result-line ok'}>
          <span>{t('cdcDetail')}</span>
          <strong>{summary.cdcMessage}</strong>
        </div>
      ) : null}
      <div className="path-grid">
        <div className="readonly-item">
          <span>{t('configPath')}</span>
          <strong>{summary.configPath || '-'}</strong>
        </div>
        <div className="readonly-item">
          <span>{t('rulesPath')}</span>
          <strong>{summary.rulesPath || '-'}</strong>
        </div>
      </div>

      <div className="overview-actions">
        <div className="toolbar-row">
          <button className="button-primary" type="button" onClick={() => void runControl('start')}>
            {t('start')}
          </button>
          <button className="button-secondary" type="button" onClick={() => void refresh()}>
            {t('refresh')}
          </button>
        </div>
        <div className="toolbar-row danger-actions">
          <button className="button-danger" type="button" onClick={() => void runControl('stop')}>
            {t('stop')}
          </button>
          <button className="button-tool" type="button" onClick={() => void runControl('restart')}>
            {t('restart')}
          </button>
        </div>
      </div>

      <MetricGrid
        items={[
          { label: t('mode'), value: summary.mode || t('unknownLabel') },
          { label: t('nodeId'), value: summary.nodeId },
          { label: t('nodeName'), value: summary.nodeName },
          { label: t('agent'), value: translateStatus(t, summary.agentStatus), tone: toneForStatus(summary.agentStatus) },
          {
            label: 'MySQL',
            value: configMissing ? t('notConfigured') : translateStatus(t, summary.mysqlStatus),
            tone: configMissing ? 'warn' : toneForStatus(summary.mysqlStatus),
          },
          {
            label: 'RabbitMQ',
            value: configMissing ? t('notConfigured') : translateStatus(t, summary.rabbitmqStatus),
            tone: configMissing ? 'warn' : toneForStatus(summary.rabbitmqStatus),
          },
          {
            label: 'CDC',
            value: configMissing ? t('notConfigured') : translateStatus(t, summary.cdcStatus),
            tone: configMissing ? 'warn' : toneForStatus(summary.cdcStatus),
          },
          { label: t('uploadBacklog'), value: summary.uploadQueueDepth },
          { label: t('downlinkBacklog'), value: summary.downlinkQueueDepth },
          { label: t('failures'), value: summary.failedEventCount, tone: summary.failedEventCount > 0 ? 'error' : 'ok' },
          { label: t('conflicts'), value: summary.conflictCount, tone: summary.conflictCount > 0 ? 'warn' : 'ok' },
          { label: t('version'), value: summary.version },
        ]}
      />

      <div className="agent-process-panel">
        <div className="agent-process-head">
          <strong>{t('agentProcess')}</strong>
          <span className={`status-chip status-${agentProcess?.status || 'unknown'}`}>
            {translateStatus(t, agentProcess?.status)}
          </span>
        </div>
        <div className="readonly-grid">
          <div className="readonly-item">
            <span>{t('agentExecutable')}</span>
            <strong>{agentProcess?.executable_path || '-'}</strong>
          </div>
          <div className="readonly-item">
            <span>PID</span>
            <strong>{agentProcess?.pid || '-'}</strong>
          </div>
          <div className="readonly-item">
            <span>{t('agentStartedAt')}</span>
            <strong>{formatTime(agentProcess?.started_at)}</strong>
          </div>
          <div className="readonly-item">
            <span>{t('agentExitedAt')}</span>
            <strong>{formatTime(agentProcess?.exited_at)}</strong>
          </div>
          <div className="readonly-item">
            <span>{t('agentLogPath')}</span>
            <strong>{agentProcess?.log_path || summary.agentLogPath || '-'}</strong>
          </div>
          <div className="readonly-item">
            <span>{t('agentLastError')}</span>
            <strong>{agentProcess?.last_error || '-'}</strong>
          </div>
        </div>
      </div>
      {pendingControl ? (
        <ConfirmDialog
          title={pendingControl === 'stop' ? t('confirmStopTitle') : t('confirmRestartTitle')}
          detail={pendingControl === 'stop' ? t('confirmStopDetail') : t('confirmRestartDetail')}
          confirmLabel={pendingControl === 'stop' ? t('stop') : t('restart')}
          cancelLabel={t('cancel')}
          onCancel={() => setPendingControl(null)}
          onConfirm={() => void executeControl(pendingControl)}
        />
      ) : null}
    </section>
  );
}
