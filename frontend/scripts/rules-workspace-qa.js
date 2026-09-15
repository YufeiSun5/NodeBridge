async (page) => {
  const errors=[]; page.on('pageerror', e=>errors.push(e.message));
  const nav={zh:'规则',en:'Rules',ja:'ルール'}; const results=[];
  for(const lang of ['zh','en','ja']) for(const [width,height] of [[1366,768],[1024,640],[800,600],[390,700]]) {
    await page.setViewportSize({width,height});
    await page.goto(`http://127.0.0.1:4971/tests/remediation.html?workspace=1&language=${lang}&theme=${width===1024?'light':'dark'}`);
    await page.getByRole('button',{name:nav[lang],exact:true}).click();
    await page.locator('.rule-id-input').waitFor();
    for(let section=0;section<4;section++) {
      await page.locator('.rule-editor-tabs button').nth(section).click();
      const visible=await page.locator('.rule-section:visible').count(); if(visible!==1)throw Error('multiple groups');
      const overflow=await page.evaluate(()=>({body:document.documentElement.scrollWidth>innerWidth+1,content:document.querySelector('.content-area').scrollWidth>document.querySelector('.content-area').clientWidth+1}));
      if(overflow.body||overflow.content)throw Error(`overflow ${lang}/${width}/${section}`);
      const bounds=await page.locator('.rule-card').boundingBox();
      results.push({lang,width,section,height:Math.round(bounds.height),bottom:Math.round(bounds.y+bounds.height)});
      if(width>=1024 && section===1)await page.screenshot({path:`output/playwright/rules-${lang}-${width}-routing.png`});
    }
    await page.locator('.rule-editor-tabs button').nth(0).click();
    await page.screenshot({path:`output/playwright/rules-${lang}-${width}.png`});
  }
  await page.setViewportSize({width:1024,height:640});
  await page.goto('http://127.0.0.1:4971/tests/remediation.html?workspace=1&language=zh&theme=light');
  await page.getByRole('button',{name:'规则',exact:true}).click();
  await page.locator('.rule-id-input').fill('edited-rule');
  await page.locator('.rule-choice').nth(1).click(); await page.locator('.rule-choice').nth(0).click();
  if(await page.locator('.rule-id-input').inputValue()!=='edited-rule')throw Error('draft lost');
  await page.locator('.rule-search input').fill('sys_detection_18');
  if(await page.locator('.rule-choice').count()!==1)throw Error('search mismatch');
  await page.locator('.rule-choice').click();
  await page.locator('.rule-search input').fill('missing-zzzz');
  if(await page.locator('.rule-choice').count()!==0)throw Error('search empty mismatch');
  await page.getByRole('button',{name:'新增规则',exact:true}).click();
  if(await page.locator('.rule-id-input').inputValue()!=='rule-19')throw Error('new not selected');
  await page.getByRole('button',{name:'删除',exact:true}).click();
  await page.getByRole('button',{name:'保存规则',exact:true}).click();
  await page.getByText('有未保存的修改',{exact:true}).waitFor({state:'hidden'});
  await page.locator('.rules-tools summary').click();
  await page.getByRole('button',{name:'预检已保存规则',exact:true}).click();
  await page.getByText('permission_delete',{exact:true}).waitFor();
  await page.goto('http://127.0.0.1:4971/tests/remediation.html?workspace=1&locked=1');
  await page.getByRole('button',{name:'规则',exact:true}).click();
  if(await page.locator('.readonly-rule-card').count()!==1)throw Error('readonly not compact');
  if(await page.locator('.rule-id-input').count())throw Error('locked editable');
  await page.goto('http://127.0.0.1:4971/tests/remediation.html?workspace=1&conflict=1');
  await page.getByRole('button',{name:'规则',exact:true}).click();
  await page.getByRole('button',{name:'保存规则',exact:true}).click();
  await page.getByText('revision_conflict: reload before saving',{exact:true}).waitFor();
  if(errors.length)throw Error(errors.join('\n'));
  return {passed:true,checks:results,drafts:true,search:true,addDelete:true,readonly:true,preflight:true,cas:true};
}
