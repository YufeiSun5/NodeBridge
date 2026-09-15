const fs=require('node:fs'), assert=require('node:assert/strict');
const {messages}=require('../site/app.js');
const base='https://yufeisun5.github.io/NodeBridge/';
for(const [key,dir] of [['en','en'],['zh','zh-CN'],['ja','ja']]) {
  const html=fs.readFileSync(`site/${dir}/index.html`,'utf8');
  assert.ok(html.includes(`lang="${dir}" data-language="${key}"`));
  assert.ok(html.includes(`rel="canonical" href="${base+dir}/"`));
  assert.ok(html.includes(messages[key].searchTitle));
  assert.ok(html.includes(messages[key].mcpTitle));
  assert.ok(!html.includes('undefined'));
  for(const d of ['en','zh-CN','ja','x-default']) assert.ok(html.includes(`hreflang="${d}"`));
  for(const asset of ['app.js','style.css','mark.svg']) assert.ok(html.includes(`../${asset}`));
  assert.ok(fs.readFileSync('site/sitemap.xml','utf8').includes(base+dir+'/'));
}
console.log('Static language content, canonical, hreflang, assets and sitemap: PASS');
