(function () {
  const messages = {
    en: {title:'Keep your edge data connected.',intro:'MySQL synchronization between a central server and multiple edge nodes, with local management and durable message queues.',download:'Download for Windows',guide:'Read the guide',recovery:'Automatic recovery',recoveryBody:'Rebuilds failed RabbitMQ connections so synchronization can resume without restarting the Agent.',rules:'Clearer rules',rulesBody:'Compact editing and prominent display names, while keeping stable rule IDs and alignment records.',upgrade:'Built-in upgrades',upgradeBody:'Updates configured NodeBridge system databases during installation; migration failures block completion.',start:'Before you start',setup:'Prepare MySQL separately. Back up configuration, rules and databases, then run the installer as administrator. This beta has isolated three-Agent recovery coverage; on-site validation remains necessary.',footer:'Open source under the MIT license.',language:'Language'},
    zh: {title:'让中心与边缘数据保持连接。',intro:'在中心服务器与多个边缘节点之间同步 MySQL 数据，提供本地管理界面与持久消息队列。',download:'下载 Windows 安装包',guide:'阅读使用指南',recovery:'断线自动恢复',recoveryBody:'重建失效的 RabbitMQ 连接，无需重启 Agent 即可恢复同步。',rules:'规则更清晰',rulesBody:'紧凑编辑、突出显示名称，同时保留稳定规则 ID 和已有对齐记录。',upgrade:'安装时自动升级',upgradeBody:'更新已配置的 NodeBridge 系统数据库；迁移失败时阻止安装完成。',start:'开始之前',setup:'请单独准备 MySQL。备份配置、规则和数据库后，以管理员身份运行安装包。此 Beta 已完成隔离三 Agent 恢复验证，仍需现场验收。',footer:'基于 MIT 许可证开源。',language:'语言'},
    ja: {title:'中央とエッジのデータをつなぐ。',intro:'中央サーバーと複数のエッジ間で MySQL を同期。ローカル管理画面と永続メッセージキューを提供します。',download:'Windows 版をダウンロード',guide:'ガイドを読む',recovery:'切断から自動復旧',recoveryBody:'失効した RabbitMQ 接続を再構築し、Agent を再起動せずに同期を再開します。',rules:'見やすいルール',rulesBody:'コンパクトな編集と目立つ表示名。安定したルール ID と整合記録を維持します。',upgrade:'インストール時に更新',upgradeBody:'設定済みの NodeBridge システムデータベースを更新し、移行失敗時は完了を停止します。',start:'開始前の準備',setup:'MySQL は別途用意してください。設定・ルール・データベースをバックアップし、管理者として実行します。隔離環境の 3 Agent で復旧を検証済みですが、現場での検証も必要です。',footer:'MIT ライセンスのオープンソース。',language:'言語'}
  };
  function choose(saved, languages) {
    if (Object.hasOwn(messages, saved)) return saved;
    for (const value of languages || []) {
      const base = String(value).toLowerCase().split(/[-_]/)[0];
      if (Object.hasOwn(messages, base)) return base;
    }
    return 'en';
  }
  if (typeof module !== 'undefined') module.exports = {choose, messages};
  if (typeof document === 'undefined') return;
  const key = 'NodeBridge.welcome.language';
  let saved; try { saved = localStorage.getItem(key); } catch {}
  function render(lang) {
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : lang;
    document.querySelectorAll('[data-text]').forEach(el => { el.textContent = messages[lang][el.dataset.text]; });
    document.getElementById('languageLabel').textContent = messages[lang].language;
    document.getElementById('language').value = lang;
    document.getElementById('readme').href = 'https://github.com/YufeiSun5/NodeBridge/blob/main/' + ({en:'README.md',zh:'README.zh-CN.md',ja:'README.ja.md'})[lang];
  }
  render(choose(saved, navigator.languages || [navigator.language]));
  document.getElementById('language').addEventListener('change', event => {
    const lang = choose(event.target.value, []);
    try { localStorage.setItem(key, lang); } catch {}
    render(lang);
  });
})();
