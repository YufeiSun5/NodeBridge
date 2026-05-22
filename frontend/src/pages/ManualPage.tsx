import { useMemo, useState } from 'react';
import { SectionHeader } from '../components/SectionHeader';
import { useI18n } from '../i18n';

type ManualChapter = {
  key: string;
  titleKey: string;
  summaryKey: string;
  bullets: string[];
};

type ReferenceItem = {
  value: string;
  detailKey: string;
};

type ReferenceGroup = {
  titleKey: string;
  items: ReferenceItem[];
};

const chapters: ManualChapter[] = [
  {
    key: 'quickstart',
    titleKey: 'manualQuickStartTitle',
    summaryKey: 'manualQuickStartSummary',
    bullets: ['manualQuickStart1', 'manualQuickStart2', 'manualQuickStart3', 'manualQuickStart4'],
  },
  {
    key: 'overview',
    titleKey: 'manualOverviewTitle',
    summaryKey: 'manualOverviewSummary',
    bullets: ['manualOverview1', 'manualOverview2', 'manualOverview3', 'manualOverview4'],
  },
  {
    key: 'config',
    titleKey: 'manualConfigTitle',
    summaryKey: 'manualConfigSummary',
    bullets: ['manualConfig1', 'manualConfig2', 'manualConfig3', 'manualConfig4', 'manualConfig5'],
  },
  {
    key: 'rules',
    titleKey: 'manualRulesTitle',
    summaryKey: 'manualRulesSummary',
    bullets: ['manualRules1', 'manualRules2', 'manualRules3', 'manualRules4', 'manualRules5'],
  },
  {
    key: 'protocol',
    titleKey: 'manualProtocolTitle',
    summaryKey: 'manualProtocolSummary',
    bullets: ['manualProtocol1', 'manualProtocol2', 'manualProtocol3', 'manualProtocol4'],
  },
  {
    key: 'queues',
    titleKey: 'manualQueuesTitle',
    summaryKey: 'manualQueuesSummary',
    bullets: ['manualQueues1', 'manualQueues2', 'manualQueues3', 'manualQueues4'],
  },
  {
    key: 'failures',
    titleKey: 'manualFailuresTitle',
    summaryKey: 'manualFailuresSummary',
    bullets: ['manualFailures1', 'manualFailures2', 'manualFailures3', 'manualFailures4'],
  },
  {
    key: 'logs',
    titleKey: 'manualLogsTitle',
    summaryKey: 'manualLogsSummary',
    bullets: ['manualLogs1', 'manualLogs2', 'manualLogs3', 'manualLogs4'],
  },
  {
    key: 'tray',
    titleKey: 'manualTrayTitle',
    summaryKey: 'manualTraySummary',
    bullets: ['manualTray1', 'manualTray2', 'manualTray3', 'manualTray4'],
  },
  {
    key: 'status',
    titleKey: 'manualStatusTitle',
    summaryKey: 'manualStatusSummary',
    bullets: ['manualStatus1', 'manualStatus2', 'manualStatus3', 'manualStatus4'],
  },
];

