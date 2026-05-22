import type { RuntimeSummary } from '../stores/uiStore';

export interface NodeConfig {
  id: string;
  name: string;
  location?: string;
}

export interface MySQLConfig {
  host: string;
  port: number;
  username: string;
  password: string;
  database: string;
}

export interface RabbitMQConfig {
  mode?: string;
  install?: boolean;
  local_url?: string;
  server_url: string;
  management_url?: string;
  username?: string;
  password?: string;
  vhost?: string;
}

export interface CDCConfig {
  type: string;
  mode?: string;
  install?: boolean;
  reader_name?: string;
  canal_addr?: string;
  config_dir?: string;
  service_name?: string;
  destination?: string;
  username?: string;
  password?: string;
  filter?: string;
  batch_size?: number;
  use_gtid?: boolean;
}

export interface SyncConfig {
  upload_batch_size?: number;
  dispatch_batch_size?: number;
  flush_interval_millis?: number;
  retry_interval_seconds: number;
  heartbeat_interval_seconds?: number;
  node_timeout_seconds?: number;
}

export interface LogWebConfig {
  enable: boolean;
  bind: string;
  port: number;
  token: string;
}

export interface MCPServerConfig {
  enable: boolean;
}

export interface SecurityConfig {
  admin_password?: string;
  exit_password?: string;
}

export interface ConfigDTO {
  mode: string;
  node: NodeConfig;
  mysql: MySQLConfig;
  rabbitmq: RabbitMQConfig;
  cdc: CDCConfig;
  sync: SyncConfig;
  log_web: LogWebConfig;
  mcp_server?: MCPServerConfig;
  security?: SecurityConfig;
}

export interface ColumnMapping {
  source_column: string;
  target_column: string;
}

export interface SyncRule {
  id: string;
  source_node_ids?: string[];
  database_name: string;
  table_name: string;
  target_database_name?: string;
  target_table_name?: string;
  direction: string;
  dispatch_target?: string;
  dispatch_node_ids?: string[];
  conflict_policy: string;
  enable: boolean;
  primary_keys: string[];
  target_primary_keys?: string[];
  include_columns: string[];
  exclude_columns: string[];
  column_mappings?: ColumnMapping[];
}

export interface QueueStatusDTO {
  name: string;
  role: string;
  messages: number;
  consumers: number;
  status: string;
}

export interface FailedEventDTO {
  event_id: string;
  target_node_id: string;
  status: string;
  error_message: string;
  created_at?: string;
}

export interface DeadLetterMessageDTO {
  queue: string;
  content_type?: string;
  body_preview: string;
  body_size: number;
  headers?: Record<string, string>;
}

export interface LogEntry {
  time: string;
  level: string;
  module: string;
  message: string;
}

export interface OperationResult {
  ok: boolean;
  status: string;
  message?: string;
}

export interface TestResult extends OperationResult {}

export interface AuthState {
  unlocked: boolean;
  status: string;
  expires_at?: string;
  timeout_seconds: number;
  message?: string;
}

export interface AutoStartStatus {
  enabled: boolean;
  status: string;
  message?: string;
}

export interface MCPServerStatus {
  enabled: boolean;
  status: string;
  message?: string;
}

export interface AgentProcessStatus {
  executable_path?: string;
  pid?: number;
  status: string;
  started_at?: string;
  exited_at?: string;
  last_error?: string;
  log_path?: string;
}

export interface ManagedInstallOperationDTO {
  component: string;
  action: string;
  target?: string;
  status: string;
  message?: string;
}

export interface ManagedInstallResponse {
  mode: string;
  manifest_path: string;
  operations: ManagedInstallOperationDTO[];
}

export interface DiagnosticPackageResponse {
  path: string;
}

type OverviewDTO = {
  product_name?: string;
  mode?: string;
  node_id?: string;
  node_name?: string;
  config_loaded?: boolean;
  config_path?: string;
  rules_path?: string;
  agent_status?: RuntimeSummary['agentStatus'];
  agent_pid?: number;
  agent_log_path?: string;
  mysql_status?: string;
  rabbitmq_status?: string;
  cdc_status?: string;
  cdc_message?: string;
  upload_queue_depth?: number;
  downlink_queue_depth?: number;
  failed_event_count?: number;
  conflict_count?: number;
  version?: string;
};

