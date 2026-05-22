---
description: "Use when: Wails2 管理端、React TypeScript 前端、frontend 页面、配置表单、状态总览、日志队列失败事件页面"
applyTo: "frontend/**,cmd/datasync-ui/**,**/*.ts,**/*.tsx,**/*.css"
---

# Frontend Wails Rules

## 设计风格

- 必须遵循 `.ai/docs/ui-design-spec.md` 的暗色工业终端风格。
- 字号默认不超过 `13px`，优先信息密度和可读性。
- 禁止 Tailwind、Bootstrap 等 CSS 框架。
- 禁止渐变背景、装饰图案和大幅动画。
- 样式 token 集中维护，不在页面里随意新增颜色。

## UI 职责

- Wails 管理端只负责配置、状态、日志、诊断和服务控制。
- 不在前端实现同步业务判断、冲突仲裁或回环抑制。
- 前端调用 Wails Go 后端接口获取状态和执行操作。
- 前端禁止使用 `fetch` / `axios` 访问后端；只通过 Wails binding 调 Go 方法。
- Wails 生产 UI 默认不占用端口；日志 Web 是独立、可选的远程诊断入口。
- MCP Server 开关只能调用 Wails 后端预留接口，默认关闭，不在前端启动 runtime。

## 协作看板

- 前端任务开始前必须读取 `.ai/docs/ai-collaboration-log.md` 的 Active Board。
- 前端发现接口缺口、DTO 疑问或需要后端处理的问题，写入 Active Board。
- 不新增单独的前端看板；稳定接口只查 `.ai/docs/frontend-backend-contract.md`。

## 页面优先级

- MVP 优先：总览、运行模式、MySQL 配置、RabbitMQ 配置、同步规则、队列状态、失败事件、日志查看。
- 第二阶段再扩展：冲突处理、节点审批、同步延迟统计、配置导入导出、完整诊断工具。

## 交互约束

- 配置保存前提供连接测试入口：MySQL、RabbitMQ、CDC。
- 危险操作需要明确状态反馈，例如停止服务、重启服务、忽略失败事件。
- 状态页面应展示最近同步时间、队列积压、失败数量、冲突数量、服务状态。

## TypeScript 约定

- DTO 字段名与 Go JSON 字段保持一致，避免前后端双重映射。
- API 调用集中在 `frontend/src/services/`。
- 页面组件放在 `frontend/src/pages/`，可复用组件放在 `frontend/src/components/`。
- 新增页面时同步检查路由、导航、空状态、错误状态和加载状态。
- React 只使用函数组件和 Hooks。