const referenceGroups: ReferenceGroup[] = [
  {
    titleKey: 'manualReferenceDirection',
    items: [
      { value: 'EDGE_TO_SERVER', detailKey: 'manualDirectionEdgeToServer' },
      { value: 'BIDIRECTIONAL', detailKey: 'manualDirectionBidirectional' },
      { value: 'SERVER_TO_EDGE', detailKey: 'manualDirectionServerToEdge' },
      { value: 'IGNORE', detailKey: 'manualDirectionIgnore' },
    ],
  },
  {
    titleKey: 'manualReferenceDispatch',
    items: [
      { value: 'AUTO', detailKey: 'manualDispatchAuto' },
      { value: 'NONE', detailKey: 'manualDispatchNone' },
      { value: 'ACTIVE_EDGES', detailKey: 'manualDispatchActiveEdges' },
      { value: 'SELECTED_EDGES', detailKey: 'manualDispatchSelectedEdges' },
    ],
  },
  {
    titleKey: 'manualReferenceConflict',
    items: [
      { value: 'NONE', detailKey: 'manualConflictNone' },
      { value: 'SERVER_WIN', detailKey: 'manualConflictServerWin' },
      { value: 'LAST_WRITE_WIN', detailKey: 'manualConflictLastWriteWin' },
    ],
  },
  {
    titleKey: 'manualReferenceBlankDefaults',
    items: [
      { value: 'source_node_ids', detailKey: 'manualBlankSourceNodes' },
      { value: 'dispatch_node_ids', detailKey: 'manualBlankDispatchNodes' },
      { value: 'target_primary_keys', detailKey: 'manualBlankTargetKeys' },
      { value: 'include_columns', detailKey: 'manualBlankIncludeColumns' },
      { value: 'exclude_columns', detailKey: 'manualBlankExcludeColumns' },
      { value: 'column_mappings', detailKey: 'manualBlankColumnMappings' },
    ],
  },
  {
    titleKey: 'manualReferenceRuleFields',
    items: [
      { value: 'database_name', detailKey: 'manualFieldDatabaseName' },
      { value: 'table_name', detailKey: 'manualFieldTableName' },
      { value: 'target_database_name', detailKey: 'manualFieldTargetDatabaseName' },
      { value: 'target_table_name', detailKey: 'manualFieldTargetTableName' },
      { value: 'primary_keys', detailKey: 'manualFieldPrimaryKeys' },
      { value: 'source_node_ids', detailKey: 'manualFieldSourceNodeIDs' },
      { value: 'dispatch_node_ids', detailKey: 'manualFieldDispatchNodeIDs' },
      { value: 'column_mappings', detailKey: 'manualFieldColumnMappings' },
    ],
  },
];

const securityReferences: ReferenceItem[] = [
  { value: 'security.admin_password', detailKey: 'manualSecurityAdminPassword' },
  { value: 'security.exit_password', detailKey: 'manualSecurityExitPassword' },
];

function ManualReference({ t }: { t: (key: string) => string }) {
  return (
    <section className="manual-reference">
      <strong>{t('manualReferenceTitle')}</strong>
      <div className="manual-reference-grid">
        {referenceGroups.map((group) => (
          <div className="manual-reference-group" key={group.titleKey}>
            <h4>{t(group.titleKey)}</h4>
            {group.items.map((item) => (
              <div className="manual-reference-row" key={`${group.titleKey}-${item.value}`}>
                <code>{item.value}</code>
                <span>{t(item.detailKey)}</span>
              </div>
            ))}
          </div>
        ))}
        <div className="manual-reference-group">
          <h4>{t('manualReferenceSecurity')}</h4>
          {securityReferences.map((item) => (
            <div className="manual-reference-row" key={item.value}>
              <code>{item.value}</code>
              <span>{t(item.detailKey)}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function ManualPage() {
  const { t } = useI18n();
  const [activeKey, setActiveKey] = useState(chapters[0].key);
  const active = useMemo(() => chapters.find((chapter) => chapter.key === activeKey) || chapters[0], [activeKey]);

  return (
    <section className="page-panel manual-page">
      <SectionHeader title={t('manual')} tone="info" />
      <div className="manual-layout">
        <aside className="manual-toc" aria-label={t('manualToc')}>
          <strong>{t('manualToc')}</strong>
          {chapters.map((chapter, index) => (
            <button
              className={chapter.key === active.key ? 'manual-toc-item active' : 'manual-toc-item'}
              key={chapter.key}
              type="button"
              onClick={() => setActiveKey(chapter.key)}
            >
              <span>{String(index + 1).padStart(2, '0')}</span>
              {t(chapter.titleKey)}
            </button>
          ))}
        </aside>

        <article className="manual-content">
          <div className="manual-heading">
            <span>{t('manualChapter')}</span>
            <h3>{t(active.titleKey)}</h3>
            <p>{t(active.summaryKey)}</p>
          </div>

          <ol className="manual-steps">
            {active.bullets.map((key) => (
              <li key={key}>{t(key)}</li>
            ))}
          </ol>

          {active.key === 'protocol' ? <ManualReference t={t} /> : null}
        </article>
      </div>
    </section>
  );
}
