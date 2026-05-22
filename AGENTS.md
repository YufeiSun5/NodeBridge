# Project Guidelines

## Project Summary / 项目摘要 / プロジェクト概要

NodeBridge is planned as an independent MySQL data synchronization product for edge nodes and a central server.  
NodeBridge 规划为独立的 MySQL 数据同步产品，用于边缘节点与中心节点之间的数据同步。  
NodeBridge は、エッジノードと中央サーバー間で MySQL データを同期する独立製品として計画されています。

Core chain / 核心链路 / コアフロー:

```text
MySQL -> CDC -> SyncEvent -> RabbitMQ -> Apply -> MySQL
```

Evidence / 依据 / 根拠: user-provided V1.0 design document plus the current Go MVP skeleton.  
依据：用户提供的 V1.0 设计文档，以及当前 Go MVP 骨架。  
根拠：ユーザー提供の V1.0 設計文書と現在の Go MVP スケルトンです。

## Technology Stack / 技术栈 / 技術スタック

- Go: `SyncAgent.exe`, Wails backend, synchronization logic, service control, diagnostics.
- Go：`SyncAgent.exe`、Wails 后端、同步逻辑、服务控制、诊断。
- Go：`SyncAgent.exe`、Wails バックエンド、同期ロジック、サービス制御、診断。
- Wails2 + React + TypeScript: `DataSync.exe` management UI.
- RabbitMQ: durable queues, publisher confirms, manual ACK, retry, dead letters.
- MySQL: source and target database.
- Canal: preferred first CDC reader; Debezium is a future optional adapter.
- Windows first; Linux Server later.

## Core Modules / 核心模块 / コアモジュール

| Module | Responsibility / 职责 / 役割 | Planned Path |
| --- | --- | --- |
| DataSync UI | Local management UI / 本地管理界面 / ローカル管理 UI | `cmd/datasync-ui/`, `frontend/` |
| SyncAgent | Long-running sync runtime / 长期运行同步核心 / 常駐同期ランタイム | `cmd/sync-agent/`, `internal/` |
| Config | Load, save, validate, encrypt config / 配置读写校验加密 / 設定の読込・保存・検証・暗号化 | `internal/appconfig/` |
| CDC | Canal/Debezium abstraction / CDC 抽象 / CDC 抽象化 | `internal/cdc/` |
| Event | `SyncEvent` model and normalization / 标准事件与标准化 / 標準イベントと正規化 | `internal/event/`, `internal/normalizer/` |
| RabbitMQ | Topology, confirm, ack / 拓扑、发布确认、消费确认 / トポロジー、confirm、ack | `internal/rabbitmq/` |
| Apply | Idempotent transactional writes / 幂等事务写入 / 冪等なトランザクション書込 | `internal/apply/` |
| Loop | Replay detection / 回放识别 / リプレイ検出 | `internal/loop/` |
| Router | Server routing and dispatch / Server 路由分发 / Server ルーティング配信 | `internal/router/` |
| Status | Health and queue metrics / 健康与队列指标 / ヘルスとキュー指標 | `internal/status/` |

## Core Conventions / 核心约定 / 主要ルール

- Current trial mode uses Wails tray persistence; backend supports auth/autostart/runtime APIs, frontend owns tray UI. / 当前试用版采用 Wails 托盘常驻；后端提供鉴权、自启动、运行时接口，前端负责托盘 UI。 / 現在の試用版は Wails トレイ常駐で、バックエンドは認証・自動起動・ランタイム API、フロントエンドはトレイ UI を担当します。
- Treat `SyncEvent` as the stable cross-module contract. / `SyncEvent` 是稳定的跨模块契约。 / `SyncEvent` はモジュール間の安定した契約です。
- Use `event_id`, `origin_node_id`, `updated_by_node`, `last_event_id`, and `sync_apply_log` for loop suppression. / 使用这些字段和表实现回环抑制。 / これらのフィールドとテーブルでループ抑制を行います。
- ACK RabbitMQ messages only after business writes and system logs are committed. / 业务写入和系统日志提交后才 ACK。 / 業務書込とシステムログのコミット後に ACK します。
- Prefer small, focused packages under `internal/`. / `internal/` 下保持小而专注的包。 / `internal/` 配下は小さく責務の明確なパッケージにします。
- Never assume source and target table or column names match. / 不得假设源表列名等于目标表列名。 / ソースとターゲットの表名・列名が同じとは限りません。
- Comments must be short and strong. / 注释必须简短有力。 / コメントは短く力強く。
- Core behavior needs complete tests. / 核心行为必须有完整测试。 / コア動作には完全なテストが必要です。
- Run tests and linter before phase handoff. / 阶段交付前运行测试和 linter。 / フェーズ引き渡し前にテストと linter を実行します。

