import { type ReactNode, useEffect, useState } from 'react';
import { EmptyState, ErrorState, LoadingState } from '../components/PageState';
import { SectionHeader } from '../components/SectionHeader';
import { SwitchControl } from '../components/SwitchControl';
import { useAuth } from '../auth';
import { useI18n } from '../i18n';
import { getSyncRules, saveSyncRules, type SyncRule } from '../services/wails';

function listValue(values?: string[]) {
  return values && values.length > 0 ? values.join(', ') : '-';
}

function sourceNodesValue(rule: SyncRule, t: (key: string) => string) {
  return rule.source_node_ids && rule.source_node_ids.length > 0 ? rule.source_node_ids.join(', ') : t('allSourceNodes');
}

function dispatchNodesValue(rule: SyncRule, t: (key: string) => string) {
  if ((rule.dispatch_target || 'AUTO') === 'SELECTED_EDGES') {
    return rule.dispatch_node_ids && rule.dispatch_node_ids.length > 0 ? rule.dispatch_node_ids.join(', ') : t('targetNodesRequired');
  }
  if ((rule.dispatch_target || 'AUTO') === 'ACTIVE_EDGES') return t('activeEdgeNodes');
  if ((rule.dispatch_target || 'AUTO') === 'NONE') return t('notRequired');
  return t('autoDispatchNodes');
}

function mappingValue(rule: SyncRule) {
  if (!rule.column_mappings || rule.column_mappings.length === 0) return '-';
  return rule.column_mappings.map((item) => `${item.source_column} -> ${item.target_column}`).join(', ');
}

function mappingDisplayValue(rule: SyncRule, t: (key: string) => string) {
  return mappingValue(rule) === '-' ? t('sameNameMapping') : mappingValue(rule);
}

function hasWhitespace(value?: string) {
  return /\s/.test(value || '');
}

function parseList(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseMappings(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      const [source, target] = item.split('->').map((part) => part.trim());
      return { source_column: source || '', target_column: target || source || '' };
    })
    .filter((item) => item.source_column);
}

function Field({
  label,
  children,
  warning,
  className = '',
}: {
  label: string;
  children: ReactNode;
  warning?: string;
  className?: string;
}) {
  return (
    <label className={`rule-field ${warning ? 'warning' : ''} ${className}`}>
      <span>{label}</span>
      {children}
      {warning ? <small className="rule-space-warning">{warning}</small> : null}
    </label>
  );
}

function DefaultHint({ children }: { children: ReactNode }) {
  return <small className="rule-default-hint">{children}</small>;
}

function ReadOnlyRuleItem({ label, value, hint }: { label: string; value: ReactNode; hint?: ReactNode }) {
  return (
    <div className="rule-readonly-item">
      <span>{label}</span>
      <strong>{value || '-'}</strong>
      {hint ? <small>{hint}</small> : null}
    </div>
  );
}

