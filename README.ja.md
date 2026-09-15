# NodeBridge

[English](README.md) · [简体中文](README.zh-CN.md) · **日本語** · [ようこそ](https://yufeisun5.github.io/NodeBridge/)

Windows のエッジネットワーク向け MySQL 同期ソフトウェアです。Canal CDC、RabbitMQ、SyncAgent で中央サーバーと複数のエッジを接続し、Wails のローカル管理画面を提供します。

## ダウンロードと更新

[Windows x64 — v0.48.7 Beta](https://github.com/YufeiSun5/NodeBridge/releases/tag/v0.48.7)

RabbitMQ 切断後の自動復旧、部分コミット済みバッチの転送漏れ、双方向同期の宛先データベース選択を修正しました。コンパクトなルール編集、目立つ表示名、インストール時の NodeBridge システムデータベース更新も含みます。

NodeBridge、SyncAgent、Erlang/OTP、RabbitMQ、Java、Canal、WinSW を同梱します。**MySQL は別途用意してください。** 設定・ルール・データベースをバックアップし、利用する Windows アカウントで管理者として実行してください。移行失敗時はインストール完了を停止します。業務テーブルへの列追加は不要です。表示名の変更ではルール ID と整合記録を維持してください。

アプリ：`C:\Program Files\NodeBridge\app`。設定・実行データ：`C:\ProgramData\NodeBridge`。画面を閉じてもトレイに常駐し、終了には設定済みパスワードが必要です。

Beta 版です。隔離環境の 3 つの Windows Agent プロセスで CRUD、RabbitMQ サービス停止・通信停止後の復旧を検証しました。物理 PC 3 台での性能や本番 SLA を保証するものではありません。[検証記録（中国語）](docs/v0.48.7-reconnect-handoff-20260915.md)。

## 運用

各ノードに `server-001`、`edge-001` など固有の `node.id` を設定し、データベース・テーブル・列の対応を明示します。書き込みコミット後に ACK し、配送結果が不明な場合はイベントの冪等性を保って再試行します。

- [LAN 配備ガイド（中国語）](docs/lan-deployment-guide.md)
- [初期整合](docs/initial-alignment.md)
- [ルーティング](docs/sync-routing-policy.md)
- [管理対象コンポーネント](docs/managed-components.md)
- [試用手順](docs/trial-runbook.md)
- [MCP 管理](docs/mcp-service.md)：SSH stdio で `SyncAgent.exe mcp-stdio` を起動。HTTP ポートは使用しません。

## 開発検証

```powershell
go test ./...
go vet ./...
golangci-lint run ./...
cd frontend
npm ci
npm test
npm run build
```

ようこそページはブラウザー言語に合わせ、中国語・日本語以外は英語を使用します。手動選択は保存されます。GitHub README は上部リンクから切り替えてください。

ライセンス：[MIT](LICENSE)。
