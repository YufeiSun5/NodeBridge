# AI 协作文档体系初始化提示词

你是一个 AI 工程化专家。请为当前项目初始化或维护完整的 AI 协作文档体系。

## 目标

在项目中建立一套让 AI Agent 跨编辑器协作的文档结构，适配 Copilot、Cursor、Trae、Codex 等常见工具。体系必须覆盖：

- 项目路引入口
- 当前项目记忆
- 活跃协作看板
- 分领域编码规范
- 稳定设计文档和归档文档
- 可复用技能
- 自定义 Agent
- Prompt 模板
- 编辑器薄适配层

## 核心文件模型

必须采用以下文件边界：

```text
AGENTS.md                     # 项目路引和自动加载入口
MEMORY.md                     # 当前阶段、完成事项、待确认项和改动记录
AI_BOARD.md                   # 唯一活跃 AI 协作看板
.ai/                          # AI 母本文档体系
  instructions/               # 分领域规范
  docs/                       # 稳定文档、闭合记录、阶段总结、归档
  skills/                     # 可复用技能
  agents/                     # 自定义 Agent
  prompts/                    # Prompt 模板
.github/                      # Copilot 薄适配层
.cursor/                      # Cursor 薄适配层
```

`AI_BOARD.md` 必须与 `MEMORY.md` 同级，是唯一活跃协作看板。`.ai/docs/` 只放稳定文档、闭合记录、阶段总结和归档材料，不承载 open/blocked 工作项，不创建第二个活跃看板。

## 执行模式

默认直接初始化或维护文件。若用户明确要求“只分析”“先给报告”“只读模式”，则只输出分析报告，不创建或修改文件。

若 `MEMORY.md` 或 `AI_BOARD.md` 不存在，不要停止。将项目判定为全新或半初始化状态，先创建根级外层协作文件，再创建 `.ai/` 母本文档体系。

若文件已存在，必须合并和补充，不得盲目覆盖。冲突内容保留原文并标注 `<!-- 待确认 -->`。

## 第一步：扫描项目

先明确扫描边界。

忽略目录和文件：

- `node_modules`
- `dist`
- `build`
- `.next`
- `coverage`
- `vendor`
- `generated`
- `__pycache__`
- `.cache`
- 锁文件
- 编译产物、依赖目录、运行缓存、生成代码

优先阅读：

- `AGENTS.md`
- `MEMORY.md`
- `AI_BOARD.md`
- 根 README
- 包管理配置：`package.json` / `go.mod` / `Cargo.toml` / `pom.xml` 等
- 入口文件
- 核心业务目录
- 测试目录
- CI 配置和构建脚本

需要分析：

- 项目定位和核心职责
- 技术栈、框架、构建工具
- 模块划分和职责
- 现有编码约定和架构决策
- 当前阶段：初建、开发中、试点、维护、交付前等
- 高频开发操作模式
- 已有 AI 协作文档是否过时、重复或冲突

信息源优先级：

```text
运行配置 / 构建脚本 / CI 配置 / 当前源码结构 > 测试用例 > README > 其他历史文档 > 注释 > issue / commit
```

大项目策略：

- 若项目超过 20 个核心模块，或存在明显多子系统、目录层级复杂、入口分散，一次无法可靠形成全局认知，则先覆盖核心模块。
- 核心模块按入口引用链、主业务目录、构建配置、CI 触达范围、测试覆盖密度和 README 明示模块综合判断。
- 未覆盖模块写入 `MEMORY.md` 的待确认或后续建议。

降级策略：

- 若规范不一致，记录现状，不强行统一。
- 若无法确认架构决策，标注 `<!-- 待确认 -->`。
- 若历史文档与源码冲突，以源码、构建、CI、测试为准，并说明冲突来源。

## 第二步：判断初始化状态

检查以下文件：

```text
AGENTS.md
MEMORY.md
AI_BOARD.md
.ai/instructions/ai-workflow.md
.ai/docs/README.md
.ai/skills/README.md
.ai/agents/README.md
.ai/prompts/README.md
.github/copilot-instructions.md
.cursor/rules/
```

按状态执行：

- 全新项目：核心文件大多不存在。先创建 `AGENTS.md`、`MEMORY.md`、`AI_BOARD.md`，再创建 `.ai/`，最后创建编辑器适配层。
- 半初始化项目：只补缺失部分，已有内容合并维护。
- 已初始化项目：只更新过时、冲突、缺失或与当前源码不一致的内容。

## 第三步：创建或维护根级文件

### `AGENTS.md`

`AGENTS.md` 是唯一项目路引和自动加载入口，控制在 300 行以内。标题使用：

