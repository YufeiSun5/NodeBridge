import { type ReactElement, useEffect, useMemo, useState } from 'react';
import { AuthProvider, useAuth } from './auth';
import { translateStatus, useI18n } from './i18n';
import { ConfigPage } from './pages/ConfigPage';
import { FailuresPage } from './pages/FailuresPage';
import { LogsPage } from './pages/LogsPage';
import { ManualPage } from './pages/ManualPage';
import { OverviewPage } from './pages/OverviewPage';
import { QueuesPage } from './pages/QueuesPage';
import { RulesPage } from './pages/RulesPage';
import { SettingsPage } from './pages/SettingsPage';
import type { PageKey } from './router/pages';
import { pages } from './router/pages';
import { onTrayExitRequest, quitApp, requestExit } from './services/wails';

const pageViews: Record<PageKey, () => ReactElement> = {
  overview: OverviewPage,
  config: ConfigPage,
  rules: RulesPage,
  queues: QueuesPage,
  failures: FailuresPage,
  logs: LogsPage,
  manual: ManualPage,
  settings: SettingsPage,
};

export function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
}

function formatAuthExpiry(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString();
}

function AppContent() {
  const [active, setActive] = useState<PageKey>('overview');
  const [trayResult, setTrayResult] = useState('');
  const [trayError, setTrayError] = useState('');
  const [exitDialogOpen, setExitDialogOpen] = useState(false);
  const [exitPassword, setExitPassword] = useState('');
  const { t } = useI18n();
  const { authState, lock } = useAuth();
  const ActivePage = useMemo(() => pageViews[active], [active]);

  useEffect(() => onTrayExitRequest(() => setExitDialogOpen(true)), []);
  useEffect(() => {
    const openExitDialog = () => setExitDialogOpen(true);
    window.addEventListener('datasync:open-exit-dialog', openExitDialog);
    return () => window.removeEventListener('datasync:open-exit-dialog', openExitDialog);
  }, []);

  async function confirmExit() {
    setTrayResult('');
    setTrayError('');
    try {
      const requested = await requestExit(exitPassword);
      if (!requested.ok) {
        setTrayError(requested.message || translateStatus(t, requested.status));
        return;
      }
      setTrayResult(t('exitVerified'));
      const exited = await quitApp();
      if (!exited.ok) {
        setTrayError(exited.message || translateStatus(t, exited.status));
        return;
      }
      setExitDialogOpen(false);
    } catch (err) {
      setTrayError(err instanceof Error ? err.message : t('operationFailed'));
    }
  }

  return (
    <div className="app-shell">
      {trayResult || trayError ? (
        <section className={trayError ? 'config-banner tray-message warn' : 'config-banner tray-message'}>
          <span className="config-tag">{t('trayControls')}</span>
          <span>{trayError || trayResult}</span>
        </section>
      ) : null}

      <section className="auth-banner">
        <span className={authState.unlocked ? 'status-dot ok' : 'status-dot'} />
        <span className={authState.unlocked ? 'auth-state unlocked' : 'auth-state locked'}>
          {authState.unlocked ? t('unlocked') : t('locked')}
        </span>
        {authState.unlocked && authState.expires_at ? (
          <span className="config-path">
            {t('expiresAt')} {formatAuthExpiry(authState.expires_at)}
          </span>
        ) : (
          <span className="config-path">{t('adminLockedHelp')}</span>
        )}
        <span className="status-spacer" />
        <button className="button-secondary compact" type="button" onClick={() => void lock()} disabled={!authState.unlocked}>
          {t('lock')}
        </button>
      </section>

      <nav className="filter-bar">
        {pages.map((page) => (
          <button
            key={page.key}
            className={page.key === active ? 'nav-button active' : 'nav-button'}
            type="button"
            onClick={() => setActive(page.key)}
          >
            {t(page.labelKey)}
          </button>
        ))}
      </nav>

      <main className="content-area">
        <ActivePage />
      </main>

      <footer className="status-bar">
        <span className="status-dot ok" />
        <span>{t('footerReady')}</span>
        <span className="status-spacer" />
        <span className="license-note">{t('licenseNotice')}</span>
        <span>{t('darkTerminal')}</span>
      </footer>

      {exitDialogOpen ? (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="exit-dialog-title">
            <h2 id="exit-dialog-title">{t('exitApp')}</h2>
            <p>{t('exitPasswordHelp')}</p>
            <label>
              {t('exitPassword')}
              <input
                autoFocus
                type="password"
                value={exitPassword}
                onChange={(event) => setExitPassword(event.target.value)}
              />
            </label>
            <div className="toolbar-row">
              <button className="button-danger" type="button" onClick={() => void confirmExit()}>
                {t('confirmExit')}
              </button>
              <button className="button-secondary" type="button" onClick={() => setExitDialogOpen(false)}>
                {t('cancel')}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
