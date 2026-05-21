---
description: "Use when: 查看 NodeBridge/DataSync 产品定位、MVP 范围、模块边界、同步架构设计摘要"
---

# Product Design Brief

## 定位

本项目规划为独立数据同步产品，用于多个边缘节点和一个中心节点之间同步 MySQL 数据。它不是 SCADA 程序，也不是中心 Web 系统。

依据：用户提供的《数据同步软件详细设计文档 V1.0》。

## 目标链路

```text
MySQL -> CDC -> Event -> RabbitMQ -> Apply -> MySQL
```

## 部署模式

- Edge Mode：部署在现场工控机、边缘服务器或产线节点。
- Server Mode：部署在中心服务器，负责仲裁、中心写入和下发。

## 推荐进程

- `DataSync.exe`: Wails2 管理端。
- `SyncAgent.exe`: Go 后台服务，长期运行同步核心。

## MVP

- 1 个 Server。
- 2 个 Edge。
- 1 张单向表：`alarm_history`，Edge -> Server。
- 1 张多向表：`device_config`，Edge <-> Server <-> Edge。

## 第一阶段必须落地

- Wails2 管理端基础框架。
- SyncAgent 独立服务。
- Edge / Server 模式切换。
- MySQL 和 RabbitMQ 连接测试。
- Canal CDC 读取。
- 标准 `SyncEvent` 生成。
- Edge -> Server 单向表同步。
- Server -> Edge 下发通道。
- 两张多向表同步。
- `last_event_id` + `sync_apply_log` 回环抑制。
- 队列积压、日志和失败事件查看。

## 最关键架构决策

- CDC 不负责回环抑制。
- RabbitMQ 不负责业务语义判断。
- Go SyncAgent 是同步大脑。
- Server 负责中心仲裁和不分发回源节点。
- Edge 本地必须识别同步回放并阻止再次上传。
- Wails 管理端默认不占用端口，通过 Wails binding 调用 Go 后端。
- 日志模块可单独启用轻量 HTTP 服务，用于远程诊断。

## 路线图

详细版本计划见 `.ai/docs/roadmap.md`。

## 待确认

- 项目对外产品名是否使用 `DataSync`，仓库名继续使用 `NodeBridge`。
- Wails 模板和前端 UI 组件库。
- Canal Go client、日志库、配置库、Windows Service 库。