type BackendApp = {
  GetOverview?: () => Promise<OverviewDTO>;
  GetConfig?: () => Promise<ConfigDTO>;
  SaveConfig?: (req: { config: ConfigDTO }) => Promise<ConfigDTO>;
  TestMySQL?: (req: MySQLConfig) => Promise<TestResult>;
  TestRabbitMQ?: (req: RabbitMQConfig) => Promise<TestResult>;
  GetSyncRules?: () => Promise<{ rules: SyncRule[] }>;
  SaveSyncRules?: (req: { rules: SyncRule[] }) => Promise<{ rules: SyncRule[] }>;
  GetQueueStatus?: () => Promise<{ queues: QueueStatusDTO[] }>;
  GetFailedEvents?: (req: { limit: number }) => Promise<{ items: FailedEventDTO[] }>;
  RetryFailedEvent?: (req: { event_id: string; target_node_id: string }) => Promise<OperationResult>;
  RetryFailedEvents?: (req: { limit: number }) => Promise<OperationResult>;
  GetDeadLetters?: (req: { queue?: string; limit?: number }) => Promise<{ items: DeadLetterMessageDTO[] }>;
  GetLogs?: (req: { level?: string; module?: string; limit?: number }) => Promise<{ items: LogEntry[] }>;
  StartAgent?: () => Promise<OperationResult>;
  StopAgent?: () => Promise<OperationResult>;
  RestartAgent?: () => Promise<OperationResult>;
  GetAgentProcessStatus?: () => Promise<AgentProcessStatus>;
  VerifyExitPassword?: (req: { password: string }) => Promise<OperationResult>;
  RequestExit?: (req: { password: string }) => Promise<OperationResult>;
  GetAutoStart?: () => Promise<AutoStartStatus>;
  SetAutoStart?: (req: { enabled: boolean }) => Promise<AutoStartStatus>;
  GetMCPServerStatus?: () => Promise<MCPServerStatus>;
  SetMCPServerEnabled?: (req: { enabled: boolean }) => Promise<MCPServerStatus>;
  GetManagedInstallPlan?: (req: { manifest_path?: string }) => Promise<ManagedInstallResponse>;
  ApplyManagedInstall?: (req: { manifest_path?: string }) => Promise<ManagedInstallResponse>;
  ExportDiagnosticPackage?: () => Promise<DiagnosticPackageResponse>;
  UnlockAdmin?: (req: { password: string }) => Promise<OperationResult>;
  LockAdmin?: () => Promise<OperationResult>;
  GetAuthState?: () => Promise<AuthState>;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: BackendApp;
      };
      datasyncui?: {
        App?: BackendApp;
      };
    };
    runtime?: {
      EventsOn?: (eventName: string, callback: (...data: unknown[]) => void) => () => void;
      WindowMinimise?: () => Promise<void>;
      WindowUnminimise?: () => Promise<void>;
      WindowHide?: () => Promise<void>;
      WindowShow?: () => Promise<void>;
      Quit?: () => Promise<void>;
    };
  }
}

export const emptyConfig: ConfigDTO = {
  mode: '',
  node: { id: '', name: '', location: '' },
  mysql: { host: '', port: 3306, username: '', password: '', database: '' },
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
    batch_size: 50,
    use_gtid: false,
  },
  sync: {
    upload_batch_size: 50,
    dispatch_batch_size: 50,
    flush_interval_millis: 500,
    retry_interval_seconds: 5,
    heartbeat_interval_seconds: 10,
    node_timeout_seconds: 30,
  },
  log_web: { enable: false, bind: '127.0.0.1', port: 18080, token: '' },
  mcp_server: { enable: false },
  security: { admin_password: '', exit_password: '' },
};

export const fallbackOverview: RuntimeSummary = {
  productName: 'NodeBridge',
  mode: 'unknown',
  nodeId: 'local',
  nodeName: 'local',
  configLoaded: false,
  configPath: '',
  rulesPath: '',
  agentStatus: 'stopped',
  agentPID: 0,
  agentLogPath: '',
  mysqlStatus: 'unknown',
  rabbitmqStatus: 'unknown',
  cdcStatus: 'unknown',
  cdcMessage: '',
  uploadQueueDepth: 0,
  downlinkQueueDepth: 0,
  failedEventCount: 0,
  conflictCount: 0,
  version: 'unknown',
};

export function isConfigMissing(config: ConfigDTO): boolean {
  return !config.mode?.trim() || !config.node?.id?.trim() || !config.mysql?.database?.trim();
}

