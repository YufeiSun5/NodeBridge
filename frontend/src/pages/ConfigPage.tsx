import { type ReactNode, useEffect, useState } from 'react';
import { useAuth } from '../auth';
import { ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { SwitchControl } from '../components/SwitchControl';
import { useI18n } from '../i18n';
import {
  emptyConfig,
  getConfig,
  saveConfig,
  testMySQL,
  testRabbitMQ,
  type CDCConfig,
  type ConfigDTO,
  type LogWebConfig,
  type MySQLConfig,
  type NodeConfig,
  type RabbitMQConfig,
  type SyncConfig,
  type TestResult,
} from '../services/wails';

type ConfigSection = keyof ConfigDTO;
type ConfigTab = 'node' | 'mysql' | 'rabbitmq' | 'cdc' | 'sync' | 'logWeb';

const configTabs: Array<{ key: ConfigTab; labelKey: string }> = [
  { key: 'node', labelKey: 'node' },
  { key: 'mysql', labelKey: 'mysql' },
  { key: 'rabbitmq', labelKey: 'rabbitmq' },
  { key: 'cdc', labelKey: 'cdc' },
  { key: 'sync', labelKey: 'sync' },
  { key: 'logWeb', labelKey: 'logWeb' },
];

function numberValue(value: string) {
  return Number.isFinite(Number(value)) ? Number(value) : 0;
}

function displayValue(value: unknown) {
  if (typeof value === 'boolean') return value ? 'ON' : 'OFF';
  if (value === undefined || value === null || value === '') return '-';
  return String(value);
}

function ReadOnlyItem({ label, value, secret = false }: { label: string; value: unknown; secret?: boolean }) {
  return (
    <div className="readonly-item">
      <span>{label}</span>
      <strong className={secret ? 'frosted-secret' : ''}>{secret ? '******' : displayValue(value)}</strong>
    </div>
  );
}

function ReadOnlySection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="readonly-section">
      <h3>{title}</h3>
      <div className="readonly-grid">{children}</div>
    </section>
  );
}

function FieldHint({ children }: { children: ReactNode }) {
  return <small className="field-hint">{children}</small>;
}

function ConfigReadOnly({ config, t }: { config: ConfigDTO; t: (key: string) => string }) {
  return (
    <div className="readonly-config">
      <ReadOnlySection title={t('node')}>
        <ReadOnlyItem label={t('mode')} value={config.mode} />
        <ReadOnlyItem label={t('id')} value={config.node.id} />
        <ReadOnlyItem label={t('name')} value={config.node.name} />
        <ReadOnlyItem label={t('location')} value={config.node.location} />
      </ReadOnlySection>
      <ReadOnlySection title="MySQL">
        <ReadOnlyItem label={t('host')} value={config.mysql.host} />
        <ReadOnlyItem label={t('port')} value={config.mysql.port} />
        <ReadOnlyItem label={t('database')} value={config.mysql.database} />
        <ReadOnlyItem label={t('username')} value={config.mysql.username} />
        <ReadOnlyItem label={t('password')} value={config.mysql.password} secret />
      </ReadOnlySection>
      <ReadOnlySection title="RabbitMQ">
        <ReadOnlyItem label={t('mode')} value={config.rabbitmq.mode} />
        <ReadOnlyItem label={t('installLocally')} value={Boolean(config.rabbitmq.install)} />
        <ReadOnlyItem label={t('localUrl')} value={config.rabbitmq.local_url} />
        <ReadOnlyItem label={t('serverUrl')} value={config.rabbitmq.server_url} />
        <ReadOnlyItem label={t('managementUrl')} value={config.rabbitmq.management_url} />
        <ReadOnlyItem label={t('username')} value={config.rabbitmq.username} />
        <ReadOnlyItem label={t('password')} value={config.rabbitmq.password} secret />
        <ReadOnlyItem label={t('vhost')} value={config.rabbitmq.vhost} />
      </ReadOnlySection>
      <ReadOnlySection title="CDC">
        <ReadOnlyItem label={t('type')} value={config.cdc.type} />
        <ReadOnlyItem label={t('mode')} value={config.cdc.mode} />
        <ReadOnlyItem label={t('installLocally')} value={Boolean(config.cdc.install)} />
        <ReadOnlyItem label={t('readerName')} value={config.cdc.reader_name} />
        <ReadOnlyItem label={t('canalAddr')} value={config.cdc.canal_addr} />
        <ReadOnlyItem label={t('configDir')} value={config.cdc.config_dir} />
        <ReadOnlyItem label={t('serviceName')} value={config.cdc.service_name} />
        <ReadOnlyItem label={t('destination')} value={config.cdc.destination} />
        <ReadOnlyItem label={t('filter')} value={config.cdc.filter} />
        <ReadOnlyItem label={t('batchSize')} value={config.cdc.batch_size} />
      </ReadOnlySection>
      <ReadOnlySection title={t('sync')}>
        <ReadOnlyItem label={t('uploadBatch')} value={config.sync.upload_batch_size} />
        <ReadOnlyItem label={t('dispatchBatch')} value={config.sync.dispatch_batch_size} />
        <ReadOnlyItem label={t('flushMs')} value={config.sync.flush_interval_millis} />
        <ReadOnlyItem label={t('retrySeconds')} value={config.sync.retry_interval_seconds} />
      </ReadOnlySection>
      <ReadOnlySection title={t('logWeb')}>
        <ReadOnlyItem label={t('enable')} value={Boolean(config.log_web.enable)} />
        <ReadOnlyItem label={t('bind')} value={config.log_web.bind} />
        <ReadOnlyItem label={t('port')} value={config.log_web.port} />
        <ReadOnlyItem label={t('token')} value={config.log_web.token} secret />
      </ReadOnlySection>
    </div>
  );
}

