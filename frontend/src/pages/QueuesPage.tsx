import { useEffect, useState } from 'react';
import { EmptyState, ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { translateStatus, useI18n } from '../i18n';
import { getConfig, getQueueStatus, isConfigMissing, type QueueStatusDTO } from '../services/wails';

function roleLabel(t: (key: string) => string, role: string) {
  const keyByRole: Record<string, string> = {
    local_upload: 'queueLocalUpload',
    server_ingress: 'queueServerIngress',
    downlink: 'queueDownlink',
    dead_letter: 'queueDeadLetter',
    retry: 'queueRetry',
  };
  return keyByRole[role] ? t(keyByRole[role]) : role.replaceAll('_', ' ');
}

export function QueuesPage() {
  const { t } = useI18n();
  const [queues, setQueues] = useState<QueueStatusDTO[]>([]);
  const [configMissing, setConfigMissing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const config = await getConfig();
      const missing = isConfigMissing(config);
      setConfigMissing(missing);
      setQueues(missing ? [] : await getQueueStatus());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('queuesError'));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <section className="page-panel">
      <SectionHeader title={t('queues')} tone="node" />
      {loading ? <LoadingState title={t('loadingQueues')} /> : null}
      {error ? <ErrorState title={t('queuesError')} detail={error} /> : null}

      <div className="toolbar-row">
        <button className="button-secondary" type="button" onClick={() => void load()}>
          {t('refresh')}
        </button>
      </div>

      {!loading && queues.length === 0 ? (
        <EmptyState
          title={configMissing ? t('configMissing') : t('noQueueStatus')}
          detail={configMissing ? t('queuesConfigMissing') : t('noQueueRows')}
        />
      ) : null}

      {queues.length > 0 ? (
        <div className="queue-grid">
          {queues.map((queue) => (
            <article className="queue-card" key={`${queue.role}-${queue.name}`}>
              <div className="queue-card-header">
                <strong>{roleLabel(t, queue.role)}</strong>
                <span className={`status-chip status-${queue.status || 'unknown'}`}>{translateStatus(t, queue.status)}</span>
              </div>
              <div className="queue-name">{queue.name}</div>
              <div className="queue-stats">
                <span>
                  {t('messages')} <strong>{queue.messages}</strong>
                </span>
                <span>
                  {t('consumers')} <strong>{queue.consumers}</strong>
                </span>
              </div>
            </article>
          ))}
        </div>
      ) : null}
    </section>
  );
}