function RulesReadOnly({ rules, t }: { rules: SyncRule[]; t: (key: string) => string }) {
  return (
    <div className="rules-card-list readonly-rule-list">
      {rules.map((rule, index) => (
        <article className="rule-card readonly-rule-card" key={rule.id || `${rule.database_name}.${rule.table_name}.${index}`}>
          <div className="rule-readonly-head">
            <div>
              <span className="rule-card-kicker">{t('ruleIdentity')}</span>
              <strong>{rule.id || '-'}</strong>
            </div>
            <span className={rule.enable ? 'status-chip status-ok' : 'status-chip status-unknown'}>
              {rule.enable ? t('enabled') : t('no')}
            </span>
          </div>

          <div className="rule-readonly-grid">
            <section className="rule-readonly-section">
              <h3>{t('tablePair')}</h3>
              <div className="rule-readonly-items two-col">
                <ReadOnlyRuleItem label={t('database')} value={rule.database_name || '-'} />
                <ReadOnlyRuleItem label={t('source')} value={rule.table_name || '-'} />
                <ReadOnlyRuleItem
                  label={t('targetDatabase')}
                  value={rule.target_database_name || rule.database_name || '-'}
                  hint={!rule.target_database_name ? t('targetDatabaseDefaultHint') : undefined}
                />
                <ReadOnlyRuleItem
                  label={t('targetTable')}
                  value={rule.target_table_name || rule.table_name || '-'}
                  hint={!rule.target_table_name ? t('targetTableDefaultHint') : undefined}
                />
              </div>
            </section>

            <section className="rule-readonly-section">
              <h3>{t('routing')}</h3>
              <div className="rule-readonly-items three-col">
                <ReadOnlyRuleItem label={t('direction')} value={rule.direction || '-'} />
                <ReadOnlyRuleItem
                  label={t('dispatch')}
                  value={rule.dispatch_target || 'AUTO'}
                  hint={dispatchNodesValue(rule, t)}
                />
                <ReadOnlyRuleItem label={t('conflict')} value={rule.conflict_policy || '-'} />
                <ReadOnlyRuleItem label={t('sourceNodes')} value={sourceNodesValue(rule, t)} />
                <ReadOnlyRuleItem label={t('selectedTargetNodes')} value={dispatchNodesValue(rule, t)} />
              </div>
            </section>

            <section className="rule-readonly-section">
              <h3>{t('keyColumns')}</h3>
              <div className="rule-readonly-items two-col">
                <ReadOnlyRuleItem label={t('keys')} value={listValue(rule.primary_keys)} />
                <ReadOnlyRuleItem
                  label={t('targetKeys')}
                  value={listValue(rule.target_primary_keys)}
                  hint={
                    rule.target_primary_keys && rule.target_primary_keys.length > 0
                      ? undefined
                      : t('targetKeysDefaultHint')
                  }
                />
              </div>
            </section>

            <section className="rule-readonly-section">
              <h3>{t('columnPolicy')}</h3>
              <div className="rule-readonly-items two-col">
                <ReadOnlyRuleItem
                  label={t('includeSourceColumns')}
                  value={rule.include_columns && rule.include_columns.length > 0 ? listValue(rule.include_columns) : t('allColumns')}
                />
                <ReadOnlyRuleItem
                  label={t('excludeSourceColumns')}
                  value={rule.exclude_columns && rule.exclude_columns.length > 0 ? listValue(rule.exclude_columns) : t('excludeNone')}
                />
                <ReadOnlyRuleItem label={t('mappings')} value={mappingDisplayValue(rule, t)} />
              </div>
            </section>
          </div>
        </article>
      ))}
    </div>
  );
}

