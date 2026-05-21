# .ai/prompts

本目录存放项目常用 Prompt 模板。

## 格式要求

- 每个模板必须包含 YAML frontmatter。
- 正文必须链接相关 instruction、agent 或 skill。
- Prompt 应描述输入、输出和关键约束。

## 引用方式

在对话中说明要使用对应模板，例如：

- “使用 `implement-sync-module.prompt.md` 规划并实现 RabbitMQ producer。”
- “使用 `architecture-review.prompt.md` 审查当前同步链路。”

## 创建条件

- 任务在本项目中会反复出现。
- 模板能明确减少上下文重复。
- 模板引用的规范文件已经存在。
