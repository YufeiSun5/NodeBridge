import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const modulePath = process.env.NODEBRIDGE_PLAYWRIGHT_MODULE;
if (!modulePath) throw new Error('NODEBRIDGE_PLAYWRIGHT_MODULE is required');
const { chromium } = await import(pathToFileURL(modulePath).href);
const base = process.argv[2] || 'http://127.0.0.1:4971';
const output = new URL('../../output/playwright/business-remediation/', import.meta.url);
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const names = {
  zh: { check: '预检已保存规则', save: '保存规则', plan: '生成隔离计划', confirm: '确认隔离', done: '隔离完成，非业务应用成功', superseded: '已被较新版本取代', unchanged: '已处理，未覆盖获胜行', unknown: '已记录，结果未确认', unverified: '仲裁结果未确认' },
  en: { check: 'Check saved rule', save: 'Save Rules', plan: 'Plan quarantine', confirm: 'Confirm quarantine', done: 'Quarantined, not business-applied', superseded: 'Superseded by a newer version', unchanged: 'Processed without replacing the winner', unknown: 'Recorded, outcome unverified', unverified: 'Conflict outcome unverified' },
  ja: { check: '保存済みルールを検査', save: 'ルールを保存', plan: '隔離計画を作成', confirm: '隔離を確定', done: '隔離完了、業務適用ではない', superseded: '新しいバージョンを優先', unchanged: '処理済み、勝者の行は変更なし', unknown: '記録済み、結果は未確認', unverified: '競合結果は未確認' },
};
try {
  for (const [language, labels] of Object.entries(names)) {
    for (const [size, viewport] of Object.entries({ desktop: { width: 1366, height: 900 }, mobile: { width: 390, height: 844 } })) {
      const page = await browser.newPage({ viewport });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const theme = size === 'desktop' ? 'dark' : 'light';
      await page.goto(`${base}/tests/remediation.html?language=${language}&theme=${theme}`);
      await page.getByRole('button', { name: labels.check, exact: true }).click();
      await page.getByText('permission_delete', { exact: true }).waitFor();
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `rules overflow ${language}/${size}`);
      assert.equal(await page.locator('.content-area').evaluate((el) => el.scrollWidth > el.clientWidth + 1), false, `internal rules overflow ${language}/${size}`);
      assert.equal(await page.locator('.rule-section').evaluateAll((elements) => elements.some((el) => el.scrollWidth > el.clientWidth + 1)), false, `rule section overflow ${language}/${size}`);
      await page.screenshot({ path: new URL(`rules-${language}-${size}.png`, output).pathname.replace(/^\/([A-Z]:)/, '$1'), fullPage: true });
      await page.goto(`${base}/tests/remediation.html?language=${language}&page=failures&theme=${theme}`);
      await page.getByText('owned-event-9007199254740993', { exact: false }).waitFor();
      const superseded = page.getByRole('row').filter({ hasText: 'owned-superseded-event' });
      assert.ok((await superseded.textContent()).includes(labels.superseded));
      assert.ok((await superseded.textContent()).includes(labels.unchanged));
      const unverified = page.getByRole('row').filter({ hasText: 'owned-unverified-event' });
      assert.ok((await unverified.textContent()).includes(labels.unknown));
      assert.ok((await unverified.textContent()).includes(labels.unverified));
      await page.locator('.queue-quarantine input[placeholder="event_id"]').fill('owned-event-9007199254740993');
      await page.getByRole('button', { name: labels.plan, exact: true }).click();
      await page.getByRole('button', { name: labels.confirm, exact: true }).click();
      await page.getByRole('dialog').waitFor();
      await page.screenshot({ path: new URL(`quarantine-${language}-${size}.png`, output).pathname.replace(/^\/([A-Z]:)/, '$1'), fullPage: true });
      await page.getByRole('dialog').getByRole('button', { name: labels.confirm, exact: true }).click();
      await page.getByText(labels.done, { exact: true }).waitFor();
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `failures overflow ${language}/${size}`);
      assert.equal(await page.locator('.content-area').evaluate((el) => el.scrollWidth > el.clientWidth + 1), false, `internal failures overflow ${language}/${size}`);
      await page.screenshot({ path: new URL(`failures-${language}-${size}.png`, output).pathname.replace(/^\/([A-Z]:)/, '$1'), fullPage: true });
      assert.deepEqual(errors, [], `${language}/${size} errors`);
      await page.close();
    }
  }
  const page = await browser.newPage();
  await page.goto(`${base}/tests/remediation.html?language=zh&conflict=1`);
  await page.getByRole('button', { name: names.zh.save, exact: true }).click();
  await page.getByText('revision_conflict: reload before saving', { exact: true }).waitFor();
  await page.close();
  console.log('PASS: rules preflight, runtime failures despite empty ACK list, CAS rejection; zh/en/ja desktop/mobile screenshots and overflow checks');
} finally {
  await browser.close();
}
