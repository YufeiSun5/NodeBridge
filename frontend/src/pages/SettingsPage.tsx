import { useEffect, useState } from 'react';
import { useAuth } from '../auth';
import { ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { SwitchControl } from '../components/SwitchControl';
import { type Language, translateStatus, useI18n } from '../i18n';
import { type ThemePreference, useTheme } from '../theme';
import {
  emptyConfig,
  getAutoStart,
  getConfig,
  getManagedInstallPlan,
  getMCPServerStatus,
  applyManagedInstall,
  saveConfig,
  setAutoStart,
  setMCPServerEnabled,
  unlockAdmin,
  type AutoStartStatus,
  type ConfigDTO,
  type ManagedInstallResponse,
  type MCPServerStatus,
  type TestResult,
} from '../services/wails';

function secretValue(value?: string) {
  return value ? '******' : '-';
}

export function SettingsPage() {
  const { language, languageNames, setLanguage, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const { authState, ensureUnlocked, refreshAuth } = useAuth();
  const [config, setConfig] = useState<ConfigDTO>(emptyConfig);
  const [autoStart, setAutoStartState] = useState<AutoStartStatus | null>(null);
  const [mcpServer, setMCPServer] = useState<MCPServerStatus | null>(null);
  const [installPlan, setInstallPlan] = useState<ManagedInstallResponse | null>(null);
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
      setConfig(await saveConfig(config));
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

  return (
    <section className="page-panel">
      <SectionHeader title={t('settings')} tone="app" />
      {loading ? <LoadingState title={t('loadingSettings')} /> : null}
      {error ? <ErrorState title={t('settingsError')} detail={error} /> : null}
      {result ? (
        <div className={result.ok ? 'result-line ok' : 'result-line warn'}>
          <span>{result.status}</span>
          <strong>{result.message || t('noDetails')}</strong>
        </div>
      ) : null}

      <section className="settings-grid">
        <h3 className="settings-group-title">{t('appearanceAndLanguage')}</h3>
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

        <h3 className="settings-group-title">{t('windowAndStartup')}</h3>
        <div className="settings-card">
          <h3>{t('windowAndExit')}</h3>
          <p>{t('windowAndExitDetail')}</p>
          <button className="button-danger compact" type="button" onClick={requestExitDialog}>
            {t('exitApp')}
          </button>
        </div>

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

        <h3 className="settings-group-title">{t('integrations')}</h3>
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
              onChange={(checked) => void toggleMCPServer(checked)}
            />
          )}
          <div className={mcpServer?.status === 'error' || mcpServer?.status === 'locked' ? 'result-line warn' : 'result-line ok'}>
            <span>{mcpServer?.status === 'configured' ? t('mcpConfigured') : translateStatus(t, mcpServer?.status)}</span>
            <strong>{mcpServer?.message || t('noDetails')}</strong>
          </div>
        </div>

        <h3 className="settings-group-title">{t('managedComponents')}</h3>
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

        <h3 className="settings-group-title">{t('securityAndAbout')}</h3>
        <div className="settings-card">
          <h3>{t('security')}</h3>
          <p>{t('securitySettingDetail')}</p>
          {locked ? (
            <>
              <div className="readonly-item">
                <span>{t('adminPassword')}</span>
                <strong className="frosted-secret">{secretValue(config.security?.admin_password)}</strong>
              </div>
              <div className="readonly-item">
                <span>{t('exitPassword')}</span>
                <strong className="frosted-secret">{secretValue(config.security?.exit_password)}</strong>
              </div>
              <button className="button-primary compact" type="button" onClick={() => void ensureUnlocked()}>
                {t('unlock')}
              </button>
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
      </section>
    </section>
  );
}