```md
# Project Guidelines
```

必须包含：

- Project Summary：项目定位 1 到 2 句。
- Technology Stack：语言、框架、构建工具、运行依赖。
- Core Modules：核心模块表格，只列关键模块。
- Core Conventions：4 到 8 条项目硬约定。
- Required Reading：至少列 `MEMORY.md`、`AI_BOARD.md`、核心 `.ai/instructions/*.md`。
- On-Demand Resources：列 `.ai/docs/`、`.ai/skills/`、`.ai/agents/`、`.ai/prompts/` 的关键文件和触发条件。
- AI Identity Model：定义 `frontend-ai`、`backend-ai`、`test-ai`、`review-ai`。
- Mandatory Workflow：编辑前读记忆和看板，操作前声明身份，实质变更后更新记忆，协作或阻塞更新看板。
- Test Gate：按项目实际技术栈列测试、lint、build、smoke、E2E、lab 验证要求。
- Language：说明代码、日志、配置、协议字段、用户文档的语言要求。

`AGENTS.md` 只做摘要和导航，不复制详细规范正文。

### `MEMORY.md`

`MEMORY.md` 是当前项目记忆，不是完整历史百科。必须包含：

```md
# MEMORY

Last updated: YYYY-MM-DD HH:MM <timezone>

## 当前阶段

## 已完成事项

## AI 工程化状态清单

## 后续建议

## 待确认

## 改动记录
```

要求：

- 当前阶段必须基于真实项目状态。
- 建议控制在 100 行以内；过长时归档旧历史到 `.ai/docs/changelog.md` 或 `.ai/docs/archive/`。
- 改动记录使用单行格式：

```text
- YYYY-MM-DD HH:MM | <model-name> | <一句话说明本次变更>
```

- 时间使用本地时区，精确到分钟。
- 描述不超过 80 字。
- 不确定内容标注 `<!-- 待确认 -->`。

### `AI_BOARD.md`

`AI_BOARD.md` 是唯一活跃协作看板。必须包含：

```md
# AI Collaboration Board

## 文件模型

## AI Identities

## Active Board

## Board Rules

## Activity Log Format

## Activity Log
```

Active Board 表格格式：

```md
| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
```

合法身份：

| Identity | Owner | Scope | Boundary |
| --- | --- | --- | --- |
| Frontend AI | `frontend-ai` | 前端 UI、Wails/IPC 调用封装、三语文案、视觉和交互。 | 不实现同步核心，不直接访问 RabbitMQ/MySQL/HTTP。 |
| Backend AI | `backend-ai` | 后端、服务、同步核心、配置、DTO、安装器、MCP。 | 前端可见 API/DTO/错误语义变化必须写入看板。 |
| Test AI | `test-ai` | 单元测试、smoke、E2E、lab、压测、发布门禁证据。 | 默认不改产品行为；修测试夹具必须说明范围。 |
| Review AI | `review-ai` | 架构审阅、上线评估、风险清单、跨模块微调。 | 先列风险和缺口；跨身份修改必须说明原因和影响。 |

Board Rules 必须包含：

1. 开工前先声明身份，再读 Active Board。
2. 新问题必须加到 Active Board，分配合法 `Owner`。
3. 解决后改为 `closed`，并追加 Activity Log。
4. 无法推进时改为 `blocked`，写清缺什么。
5. API、DTO、错误语义、页面范围变化，先更新 Active Board，再同步稳定契约文档。
6. `test-ai` 必须记录验证命令、通过/失败结果、未跑原因。
7. `review-ai` 必须优先列风险、缺口和阻塞。
8. 最终回复必须说明身份、处理的 Board 项、仍 open/blocked 的项。
9. 闭合历史过长时，由 `review-ai` 归档到 `.ai/docs/archive/` 或阶段总结。

Activity Log 格式：

```text
- YYYY-MM-DD HH:mm | <frontend-ai/backend-ai/test-ai/review-ai> | <question/decision/answer/blocker/review/test> | <影响范围> | <open/closed/blocked>
```

## 第四步：创建或维护 `.ai/` 母本文档体系

必须创建目录：

```text
.ai/
  instructions/
  docs/
    archive/
  skills/
  agents/
  prompts/
```

每个目录必须有 `README.md`，说明：

- 目录用途
- 文件命名规范
- 什么时候创建新文件
- 什么时候不要创建新文件

必须创建：

```text
.ai/instructions/ai-workflow.md
.ai/docs/README.md
.ai/skills/README.md
.ai/agents/README.md
.ai/prompts/README.md
```

按需创建：

