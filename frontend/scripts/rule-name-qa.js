async (page) => {
  const cases = [];
  for (const [language, nav, label] of [['zh','规则','规则名称'],['en','Rules','Rule name'],['ja','ルール','ルール名']]) {
    for (const width of [1366,1024,390]) {
      await page.setViewportSize({width,height:width===390?844:640});
      await page.goto(`http://127.0.0.1:4971/tests/remediation.html?workspace=1&names=1&language=${language}&theme=light`);
      await page.getByRole('button',{name:nav,exact:true}).click();
      await page.getByRole('textbox',{name:label,exact:true}).waitFor();
      const style = await page.evaluate(() => {
        const name = document.querySelector('.rule-name-input');
        const id = document.querySelector('.rule-id-input');
        return {nameFont:parseFloat(getComputedStyle(name).fontSize),idFont:parseFloat(getComputedStyle(id).fontSize),idReadOnly:id.readOnly,overflow:document.documentElement.scrollWidth>innerWidth+1||document.querySelector('.content-area').scrollWidth>document.querySelector('.content-area').clientWidth+1};
      });
      if(style.nameFont<=style.idFont||!style.idReadOnly||style.overflow)throw Error(JSON.stringify({language,width,...style}));
      cases.push({language,width,...style});
      if(language==='zh')await page.screenshot({path:`output/playwright/rule-names-${width}.png`});
    }
  }
  await page.setViewportSize({width:1024,height:640});
  await page.goto('http://127.0.0.1:4971/tests/remediation.html?workspace=1&names=1&language=zh&theme=light');
  await page.getByRole('button',{name:'规则',exact:true}).click();
  const id=await page.locator('.rule-id-input').inputValue();
  await page.getByRole('textbox',{name:'规则名称',exact:true}).fill('主站与边缘检测标准');
  await page.getByRole('button',{name:'保存规则',exact:true}).click();
  await page.getByRole('button',{name:'刷新',exact:true}).click();
  if(await page.locator('.rule-name-input').inputValue()!=='主站与边缘检测标准'||await page.locator('.rule-id-input').inputValue()!==id)throw Error('name save changed ID or lost name');
  await page.locator('.rule-search input').fill('主站与边缘检测标准');
  if(await page.locator('.rule-choice').count()!==1)throw Error('name search failed');
  await page.screenshot({path:'output/playwright/rule-names-saved.png'});
  return {passed:true,cases,nameSaveAndSearch:true,idPreserved:true};
}
