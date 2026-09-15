async (page) => {
  const errors=[]; page.on('pageerror', e=>errors.push(e.message));
  const results=[];
  for (const lang of ['en','zh','ja']) {
    await page.locator('#language').selectOption(lang);
    await page.reload();
    for (const width of [320,390,768,1366]) {
      await page.setViewportSize({width,height:900});
      for (const panel of ['overview','rules','recovery']) {
        await page.locator('#tab-'+panel).click();
        if(await page.locator('[role=tabpanel]:visible').count()!==1) throw Error('panel count');
        const state=await page.evaluate(()=>({lang:document.documentElement.lang,overflow:document.documentElement.scrollWidth>innerWidth,missing:[...document.querySelectorAll('[data-text]')].filter(x=>!x.textContent||x.textContent==='undefined').map(x=>x.dataset.text)}));
        if(state.overflow||state.missing.length||state.lang!==(lang==='zh'?'zh-CN':lang)) throw Error(JSON.stringify({width,panel,...state}));
        results.push({lang,width,panel});
      }
      await page.locator('#tab-overview').click();
      if(width===390||width===1366) await page.screenshot({path:`output/playwright/welcome-v2-${lang}-${width}.png`,fullPage:true});
    }
  }
  await page.locator('#tab-overview').focus();
  await page.keyboard.press('ArrowRight');
  if(await page.locator('#tab-rules').getAttribute('aria-selected')!=='true')throw Error('keyboard tab');
  await page.keyboard.press('End');
  if(await page.locator('#tab-recovery').getAttribute('aria-selected')!=='true')throw Error('keyboard end');
  if(errors.length)throw Error(JSON.stringify(errors));
  return {passed:results.length,keyboard:true,errors};
}