export function ConfigPage() {
  const { t } = useI18n();
  const { authState, ensureUnlocked, refreshAuth } = useAuth();
  const [config, setConfig] = useState<ConfigDTO>(emptyConfig);
  const [activeTab, setActiveTab] = useState<ConfigTab>('node');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [result, setResult] = useState<TestResult | null>(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        setConfig(await getConfig());
      } catch (err) {
        setError(err instanceof Error ? err.message : t('configError'));
      } finally {
        setLoading(false);
      }
    }
    void load();
  }, []);

  function update<K extends ConfigSection>(section: K, value: ConfigDTO[K]) {
    setConfig((current) => ({ ...current, [section]: value }));
  }

  function updateNode(field: keyof NodeConfig, value: string) {
    update('node', { ...config.node, [field]: value });
  }

  function updateMySQL(field: keyof MySQLConfig, value: string | number) {
    update('mysql', { ...config.mysql, [field]: value });
  }

  function updateRabbitMQ(field: keyof RabbitMQConfig, value: string | boolean) {
    update('rabbitmq', { ...config.rabbitmq, [field]: value });
  }

  function updateCDC(field: keyof CDCConfig, value: string | number | boolean) {
    update('cdc', { ...config.cdc, [field]: value });
  }

  function updateSync(field: keyof SyncConfig, value: number) {
    update('sync', { ...config.sync, [field]: value });
  }

  function updateLogWeb(field: keyof LogWebConfig, value: string | number | boolean) {
    update('log_web', { ...config.log_web, [field]: value });
  }

  async function save() {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) return;
    try {
      setConfig(await saveConfig(config));
      await refreshAuth();
      setResult({ ok: true, status: t('saved'), message: t('configSaved') });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('configError'));
    }
  }

  async function runTest(kind: 'mysql' | 'rabbitmq') {
    setResult(null);
    setError('');
    try {
      setResult(kind === 'mysql' ? await testMySQL(config.mysql) : await testRabbitMQ(config.rabbitmq));
    } catch (err) {
      setError(err instanceof Error ? err.message : `${t('configError')}: ${kind}`);
    }
  }

  function renderActiveConfig() {
    if (activeTab === 'node') {
      return (
        <fieldset className="config-tab-panel">
          <legend>{t('node')}</legend>
          <div className="form-grid">
            <label>
              {t('mode')}
              <select value={config.mode} onChange={(event) => update('mode', event.target.value)}>
                <option value="">{t('unknownLabel')}</option>
                <option value="edge">edge</option>
                <option value="server">server</option>
              </select>
            </label>
            <label>
              {t('id')}
              <input value={config.node.id || ''} onChange={(event) => updateNode('id', event.target.value)} />
            </label>
            <label>
              {t('name')}
              <input value={config.node.name || ''} onChange={(event) => updateNode('name', event.target.value)} />
            </label>
            <label>
              {t('location')}
              <input value={config.node.location || ''} onChange={(event) => updateNode('location', event.target.value)} />
            </label>
          </div>
        </fieldset>
      );
    }

    if (activeTab === 'mysql') {
      return (
        <fieldset className="config-tab-panel">
          <legend>MySQL</legend>
          <div className="form-grid">
            <label>
              {t('host')}
              <input value={config.mysql.host || ''} onChange={(event) => updateMySQL('host', event.target.value)} />
            </label>
            <label>
              {t('port')}
              <input
                min={1}
                max={65535}
                step={1}
                type="number"
                value={config.mysql.port || 0}
                onChange={(event) => updateMySQL('port', numberValue(event.target.value))}
              />
              <FieldHint>{t('portHint')}</FieldHint>
            </label>
            <label>
              {t('database')}
              <input value={config.mysql.database || ''} onChange={(event) => updateMySQL('database', event.target.value)} />
            </label>
            <label>
              {t('username')}
              <input value={config.mysql.username || ''} onChange={(event) => updateMySQL('username', event.target.value)} />
            </label>
            <label>
              {t('password')}
              <input
                type="password"
                value={config.mysql.password || ''}
                onChange={(event) => updateMySQL('password', event.target.value)}
              />
            </label>
          </div>
        </fieldset>
      );
    }

    if (activeTab === 'rabbitmq') {
      return (
        <fieldset className="config-tab-panel">
          <legend>RabbitMQ</legend>
          <div className="form-grid">
            <label>
              {t('mode')}
              <select value={config.rabbitmq.mode || ''} onChange={(event) => updateRabbitMQ('mode', event.target.value)}>
                <option value="">{t('unknownLabel')}</option>
                <option value="managed">managed</option>
                <option value="external">external</option>
              </select>
            </label>
            <div className="switch-field">
              <span>{t('installLocally')}</span>
              <SwitchControl
                checked={Boolean(config.rabbitmq.install)}
                label={config.rabbitmq.install ? t('enabled') : t('no')}
                onChange={(checked) => updateRabbitMQ('install', checked)}
              />
            </div>
            <label>
              {t('localUrl')}
              <input value={config.rabbitmq.local_url || ''} onChange={(event) => updateRabbitMQ('local_url', event.target.value)} />
            </label>
            <label>
              {t('serverUrl')}
              <input value={config.rabbitmq.server_url || ''} onChange={(event) => updateRabbitMQ('server_url', event.target.value)} />
            </label>
            <label>
              {t('managementUrl')}
              <input
                value={config.rabbitmq.management_url || ''}
                onChange={(event) => updateRabbitMQ('management_url', event.target.value)}
              />
            </label>
            <label>
              {t('username')}
              <input value={config.rabbitmq.username || ''} onChange={(event) => updateRabbitMQ('username', event.target.value)} />
            </label>
            <label>
              {t('password')}
              <input
                type="password"
                value={config.rabbitmq.password || ''}
                onChange={(event) => updateRabbitMQ('password', event.target.value)}
              />
            </label>
            <label>
              {t('vhost')}
              <input value={config.rabbitmq.vhost || ''} onChange={(event) => updateRabbitMQ('vhost', event.target.value)} />
            </label>
          </div>
        </fieldset>
      );
    }

    if (activeTab === 'cdc') {
      return (
        <fieldset className="config-tab-panel">
          <legend>CDC</legend>
          <div className="form-grid">
            <label>
              {t('type')}
              <select value={config.cdc.type || ''} onChange={(event) => updateCDC('type', event.target.value)}>
                <option value="">{t('unknownLabel')}</option>
                <option value="canal">canal</option>
                <option value="stub">stub</option>
                <option value="none">none</option>
              </select>
            </label>
            <label>
              {t('mode')}
              <select value={config.cdc.mode || ''} onChange={(event) => updateCDC('mode', event.target.value)}>
                <option value="">{t('unknownLabel')}</option>
                <option value="managed">managed</option>
                <option value="external">external</option>
              </select>
            </label>
            <div className="switch-field">
              <span>{t('installLocally')}</span>
              <SwitchControl
                checked={Boolean(config.cdc.install)}
                label={config.cdc.install ? t('enabled') : t('no')}
                onChange={(checked) => updateCDC('install', checked)}
              />
            </div>
            <label>
              {t('readerName')}
              <input value={config.cdc.reader_name || ''} onChange={(event) => updateCDC('reader_name', event.target.value)} />
            </label>
            <label>
              {t('canalAddr')}
              <input value={config.cdc.canal_addr || ''} onChange={(event) => updateCDC('canal_addr', event.target.value)} />
            </label>
            <label>
              {t('configDir')}
              <input value={config.cdc.config_dir || ''} onChange={(event) => updateCDC('config_dir', event.target.value)} />
            </label>
            <label>
              {t('serviceName')}
              <input value={config.cdc.service_name || ''} onChange={(event) => updateCDC('service_name', event.target.value)} />
            </label>
            <label>
              {t('destination')}
              <input value={config.cdc.destination || ''} onChange={(event) => updateCDC('destination', event.target.value)} />
            </label>
            <label>
              {t('filter')}
              <input value={config.cdc.filter || ''} onChange={(event) => updateCDC('filter', event.target.value)} />
            </label>
            <label>
              {t('batchSize')}
              <input
                min={1}
                step={1}
                type="number"
                value={config.cdc.batch_size || 0}
                onChange={(event) => updateCDC('batch_size', numberValue(event.target.value))}
              />
              <FieldHint>{t('batchSizeHint')}</FieldHint>
            </label>
          </div>
        </fieldset>
      );
    }

    if (activeTab === 'sync') {
      return (
        <fieldset className="config-tab-panel">
          <legend>{t('sync')}</legend>
          <div className="form-grid">
            <label>
              {t('uploadBatch')}
              <input
                min={1}
                step={1}
                type="number"
                value={config.sync.upload_batch_size || 0}
                onChange={(event) => updateSync('upload_batch_size', numberValue(event.target.value))}
              />
              <FieldHint>{t('batchSizeHint')}</FieldHint>
            </label>
            <label>
              {t('dispatchBatch')}
              <input
                min={1}
                step={1}
                type="number"
                value={config.sync.dispatch_batch_size || 0}
                onChange={(event) => updateSync('dispatch_batch_size', numberValue(event.target.value))}
              />
              <FieldHint>{t('batchSizeHint')}</FieldHint>
            </label>
            <label>
              {t('flushMs')}
              <input
                min={100}
                step={100}
                type="number"
                value={config.sync.flush_interval_millis || 0}
                onChange={(event) => updateSync('flush_interval_millis', numberValue(event.target.value))}
              />
              <FieldHint>{t('millisecondsHint')}</FieldHint>
            </label>
            <label>
              {t('retrySeconds')}
              <input
                min={1}
                step={1}
                type="number"
                value={config.sync.retry_interval_seconds || 0}
                onChange={(event) => updateSync('retry_interval_seconds', numberValue(event.target.value))}
              />
              <FieldHint>{t('secondsHint')}</FieldHint>
            </label>
          </div>
        </fieldset>
      );
    }

    return (
      <fieldset className="config-tab-panel">
        <legend>{t('logWeb')}</legend>
        <div className="form-grid">
          <div className="switch-field">
            <span>{t('enable')}</span>
            <SwitchControl
              checked={Boolean(config.log_web.enable)}
              label={config.log_web.enable ? t('enabled') : t('no')}
              onChange={(checked) => updateLogWeb('enable', checked)}
            />
          </div>
          <label>
            {t('bind')}
            <input value={config.log_web.bind || ''} onChange={(event) => updateLogWeb('bind', event.target.value)} />
          </label>
          <label>
            {t('port')}
            <input
              min={1}
              max={65535}
              step={1}
              type="number"
              value={config.log_web.port || 0}
              onChange={(event) => updateLogWeb('port', numberValue(event.target.value))}
            />
            <FieldHint>{t('portHint')}</FieldHint>
          </label>
          <label>
            {t('token')}
            <input
              type="password"
              value={config.log_web.token || ''}
              onChange={(event) => updateLogWeb('token', event.target.value)}
            />
          </label>
        </div>
      </fieldset>
    );
  }

  const locked = !authState.unlocked;

  return (
    <section className="page-panel">
      <SectionHeader title={t('config')} tone="app" />
      {loading ? <LoadingState title={t('loadingConfig')} /> : null}
      {error ? <ErrorState title={t('configError')} detail={error} /> : null}
      {result ? (
        <div className={result.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{result.status}</span>
          <strong>{result.message || t('noDetails')}</strong>
        </div>
      ) : null}

      {locked ? (
        <>
          <div className="readonly-banner">
            <div>
              <strong>{t('readOnlyLockedTitle')}</strong>
              <span>{t('readOnlyLockedDetail')}</span>
            </div>
            <button className="button-primary compact" type="button" onClick={() => void ensureUnlocked()}>
              {t('unlock')}
            </button>
          </div>
          <ConfigReadOnly config={config} t={t} />
        </>
      ) : (
        <>
          <div className="toolbar-row">
            <button className="button-primary" type="button" onClick={() => void save()}>
              {t('saveConfig')}
            </button>
            <button className="button-tool" type="button" onClick={() => void runTest('mysql')}>
              {t('testMysql')}
            </button>
            <button className="button-tool" type="button" onClick={() => void runTest('rabbitmq')}>
              {t('testRabbitmq')}
            </button>
          </div>

          <form className="config-editor" onSubmit={(event) => event.preventDefault()}>
            <nav className="config-tabs" aria-label={t('configSections')}>
              {configTabs.map((tab) => (
                <button
                  className={activeTab === tab.key ? 'config-tab active' : 'config-tab'}
                  key={tab.key}
                  type="button"
                  onClick={() => setActiveTab(tab.key)}
                >
                  {t(tab.labelKey)}
                </button>
              ))}
            </nav>
            {renderActiveConfig()}
          </form>
        </>
      )}
    </section>
  );
}
