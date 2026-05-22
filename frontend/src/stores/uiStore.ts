export type AgentStatus = 'running' | 'stopped' | 'error' | 'unknown' | 'unsupported' | 'forced_stopped' | 'exited';

export interface RuntimeSummary {
  productName: string;
  mode: string;
  nodeId: string;
  nodeName: string;
  configLoaded: boolean;
  configPath: string;
  rulesPath: string;
  agentStatus: AgentStatus;
  agentPID: number;
  agentLogPath: string;
  mysqlStatus: string;
  rabbitmqStatus: string;
  cdcStatus: string;
  cdcMessage: string;
  uploadQueueDepth: number;
  downlinkQueueDepth: number;
  failedEventCount: number;
  conflictCount: number;
  version: string;
}
