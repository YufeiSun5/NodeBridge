---
description: "Use when: AI 协作文档更新、规范维护、MEMORY.md 记录、AGENTS.md 路引同步、Copilot Cursor Codex 适配"
applyTo: "**/*"
---

# AI Workflow

## 信息源优先级

1. 运行配置、构建脚本、CI 配置、当前源码结构。
2. 测试用例。
3. 根 README 和正式设计文档。
4. 其他历史文档。
5. 注释、issue、commit。

当前仓库尚无源码，本阶段文档依据用户提供的 V1.0 设计文档；源码出现后应优先以源码修正文档。

## 开始前

- 先读 `AGENTS.md` 和 `MEMORY.md`。
- 按任务类型读取 `.ai/instructions/` 中对应规范。
- 遇到文档与代码冲突时，以代码和构建配置为准，并在文档中标注 `<!-- 待确认 -->`。

## 完成后

- 发生架构、目录、接口、配置、构建、部署、测试策略变化时，必须同步检查 `.ai/` 母本文档。
- 有实质性变更时，向 `MEMORY.md` 的 `## 改动记录` 追加一条单行记录：

```text
- YYYY-MM-DD HH:MM | <model-name> | <一句话说明本次变更>
```

- `MEMORY.md` 超过 100 行时，把旧记录归档到 `.ai/docs/changelog.md`。

## 适配层规则

- `.ai/` 是母本，`.github/` 和 `.cursor/` 只做薄入口。
- 规范变化先改 `.ai/`，只有路径或文件名变化时才同步适配层引用。
- 适配层不得复制完整规范正文。

## 归档规则

- 失效 prompt、skill、doc 移入 `.ai/docs/archive/`。
- 不确定内容必须显式写 `<!-- 待确认 -->`，不要伪装成确定事实。
