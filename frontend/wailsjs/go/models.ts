export namespace appconfig {

	export class CDCConfig {
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

	    static createFrom(source: any = {}) {
	        return new CDCConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.mode = source["mode"];
	        this.install = source["install"];
	        this.reader_name = source["reader_name"];
	        this.canal_addr = source["canal_addr"];
	        this.config_dir = source["config_dir"];
	        this.service_name = source["service_name"];
	        this.destination = source["destination"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.filter = source["filter"];
	        this.batch_size = source["batch_size"];
	        this.use_gtid = source["use_gtid"];
	    }
	}
	export class SecurityConfig {
	    admin_password?: string;
	    exit_password?: string;

	    static createFrom(source: any = {}) {
	        return new SecurityConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.admin_password = source["admin_password"];
	        this.exit_password = source["exit_password"];
	    }
	}
	export class MCPServerConfig {
	    enable: boolean;

	    static createFrom(source: any = {}) {
	        return new MCPServerConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enable = source["enable"];
	    }
	}
	export class LogWebConfig {
	    enable: boolean;
	    bind: string;
	    port: number;
	    token: string;

	    static createFrom(source: any = {}) {
	        return new LogWebConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enable = source["enable"];
	        this.bind = source["bind"];
	        this.port = source["port"];
	        this.token = source["token"];
	    }
	}
	export class SyncConfig {
	    upload_batch_size?: number;
	    dispatch_batch_size?: number;
	    flush_interval_millis?: number;
	    retry_interval_seconds: number;
	    heartbeat_interval_seconds?: number;
	    node_timeout_seconds?: number;

	    static createFrom(source: any = {}) {
	        return new SyncConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upload_batch_size = source["upload_batch_size"];
	        this.dispatch_batch_size = source["dispatch_batch_size"];
	        this.flush_interval_millis = source["flush_interval_millis"];
	        this.retry_interval_seconds = source["retry_interval_seconds"];
	        this.heartbeat_interval_seconds = source["heartbeat_interval_seconds"];
	        this.node_timeout_seconds = source["node_timeout_seconds"];
	    }
	}
	export class RabbitMQConfig {
	    mode?: string;
	    install?: boolean;
	    local_url?: string;
	    server_url: string;
	    management_url?: string;
	    username?: string;
	    password?: string;
	    vhost?: string;

	    static createFrom(source: any = {}) {
	        return new RabbitMQConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.install = source["install"];
	        this.local_url = source["local_url"];
	        this.server_url = source["server_url"];
	        this.management_url = source["management_url"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.vhost = source["vhost"];
	    }
	}
	export class MySQLConfig {
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    database: string;

	    static createFrom(source: any = {}) {
	        return new MySQLConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.database = source["database"];
	    }
	}
	export class NodeConfig {
	    id: string;
	    name: string;
	    location?: string;

	    static createFrom(source: any = {}) {
	        return new NodeConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.location = source["location"];
	    }
	}
	export class Config {
	    mode: string;
	    node: NodeConfig;
	    mysql: MySQLConfig;
	    rabbitmq: RabbitMQConfig;
	    cdc: CDCConfig;
	    sync: SyncConfig;
	    log_web: LogWebConfig;
	    mcp_server?: MCPServerConfig;
	    security?: SecurityConfig;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.node = this.convertValues(source["node"], NodeConfig);
	        this.mysql = this.convertValues(source["mysql"], MySQLConfig);
	        this.rabbitmq = this.convertValues(source["rabbitmq"], RabbitMQConfig);
	        this.cdc = this.convertValues(source["cdc"], CDCConfig);
	        this.sync = this.convertValues(source["sync"], SyncConfig);
	        this.log_web = this.convertValues(source["log_web"], LogWebConfig);
	        this.mcp_server = this.convertValues(source["mcp_server"], MCPServerConfig);
	        this.security = this.convertValues(source["security"], SecurityConfig);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}







}

export namespace rules {

	export class ColumnMapping {
	    source_column: string;
	    target_column: string;

	    static createFrom(source: any = {}) {
	        return new ColumnMapping(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source_column = source["source_column"];
	        this.target_column = source["target_column"];
	    }
	}
	export class SyncRule {
	    id: string;
	    database_name: string;
	    table_name: string;
	    source_node_ids?: string[];
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

	    static createFrom(source: any = {}) {
	        return new SyncRule(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.database_name = source["database_name"];
	        this.table_name = source["table_name"];
	        this.source_node_ids = source["source_node_ids"];
	        this.target_database_name = source["target_database_name"];
	        this.target_table_name = source["target_table_name"];
	        this.direction = source["direction"];
	        this.dispatch_target = source["dispatch_target"];
	        this.dispatch_node_ids = source["dispatch_node_ids"];
	        this.conflict_policy = source["conflict_policy"];
	        this.enable = source["enable"];
	        this.primary_keys = source["primary_keys"];
	        this.target_primary_keys = source["target_primary_keys"];
	        this.include_columns = source["include_columns"];
	        this.exclude_columns = source["exclude_columns"];
	        this.column_mappings = this.convertValues(source["column_mappings"], ColumnMapping);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace status {

	export class Overview {
	    product_name: string;
	    mode: string;
	    node_id?: string;
	    node_name?: string;
	    config_loaded: boolean;
	    config_path?: string;
	    rules_path?: string;
	    agent_status: string;
	    agent_pid?: number;
	    agent_log_path?: string;
	    mysql_status?: string;
	    rabbitmq_status?: string;
	    cdc_status?: string;
	    cdc_message?: string;
	    server_status?: string;
	    upload_queue_depth: number;
	    downlink_queue_depth: number;
	    failed_event_count: number;
	    conflict_count: number;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new Overview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product_name = source["product_name"];
	        this.mode = source["mode"];
	        this.node_id = source["node_id"];
	        this.node_name = source["node_name"];
	        this.config_loaded = source["config_loaded"];
	        this.config_path = source["config_path"];
	        this.rules_path = source["rules_path"];
	        this.agent_status = source["agent_status"];
	        this.agent_pid = source["agent_pid"];
	        this.agent_log_path = source["agent_log_path"];
	        this.mysql_status = source["mysql_status"];
	        this.rabbitmq_status = source["rabbitmq_status"];
	        this.cdc_status = source["cdc_status"];
	        this.cdc_message = source["cdc_message"];
	        this.server_status = source["server_status"];
	        this.upload_queue_depth = source["upload_queue_depth"];
	        this.downlink_queue_depth = source["downlink_queue_depth"];
	        this.failed_event_count = source["failed_event_count"];
	        this.conflict_count = source["conflict_count"];
	        this.version = source["version"];
	    }
	}

}

export namespace uiapi {

	export class AgentProcessStatus {
	    executable_path?: string;
	    pid?: number;
	    status: string;
	    started_at?: string;
	    exited_at?: string;
	    last_error?: string;
	    log_path?: string;

	    static createFrom(source: any = {}) {
	        return new AgentProcessStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executable_path = source["executable_path"];
	        this.pid = source["pid"];
	        this.status = source["status"];
	        this.started_at = source["started_at"];
	        this.exited_at = source["exited_at"];
	        this.last_error = source["last_error"];
	        this.log_path = source["log_path"];
	    }
	}
	export class AuthState {
	    unlocked: boolean;
	    status: string;
	    expires_at?: string;
	    timeout_seconds: number;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new AuthState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unlocked = source["unlocked"];
	        this.status = source["status"];
	        this.expires_at = source["expires_at"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.message = source["message"];
	    }
	}
	export class AutoStartStatus {
	    enabled: boolean;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new AutoStartStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class ConfigDTO {
	    mode: string;
	    node: appconfig.NodeConfig;
	    mysql: appconfig.MySQLConfig;
	    rabbitmq: appconfig.RabbitMQConfig;
	    cdc: appconfig.CDCConfig;
	    sync: appconfig.SyncConfig;
	    log_web: appconfig.LogWebConfig;
	    mcp_server?: appconfig.MCPServerConfig;
	    security?: appconfig.SecurityConfig;

	    static createFrom(source: any = {}) {
	        return new ConfigDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.node = this.convertValues(source["node"], appconfig.NodeConfig);
	        this.mysql = this.convertValues(source["mysql"], appconfig.MySQLConfig);
	        this.rabbitmq = this.convertValues(source["rabbitmq"], appconfig.RabbitMQConfig);
	        this.cdc = this.convertValues(source["cdc"], appconfig.CDCConfig);
	        this.sync = this.convertValues(source["sync"], appconfig.SyncConfig);
	        this.log_web = this.convertValues(source["log_web"], appconfig.LogWebConfig);
	        this.mcp_server = this.convertValues(source["mcp_server"], appconfig.MCPServerConfig);
	        this.security = this.convertValues(source["security"], appconfig.SecurityConfig);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeadLetterMessageDTO {
	    queue: string;
	    content_type?: string;
	    body_preview: string;
	    body_size: number;
	    headers?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new DeadLetterMessageDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queue = source["queue"];
	        this.content_type = source["content_type"];
	        this.body_preview = source["body_preview"];
	        this.body_size = source["body_size"];
	        this.headers = source["headers"];
	    }
	}
	export class DeadLetterRequest {
	    queue?: string;
	    limit?: number;

	    static createFrom(source: any = {}) {
	        return new DeadLetterRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queue = source["queue"];
	        this.limit = source["limit"];
	    }
	}
	export class DeadLetterResponse {
	    items: DeadLetterMessageDTO[];

	    static createFrom(source: any = {}) {
	        return new DeadLetterResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], DeadLetterMessageDTO);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiagnosticPackageResponse {
	    path: string;

	    static createFrom(source: any = {}) {
	        return new DiagnosticPackageResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}
	export class FailedEventDTO {
	    event_id: string;
	    target_node_id: string;
	    status: string;
	    error_message: string;
	    created_at?: string;

	    static createFrom(source: any = {}) {
	        return new FailedEventDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event_id = source["event_id"];
	        this.target_node_id = source["target_node_id"];
	        this.status = source["status"];
	        this.error_message = source["error_message"];
	        this.created_at = source["created_at"];
	    }
	}
	export class FailedEventsRequest {
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new FailedEventsRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	    }
	}
	export class FailedEventsResponse {
	    items: FailedEventDTO[];

	    static createFrom(source: any = {}) {
	        return new FailedEventsResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], FailedEventDTO);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LogEntry {
	    time: string;
	    level: string;
	    module: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.module = source["module"];
	        this.message = source["message"];
	    }
	}
	export class LogQuery {
	    level?: string;
	    module?: string;
	    limit?: number;

	    static createFrom(source: any = {}) {
	        return new LogQuery(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.module = source["module"];
	        this.limit = source["limit"];
	    }
	}
	export class LogsResponse {
	    items: LogEntry[];

	    static createFrom(source: any = {}) {
	        return new LogsResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], LogEntry);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MCPServerStatus {
	    enabled: boolean;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new MCPServerStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class ManagedInstallOperationDTO {
	    component: string;
	    action: string;
	    target?: string;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new ManagedInstallOperationDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = source["component"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class ManagedInstallRequest {
	    manifest_path?: string;

	    static createFrom(source: any = {}) {
	        return new ManagedInstallRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manifest_path = source["manifest_path"];
	    }
	}
	export class ManagedInstallResponse {
	    mode: string;
	    manifest_path: string;
	    operations: ManagedInstallOperationDTO[];

	    static createFrom(source: any = {}) {
	        return new ManagedInstallResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.manifest_path = source["manifest_path"];
	        this.operations = this.convertValues(source["operations"], ManagedInstallOperationDTO);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OperationResult {
	    ok: boolean;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class QueueStatusDTO {
	    name: string;
	    role: string;
	    messages: number;
	    consumers: number;
	    status: string;

	    static createFrom(source: any = {}) {
	        return new QueueStatusDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role = source["role"];
	        this.messages = source["messages"];
	        this.consumers = source["consumers"];
	        this.status = source["status"];
	    }
	}
	export class QueueStatusResponse {
	    queues: QueueStatusDTO[];

	    static createFrom(source: any = {}) {
	        return new QueueStatusResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queues = this.convertValues(source["queues"], QueueStatusDTO);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RetryFailedEventRequest {
	    event_id: string;
	    target_node_id: string;

	    static createFrom(source: any = {}) {
	        return new RetryFailedEventRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event_id = source["event_id"];
	        this.target_node_id = source["target_node_id"];
	    }
	}
	export class RetryFailedEventsRequest {
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new RetryFailedEventsRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	    }
	}
	export class SaveConfigRequest {
	    config: ConfigDTO;

	    static createFrom(source: any = {}) {
	        return new SaveConfigRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], ConfigDTO);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveSyncRulesRequest {
	    rules: rules.SyncRule[];

	    static createFrom(source: any = {}) {
	        return new SaveSyncRulesRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], rules.SyncRule);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SetAutoStartRequest {
	    enabled: boolean;

	    static createFrom(source: any = {}) {
	        return new SetAutoStartRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class SetMCPServerEnabledRequest {
	    enabled: boolean;

	    static createFrom(source: any = {}) {
	        return new SetMCPServerEnabledRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class SyncRulesDTO {
	    rules: rules.SyncRule[];

	    static createFrom(source: any = {}) {
	        return new SyncRulesDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], rules.SyncRule);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TestResult {
	    ok: boolean;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class UnlockAdminRequest {
	    password: string;

	    static createFrom(source: any = {}) {
	        return new UnlockAdminRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	    }
	}
	export class VerifyExitPasswordRequest {
	    password: string;

	    static createFrom(source: any = {}) {
	        return new VerifyExitPasswordRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	    }
	}

}