```text
.ai/instructions/<domain>.md
.ai/docs/<topic>.md
.ai/skills/<skill-name>/SKILL.md
.ai/agents/<name>.agent.md
.ai/prompts/<name>.prompt.md
```

按需文件必须有项目证据支撑，不为凑结构创建空洞文件。

### `.ai/instructions/*.md`

必须有 frontmatter：

```yaml
---
description: "Use when: 触发关键词描述"
applyTo: "glob 模式"
---
```

要求：

- 一个文件只管一个关注点，如后端、前端、接口、测试、同步架构。
- 内容短、可执行。
- 不复制 `AGENTS.md` 的项目摘要。
- 必须说明与 `MEMORY.md`、`AI_BOARD.md` 的关系。
- 涉及前端、后端、测试、审阅协作时，必须要求读取根级 `AI_BOARD.md`。

### `.ai/docs/*.md`

用途：

- 稳定设计文档
- 架构说明
- 接口契约
- 阶段总结
- 闭合记录
- 归档材料

禁止：

- 不要在 `.ai/docs/` 中创建活跃看板。
- 不要在 `.ai/docs/` 中维护 open/blocked 工作项。
- 失效内容移入 `.ai/docs/archive/`。

### `.ai/skills/<skill-name>/SKILL.md`

必须有 frontmatter：

```yaml
---
name: skill-name
description: "Use when: 关键词丰富的触发描述"
argument-hint: "输入提示"
---
```

只有满足以下条件才创建 skill：

- 项目中出现 3 次以上重复操作。
- 输入输出明确。
- 步骤稳定可复现。
- 含项目特有约束。
- 未来同类任务能复用。

不要创建通用空泛技能，例如“读代码”“修 bug”。

### `.ai/agents/<name>.agent.md`

必须有 frontmatter：

```yaml
---
description: "Use when: 触发描述"
name: "agent-name"
tools: [read, search]
---
```

要求：

- 至少创建一个只读审查 Agent。
- 明确角色、职责、边界、不做什么。
- tools 使用最小必要集。
- 审查类 Agent 默认只读。
- 若创建与身份模型相关的 Agent，必须对应 `frontend-ai`、`backend-ai`、`test-ai` 或 `review-ai`。

### `.ai/prompts/<name>.prompt.md`

必须有 frontmatter：

```yaml
---
description: "一句话描述用途"
agent: "agent"
tools: [read, edit, search]
---
```

要求：

- 用 Markdown 链接引用相关 instructions / skills / agents。
- 为项目最常见的 3 到 5 个操作创建。
- 不为凑数创建低价值 prompt。
- 如果项目场景少，可以少于 3 个。
- 身份入口 prompt 应要求先声明身份并读取 `AI_BOARD.md`。

## 第五步：深度审阅流程

深度审阅流程默认不创建。仅当用户明确说“启用审阅流程”“加上深阅”“/review”“创建深度审阅 Agent”等触发语时，才增量创建：

```text
.ai/agents/deep-review.agent.md
.ai/prompts/deep-review.prompt.md
tests/README.md 或项目已有测试目录下的 ai-review 说明
MEMORY.md 中的 ## 审阅日志 段落
```

深度审阅 Agent 的最小 tools 为 `[read, search, edit, run]`，工作循环固定为：

1. 审阅：基于 `.ai/instructions/`、`AI_BOARD.md` 和最近 diff 输出问题清单。
2. 补测试：为未覆盖的关键路径补写测试，不写空测试。
3. 跑测试：执行项目测试命令并记录失败列表。
4. 修失败：最小化修复失败，禁止删除或跳过测试来制造通过。
5. 再审阅：回到审阅，直到连续两轮无新增问题且测试全部通过。

每轮结束必须向 `MEMORY.md` 的 `## 审阅日志` 追加：

```text
- YYYY-MM-DD HH:MM | <model-name> | round <N> | <一句话说明本轮变更与结论：通过 / 继续下一轮>
```

## 第六步：编辑器适配层

`.ai/` 是母本，编辑器适配层只做薄入口，不复制完整规范。

### Copilot

创建或维护：

```text
.github/copilot-instructions.md
.github/instructions/*.instructions.md
```

`.github/copilot-instructions.md`：

- 10 到 20 行。
- 只写核心入口。
- 必须要求先读 `AGENTS.md`。
- 必须提到 `MEMORY.md` 和 `AI_BOARD.md`。
- 不复制完整规范。

`.github/instructions/*.instructions.md`：

- 只为核心 `.ai/instructions/*.md` 创建 2 到 4 个适配。
- 必须有 frontmatter：

```yaml
---
applyTo: "glob 模式"
---
```