function app(): BackendApp | undefined {
  return window.go?.datasyncui?.App || window.go?.main?.App;
}

function unavailable(message: string): OperationResult {
  return { ok: false, status: 'unsupported', message };
}

function toSummary(overview?: OverviewDTO): RuntimeSummary {
  return {
    productName: overview?.product_name || fallbackOverview.productName,
    mode: overview?.mode || 'unknown',
    nodeId: overview?.node_id || 'local',
    nodeName: overview?.node_name || 'local',
    configLoaded: Boolean(overview?.config_loaded),
    configPath: overview?.config_path || '',
    rulesPath: overview?.rules_path || '',
    agentStatus: overview?.agent_status || 'stopped',
    agentPID: overview?.agent_pid || 0,
    agentLogPath: overview?.agent_log_path || '',
    mysqlStatus: overview?.mysql_status || 'unknown',
    rabbitmqStatus: overview?.rabbitmq_status || 'unknown',
    cdcStatus: overview?.cdc_status || 'unknown',
    cdcMessage: overview?.cdc_message || '',
    uploadQueueDepth: overview?.upload_queue_depth || 0,
    downlinkQueueDepth: overview?.downlink_queue_depth || 0,
    failedEventCount: overview?.failed_event_count || 0,
    conflictCount: overview?.conflict_count || 0,
    version: overview?.version || 'unknown',
  };
}

export async function getOverview(): Promise<RuntimeSummary> {
  const overview = await app()?.GetOverview?.();
  return toSummary(overview);
}

export async function getConfig(): Promise<ConfigDTO> {
  return (await app()?.GetConfig?.()) || emptyConfig;
}

export async function saveConfig(config: ConfigDTO): Promise<ConfigDTO> {
  const fn = app()?.SaveConfig;
  if (!fn) {
    throw new Error('Wails SaveConfig binding is not available');
  }
  return fn({ config });
}

export async function testMySQL(mysql: MySQLConfig): Promise<TestResult> {
  return (await app()?.TestMySQL?.(mysql)) || unavailable('Wails TestMySQL binding is not available');
}

export async function testRabbitMQ(rabbitmq: RabbitMQConfig): Promise<TestResult> {
  return (await app()?.TestRabbitMQ?.(rabbitmq)) || unavailable('Wails TestRabbitMQ binding is not available');
}

export async function getSyncRules(): Promise<SyncRule[]> {
  return (await app()?.GetSyncRules?.())?.rules || [];
}

export async function saveSyncRules(rules: SyncRule[]): Promise<SyncRule[]> {
  const fn = app()?.SaveSyncRules;
  if (!fn) {
    throw new Error('Wails SaveSyncRules binding is not available');
  }
  return (await fn({ rules })).rules || [];
}

export async function getQueueStatus(): Promise<QueueStatusDTO[]> {
  return (await app()?.GetQueueStatus?.())?.queues || [];
}

export async function getFailedEvents(limit = 50): Promise<FailedEventDTO[]> {
  return (await app()?.GetFailedEvents?.({ limit }))?.items || [];
}

export async function retryFailedEvent(event_id: string, target_node_id: string): Promise<OperationResult> {
  return (
    (await app()?.RetryFailedEvent?.({ event_id, target_node_id })) ||
    unavailable('Wails RetryFailedEvent binding is not available')
  );
}

export async function retryFailedEvents(limit = 100): Promise<OperationResult> {
  return (
    (await app()?.RetryFailedEvents?.({ limit })) ||
    unavailable('Wails RetryFailedEvents binding is not available')
  );
}

export async function getDeadLetters(query: { queue?: string; limit?: number } = {}): Promise<DeadLetterMessageDTO[]> {
  return (await app()?.GetDeadLetters?.(query))?.items || [];
}

export async function getLogs(query: { level?: string; module?: string; limit?: number }): Promise<LogEntry[]> {
  return (await app()?.GetLogs?.(query))?.items || [];
}

export async function startAgent(): Promise<OperationResult> {
  return (await app()?.StartAgent?.()) || unavailable('Wails StartAgent binding is not available');
}

export async function stopAgent(): Promise<OperationResult> {
  return (await app()?.StopAgent?.()) || unavailable('Wails StopAgent binding is not available');
}

export async function restartAgent(): Promise<OperationResult> {
  return (await app()?.RestartAgent?.()) || unavailable('Wails RestartAgent binding is not available');
}

