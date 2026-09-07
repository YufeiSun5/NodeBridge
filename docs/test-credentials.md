# Test Credentials / 测试凭据 / テスト認証情報

This file is the single source of truth for NodeBridge lab-only passwords.
本文件是 NodeBridge 测试环境密码的唯一说明来源。
本ファイルは NodeBridge テスト環境専用パスワードの唯一の参照元です。

Do not use these values in customer, production, shared, or internet-facing environments.
不要在客户、生产、共享或公网环境使用这些值。
顧客、商用、共有、またはインターネット公開環境では使わないでください。

## Scope / 范围 / 範囲

- Applies only to Docker lab, longtest lab, and isolated installer VM validation.
- 仅适用于 Docker lab、长测 lab 和隔离安装器 VM 验证。
- Docker lab、長期テスト lab、隔離インストーラー VM 検証のみに適用します。

## Docker Lab / Docker 测试环境 / Docker テスト環境

| Purpose | User / Key | Password / Value |
| --- | --- | --- |
| MySQL root | `root` | `root_password` |
| MySQL sync user | `sync_user` | `sync_password` |
| RabbitMQ user | `sync` | `sync_password` |
| Canal MySQL user | `sync_user` | `sync_password` |
| Lab Log Web token | `log_web.token` | `lab_token` |
| Longtest Log Web token | `log_web.token` | `longtest_token` |
| Config admin password | `security.admin_password` | `admin-pass` |
| Config exit password | `security.exit_password` | `exit-pass` |

## Headless Installer Lab / 无界面安装器测试 / ヘッドレスインストーラー検証

| Purpose | User / Key | Password / Value |
| --- | --- | --- |
| Managed RabbitMQ bootstrap | NodeBridge-owned RabbitMQ user | `1234` |
| New-install config admin password | `security.admin_password` | `1234` |
| New-install config exit password | `security.exit_password` | `1234` |
| Hyper-V VM local admin | `Administrator` | `NodeBridge@2026!` |

## Rules / 规则 / ルール

- These are test-only values. Rotate by editing this file, configs, and scripts together.
- 这些值只用于测试。轮换时必须同时修改本文档、配置和脚本。
- これらはテスト専用値です。ローテーション時は本書、設定、スクリプトを同時に更新してください。
- Example files may use `encrypted_password` or `encrypted_token`; those are placeholders, not lab credentials.
- 示例文件中的 `encrypted_password` 或 `encrypted_token` 是占位符，不是测试凭据。
- サンプル内の `encrypted_password` や `encrypted_token` はプレースホルダーであり、テスト認証情報ではありません。
- Customer-owned RabbitMQ, MySQL, Canal, and Windows accounts must never be overwritten. Only NodeBridge-named managed RabbitMQ users are migrated.
- 客户自有 RabbitMQ、MySQL、Canal 和 Windows 账号绝不能被覆盖；只迁移 NodeBridge 命名的托管 RabbitMQ 用户。
- 顧客所有の RabbitMQ、MySQL、Canal、Windows アカウントは上書きせず、NodeBridge 名義の管理対象 RabbitMQ ユーザーだけを移行します。
- Installer upgrades preserve MySQL, Canal, node identity, and sync rules while migrating NodeBridge admin, exit, and managed RabbitMQ passwords to `1234`.
- 安装器升级保留 MySQL、Canal、节点身份和同步规则，同时把 NodeBridge 解锁、退出及托管 RabbitMQ 密码迁移为 `1234`。
- インストーラー更新では MySQL、Canal、ノード ID、同期ルールを維持し、NodeBridge の管理・終了・管理対象 RabbitMQ パスワードを `1234` に移行します。
