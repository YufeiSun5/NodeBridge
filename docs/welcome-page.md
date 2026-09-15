# Welcome page / 欢迎页 / ようこそページ

`site/` is the standalone public website. It does not alter the Wails application or the release installer. GitHub Actions publishes only this directory through `.github/workflows/pages.yml`.

独立官网采用浅色背景、深绿产品预览和响应式布局；不属于 Wails 管理端视觉规范范围。所有预览为明确标注的示例，不连接业务服务、不表示实时指标。

独立サイトはライト背景とダークグリーンの製品プレビューを使用します。プレビューはサンプルであり、業務サービスや実際の稼働状況を表示しません。

- Language: saved selection first, then browser preference order among English/Chinese/Japanese, English fallback. No external font or asset dependencies.
- Accessibility: skip link, labeled language selector, keyboard tabs (arrows/Home/End), visible focus, reduced-motion preference.
- Validation: `node scripts/test-welcome.cjs`; serve `site/` locally, then run `scripts/test-welcome-browser.js` with Playwright CLI `run-code --filename=...`. The browser check covers three languages, four widths (320/390/768/1366), three panels, selection persistence, keyboard operation and missing text.
- Deployment: pushes changing `site/**` on `main` trigger Pages. Release artifacts remain unchanged.