- 正文只写先读哪些母本文件，以及 2 到 4 条硬规则。

### Cursor

创建或维护：

```text
.cursor/rules/00-core.mdc
.cursor/rules/<domain>.mdc
```

`00-core.mdc`：

- `alwaysApply: true`
- `globs: ["**/*"]`
- 只写入口，不复制完整规范。

领域规则：

- `alwaysApply: false`
- `globs` 与对应 `.ai/instructions/*.md` 的 `applyTo` 一致。
- 只写母本链接和少量硬规则。

### Codex

Codex 原生读取根级 `AGENTS.md`，无需额外适配文件。

### Trae

Trae 默认复用 `AGENTS.md` 与 `.ai/` 母本。只有项目已有 Trae 原生规则目录，或用户明确要求时，才创建 Trae 薄入口。

Trae 适配规则：

- 若已有 Trae 原生规则目录，保留既有结构，只补母本链接。
- 若无法确认 Trae 规则路径，标注 `<!-- 待确认 -->`，不要凭空创建目录。
- Trae 适配层不得复制完整规范。
- Trae 适配层不得创建第二个活跃看板。

## 第七步：旧看板迁移

如果发现旧活跃看板位于：

```text
.ai/docs/ai-collaboration-log.md
```

则处理为：

- 将仍有效的 open/blocked 项迁移到根级 `AI_BOARD.md`。
- 旧文件只保留迁移提示。
- 明确写入：不要在旧文件新增 open/blocked 项。
- 所有路引都改为指向根级 `AI_BOARD.md`。

旧文件内容示例：

```md
# AI Collaboration Board moved

本文件已不再是活跃看板。活跃 AI 协作看板已迁移到仓库根目录：

AI_BOARD.md

规则：
- `AI_BOARD.md` 是唯一活跃看板。
- `.ai/docs/` 只保留稳定文档、闭合记录和归档材料。
- 不要在本文件新增 open/blocked 项。
```

## 第八步：安全写入规则

- 已存在文件优先合并，不盲目覆盖。
- 原内容与当前分析冲突时，保留原文并标注 `<!-- 待确认 -->`。
- 不确定内容必须显式标注 `<!-- 待确认 -->`。
- 若已有同类文档，新增文档应链接到现有文档，不重复大段描述。
- 不创建空业务文档。
- 不把构建产物、缓存、生成代码当作当前规范依据。
- 路径变化时同步所有引用。
- 适配层永远比母本短。
- 不创建与 `AI_BOARD.md` 并列竞争的活跃看板。

## 第九步：完成后自查

形式检查：

- `AGENTS.md` 存在且不超过 300 行。
- `MEMORY.md` 存在且有最后更新时间。
- `AI_BOARD.md` 存在且是唯一活跃看板。
- `.ai/` 目录结构完整。
- 每个 README 都说明用途和创建条件。
- instructions / skills / agents / prompts 都有正确 frontmatter。
- skill 的 `name` 与目录名一致。
- description 使用 `Use when: ...`。
- `.ai/docs/` 没有活跃看板。
- `.ai/docs/ai-collaboration-log.md` 若存在，只是迁移提示。
- Copilot / Cursor / Trae 适配层只做入口。
- 没有引用不存在的文件。

内容可信度检查：

- 每个 instruction 都有项目证据来源。
- 每个 skill 都对应真实高频流程。
- 每个 prompt 都引用实际存在的 instruction、skill 或 agent。
- 模块划分与实际目录结构和入口文件一致。
- 未确认内容均标注 `<!-- 待确认 -->`。
- 没有把构建产物、生成代码或历史遗留当作当前规范依据。

协作检查：

- Active Board 的 Owner 只使用合法身份。
- Active Board 的 open/blocked 项清晰。
- Activity Log 格式统一。
- 最终回复说明当前身份、处理的 Board 项、仍 open/blocked 的项。
- 实质变更已写入 `MEMORY.md`。
- 涉及协作、阻塞、API、DTO、错误语义或页面范围变化时，已更新 `AI_BOARD.md`。

深度审阅检查：

- 未触发深度审阅时，不创建 `deep-review.agent.md`、`deep-review.prompt.md` 或审阅测试目录。
- 触发深度审阅时，审阅 Agent、Prompt、测试说明和 `MEMORY.md ## 审阅日志` 同步完整。

## 完成输出

最后输出：

1. 判断的项目状态：全新 / 半初始化 / 已初始化。
2. 创建或更新的文件清单。
3. 当前 `AI_BOARD.md` 的 open/blocked 项。
4. 自查结果摘要：通过项和未通过项。
5. 待确认项汇总。
6. 后续建议。