## Required Reading / 必读文件 / 必読ファイル

- `MEMORY.md`: current phase and change log. / 当前阶段与改动记录。 / 現在の段階と変更記録。
- `.ai/instructions/ai-workflow.md`: AI documentation workflow. / AI 文档维护流程。 / AI ドキュメント運用手順。
- `.ai/instructions/go-syncagent.md`: Go backend and SyncAgent rules. / Go 后端与 SyncAgent 规范。 / Go バックエンドと SyncAgent ルール。
- `.ai/instructions/frontend-wails.md`: Wails/React UI rules. / Wails/React UI 规范。 / Wails/React UI ルール。
- `.ai/instructions/sync-architecture.md`: synchronization invariants. / 同步架构不变量。 / 同期アーキテクチャ不変条件。
- `.ai/docs/ui-design-spec.md`: dark industrial terminal UI style. / 暗色工业终端 UI 风格。 / ダーク産業端末 UI スタイル。
- `.ai/docs/frontend-backend-contract.md`: stable Wails UI API contract. / 稳定 Wails UI 接口契约。 / 安定した Wails UI API 契約。
- `.ai/docs/frontend-requirements.md`: frontend implementation scope. / 前端实现范围。 / フロントエンド実装範囲。
- `.ai/docs/ai-collaboration-log.md`: only active frontend/backend collaboration board. / 唯一活跃前后端协作看板。 / 唯一の有効なフロント/バックエンド連携ボード。

## On-Demand Resources / 按需资源 / 必要時のリソース

| Resource | Trigger / 触发条件 / 利用条件 |
| --- | --- |
| `.ai/docs/product-design.md` | Product and architecture brief / 产品与架构摘要 / 製品・構成概要 |
| `.ai/docs/roadmap.md` | Version roadmap and next milestone / 版本路线与下一里程碑 / バージョン計画と次の節目 |
| `.ai/docs/ui-design-spec.md` | Frontend visual style / 前端视觉风格 / フロントエンド視覚スタイル |
| `.ai/docs/frontend-requirements.md` | Frontend AI page scope / 前端 AI 页面范围 / フロントエンド AI 画面範囲 |
| `.ai/docs/frontend-backend-contract.md` | Stable API/DTO lookup / 稳定接口与 DTO 查询 / 安定 API・DTO 確認 |
| `.ai/docs/ai-collaboration-log.md` | Active cross-AI board / 活跃跨 AI 看板 / 有効な AI 間ボード |
| `docs/managed-components.md` | Installer ownership boundary / 安装器资源归属边界 / インストーラー所有境界 |
| `.ai/skills/README.md` | Skill creation and usage rules / 技能创建与使用规则 / スキル作成・利用ルール |
| `.ai/agents/architecture-review.agent.md` | Read-only architecture review / 只读架构审查 / 読み取り専用構成レビュー |
| `.ai/prompts/implement-sync-module.prompt.md` | Implement a sync module / 实现同步模块 / 同期モジュール実装 |
| `.ai/prompts/add-management-page.prompt.md` | Add a management page / 新增管理页面 / 管理画面追加 |
| `.ai/prompts/architecture-review.prompt.md` | Focused design/code review / 设计或代码审查 / 設計・コードレビュー |

## Mandatory Workflow / 强制工作流 / 必須ワークフロー

1. Read `MEMORY.md` and relevant `.ai/instructions/` before editing. / 编辑前读取 `MEMORY.md` 和相关规范。 / 編集前に `MEMORY.md` と関連ルールを読みます。
2. Prefer source, build scripts, CI, and tests over older docs. / 源码、构建、CI、测试优先于旧文档。 / ソース、ビルド、CI、テストを古い文書より優先します。
3. Mark unresolved uncertainty with `<!-- 待确认 -->`. / 不确定内容标记为 `<!-- 待确认 -->`。 / 未確定事項は `<!-- 待确认 -->` と記載します。
4. Keep changes scoped and update tests/examples when behavior changes. / 控制改动范围，行为变化时更新测试或示例。 / 変更範囲を絞り、挙動変更時はテストや例を更新します。
5. After meaningful changes, update `MEMORY.md`. / 有实质变更后更新 `MEMORY.md`。 / 重要な変更後は `MEMORY.md` を更新します。
6. For frontend/backend split work, use only two core files: contract for stable API, collaboration board for active questions. / 前后端分工只用两个核心文件：contract 管稳定接口，协作看板管活跃问题。 / フロント/バックエンド分担では、契約は安定 API、連携ボードは有効な質問に限定します。
7. Before every frontend/backend task, including backend-only dialogs, read `.ai/docs/ai-collaboration-log.md` Active Board; close, answer, or block relevant items before handoff. / 每次前后端任务开始前，包括纯后端对话，都必须读取协作看板 Active Board；交付前关闭、回复或标记阻塞。 / フロント/バックエンド作業前、バックエンドのみでも Active Board を読み、引き渡し前に完了・回答・ブロックを記録します。
8. Backend must actively record frontend-facing questions, required UI changes, DTO changes, and blockers in the Active Board. / 后端必须主动把需要前端处理的问题、UI 变更、DTO 变更和阻塞写入 Active Board。 / バックエンドはフロント側対応、UI 変更、DTO 変更、ブロッカーを Active Board に記録します。

