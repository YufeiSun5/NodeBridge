export type PageKey = 'overview' | 'config' | 'rules' | 'queues' | 'failures' | 'logs' | 'manual' | 'settings';

export const pages: Array<{ key: PageKey; labelKey: string }> = [
  { key: 'overview', labelKey: 'overview' },
  { key: 'config', labelKey: 'config' },
  { key: 'rules', labelKey: 'rules' },
  { key: 'queues', labelKey: 'queues' },
  { key: 'failures', labelKey: 'failures' },
  { key: 'logs', labelKey: 'logs' },
  { key: 'manual', labelKey: 'manual' },
  { key: 'settings', labelKey: 'settings' },
];
