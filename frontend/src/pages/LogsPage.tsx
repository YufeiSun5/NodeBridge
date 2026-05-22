import { useEffect, useMemo, useState } from 'react';
import { EmptyState, ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { useAuth } from '../auth';
import { translateStatus, useI18n } from '../i18n';
import {
  exportDiagnosticPackage,
  getConfig,
  getLogs,
  isConfigMissing,
  type LogEntry,
  type TestResult,
} from '../services/wails';

function formatTime(value: string) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString();
}

export function LogsPage() {
  const { t } = useI18n();
  const { ensureUnlocked } = useAuth();
  const [rows, setRows] = useState<LogEntry[]>([]);
  const [configMissing, setConfigMissing] = useState(false);
  const [level, setLevel] = useState('');
  const [module, setModule] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState('');
  const [result, setResult] = useState<TestResult | null>(null);

  const modules = useMemo(() => Array.from(new Set(rows.map((row) => row.module).filter(Boolean))).sort(), [rows]);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [config, logs] = await Promise.all([
        getConfig(),
        getLogs({ level: level || undefined, module: module || undefined, limit: 200 }),
      ]);
      setConfigMissing(isConfigMissing(config));
      setRows(logs);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('logsError'));
    } finally {
      setLoading(false);
    }
  }

  async function copy(row: LogEntry) {
    const text = row.message || '';
    if (!navigator.clipboard) return;
    await navigator.clipboard.writeText(text);
    setCopied(text);
    window.setTimeout(() => setCopied(''), 1200);
  }

  async function exportDiagnostics() {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      const diagnostic = await exportDiagnosticPackage();
      setResult({ ok: true, status: t('diagnosticExported'), message: diagnostic.path });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('logsError'));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <section className="page-panel logs-panel">
      <SectionHeader title={t('logs')} tone="http" />
      {loading ? <LoadingState title={t('loadingLogs')} /> : null}
      {error ? <ErrorState title={t('logsError')} detail={error} /> : null}

      <div className="toolbar-row">
        <select value={level} onChange={(event) => setLevel(event.target.value)}>
          <option value="">{t('allLevels')}</option>
          <option value="debug">debug</option>
          <option value="info">info</option>
          <option value="warn">warn</option>
          <option value="error">error</option>
        </select>
        <select value={module} onChange={(event) => setModule(event.target.value)}>
          <option value="">{t('allModules')}</option>
          {modules.map((item) => (
            <option value={item} key={item}>
              {item}
            </option>
          ))}
        </select>
        <button className="button-secondary" type="button" onClick={() => void load()}>
          {t('applyFilters')}
        </button>
        <button className="button-secondary" type="button" onClick={() => void exportDiagnostics()}>
          {t('exportDiagnostic')}
        </button>
      </div>

      <div className="notice-line">{t('logWebNotice')}</div>
      {copied ? <div className="result-line ok">{t('copied')}</div> : null}
      {result ? (
        <div className={result.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{translateStatus(t, result.status)}</span>
          <strong>{result.message || t('noDetails')}</strong>
        </div>
      ) : null}

      {!loading && rows.length === 0 ? (
        <EmptyState
          title={configMissing ? t('configMissing') : t('noLogs')}
          detail={configMissing ? t('logsConfigMissing') : t('emptyLogBuffer')}
        />
      ) : null}

      {rows.length > 0 ? (
        <div className="log-list">
          {rows.map((row, index) => (
            <div className={`log-row level-${row.level || 'info'}`} key={`${row.time}-${row.module}-${index}`}>
              <span className="log-time">{formatTime(row.time)}</span>
              <span className="log-tag">{row.module || row.level || 'APP'}</span>
              <span className="log-level">{row.level || 'info'}</span>
              <span className="log-message">{row.message}</span>
              <button className="copy-button" type="button" onClick={() => void copy(row)}>
                {t('copy')}
              </button>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