## Test Gate / 测试门禁 / テストゲート

- Unit tests for pure logic are mandatory. / 纯逻辑必须写单元测试。 / 純粋ロジックには単体テストが必須です。
- CLI and config changes need smoke or validation tests. / CLI 和配置变更需要 smoke 或 validation 测试。 / CLI と設定変更には smoke または validation テストが必要です。
- Run `go test ./...` and `go vet ./...` before handoff. / 交付前运行 `go test ./...` 和 `go vet ./...`。 / 引き渡し前に `go test ./...` と `go vet ./...` を実行します。
- Run `golangci-lint run ./...` when installed. / 已安装时运行 `golangci-lint run ./...`。 / インストール済みなら `golangci-lint run ./...` を実行します。
- For split frontend/backend work, final response must summarize Active Board items handled and still open/blocked. / 前后端分工任务的最终回复必须说明已处理和仍 open/blocked 的看板项。 / 分担作業の最終応答では処理済み項目と open/blocked 項目を要約します。

## Wails And Logs / Wails 与日志 / Wails とログ

- Wails UI should not occupy a port by default. / Wails UI 默认不占端口。 / Wails UI は既定でポートを占有しません。
- Frontend calls Go through Wails bindings. / 前端通过 Wails binding 调 Go。 / フロントエンドは Wails binding で Go を呼びます。
- Closing the Wails window should hide it to tray; authenticated exit must use the explicit exit flow. / 关闭 Wails 窗口应隐藏到托盘；真正退出必须走显式退出鉴权流程。 / Wails ウィンドウを閉じる場合はトレイへ隠し、実際の終了は明示的な認証付き終了フローを使います。
- Log Web is separate and opt-in. / 日志 Web 独立且默认关闭。 / ログ Web は独立で任意有効です。
- MCP Server is reserved and disabled by default; the UI may toggle only the Wails backend config switch. / MCP Server 预留且默认关闭；前端只能切换 Wails 后端配置开关。 / MCP Server は予約機能で既定は無効、UI は Wails バックエンド設定スイッチのみ切替可能です。

## Language / 语言 / 言語

- `AGENTS.md` and active skill docs should use Chinese, English, and Japanese where practical. / `AGENTS.md` 与活跃技能文档尽量使用中英日三语。 / `AGENTS.md` と有効なスキル文書は可能な範囲で中国語・英語・日本語を併記します。
- DataSync frontend must support Chinese, English, and Japanese UI text; default to Chinese and provide an in-app language switch. / DataSync 前端必须支持中文、英文、日文界面文本，默认中文，并提供界面内语言切换。 / DataSync フロントエンドは中国語・英語・日本語の UI 文言に対応し、既定は中国語、画面内で言語を切り替えられるようにします。
- Keep protocol values, DTO fields, config keys, queue names, table names, log levels, and technical identifiers in English even when UI labels are translated. / 即使界面标签被翻译，协议值、DTO 字段、配置键、队列名、表名、日志级别和技术标识仍保持英文。 / UI ラベルを翻訳しても、プロトコル値、DTO 項目、設定キー、キュー名、テーブル名、ログレベル、技術識別子は英語のままにします。
- Use English for identifiers, package names, config keys, logs, protocol fields, and code comments unless local context requires otherwise. / 标识符、包名、配置键、日志、协议字段和代码注释默认使用英文。 / 識別子、パッケージ名、設定キー、ログ、プロトコル項目、コードコメントは原則英語を使います。
- User-facing documentation may be Chinese-first, with concise English and Japanese equivalents. / 面向用户的文档可中文优先，并附简洁英文和日文。 / ユーザー向け文書は中国語を主にし、簡潔な英語・日本語を併記できます。
