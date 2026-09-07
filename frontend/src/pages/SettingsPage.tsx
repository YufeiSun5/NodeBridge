import { useEffect, useState } from 'react';
import { useAuth } from '../auth';
import { InlineErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { SwitchControl } from '../components/SwitchControl';
import { type Language, translateStatus, useI18n } from '../i18n';
import { type ThemePreference, useTheme } from '../theme';
import {
  emptyConfig,
  getAgentProcessStatus,
  getAutoStart,
  getConfig,
  getManagedInstallPlan,
  getMCPServerStatus,
  getOverview,
  applyManagedInstall,
  saveConfig,
  setAutoStart,
  setMCPServerEnabled,
  unlockAdmin,
  type AutoStartStatus,
  type AgentProcessStatus,
  type ConfigDTO,
  type ManagedInstallResponse,
  type MCPServerStatus,
  type TestResult,
} from '../services/wails';
import type { RuntimeSummary } from '../stores/uiStore';

function secretValue(value?: string) {
  return value ? '******' : '-';
}

function SecretReadonlyItem({ label, value, fallback }: { label: string; value?: string; fallback: string }) {
  const hasValue = Boolean(value?.trim());
  return (
    <div className="readonly-item">
      <span>{label}</span>
      <strong className={hasValue ? 'frosted-secret' : ''}>{hasValue ? secretValue(value) : fallback}</strong>
    </div>
  );
}

function isSecurityDraftTarget(config: ConfigDTO) {
  return !config.mode?.trim() && !config.node?.id?.trim() && !config.mysql?.database?.trim();
}

function securityDraftConfig(config: ConfigDTO): ConfigDTO {
  return {
    mode: '',
    node: { id: '', name: '', location: '' },
    mysql: { host: '', port: 0, username: '', password: '', database: '' },
    rabbitmq: {
      mode: '',
      install: false,
      local_url: '',
      server_url: '',
      management_url: '',
      username: '',
      password: '',
      vhost: '',
    },
    cdc: {
      type: '',
      mode: '',
      install: false,
      reader_name: '',
      canal_addr: '',
      config_dir: '',
      service_name: '',
      destination: '',
      username: '',
      password: '',
      filter: '',
      batch_size: 0,
      use_gtid: false,
    },
    sync: {
      upload_batch_size: 0,
      dispatch_batch_size: 0,
      flush_interval_millis: 0,
      retry_interval_seconds: 0,
      heartbeat_interval_seconds: 0,
      node_timeout_seconds: 0,
      apply_lanes: 0,
      enable_crud_compact: false,
    },
    log_web: { enable: false, bind: '', port: 0, token: '' },
    mcp_server: { enable: false },
    security: { ...(config.security || {}) },
  };
}

type DiagnosticPaths = {
  configPath: string;
  rulesPath: string;
  agentExecutable: string;
  agentLogPath: string;
};

export function SettingsPage() {
  const { language, languageNames, setLanguage, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const { authState, ensureUnlocked, refreshAuth } = useAuth();
  const [config, setConfig] = useState<ConfigDTO>(emptyConfig);
  const [autoStart, setAutoStartState] = useState<AutoStartStatus | null>(null);
  const [mcpServer, setMCPServer] = useState<MCPServerStatus | null>(null);
  const [installPlan, setInstallPlan] = useState<ManagedInstallResponse | null>(null);
  const [diagnosticPaths, setDiagnosticPaths] = useState<DiagnosticPaths>({
    configPath: '',
    rulesPath: '',
    agentExecutable: '',
    agentLogPath: '',
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [result, setResult] = useState<TestResult | null>(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [nextConfig, nextAutoStart, nextMCPServer, nextInstallPlan] = await Promise.all([
          getConfig(),
          getAutoStart(),
          getMCPServerStatus(),
          getManagedInstallPlan(),
        ]);
        setConfig(nextConfig);
        setAutoStartState(nextAutoStart);
        setMCPServer(nextMCPServer);
        setInstallPlan(nextInstallPlan);

        const [overviewResult, processResult] = await Promise.allSettled([getOverview(), getAgentProcessStatus()]);
        const overview = overviewResult.status === 'fulfilled' ? (overviewResult.value as RuntimeSummary) : null;
        const processStatus = processResult.status === 'fulfilled' ? (processResult.value as AgentProcessStatus) : null;
        setDiagnosticPaths({
          configPath: overview?.configPath || '',
          rulesPath: overview?.rulesPath || '',
          agentExecutable: processStatus?.executable_path || '',
          agentLogPath: processStatus?.log_path || overview?.agentLogPath || '',
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : t('settingsError'));
      } finally {
        setLoading(false);
      }
    }
    void load();
  }, []);

  function updateSecurity(field: 'admin_password' | 'exit_password', value: string) {
    setConfig((current) => ({
      ...current,
      security: { ...(current.security || {}), [field]: value },
    }));
  }

  async function saveSettings() {
    setResult(null);
    setError('');
    if (!config.security?.admin_password?.trim()) {
      setError(t('adminPasswordRequired'));
      return;
    }
    if (!authState.unlocked) {
      const unlock = await unlockAdmin(config.security.admin_password);
      if (!unlock.ok) {
        setError(unlock.message || t('unlockFailed'));
        return;
      }
    }
    try {
      const payload = isSecurityDraftTarget(config) ? securityDraftConfig(config) : config;
      setConfig(await saveConfig(payload));
      await refreshAuth();
      setResult({ ok: true, status: t('saved'), message: t('settingsSaved') });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  async function toggleAutoStart(enabled: boolean) {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      const next = await setAutoStart(enabled);
      setAutoStartState(next);
      setResult({
        ok: next.status !== 'error',
        status: next.status,
        message: next.message || (next.enabled ? t('autoStartEnabled') : t('autoStartDisabled')),
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  async function toggleMCPServer(enabled: boolean) {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      const next = await setMCPServerEnabled(enabled);
      setMCPServer(next);
      setResult({
        ok: next.status !== 'error' && next.status !== 'locked',
        status: next.status,
        message: next.message || (next.enabled ? t('mcpServerEnabled') : t('mcpServerDisabled')),
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  async function toggleCRUDCompact(enabled: boolean) {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      const nextConfig = {
        ...config,
        sync: {
          ...config.sync,
          enable_crud_compact: enabled,
        },
      };
      setConfig(await saveConfig(nextConfig));
      setResult({
        ok: true,
        status: t('saved'),
        message: enabled ? t('crudCompactEnabled') : t('crudCompactDisabled'),
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  async function refreshInstallPlan() {
    setResult(null);
    setError('');
    try {
      setInstallPlan(await getManagedInstallPlan());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  async function runManagedInstall() {
    setResult(null);
    setError('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      const next = await applyManagedInstall();
      setInstallPlan(next);
      const failed = next.operations.some((operation) => operation.status === 'error');
      setResult({
        ok: !failed,
        status: next.mode || 'managed',
        message: failed ? t('managedInstallFailed') : t('managedInstallApplied'),
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settingsError'));
    }
  }

  function requestExitDialog() {
    window.dispatchEvent(new Event('datasync:open-exit-dialog'));
  }

  const locked = !authState.unlocked && Boolean(config.security?.admin_password?.trim());
  const mcpConfigBlocked =
    mcpServer?.status === 'unsupported' ||
    (mcpServer?.status === 'unknown' && (mcpServer.message || '').toLowerCase().includes('config is not loaded'));
  const mcpDisabled = locked || mcpConfigBlocked;
  const crudCompactDisabled = locked || isSecurityDraftTarget(config);

  return (
    <section className="page-panel">
      <SectionHeader title={t('settings')} tone="app" />
      {loading ? <LoadingState title={t('loadingSettings')} /> : null}
      {error ? <InlineErrorState title={t('settingsError')} detail={error} /> : null}
      {result ? (
        <div className={result.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{result.status}</span>
          <strong>{result.message || t('noDetails')}</strong>
        </div>
      ) : null}

      <section className="settings-grid">
        <section className="settings-section">
          <h3 className="settings-group-title">{t('appearanceAndLanguage')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card">
              <h3>{t('theme')}</h3>
              <p>{t('themeSettingDetail')}</p>
              <label className="settings-language">
                <span>{t('theme')}</span>
                <select value={theme} onChange={(event) => setTheme(event.target.value as ThemePreference)}>
                  <option value="system">{t('themeSystem')}</option>
                  <option value="dark">{t('themeDark')}</option>
                  <option value="light">{t('themeLight')}</option>
                </select>
              </label>
            </div>

            <div className="settings-card">
              <h3>{t('language')}</h3>
              <p>{t('languageSettingDetail')}</p>
              <label className="settings-language">
                <span>{t('language')}</span>
                <select value={language} onChange={(event) => setLanguage(event.target.value as Language)}>
                  {(Object.keys(languageNames) as Language[]).map((key) => (
                    <option value={key} key={key}>
                      {languageNames[key]}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </div>
        </section>

        <section className="settings-section">
          <h3 className="settings-group-title">{t('windowAndStartup')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card">
              <h3>{t('autoStart')}</h3>
              <p>{t('autoStartSettingDetail')}</p>
              {locked ? (
                <div className="readonly-item">
                  <span>{t('status')}</span>
                  <strong>{autoStart?.enabled ? t('autoStartEnabled') : t('autoStartDisabled')}</strong>
                </div>
              ) : (
                <SwitchControl
                  checked={Boolean(autoStart?.enabled)}
                  label={autoStart?.enabled ? t('autoStartEnabled') : t('autoStartDisabled')}
                  onChange={(checked) => void toggleAutoStart(checked)}
                />
              )}
              <div className={autoStart?.status === 'unsupported' ? 'result-line warn' : 'result-line ok'}>
                <span>{translateStatus(t, autoStart?.status)}</span>
                <strong>{autoStart?.message || t('noDetails')}</strong>
              </div>
            </div>

            <div className="settings-card">
              <h3>{t('windowAndExit')}</h3>
              <p>{t('windowAndExitDetail')}</p>
              <button className="button-danger compact button-danger-muted" type="button" onClick={requestExitDialog}>
                {t('exitApp')}
              </button>
            </div>
          </div>
        </section>

        <section className="settings-section">
          <h3 className="settings-group-title">{t('integrations')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card">
              <h3>{t('mcpServer')}</h3>
              <p>{t('mcpServerSettingDetail')}</p>
              {locked ? (
                <div className="readonly-item">
                  <span>{t('status')}</span>
                  <strong>{mcpServer?.enabled ? t('mcpServerEnabled') : t('mcpServerDisabled')}</strong>
                </div>
              ) : (
                <SwitchControl
                  checked={Boolean(mcpServer?.enabled)}
                  label={mcpServer?.enabled ? t('mcpServerEnabled') : t('mcpServerDisabled')}
                  disabled={mcpDisabled}
                  onChange={(checked) => void toggleMCPServer(checked)}
                />
              )}
              {mcpConfigBlocked ? <div className="notice-line warn">{t('mcpConfigRequired')}</div> : null}
              {mcpServer?.ephemeral ? <div className="notice-line">{t('mcpEphemeralNotice')}</div> : null}
              <div className="readonly-item long-value">
                <span>{t('mcpClientConfigCommand')}</span>
                <strong>
                  {diagnosticPaths.agentExecutable
                    ? `${diagnosticPaths.agentExecutable} mcp-client-config`
                    : 'SyncAgent.exe mcp-client-config'}
                </strong>
              </div>
              <div className="notice-line">{t('mcpClientConfigHelp')}</div>
              <div className={mcpServer?.status === 'error' || mcpServer?.status === 'locked' || mcpConfigBlocked ? 'result-line warn' : 'result-line ok'}>
                <span>{mcpServer?.status === 'configured' ? t('mcpConfigured') : translateStatus(t, mcpServer?.status)}</span>
                <strong>{mcpServer?.message || t('noDetails')}</strong>
              </div>
            </div>
          </div>
        </section>

        <section className="settings-section">
          <h3 className="settings-group-title">{t('syncPerformance')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card">
              <h3>{t('crudCompactMode')}</h3>
              <p>{t('crudCompactSettingDetail')}</p>
              {locked ? (
                <div className="readonly-item">
                  <span>{t('status')}</span>
                  <strong>{config.sync?.enable_crud_compact ? t('crudCompactEnabled') : t('crudCompactDisabled')}</strong>
                </div>
              ) : (
                <SwitchControl
                  checked={Boolean(config.sync?.enable_crud_compact)}
                  label={config.sync?.enable_crud_compact ? t('crudCompactEnabled') : t('crudCompactDisabled')}
                  disabled={crudCompactDisabled}
                  onChange={(checked) => void toggleCRUDCompact(checked)}
                />
              )}
              {isSecurityDraftTarget(config) ? <div className="notice-line warn">{t('crudCompactConfigRequired')}</div> : null}
              <div className="notice-line warn">{t('crudCompactGlobalWarning')}</div>
            </div>
          </div>
        </section>

        <section className="settings-section">
          <h3 className="settings-group-title">{t('managedComponents')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card wide-settings-card">
              <h3>{t('managedInstall')}</h3>
              <p>{t('managedInstallDetail')}</p>
              <div className="notice-line">{t('managedInstallAlphaNotice')}</div>
              <div className="readonly-grid">
                <div className="readonly-item">
                  <span>{t('mode')}</span>
                  <strong>{installPlan?.mode || '-'}</strong>
                </div>
                <div className="readonly-item">
                  <span>{t('manifestPath')}</span>
                  <strong>{installPlan?.manifest_path || '-'}</strong>
                </div>
              </div>
              <div className="toolbar-row">
                <button className="button-secondary compact" type="button" onClick={() => void refreshInstallPlan()}>
                  {t('refresh')}
                </button>
                <button className="button-primary compact" type="button" onClick={() => void runManagedInstall()}>
                  {t('applyManagedInstall')}
                </button>
              </div>
              {installPlan?.operations?.length ? (
                <div className="table-scroll">
                  <table className="terminal-table wide-content-table">
                    <thead>
                      <tr>
                        <th>{t('component')}</th>
                        <th>{t('action')}</th>
                        <th>{t('target')}</th>
                        <th>{t('status')}</th>
                        <th>{t('message')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {installPlan.operations.map((operation, index) => (
                        <tr key={`${operation.component}-${operation.action}-${operation.target || index}`}>
                          <td>{operation.component}</td>
                          <td>{operation.action}</td>
                          <td>{operation.target || '-'}</td>
                          <td>{translateStatus(t, operation.status)}</td>
                          <td>{operation.message || '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="readonly-item">
                  <span>{t('status')}</span>
                  <strong>{t('noManagedInstallOperations')}</strong>
                </div>
              )}
            </div>
            <div className="settings-card">
              <h3>{t('diagnosticLocations')}</h3>
              <p>{t('diagnosticLocationsDetail')}</p>
              <div className="readonly-grid">
                <div className="readonly-item long-value">
                  <span>{t('configPath')}</span>
                  <strong>{diagnosticPaths.configPath || '-'}</strong>
                </div>
                <div className="readonly-item long-value">
                  <span>{t('rulesPath')}</span>
                  <strong>{diagnosticPaths.rulesPath || '-'}</strong>
                </div>
                <div className="readonly-item long-value">
                  <span>{t('agentExecutable')}</span>
                  <strong>{diagnosticPaths.agentExecutable || '-'}</strong>
                </div>
                <div className="readonly-item long-value">
                  <span>{t('agentLogPath')}</span>
                  <strong>{diagnosticPaths.agentLogPath || '-'}</strong>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="settings-section">
          <h3 className="settings-group-title">{t('securityAndAbout')}</h3>
          <div className="settings-section-grid">
            <div className="settings-card">
              <h3>{t('security')}</h3>
              <p>{t('securitySettingDetail')}</p>
              {!config.security?.admin_password?.trim() ? <div className="notice-line warn">{t('securityInitialPasswordNotice')}</div> : null}
              {locked ? (
                <>
                  <SecretReadonlyItem label={t('adminPassword')} value={config.security?.admin_password} fallback={t('notSet')} />
                  <SecretReadonlyItem label={t('exitPassword')} value={config.security?.exit_password} fallback={t('notSet')} />
                  <div className="notice-line">{t('settingsUnlockHint')}</div>
                  <div className="notice-line">{t('passwordForgotHelp')}</div>
                </>
              ) : (
                <>
                  <label>
                    {t('adminPassword')}
                    <input
                      type="password"
                      value={config.security?.admin_password || ''}
                      onChange={(event) => updateSecurity('admin_password', event.target.value)}
                    />
                  </label>
                  <div className="notice-line">{t('adminPasswordHelp')}</div>
                  <label>
                    {t('exitPassword')}
                    <input
                      type="password"
                      value={config.security?.exit_password || ''}
                      onChange={(event) => updateSecurity('exit_password', event.target.value)}
                    />
                  </label>
                  <div className="notice-line">{t('exitPasswordHelp')}</div>
                  <div className="notice-line">{t('passwordForgotHelp')}</div>
                  <button className="button-primary" type="button" onClick={() => void saveSettings()}>
                    {t('saveSettings')}
                  </button>
                </>
              )}
            </div>

            <div className="settings-card">
              <h3>{t('openSourceLicense')}</h3>
              <p>{t('openSourceLicenseDetail')}</p>
              <div className="license-scope-list">
                <span>{t('licenseAllowUse')}</span>
                <span>{t('licenseAllowModify')}</span>
                <span>{t('licenseAllowDistribute')}</span>
                <span>{t('licenseAllowCommercial')}</span>
              </div>
              <div className="notice-line">{t('licenseNoticeDetail')}</div>
            </div>
          </div>
        </section>
      </section>
    </section>
  );
}
