# Welcome page / 欢迎页 / ようこそページ

`site/` is the standalone public website. It does not alter the Wails application or the release installer. GitHub Actions publishes only this directory through `.github/workflows/pages.yml`.

独立官网采用软件 tokens.css 的灰白/蓝/绿配色，并提供持久化深色切换和响应式布局。所有预览为明确标注的示例，不连接业务服务、不表示实时指标。

独立サイトはアプリと共通のグレー・青・緑を使用し、ダークモードにも対応します。プレビューはサンプルであり、業務サービスや実際の稼働状況を表示しません。

- Language: saved selection first, then browser preference order among English/Chinese/Japanese, English fallback. No external font or asset dependencies.
- Accessibility: skip link, labeled language selector, keyboard tabs (arrows/Home/End), visible focus, reduced-motion preference.
- Validation: `node scripts/test-welcome.cjs`; serve `site/` locally, then run `scripts/test-welcome-browser.js` with Playwright CLI `run-code --filename=...`. The browser check covers three languages, four widths (320/390/768/1366), three panels, selection persistence, keyboard operation and missing text.
- Deployment: pushes changing `site/**` on `main` trigger Pages. Release artifacts remain unchanged.

- MCP: three interactive example workflows (diagnosis, rules, alignment), capability summaries and valid local client JSON. Claims checked against internal/datasyncui/mcp.go; static examples never invoke node tools. 三语 README 同步突出 MCP。
- Expanded browser gate: 36 product states plus 72 MCP scenario/language/theme/width combinations; JSON parsing, theme persistence and keyboard navigation passed.

## Search discovery / 搜索发现 / 検索

- Generate with `node scripts/build-welcome-locales.cjs`; CI regenerates before publishing. `/en/`, `/zh-CN/`, `/ja/` contain translated HTML without JavaScript. Explicit locale URLs override browser/storage preference; root retains automatic selection.
- Each page has its own title, description, canonical, reciprocal hreflang and Open Graph metadata. `site/sitemap.xml` lists four URLs. Topics now describe MySQL sync, CDC, Windows, MCP and edge computing.
- Validate with `node scripts/test-welcome-seo.cjs` and `scripts/test-welcome-locales-browser.js` (start at site root).
- Sitemap: https://yufeisun5.github.io/NodeBridge/sitemap.xml . Search Console/Bing submission requires account access and site ownership verification; not submitted in this task. A project-path robots.txt would not control the host root, so none is added. Indexing/ranking is not guaranteed.
