import { createContext, type ReactNode, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useI18n } from './i18n';
import { getAuthState, lockAdmin, unlockAdmin, type AuthState } from './services/wails';

type AuthContextValue = {
  authState: AuthState;
  ensureUnlocked: () => Promise<boolean>;
  refreshAuth: () => Promise<AuthState>;
  lock: () => Promise<void>;
};

const lockedState: AuthState = {
  unlocked: false,
  status: 'locked',
  timeout_seconds: 0,
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const [authState, setAuthState] = useState<AuthState>(lockedState);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const resolverRef = useRef<((value: boolean) => void) | null>(null);

  const refreshAuth = useCallback(async () => {
    const next = await getAuthState();
    setAuthState(next);
    return next;
  }, []);

  useEffect(() => {
    void refreshAuth();
    const timer = window.setInterval(() => {
      void refreshAuth();
    }, 30_000);
    return () => window.clearInterval(timer);
  }, [refreshAuth]);

  async function ensureUnlocked() {
    const latest = await refreshAuth();
    if (latest.unlocked) {
      return true;
    }
    setPassword('');
    setError('');
    setDialogOpen(true);
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve;
    });
  }

  async function confirmUnlock() {
    setError('');
    try {
      const result = await unlockAdmin(password);
      if (!result.ok) {
        setError(result.message || result.status);
        return;
      }
      const latest = await refreshAuth();
      setPassword('');
      setDialogOpen(false);
      resolverRef.current?.(latest.unlocked);
      resolverRef.current = null;
    } catch (err) {
      setError(err instanceof Error ? err.message : t('unlockFailed'));
    }
  }

  function cancelUnlock() {
    setDialogOpen(false);
    setPassword('');
    setError('');
    resolverRef.current?.(false);
    resolverRef.current = null;
  }

  async function lock() {
    await lockAdmin();
    await refreshAuth();
  }

  return (
    <AuthContext.Provider value={{ authState, ensureUnlocked, refreshAuth, lock }}>
      {children}
      {dialogOpen ? (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="unlock-dialog-title">
            <h2 id="unlock-dialog-title">{t('unlockAdmin')}</h2>
            <p>{t('adminUnlockHelp')}</p>
            {error ? <div className="result-line warn">{error}</div> : null}
            <label>
              {t('adminPassword')}
              <input
                autoFocus
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    void confirmUnlock();
                  }
                }}
              />
            </label>
            <div className="toolbar-row">
              <button className="button-primary" type="button" onClick={() => void confirmUnlock()}>
                {t('unlock')}
              </button>
              <button className="button-secondary" type="button" onClick={cancelUnlock}>
                {t('cancel')}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('AuthProvider is required');
  }
  return context;
}
