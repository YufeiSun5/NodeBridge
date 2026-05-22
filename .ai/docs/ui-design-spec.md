# UI Design Spec / UI 设计规范 / UI デザイン仕様

> Scope / 范围 / 範囲: `frontend/src/**` React components and styles.
> Style / 风格 / スタイル: Dark industrial terminal. / 暗色工业终端。 / ダークな産業端末。
> Priority / 优先级 / 優先度: readability, density, operational clarity. / 可读、紧凑、运维清晰。 / 読みやすさ、密度、運用明快さ。

## Core Rules / 核心规则 / 基本ルール

- Wails IPC only. / 只走 Wails IPC。 / Wails IPC のみ。
- No `fetch`, no `axios`. / 禁止 `fetch` 和 `axios`。 / `fetch` と `axios` 禁止。
- No CSS framework. / 禁止 CSS 框架。 / CSS フレームワーク禁止。
- Function components + Hooks only. / 只用函数组件和 Hooks。 / 関数コンポーネントと Hooks のみ。
- No gradients, decorative patterns, or ornamental animation. / 禁止渐变、装饰图案和装饰动画。 / グラデーション、装飾模様、装飾アニメーションは禁止。
- Font size must stay at `13px` or below unless explicitly approved. / 字号不得超过 `13px`，除非明确批准。 / 明示承認なしに `13px` 超は禁止。

## Colors / 色彩 / 色

### Backgrounds / 背景 / 背景

| Usage | Color |
| --- | --- |
| App background | `#0d0d0d` |
| Title bar | `#080808` |
| Filter/status bar | `#0a0a0a` |
| Config banner | `#0b0f0b` |
| Modal | `#0f0f0f` |
| Input/card | `#111` |
| Database card | `#0d0d12` |

### Borders / 边框 / 境界線

| Usage | Color |
| --- | --- |
| Main divider | `#1a1a1a` |
| Normal border | `#222` |
| Input border | `#2a2a2a` |
| Card border | `#252530` |
| Inline divider | `#333` |

### Text / 文本 / テキスト

| Usage | Color |
| --- | --- |
| Title / important | `#e0e0e0` |
| Body / logs | `#ccc` |
| Secondary label | `#888` |
| Muted | `#666` |
| Placeholder | `#555` |
| Faint | `#444` |

### Module Colors / 模块色 / モジュール色

| Module | Color |
| --- | --- |
| Sync / SCADA | `#4caf50` |
| Node / Menu | `#2196f3` |
| HTTP | `#9c27b0` |
| App / Batch | `#ff9800` |
| DB / Failure | `#f44336` |
| Info | `#00bcd4` |

## Typography / 字体 / タイポグラフィ

```css
font-family: "Consolas", "JetBrains Mono", "Noto Sans SC", monospace;
```

| Scenario | Size |
| --- | --- |
| Badge | `10px` |
| Status bar / labels / buttons | `11px` |
| Inputs / modal text | `12px` |
| Log body | `12.5px` |
| Page or modal title | `13px` |

- Use bold for tags and section headers. / 标签和区段标题用粗体。 / タグとセクション見出しは太字。
- Section headers use `letter-spacing: 0.08em`. / 区段标题使用 `0.08em` 字距。 / セクション見出しは `0.08em`。
- Do not scale font size with viewport width. / 不按视口缩放字号。 / ビューポート幅で文字サイズを変えない。

## Layout / 布局 / レイアウト

Use a full-height flex column:

```text
Title bar          #080808 fixed
Config banner      #0b0f0b optional
Filter toolbar     #0a0a0a fixed
Content/log area   #0d0d0d flex:1 overflow:auto
Status bar         #0a0a0a fixed
```

- Fixed regions use `flex-shrink: 0`. / 固定区域使用 `flex-shrink: 0`。 / 固定領域は `flex-shrink: 0`。
- Scrollable regions use `flex: 1; overflow: auto`. / 滚动区域使用 `flex: 1; overflow: auto`。 / スクロール領域は `flex: 1; overflow: auto`。
- Page sections are unframed layouts; cards are for repeated items and modals only. / 页面区段不做大卡片；卡片只用于重复项和弹窗。 / ページ区画は大きなカード化しない。

## Components / 组件 / コンポーネント

### Buttons / 按钮 / ボタン

Base:

```css
border-radius: 4px;
padding: 5px 16px;
font-size: 12px;
border: 1px solid;
transition: opacity 0.15s;
```

| Type | Background | Border | Text |
| --- | --- | --- | --- |
| Primary | `#1a3a1a` | `#4caf50` | `#4caf50` |
| Secondary | `#111` | `#333` | `#666` |
| Danger | `#2a1a1a` | `#443` | `#f44336` |
| Tool | `#1a1a2a` | `#333` | `#90caf9` |
| Add | `#1a2a3a` | `#2196f3` | `#90caf9` |

Hover uses `opacity: 0.85`; active uses `opacity: 0.7`.

### Inputs / 输入 / 入力

```css
background: #111;
border: 1px solid #2a2a2a;
color: #ccc;
border-radius: 4px;
padding: 5px 8px;
font-size: 12px;
outline: none;
```

### Section Header / 区段标题 / セクション見出し

```css
font-size: 11px;
font-weight: bold;
color: var(--module-color);
letter-spacing: 0.08em;
border-bottom: 1px solid color-mix(in srgb, var(--module-color) 20%, transparent);
padding-bottom: 4px;
```

### Logs / 日志 / ログ

Log rows are dense and source-colored:

```text
[timestamp] [TAG] [connection] message [copy]
```

- Timestamp: `#555`, `11px`.
- Tag: module color, `10px`, bold, right aligned.
- Message: level color, `word-break: break-all`.
- Left border: module color with low opacity.
- Copy button: text-only, turns `#4caf50` on hover.

### Scrollbar / 滚动条 / スクロールバー

```css
::-webkit-scrollbar { width: 5px; }
::-webkit-scrollbar-track { background: #0d0d0d; }
::-webkit-scrollbar-thumb { background: #2a2a2a; border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: #3a3a3a; }
```

## Motion / 动效 / モーション

- Use only tiny transitions. / 只用轻微过渡。 / 小さな遷移のみ。
- Button opacity: `0.15s`.
- Toggle movement: `0.2s`.
- Copy color feedback: `0.2s`, reset after `1.2s`.
- No keyframes. / 禁止 keyframes。 / keyframes 禁止。

## Wails Integration / Wails 集成 / Wails 連携

- UI calls Go backend methods through generated Wails bindings. / UI 通过 Wails 生成绑定调用 Go 后端。 / UI は Wails 生成バインディングで Go を呼ぶ。
- Do not call SyncAgent HTTP APIs directly from React. / React 不直接调用 SyncAgent HTTP API。 / React から SyncAgent HTTP API を直接呼ばない。
- Log Web remains separate and opt-in for remote diagnostics. / 日志 Web 独立且按需开启。 / ログ Web は独立し任意有効。