export function RulesPage() {
  const { t } = useI18n();
  const { authState, ensureUnlocked } = useAuth();
  const [rules, setRules] = useState<SyncRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      setRules(await getSyncRules());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('rulesError'));
    } finally {
      setLoading(false);
    }
  }

  async function save() {
    setError('');
    setMessage('');
    if (!(await ensureUnlocked())) {
      return;
    }
    try {
      setRules(await saveSyncRules(rules));
      setMessage(t('rulesSaved'));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('rulesError'));
    }
  }

  function updateRule(index: number, patch: Partial<SyncRule>) {
    setRules((current) => current.map((rule, ruleIndex) => (ruleIndex === index ? { ...rule, ...patch } : rule)));
  }

  async function addRule() {
    if (!(await ensureUnlocked())) {
      return;
    }
    setRules((current) => [
      ...current,
      {
        id: `rule-${current.length + 1}`,
        database_name: '',
        table_name: '',
        target_database_name: '',
        target_table_name: '',
        direction: 'BIDIRECTIONAL',
        dispatch_target: 'AUTO',
        dispatch_node_ids: [],
        conflict_policy: 'LAST_WRITE_WIN',
        enable: true,
        source_node_ids: [],
        primary_keys: [],
        target_primary_keys: [],
        include_columns: [],
        exclude_columns: [],
        column_mappings: [],
      },
    ]);
  }

  async function deleteRule(index: number) {
    if (!(await ensureUnlocked())) {
      return;
    }
    setRules((current) => current.filter((_, ruleIndex) => ruleIndex !== index));
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <section className="page-panel">
      <SectionHeader title={t('rules')} tone="sync" />
      {loading ? <LoadingState title={t('loadingRules')} /> : null}
      {error ? <ErrorState title={t('rulesError')} detail={error} /> : null}
      {message ? (
        <div className="result-line ok">
          <span>{t('saved')}</span>
          <strong>{message}</strong>
        </div>
      ) : null}

      {!authState.unlocked ? (
        <div className="readonly-banner">
          <div>
            <strong>{t('readOnlyLockedTitle')}</strong>
            <span>{t('rulesReadOnlyDetail')}</span>
          </div>
          <button className="button-primary compact" type="button" onClick={() => void ensureUnlocked()}>
            {t('unlock')}
          </button>
        </div>
      ) : (
        <div className="toolbar-row">
          <button className="button-primary" type="button" onClick={() => void save()}>
            {t('saveRules')}
          </button>
          <button className="button-tool" type="button" onClick={() => void addRule()}>
            {t('addRule')}
          </button>
          <button className="button-secondary" type="button" onClick={() => void load()}>
            {t('refresh')}
          </button>
        </div>
      )}

      {!loading && rules.length === 0 ? <EmptyState title={t('noSyncRules')} detail={t('emptyRuleSet')} /> : null}

      {rules.length > 0 && !authState.unlocked ? <RulesReadOnly rules={rules} t={t} /> : null}

      {rules.length > 0 && authState.unlocked ? (
        <div className="rules-card-list">
          {rules.map((rule, index) => (
            <article className="rule-card" key={rule.id || `${rule.database_name}.${rule.table_name}.${index}`}>
              <div className="rule-card-head">
                <div>
                  <span className="rule-card-kicker">{t('ruleIdentity')}</span>
                  <input
                    className={hasWhitespace(rule.id) ? 'rule-id-input input-warning' : 'rule-id-input'}
                    value={rule.id || ''}
                    onChange={(event) => updateRule(index, { id: event.target.value })}
                  />
                  {hasWhitespace(rule.id) ? <small className="rule-space-warning">{t('spaceWarning')}</small> : null}
                </div>
                <SwitchControl
                  checked={rule.enable}
                  label={rule.enable ? t('enabled') : t('no')}
                  onChange={(checked) => updateRule(index, { enable: checked })}
                />
                <button className="button-danger compact" type="button" onClick={() => void deleteRule(index)}>
                  {t('deleteRule')}
                </button>
              </div>

              <div className="rule-section-grid">
                <fieldset className="rule-section">
                  <legend>{t('tablePair')}</legend>
                  <div className="rule-field-grid two-col">
                    <Field label={t('database')} warning={hasWhitespace(rule.database_name) ? t('spaceWarning') : ''}>
                      <input
                        value={rule.database_name || ''}
                        onChange={(event) => updateRule(index, { database_name: event.target.value })}
                      />
                    </Field>
                    <Field label={t('source')} warning={hasWhitespace(rule.table_name) ? t('spaceWarning') : ''}>
                      <input
                        value={rule.table_name || ''}
                        onChange={(event) => updateRule(index, { table_name: event.target.value })}
                      />
                    </Field>
                    <Field label={t('targetDatabase')} warning={hasWhitespace(rule.target_database_name) ? t('spaceWarning') : ''}>
                      <input
                        value={rule.target_database_name || ''}
                        onChange={(event) => updateRule(index, { target_database_name: event.target.value })}
                      />
                      {!rule.target_database_name ? <DefaultHint>{t('targetDatabaseDefaultHint')}</DefaultHint> : null}
                    </Field>
                    <Field label={t('targetTable')} warning={hasWhitespace(rule.target_table_name) ? t('spaceWarning') : ''}>
                      <input
                        value={rule.target_table_name || ''}
                        onChange={(event) => updateRule(index, { target_table_name: event.target.value })}
                      />
                      {!rule.target_table_name ? <DefaultHint>{t('targetTableDefaultHint')}</DefaultHint> : null}
                    </Field>
                  </div>
                </fieldset>

                <fieldset className="rule-section">
                  <legend>{t('routing')}</legend>
                  <div className="rule-field-grid routing-grid">
                    <Field label={t('direction')}>
                      <select value={rule.direction} onChange={(event) => updateRule(index, { direction: event.target.value })}>
                        <option value="EDGE_TO_SERVER">EDGE_TO_SERVER</option>
                        <option value="BIDIRECTIONAL">BIDIRECTIONAL</option>
                        <option value="SERVER_TO_EDGE">SERVER_TO_EDGE</option>
                        <option value="IGNORE">IGNORE</option>
                      </select>
                    </Field>
                    <Field label={t('dispatch')}>
                      <select
                        value={rule.dispatch_target || 'AUTO'}
                        onChange={(event) =>
                          updateRule(index, {
                            dispatch_target: event.target.value,
                            dispatch_node_ids: event.target.value === 'SELECTED_EDGES' ? rule.dispatch_node_ids || [] : [],
                          })
                        }
                      >
                        <option value="AUTO">AUTO</option>
                        <option value="NONE">NONE</option>
                        <option value="ACTIVE_EDGES">ACTIVE_EDGES</option>
                        <option value="SELECTED_EDGES">SELECTED_EDGES</option>
                      </select>
                    </Field>
                    <Field label={t('conflict')}>
                      <select
                        value={rule.conflict_policy}
                        onChange={(event) => updateRule(index, { conflict_policy: event.target.value })}
                      >
                        <option value="NONE">NONE</option>
                        <option value="SERVER_WIN">SERVER_WIN</option>
                        <option value="LAST_WRITE_WIN">LAST_WRITE_WIN</option>
                      </select>
                    </Field>
                    <Field label={t('sourceNodes')} className="wide-field">
                      <input
                        value={listValue(rule.source_node_ids) === '-' ? '' : listValue(rule.source_node_ids)}
                        onChange={(event) => updateRule(index, { source_node_ids: parseList(event.target.value) })}
                      />
                      <DefaultHint>{t('sourceNodesDefaultHint')}</DefaultHint>
                    </Field>
                    <Field label={t('selectedTargetNodes')} className="wide-field">
                      <input
                        value={listValue(rule.dispatch_node_ids) === '-' ? '' : listValue(rule.dispatch_node_ids)}
                        onChange={(event) => updateRule(index, { dispatch_node_ids: parseList(event.target.value) })}
                        disabled={(rule.dispatch_target || 'AUTO') !== 'SELECTED_EDGES'}
                      />
                      <DefaultHint>
                        {(rule.dispatch_target || 'AUTO') === 'SELECTED_EDGES'
                          ? t('targetNodesManualOnlyDetailed')
                          : t('targetNodesDefaultHint')}
                      </DefaultHint>
                    </Field>
                  </div>
                </fieldset>

                <fieldset className="rule-section">
                  <legend>{t('keyColumns')}</legend>
                  <div className="rule-field-grid two-col">
                    <Field label={t('keys')}>
                      <input
                        value={listValue(rule.primary_keys) === '-' ? '' : listValue(rule.primary_keys)}
                        onChange={(event) => updateRule(index, { primary_keys: parseList(event.target.value) })}
                      />
                    </Field>
                    <Field label={t('targetKeys')}>
                      <input
                        value={listValue(rule.target_primary_keys) === '-' ? '' : listValue(rule.target_primary_keys)}
                        onChange={(event) => updateRule(index, { target_primary_keys: parseList(event.target.value) })}
                      />
                      <DefaultHint>{t('targetKeysDefaultHint')}</DefaultHint>
                    </Field>
                  </div>
                </fieldset>

                <fieldset className="rule-section">
                  <legend>{t('columnPolicy')}</legend>
                  <div className="rule-field-grid two-col">
                    <Field label={t('includeSourceColumns')}>
                      <input
                        value={listValue(rule.include_columns) === '-' ? '' : listValue(rule.include_columns)}
                        onChange={(event) => updateRule(index, { include_columns: parseList(event.target.value) })}
                      />
                      <DefaultHint>{t('includeColumnsDefaultHint')}</DefaultHint>
                    </Field>
                    <Field label={t('excludeSourceColumns')}>
                      <input
                        value={listValue(rule.exclude_columns) === '-' ? '' : listValue(rule.exclude_columns)}
                        onChange={(event) => updateRule(index, { exclude_columns: parseList(event.target.value) })}
                      />
                      <DefaultHint>{t('excludeColumnsDefaultHint')}</DefaultHint>
                    </Field>
                    <Field label={t('mappings')} className="wide-field">
                      <input
                        value={mappingValue(rule) === '-' ? '' : mappingValue(rule)}
                        onChange={(event) => updateRule(index, { column_mappings: parseMappings(event.target.value) })}
                      />
                      <DefaultHint>{t('mappingDefaultHint')}</DefaultHint>
                    </Field>
                  </div>
                </fieldset>
              </div>
            </article>
          ))}
        </div>
        ) : null}
    </section>
  );
}
