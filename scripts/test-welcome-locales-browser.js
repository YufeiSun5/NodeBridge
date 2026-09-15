async (page) => {
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 const base=page.url().replace(/\/$/,'')+'/';
 for(const [lang,dir] of [['en','en'],['zh','zh-CN'],['ja','ja']]) {
  await page.goto(base+dir+'/');
  await page.evaluate(()=>localStorage.setItem('NodeBridge.welcome.language','ja'));
  await page.reload();
  if(await page.locator('html').getAttribute('lang')!==dir)throw Error('explicit locale');
  for(const width of [320,1366]) {
   await page.setViewportSize({width,height:900});
   if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('overflow');
   await page.locator('[data-scenario=rules]').click();
   if(!(await page.locator('#mcp-outcome').innerText()))throw Error('MCP text');
  }
 }
 await page.locator('#language').selectOption('zh');
 await page.waitForURL('**/zh-CN/');
 if(await page.locator('html').getAttribute('lang')!=='zh-CN')throw Error('navigation');
 await page.locator('.search-summary').screenshot({path:'output/playwright/search-summary.png'});
 if(errors.length)throw Error(errors.join(','));
 return {localePages:3,widths:2,explicitLanguage:true,switchNavigation:true,errors};
}
