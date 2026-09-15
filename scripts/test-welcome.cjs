const assert = require('node:assert/strict');
const {choose, messages} = require('../site/app.js');
for (const [saved, locales, expected] of [[null,['zh-CN'],'zh'],[null,['ja-JP'],'ja'],[null,['en-US'],'en'],[null,['fr-FR'],'en'],[null,[],'en'],['ja',['zh-CN'],'ja'],['invalid',['en-GB'],'en'],[null,['de-DE','ja-JP'],'ja'],[null,['zh-TW'],'zh']]) assert.equal(choose(saved,locales),expected);
for (const lang of ['zh','ja']) assert.deepEqual(Object.keys(messages[lang]).sort(),Object.keys(messages.en).sort());
console.log('Welcome language selection and translation parity: PASS');
