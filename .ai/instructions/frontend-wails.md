---
description: "Use when: Wails2 管理端、React TypeScript 前端、frontend 页面、配置表单、状态总览、日志队列失败事件页面"
applyTo: "frontend/**,cmd/datasync-ui/**,**/*.ts,**/*.tsx,**/*.css"
---

# Frontend Wails Rules

## UI 职责

- Wails 管理端只负责配置、状态、日志、诊断和服务控制。
- 不在前端实现同步业务判断、冲突仲裁或回环抑制。
- 前端调用 Wails Go 后端接口获取状态和执行操作。

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
