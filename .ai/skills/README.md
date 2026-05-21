# .ai/skills

## Purpose / 用途 / 目的

This directory stores project-specific reusable skills for AI agents.  
本目录存放项目专用、可复用的 AI Agent 操作技能。  
このディレクトリは、プロジェクト固有で再利用可能な AI Agent スキルを保存します。

## What Is A Skill / 什么是技能 / スキルとは

A skill is a stable, repeatable workflow with clear input and output, such as adding a sync event pipeline or a diagnosable worker.  
技能是稳定、可复现、有明确输入输出的项目操作流程，例如新增同步事件链路或可诊断后台 worker。  
スキルとは、入力と出力が明確で、安定して再現できる作業手順です。例：同期イベント処理や診断可能な worker の追加。

## Creation Threshold / 提炼门槛 / 作成基準

Create a concrete `<skill-name>/SKILL.md` only when all conditions are met.  
只有同时满足以下条件，才创建具体的 `<skill-name>/SKILL.md`。  
次の条件をすべて満たす場合のみ、具体的な `<skill-name>/SKILL.md` を作成します。

- The pattern appears more than 3 times in this project. / 在项目中出现 3 次以上。 / プロジェクト内で 3 回以上出現する。
- It has clear input and output. / 有明确输入和输出。 / 入力と出力が明確である。
- The steps are stable and repeatable. / 步骤稳定可复现。 / 手順が安定して再現可能である。
- It contains project-specific constraints. / 含项目特有约束或规范。 / プロジェクト固有の制約を含む。
- It will be reused in future similar tasks. / 能复用到未来同类任务。 / 将来の類似作業で再利用できる。

The repository currently has no source code or repeated implementation patterns, so no concrete skill is created yet.  
当前仓库尚无源码和重复实现模式，因此暂不创建具体技能，避免空洞模板。  
現時点ではソースコードや反復実装パターンがないため、空のテンプレートを避ける目的で具体的なスキルは未作成です。

## Language Rule / 语言规则 / 言語ルール

Active project skills should use Chinese, English, and Japanese where practical.  
活跃项目技能文档尽量使用中英日三语。  
有効なプロジェクトスキル文書は、可能な範囲で中国語・英語・日本語を併記します。

Recommended order: English -> Chinese -> Japanese for frontmatter-facing summaries; Chinese-first is acceptable for detailed project notes.  
推荐顺序：面向检索的摘要使用 English -> Chinese -> Japanese；详细项目说明可中文优先。  
推奨順序：検索用の概要は English -> Chinese -> Japanese、詳細なプロジェクト説明は中国語優先でも構いません。

## Required Structure / 目录结构 / 必須構成

```text
.ai/skills/
  README.md
  <skill-name>/
    SKILL.md
```

Each `SKILL.md` must include frontmatter.  
每个 `SKILL.md` 必须包含 frontmatter。  
各 `SKILL.md` には frontmatter が必要です。

```yaml
---
name: skill-name
description: "Use when: English keywords / 中文关键词 / 日本語キーワード"
argument-hint: "input / 输入 / 入力"
---
```
