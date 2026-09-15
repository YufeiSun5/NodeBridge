(function () {
  const messages = {
    en: {
      skip:'Skip to content', navProduct:'Product', navDeploy:'Get started', release:'Built to reconnect',
      title:'Your data.', titleAccent:'Closer together.', intro:'From the factory floor to your central server. Keep MySQL data moving across Windows edge networks, with control that stays in your hands.',
      download:'Download for Windows', guide:'Explore the docs', localFirst:'Local control', previewLabel:'A closer look', demo:'Illustrative preview',
      tabOverview:'Overview', tabRules:'Rules', tabRecovery:'Recovery', network:'YOUR SYNC NETWORK', oneView:'Every node. One view.', connected:'Connected', central:'Central server', edgeNetwork:'EDGE NETWORK', edgeOne:'Production line A', edgeTwo:'Production line B', syncing:'Syncing', direction:'Bidirectional rules. Explicit destinations.',
      ruleWorkspace:'RULE WORKSPACE', rulePreviewTitle:'Names that make sense.', enabled:'Enabled', ruleName:'Production events', source:'Source', destination:'Destination', key:'Primary key', mode:'Direction', bidirectional:'Bidirectional', stableIdentity:'A new display name. The same stable identity.',
      recoveryWorkspace:'CONNECTION RECOVERY', recoveryPreviewTitle:'Ready to pick up again.', disconnect:'Connection interrupted', disconnectBody:'Keep unacknowledged messages available for retry.', reconnect:'Rebuild the connection', reconnectBody:'Replace the failed channel and publisher.', resume:'Resume synchronization', resumeBody:'Retry with event idempotency after connectivity returns.', noRestart:'No manual Agent restart for this recovery path.', demoFooter:'Sample configuration · not live data', tryTabs:'Explore the tabs above ↑',
      builtWith:'Built on a familiar foundation', sectionLabel:'DESIGNED FOR THE EDGE', featureTitle:'Complex networks.', featureAccent:'A clearer way to manage them.', featureIntro:'Practical tools for the moments that matter: configuring a rule, losing a connection, rolling out an update.',
      recovery:'Reconnect. Resume.', recoveryBody:'Failed RabbitMQ connections are rebuilt automatically, so the Agent can resume synchronization when connectivity returns.', recoveryTag:'Automatic reconnection',
      rules:'Less scrolling. More context.', rulesBody:'Compact rule editing and prominent display names keep configuration readable, without replacing IDs or existing alignment records.', rulesTag:'Human-readable rule names',
      upgrade:'Move forward, together.', upgradeBody:'The installer updates configured NodeBridge system databases. If a migration fails, installation completion is blocked.', upgradeTag:'Built-in system migrations',
      deployLabel:'FROM DOWNLOAD TO DEPLOYMENT', start:'Your network. Your next step.', deployIntro:'Start with the guide. Keep the configuration explicit. Validate in your own environment.', releaseNotes:'Read release notes',
      prepareTitle:'Prepare your environment', prepareBody:'Install MySQL separately, assign unique node IDs, and back up configuration, rules and databases.', installTitle:'Install and configure', installBody:'Run the Windows installer as administrator. Define your source and target databases, tables and columns.', validateTitle:'Validate your deployment', validateBody:'Check alignment, CRUD and recovery in your environment. This beta has isolated three-Agent coverage, not a production SLA.',
      closingTitle:'Bring your data together.', closingBody:'Open source. Windows native. Ready for your next test.', footer:'Made for connected operations. Open source under MIT.', language:'Language'
    },
    zh: {
      skip:'跳转到正文', navProduct:'产品概览', navDeploy:'开始使用', release:'断线之后，自动接续',
      title:'让数据流动，', titleAccent:'让协作更近。', intro:'从生产现场到中心服务器，让 MySQL 数据在 Windows 边缘网络中持续流转。连接由软件处理，掌控始终在你手中。',
      download:'下载 Windows 版', guide:'查看使用文档', localFirst:'本地掌控', previewLabel:'走近 NodeBridge', demo:'交互示意预览',
      tabOverview:'同步总览', tabRules:'规则配置', tabRecovery:'断线恢复', network:'你的同步网络', oneView:'每个节点，一目了然。', connected:'已连接', central:'中心服务器', edgeNetwork:'边缘网络', edgeOne:'生产线 A', edgeTwo:'生产线 B', syncing:'同步中', direction:'双向同步规则，明确每一个目标。',
      ruleWorkspace:'规则工作区', rulePreviewTitle:'看得懂的名字，好管理的规则。', enabled:'已启用', ruleName:'生产事件同步', source:'源表', destination:'目标表', key:'主键', mode:'同步方向', bidirectional:'双向同步', stableIdentity:'显示名称可以改变，规则身份始终保留。',
      recoveryWorkspace:'连接恢复', recoveryPreviewTitle:'连接回来，继续向前。', disconnect:'连接暂时中断', disconnectBody:'未确认消息保留，等待重新投递。', reconnect:'重建失效连接', reconnectBody:'替换旧通道与发布器，重新建立连接。', resume:'恢复数据同步', resumeBody:'网络恢复后，按事件幂等语义重试。', noRestart:'此恢复路径无需手动重启 Agent。', demoFooter:'示例配置 · 非实时运行数据', tryTabs:'点击上方标签，切换预览 ↑',
      builtWith:'建立在成熟技术之上', sectionLabel:'为边缘网络而设计', featureTitle:'复杂的网络，', featureAccent:'也能有清晰的管理方式。', featureIntro:'配置一条规则、应对一次断线、完成一次升级。把日常运维中重要的事情做好。',
      recovery:'连接回来，同步接续。', recoveryBody:'自动重建失效的 RabbitMQ 连接，网络恢复后，Agent 可以重新接续数据同步。', recoveryTag:'自动重连与恢复',
      rules:'少些滚动，多些清晰。', rulesBody:'紧凑的规则工作区，让显示名称更加醒目。调整名称时，保留规则 ID 和已有对齐记录。', rulesTag:'一眼读懂规则名称',
      upgrade:'软件与系统库，一起更新。', upgradeBody:'安装时自动升级已配置的 NodeBridge 系统数据库。迁移失败时阻止安装完成，明确呈现升级结果。', upgradeTag:'内置系统数据库迁移',
      deployLabel:'从下载，到部署', start:'你的网络，下一步由你开启。', deployIntro:'从文档出发，明确每一项配置，再在自己的环境中完成验证。', releaseNotes:'查看发行说明',
      prepareTitle:'准备运行环境', prepareBody:'单独准备 MySQL，为每个节点设置唯一 ID，并备份配置、规则和数据库。', installTitle:'安装并完成配置', installBody:'以管理员身份运行 Windows 安装包，明确源与目标数据库、表和列的映射。', validateTitle:'验证你的部署', validateBody:'在现场验证对齐、增删改与断线恢复。当前 Beta 已覆盖隔离三 Agent 测试，不代表生产 SLA。',
      closingTitle:'让分散的数据，重新连接。', closingBody:'开源、Windows 原生，为下一次验证做好准备。', footer:'为连接而生 · 基于 MIT 许可证开源', language:'语言'
    },
    ja: {
      skip:'本文へ移動', navProduct:'製品概要', navDeploy:'はじめる', release:'切断後も、自動で再接続',
      title:'データをつなぐ。', titleAccent:'現場が近づく。', intro:'生産現場から中央サーバーへ。Windows エッジネットワークで MySQL データを同期。管理の主導権は、いつもあなたの手に。',
      download:'Windows 版をダウンロード', guide:'ドキュメント', localFirst:'ローカル管理', previewLabel:'NodeBridge を体験', demo:'操作イメージ',
      tabOverview:'同期の概要', tabRules:'ルール', tabRecovery:'接続復旧', network:'同期ネットワーク', oneView:'すべてのノードを、一目で。', connected:'接続済み', central:'中央サーバー', edgeNetwork:'エッジネットワーク', edgeOne:'生産ライン A', edgeTwo:'生産ライン B', syncing:'同期中', direction:'双方向ルールで、宛先を明確に。',
      ruleWorkspace:'ルール設定', rulePreviewTitle:'わかる名前で、迷わず管理。', enabled:'有効', ruleName:'生産イベント同期', source:'ソース', destination:'宛先', key:'主キー', mode:'同期方向', bidirectional:'双方向', stableIdentity:'表示名を変えても、ルール ID はそのまま。',
      recoveryWorkspace:'接続の復旧', recoveryPreviewTitle:'つながれば、また進める。', disconnect:'接続が中断', disconnectBody:'未確認メッセージを再試行のために保持。', reconnect:'接続を再構築', reconnectBody:'失効したチャネルと発行器を置き換えます。', resume:'同期を再開', resumeBody:'接続復旧後、イベントの冪等性を保ち再試行。', noRestart:'この復旧経路では Agent の手動再起動は不要。', demoFooter:'設定例 · 実際の稼働データではありません', tryTabs:'上のタブで表示を切り替え ↑',
      builtWith:'実績ある技術を基盤に', sectionLabel:'エッジネットワークのために', featureTitle:'複雑なネットワークを、', featureAccent:'もっと明快に管理。', featureIntro:'ルール設定、接続の中断、アップグレード。日々の運用で大切な場面を支えるツールです。',
      recovery:'再接続して、その先へ。', recoveryBody:'失効した RabbitMQ 接続を自動再構築。接続が戻れば、Agent はデータ同期を再開できます。', recoveryTag:'自動再接続と復旧',
      rules:'スクロールを減らし、明快に。', rulesBody:'コンパクトな編集画面と目立つ表示名。名前を変えても、ルール ID と既存の整合記録を維持します。', rulesTag:'わかりやすいルール名',
      upgrade:'ソフトもシステム DB も更新。', upgradeBody:'設定済みの NodeBridge システム DB をインストール時に更新。移行に失敗すると完了を停止します。', upgradeTag:'システム DB 移行を内蔵',
      deployLabel:'ダウンロードから導入まで', start:'あなたのネットワークで、次の一歩を。', deployIntro:'まずガイドを確認。設定を明確にし、ご自身の環境で検証してください。', releaseNotes:'リリースノート',
      prepareTitle:'環境を準備', prepareBody:'MySQL を別途用意し、固有のノード ID を設定。設定・ルール・DB をバックアップします。', installTitle:'インストールと設定', installBody:'Windows インストーラーを管理者として実行し、ソースと宛先の DB・テーブル・列を指定します。', validateTitle:'導入環境で検証', validateBody:'整合、CRUD、接続復旧を現場で確認。隔離 3 Agent の Beta 検証は本番 SLA を保証しません。',
      closingTitle:'離れたデータを、ひとつに。', closingBody:'オープンソース。Windows ネイティブ。次の検証へ。', footer:'現場をつなぐために。MIT ライセンスで公開。', language:'言語'
    }
  };
  Object.assign(messages.en, {"theme":"Dark mode","mcpTitle":"Your AI assistant.","mcpAccent":"Connected to your operations.","mcpIntro":"Give your MCP client structured access to NodeBridge. Inspect a queue, review a rule, or track an alignment task using the same management capabilities as the application.","mcpChoose":"What would you ask?","mcpDiagnose":"Find the sync bottleneck","mcpDiagnoseSub":"Status · queues · logs · events","mcpRules":"Review a sync rule","mcpRulesSub":"Schema · mappings · revisions","mcpAlignment":"Track initial alignment","mcpAlignmentSub":"Start · poll · reconcile","mcpLocalNote":"Local stdio or remote SSH. Your client stays in control of the conversation.","mcpExample":"Example workflow","mcpPromptLabel":"YOU ASK","mcpToolLabel":"TOOLS THE CLIENT CAN CALL","mcpDemoNote":"Illustration only. This page does not connect to your nodes.","mcpInspect":"See what is happening","mcpInspectBody":"Read overview, schema, queue depth, bounded logs and event receipts. Export a diagnostic archive for investigation.","mcpManage":"Move from insight to action","mcpManageBody":"Validate and save configuration, manage rules, control the Agent and retry failed events. Data mutations use structured plans and explicit confirmation.","mcpConnect":"Work across your nodes","mcpConnectBody":"Add one client entry per Windows node. Use local stdio or SSH from Windows or Mac, with the configuration-owning account.","mcpSetupLabel":"CONNECT YOUR MCP CLIENT","mcpSetupTitle":"Start with a local connection.","mcpSetupBody":"Save a complete configuration and enable MCP in NodeBridge. Run the client as the Windows account that owns the configuration. Adapt command and args to your client’s format.","mcpReference":"MCP tool reference","mcpRemote":"Remote SSH setup","mcpConfigExample":"Common client configuration","mcpBoundaryLabel":"Built-in boundaries.","mcpBoundary":"No MCP HTTP listener, arbitrary shell or raw SQL. Configuration validation, redacted output and write auditing remain in place. Initial alignment requires stopped Agents on every participant, explicit confirmation and status polling; running is not completion.","mcpPromptDiagnose":"Why has edge synchronization stopped progressing?","mcpOutcomeDiagnose":"Correlate process status with queue depth and event evidence. A running process alone does not prove healthy synchronization.","mcpPromptRules":"Review my table mapping before saving a rule change.","mcpOutcomeRules":"Read the saved revision, inspect table structure and preflight the local rule. Saving requires expected_revision; check active_revision afterward instead of assuming hot reload.","mcpPromptAlignment":"Start the approved initial alignment and track its progress.","mcpOutcomeAlignment":"Stop Agents on every participant and confirm the operation. Keep every MCP session open and poll until the complete group is ready. This does not start synchronization automatically."});
  Object.assign(messages.zh, {"theme":"深色模式","mcpTitle":"让 AI 助手，","mcpAccent":"真正接入同步运维。","mcpIntro":"通过 MCP，让 AI 客户端调用 NodeBridge 的结构化管理能力。查队列、核规则、跟踪对齐进度，把对话落到具体的工具与执行证据上。","mcpChoose":"你想让 AI 协助什么？","mcpDiagnose":"定位同步为什么卡住","mcpDiagnoseSub":"状态 · 队列 · 日志 · 事件","mcpRules":"核对一条同步规则","mcpRulesSub":"表结构 · 映射 · 配置版本","mcpAlignment":"跟踪首次全量对齐","mcpAlignmentSub":"发起 · 轮询 · 核对恢复","mcpLocalNote":"支持本机 stdio 与远程 SSH，使用你自己的 MCP 客户端进行交互。","mcpExample":"示例工作流","mcpPromptLabel":"你可以这样问","mcpToolLabel":"客户端可调用的工具","mcpDemoNote":"仅为能力示意，此页面不连接你的节点。","mcpInspect":"看清发生了什么","mcpInspectBody":"读取概览、表结构、队列深度、有限日志和事件收据，导出诊断包，沿着证据排查问题。","mcpManage":"从定位到处理","mcpManageBody":"校验并保存配置、管理规则、控制 Agent、重试失败事件。数据修改使用结构化计划和显式确认。","mcpConnect":"管理多个节点","mcpConnectBody":"每个 Windows 节点独立配置一个客户端条目。本机走 stdio，Windows 或 Mac 可通过 SSH 使用配置所属账户远程连接。","mcpSetupLabel":"接入你的 MCP 客户端","mcpSetupTitle":"先从本机连接开始。","mcpSetupBody":"在 NodeBridge 中保存完整配置并开启 MCP。使用配置所属的 Windows 账户运行客户端，按客户端要求调整 command 和 args 的配置格式。","mcpReference":"查看 MCP 工具文档","mcpRemote":"远程 SSH 接入指南","mcpConfigExample":"常见客户端配置格式","mcpBoundaryLabel":"明确的操作边界。","mcpBoundary":"MCP 不监听 HTTP 端口，不提供任意 Shell 或原始 SQL。保留配置校验、输出脱敏和写操作审计。首次对齐需所有参与端停止 Agent、显式确认并持续轮询；running 不代表完成。","mcpPromptDiagnose":"帮我检查边缘节点为什么不再同步，先查看状态和日志。","mcpOutcomeDiagnose":"结合进程状态、队列深度与事件证据定位问题。进程正在运行，不等于同步链路健康。","mcpPromptRules":"保存修改前，帮我核对这条规则的表结构和映射。","mcpOutcomeRules":"先读取已保存版本、检查表结构，再进行本机规则预检。保存需携带 expected_revision，之后核对 active_revision，不能假定已热加载。","mcpPromptAlignment":"执行已授权的首次对齐，并持续跟踪进度。","mcpOutcomeAlignment":"所有参与端先停 Agent 并确认操作。保持各端 MCP 会话并轮询，直到整组就绪；完成对齐不会自动启动同步。"});
  Object.assign(messages.ja, {"theme":"ダークモード","mcpTitle":"AI アシスタントを、","mcpAccent":"同期運用につなぐ。","mcpIntro":"MCP クライアントから NodeBridge の構造化された管理機能へ。キューの確認、ルールの検証、整合タスクの追跡を、具体的なツールと実行結果で支援します。","mcpChoose":"AI に何を頼みますか？","mcpDiagnose":"同期の停滞を調べる","mcpDiagnoseSub":"状態 · キュー · ログ · イベント","mcpRules":"同期ルールを確認","mcpRulesSub":"スキーマ · マッピング · リビジョン","mcpAlignment":"初期整合を追跡","mcpAlignmentSub":"開始 · ポーリング · 復旧確認","mcpLocalNote":"ローカル stdio または SSH で接続。ご自身の MCP クライアントで操作します。","mcpExample":"ワークフローの例","mcpPromptLabel":"依頼の例","mcpToolLabel":"クライアントが呼び出せるツール","mcpDemoNote":"機能のイメージです。このページはノードに接続しません。","mcpInspect":"何が起きたかを把握","mcpInspectBody":"概要、スキーマ、キュー深度、有限のログとイベント記録を確認。診断アーカイブを出力して調査できます。","mcpManage":"調査から対応へ","mcpManageBody":"設定の検証・保存、ルール管理、Agent 制御、失敗イベントの再試行。データ変更は構造化計画と明示確認で行います。","mcpConnect":"複数ノードを管理","mcpConnectBody":"Windows ノードごとにクライアント項目を追加。ローカル stdio または Windows・Mac から SSH で設定所有アカウントに接続します。","mcpSetupLabel":"MCP クライアントを接続","mcpSetupTitle":"まずはローカル接続から。","mcpSetupBody":"NodeBridge で完全な設定を保存し MCP を有効化。設定所有者の Windows アカウントで実行し、command と args をクライアントの形式に合わせます。","mcpReference":"MCP ツール資料","mcpRemote":"SSH 接続ガイド","mcpConfigExample":"一般的なクライアント設定","mcpBoundaryLabel":"明確な操作範囲。","mcpBoundary":"MCP HTTP リスナー、任意 Shell、生 SQL は提供しません。設定検証、マスキング、書き込み監査を維持。初期整合には全参加端の Agent 停止、明示確認とポーリングが必要です。running は完了ではありません。","mcpPromptDiagnose":"エッジの同期が止まった理由を、状態とログから調べて。","mcpOutcomeDiagnose":"プロセス状態、キュー深度、イベント記録を照合。プロセス実行中だけでは同期の健全性を証明できません。","mcpPromptRules":"保存する前に、このルールのスキーマとマッピングを確認して。","mcpOutcomeRules":"保存済みリビジョンとスキーマを読み、ローカルルールを事前確認。保存時は expected_revision が必要です。その後 active_revision を確認し、自動反映を仮定しません。","mcpPromptAlignment":"承認済みの初期整合を開始し、進行状況を追跡して。","mcpOutcomeAlignment":"全参加端で Agent を停止して確認。各 MCP セッションを維持し、グループ全体の準備完了までポーリング。同期は自動起動されません。"});
  function choose(saved, languages) {
    if (Object.hasOwn(messages, saved)) return saved;
    for (const value of languages || []) {
      const base = String(value).toLowerCase().split(/[-_]/)[0];
      if (Object.hasOwn(messages, base)) return base;
    }
    return 'en';
  }
  Object.assign(messages.en, {searchTitle:'MySQL synchronization for Windows edge networks',searchBody:'Synchronize MySQL between edge nodes and a central server with bidirectional mappings, automatic reconnection and MCP tools for AI-assisted operations. Windows installer available.'});
  Object.assign(messages.zh, {searchTitle:'面向 Windows 边缘网络的 MySQL 双向同步软件',searchBody:'在边缘节点与中心服务器之间同步 MySQL 数据，支持双向映射、断线自动重连，以及通过 MCP 查队列、管理规则、跟踪首次对齐。提供 Windows 直装版。'});
  Object.assign(messages.ja, {searchTitle:'Windows エッジ環境向け MySQL 双方向同期',searchBody:'エッジと中央サーバーの MySQL データを同期。双方向マッピング、自動再接続、MCP によるキュー確認・ルール管理・初期整合追跡に対応。Windows インストーラーを提供。'});
  if (typeof module !== 'undefined') module.exports = {choose, messages};
  if (typeof document === 'undefined') return;
  const key = 'NodeBridge.welcome.language';
  let saved;
  try { saved = localStorage.getItem(key); } catch {}
  let activeScenario = "diagnose";
  function render(lang) {
    document.getElementById('theme-toggle').setAttribute('aria-label', messages[lang].theme);
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : lang;
    document.querySelectorAll('[data-text]').forEach(el => { el.textContent = messages[lang][el.dataset.text]; });
    document.getElementById('languageLabel').textContent = messages[lang].language;
    document.getElementById('language').value = lang;
    renderScenario(lang);
    document.getElementById('readme').href = 'https://github.com/YufeiSun5/NodeBridge/blob/main/' + ({en:'README.md',zh:'README.zh-CN.md',ja:'README.ja.md'})[lang];
  }
  const pageLanguage = document.documentElement.dataset.language;
  render(pageLanguage || choose(saved, navigator.languages || [navigator.language]));
  document.getElementById('language').addEventListener('change', event => {
    const lang = choose(event.target.value, []);
    try { localStorage.setItem(key, lang); } catch {}
    if (pageLanguage) {
      location.assign('../' + ({en:'en',zh:'zh-CN',ja:'ja'})[lang] + '/' + location.hash);
      return;
    }
    render(lang);
  });

  const themeKey = 'NodeBridge.welcome.theme';
  function applyTheme(theme) {
    document.documentElement.dataset.theme = theme;
    document.getElementById('theme-toggle').setAttribute('aria-pressed', String(theme === 'dark'));
    document.querySelector('meta[name="theme-color"]').content = theme === 'dark' ? '#0d0d0d' : '#f4f6f8';
  }
  let savedTheme; try { savedTheme = localStorage.getItem(themeKey); } catch {}
  applyTheme(savedTheme === 'dark' ? 'dark' : 'light');
  document.getElementById('theme-toggle').addEventListener('click', () => {
    const theme = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
    applyTheme(theme); try { localStorage.setItem(themeKey, theme); } catch {}
  });
  function renderScenario(lang) {
    const scenarios = {
      diagnose: {suffix:'Diagnose',tools:['nodebridge_overview','nodebridge_queue_status','nodebridge_logs','nodebridge_event_status']},
      rules: {suffix:'Rules',tools:['nodebridge_sync_rules','nodebridge_mysql_schema','nodebridge_rule_preflight','nodebridge_save_sync_rules']},
      alignment: {suffix:'Alignment',tools:['nodebridge_agent_status','nodebridge_start_initial_alignment','nodebridge_initial_alignment_status']}
    };
    const scenario = scenarios[activeScenario];
    document.getElementById('mcp-prompt').textContent = messages[lang]['mcpPrompt'+scenario.suffix];
    document.getElementById('mcp-outcome').textContent = messages[lang]['mcpOutcome'+scenario.suffix];
    const list = document.getElementById('mcp-tools');
    list.replaceChildren(...scenario.tools.map((name,index) => {
      const row = document.createElement('li'), number = document.createElement('span'), code = document.createElement('code');
      number.textContent = String(index+1).padStart(2,'0'); code.textContent = name;
      row.append(number, code); return row;
    }));
    document.querySelectorAll('[data-scenario]').forEach(button => button.setAttribute('aria-pressed',String(button.dataset.scenario === activeScenario)));
  }
  document.querySelectorAll('[data-scenario]').forEach(button => button.addEventListener('click', () => {
    activeScenario = button.dataset.scenario;
    renderScenario(document.getElementById('language').value);
  }));

  const tabs = Array.from(document.querySelectorAll('[role=tab]'));
  function activate(tab) {
    tabs.forEach(item => {
      const selected = item === tab;
      item.setAttribute('aria-selected', String(selected));
      item.tabIndex = selected ? 0 : -1;
      document.getElementById(item.getAttribute('aria-controls')).hidden = !selected;
    });
  }
  tabs.forEach((tab, index) => {
    tab.addEventListener('click', () => activate(tab));
    tab.addEventListener('keydown', event => {
      let next;
      if (event.key === 'ArrowRight') next = tabs[(index + 1) % tabs.length];
      if (event.key === 'ArrowLeft') next = tabs[(index + tabs.length - 1) % tabs.length];
      if (event.key === 'Home') next = tabs[0];
      if (event.key === 'End') next = tabs[tabs.length - 1];
      if (next) { event.preventDefault(); activate(next); next.focus(); }
    });
  });
})();