export async function getAgentProcessStatus(): Promise<AgentProcessStatus> {
  return (
    (await app()?.GetAgentProcessStatus?.()) || {
      status: 'unsupported',
      last_error: 'Wails GetAgentProcessStatus binding is not available',
    }
  );
}

export async function verifyExitPassword(password: string): Promise<OperationResult> {
  return (
    (await app()?.VerifyExitPassword?.({ password })) ||
    unavailable('Wails VerifyExitPassword binding is not available')
  );
}

export async function requestExit(password: string): Promise<OperationResult> {
  return (await app()?.RequestExit?.({ password })) || unavailable('Wails RequestExit binding is not available');
}

export async function unlockAdmin(password: string): Promise<OperationResult> {
  return (await app()?.UnlockAdmin?.({ password })) || unavailable('Wails UnlockAdmin binding is not available');
}

export async function lockAdmin(): Promise<OperationResult> {
  return (await app()?.LockAdmin?.()) || unavailable('Wails LockAdmin binding is not available');
}

export async function getAuthState(): Promise<AuthState> {
  return (
    (await app()?.GetAuthState?.()) || {
      unlocked: false,
      status: 'unsupported',
      timeout_seconds: 0,
      message: 'Wails GetAuthState binding is not available',
    }
  );
}

export async function getAutoStart(): Promise<AutoStartStatus> {
  return (
    (await app()?.GetAutoStart?.()) || {
      enabled: false,
      status: 'unsupported',
      message: 'Wails GetAutoStart binding is not available',
    }
  );
}

export async function setAutoStart(enabled: boolean): Promise<AutoStartStatus> {
  return (
    (await app()?.SetAutoStart?.({ enabled })) || {
      enabled: false,
      status: 'unsupported',
      message: 'Wails SetAutoStart binding is not available',
    }
  );
}

export async function getMCPServerStatus(): Promise<MCPServerStatus> {
  return (
    (await app()?.GetMCPServerStatus?.()) || {
      enabled: false,
      status: 'unsupported',
      message: 'Wails GetMCPServerStatus binding is not available',
    }
  );
}

export async function setMCPServerEnabled(enabled: boolean): Promise<MCPServerStatus> {
  return (
    (await app()?.SetMCPServerEnabled?.({ enabled })) || {
      enabled: false,
      status: 'unsupported',
      message: 'Wails SetMCPServerEnabled binding is not available',
    }
  );
}

export async function getManagedInstallPlan(manifest_path = ''): Promise<ManagedInstallResponse> {
  return (
    (await app()?.GetManagedInstallPlan?.({ manifest_path })) || {
      mode: 'unsupported',
      manifest_path,
      operations: [],
    }
  );
}

export async function applyManagedInstall(manifest_path = ''): Promise<ManagedInstallResponse> {
  const fn = app()?.ApplyManagedInstall;
  if (!fn) {
    throw new Error('Wails ApplyManagedInstall binding is not available');
  }
  return fn({ manifest_path });
}

export async function exportDiagnosticPackage(): Promise<DiagnosticPackageResponse> {
  const fn = app()?.ExportDiagnosticPackage;
  if (!fn) {
    throw new Error('Wails ExportDiagnosticPackage binding is not available');
  }
  return fn();
}

export function onTrayExitRequest(callback: () => void): () => void {
  return window.runtime?.EventsOn?.('datasync:request-exit', callback) || (() => undefined);
}

export async function hideWindowToTray(): Promise<OperationResult> {
  if (!window.runtime?.WindowHide) {
    return unavailable('Wails WindowHide runtime binding is not available');
  }
  await window.runtime.WindowHide();
  return { ok: true, status: 'hidden_to_tray', message: 'window hidden to tray' };
}

export async function showWindowFromTray(): Promise<OperationResult> {
  if (!window.runtime?.WindowShow) {
    return unavailable('Wails WindowShow runtime binding is not available');
  }
  await window.runtime.WindowShow();
  await window.runtime.WindowUnminimise?.();
  return { ok: true, status: 'shown', message: 'window shown' };
}

export async function quitApp(): Promise<OperationResult> {
  if (!window.runtime?.Quit) {
    return unavailable('Wails Quit runtime binding is not available');
  }
  await window.runtime.Quit();
  return { ok: true, status: 'exiting', message: 'application exit requested' };
}
