const fs = require('node:fs');
const path = require('node:path');
const {messages} = require('../site/app.js');
const root = path.join(__dirname, '../site');
const base = 'https://yufeisun5.github.io/NodeBridge/';
const locales = {
  en: {dir:'en',title:'NodeBridge — MySQL Bidirectional Sync & MCP | Windows',description:'MySQL data synchronization for edge nodes and central servers. Bidirectional mappings, automatic reconnection and MCP AI operations. Windows installer; Chinese, English and Japanese.',heading:'MySQL synchronization for Windows edge networks',body:'Synchronize MySQL between edge nodes and a central server with explicit bidirectional mappings. Install the Windows desktop application, recover connections automatically, and use MCP to inspect queues, manage rules and track initial alignment.'},
  zh: {dir:'zh-CN',title:'NodeBridge — MySQL 双向同步与 MCP 运维 | Windows',description:'面向边缘节点与中心服务器的 MySQL 数据同步软件，支持双向映射、断线自动重连和 MCP AI 运维。提供 Windows 直装版及中英日三语界面。',heading:'面向 Windows 边缘网络的 MySQL 双向同步软件',body:'在边缘节点与中心服务器之间同步 MySQL 数据，显式配置双向表列映射。提供 Windows 直装版、断线自动重连，以及通过 MCP 查队列、管理规则、跟踪首次对齐的 AI 运维能力。'},
  ja: {dir:'ja',title:'NodeBridge — MySQL 双方向同期と MCP 運用 | Windows',description:'エッジと中央サーバー向け MySQL データ同期。双方向マッピング、自動再接続、MCP による AI 運用を提供。Windows インストーラーと中英日 UI に対応。',heading:'Windows エッジ環境向け MySQL 双方向同期',body:'エッジノードと中央サーバーの MySQL データを、明示的な双方向マッピングで同期。Windows インストーラー、自動再接続に加え、MCP でキュー確認、ルール管理、初期整合の追跡を行えます。'}
};
const escape = s => s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/"/g,'&quot;');
let template = fs.readFileSync(path.join(root,'index.html'),'utf8');
template = template.replace(/\n?<!-- SEO START -->[\s\S]*?<!-- SEO END -->/g,'').replace(/\n?<!-- SEARCH CONTENT START -->[\s\S]*?<!-- SEARCH CONTENT END -->/g,'');
function render(lang, fixed) {
  const cfg = locales[lang], url = base + (fixed ? cfg.dir+'/' : '');
  let html = template.replace(/<title>.*?<\/title>/, '<title>'+escape(cfg.title)+'</title>').replace(/<meta name="description"[^>]*>/,'<meta name="description" content="'+escape(cfg.description)+'">');
  const alternate = Object.values(locales).map(l=>`<link rel="alternate" hreflang="${l.dir}" href="${base+l.dir}/">`).join('\n');
  html = html.replace('</head>', `<!-- SEO START -->\n<link rel="canonical" href="${url}">\n${alternate}\n<link rel="alternate" hreflang="x-default" href="${base}">\n<meta property="og:type" content="website">\n<meta property="og:title" content="${escape(cfg.title)}">\n<meta property="og:description" content="${escape(cfg.description)}">\n<meta property="og:url" content="${url}">\n<!-- SEO END -->\n</head>`);
  html = html.replace('</main>',`<!-- SEARCH CONTENT START -->\n<section class="wrap search-summary"><h2 data-text="searchTitle">${escape(messages[lang].searchTitle)}</h2><p data-text="searchBody">${escape(messages[lang].searchBody)}</p><nav aria-label="Languages"><a href="${base}en/" lang="en">English</a> · <a href="${base}zh-CN/" lang="zh-CN">简体中文</a> · <a href="${base}ja/" lang="ja">日本語</a></nav></section>\n<!-- SEARCH CONTENT END -->\n</main>`);
  if(fixed) {
    html = html.replace('<html lang="en">',`<html lang="${cfg.dir}" data-language="${lang}">`);
    html = html.replace(/(data-text="([^"]+)"[^>]*>)([^<]*)/g,(all,prefix,key)=>prefix+escape(messages[lang][key]));
    html = html.replace(/(id="mcp-prompt"[^>]*>)[^<]*/, '$1'+escape(messages[lang].mcpPromptDiagnose)).replace(/(id="mcp-outcome"[^>]*>)[^<]*/, '$1'+escape(messages[lang].mcpOutcomeDiagnose));
    html = html.replace(/(href|src)="(mark.svg|style.css|app.js)"/g,'$1="../$2"');
    html = html.replace(/href="https:\/\/github.com\/YufeiSun5\/NodeBridge\/blob\/main\/README.md"/g,'href="https://github.com/YufeiSun5/NodeBridge/blob/main/'+({en:'README.md',zh:'README.zh-CN.md',ja:'README.ja.md'})[lang]+'"');
  }
  return html;
}
fs.writeFileSync(path.join(root,'index.html'),render('en',false));
for(const [lang,cfg] of Object.entries(locales)) {fs.mkdirSync(path.join(root,cfg.dir),{recursive:true});fs.writeFileSync(path.join(root,cfg.dir,'index.html'),render(lang,true));}
fs.writeFileSync(path.join(root,'sitemap.xml'),'<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n'+['',...Object.values(locales).map(l=>l.dir+'/')].map(p=>`  <url><loc>${base+p}</loc></url>`).join('\n')+'\n</urlset>\n');
console.log('Generated three static language pages and sitemap.');
