# Frontend 16:9 Visual Review

Review date: 2026-05-22
Identity: `frontend-ai`
Target: Wails dev URL `http://localhost:34115`

## Scope

Checked these 16:9 viewports:

| Viewport | Purpose |
| --- | --- |
| `1366x768` | Minimum common laptop-like desktop |
| `1600x900` | Mid desktop |
| `1920x1080` | Full HD desktop |

Checked pages:

- Overview
- Sync Config
- Rules
- Queues
- Failures
- Logs
- Manual
- Settings

Evidence files:

- Screenshots: `.cache/frontend-visual-review-16x9/*.png`
- Automated audit: `.cache/frontend-visual-review-16x9/audit.json`

## Automated Findings

- No tested page produced document-level horizontal overflow at `1366x768`, `1600x900`, or `1920x1080`.
- No visible element was detected outside the viewport bounds.
- Repeated text clipping was detected on Sync Config read-only values, especially RabbitMQ URLs and Canal paths.
- Overview at `1366x768` clips the SyncAgent log path in the Agent process card.

## Page Findings

### Overview

Status: usable, but not fully polished.

Findings:

- The Start / Refresh controls sit left while Stop / Restart sit far right. This is functionally clear, but visually disconnected on wide screens.
- The Agent process log path is ellipsized at `1366x768`; users cannot inspect the full path without another affordance.
- Full HD leaves a large unused lower area. This is acceptable for a status dashboard, but the top half feels stretched horizontally.

Suggested fixes:

- Add a copy/full-value affordance for long paths.
- Consider grouping Start/Refresh and Stop/Restart into two labeled action blocks instead of pushing danger actions to the far edge.
- Keep status cards capped to a reasonable max width or increase density with secondary metrics.

### Sync Config

Status: most problematic page in this review.

Findings:

- The read-only card grid auto-fits too aggressively. At `1600x900` and `1920x1080`, it creates more columns, but each card becomes only about `305px` wide.
- Long RabbitMQ URLs, management URL, and Canal config paths are heavily ellipsized.
- On large screens, the layout paradoxically becomes less readable because cards get narrower instead of wider.
- The page is visually dense but not necessarily clearer; many important connection strings are reduced to truncated fragments.

Suggested fixes:

- Limit read-only config sections to 3 or 4 columns maximum.
- Raise the minimum card width to around `340-380px`.
- Allow long values to wrap or use a dedicated monospace full-width value row for URLs and paths.
- Add copy buttons for URLs, paths, and DSN-like values.

### Rules

Status: much better than the old table, but still dense.

Findings:

- No horizontal overflow at all tested widths.
- The card layout is readable at `1366x768`, though dense.
- At `1920x1080`, rule cards stretch across the full width and become visually long. This works for scanning but feels heavy.
- The small `启用` button at the far right of each rule is easy to miss on wide screens.
- Rule sections use many nested boxes; the visual hierarchy is correct but busy.

Suggested fixes:

- Add a max content width for each rule card or a fixed grid rhythm inside rule cards.
- Make the enabled state part of the rule header with stronger alignment, not a tiny button at the far right.
- Keep the current grouped card model; do not return to the wide table.

### Queues

Status: visually too empty.

Findings:

- At all tested sizes, the page shows a small loading/empty strip near the top and a large blank area.
- The empty/loading state does not tell the user whether this is due to missing config, RabbitMQ connection failure, or just no queue data.

Suggested fixes:

- Use a centered or full-width empty state panel with reason, next action, and last refresh time.
- When config is missing or RabbitMQ is unavailable, show that as the primary state rather than a generic top strip.

### Failures

Status: functional, but visual hierarchy is harsh.

Findings:

- The error banner is large and repeats "失败事件错误" twice.
- The toolbar is clear, but the page still shows a large empty black area after the empty state.
- Batch retry remains visually prominent even when there are no failures.

Suggested fixes:

- Remove duplicate error wording and keep one concise error message.
- Disable or visually de-emphasize batch retry when there are zero failures.
- Replace the empty area with a structured empty state.

### Logs

Status: clean controls, empty state too sparse.

Findings:

- Filters are aligned and readable.
- The page becomes mostly empty black space when there are no logs.
- The info line is useful, but the empty state below it is weak.

Suggested fixes:

- Add a stronger empty state with "no logs yet", current filters, and diagnostic export guidance.
- Keep "应用筛选" as an explicit action; it reads correctly.

### Manual

Status: usable and clear at `1366x768`; overly wide at Full HD.

Findings:

- The left chapter list is good and stable.
- At `1920x1080`, the content cards stretch too wide, producing long reading lines.
- The first viewport has large unused lower space, but that is less harmful for a manual page.

Suggested fixes:

- Cap manual content width or use a 3-column max card grid.
- Keep the left table of contents; it works well.

### Settings

Status: restored to a clearer structure after the latest fix.

Findings:

- The page now uses stable sections: Appearance and Language, Window and Startup, Integrations, Managed Components, Security and About.
- `1366x768` is readable after sectioning.
- `1920x1080` is clearer, but the page becomes long because Managed Components and Security are below the first fold.
- The Exit button is still a strong red full-width action in Settings. It is not on the top surface anymore, but it can still draw too much attention.

Suggested fixes:

- Keep the new sectioned layout.
- Consider moving Exit deeper into Security and About, or make it a less dominant secondary-danger action until clicked.
- Keep Managed Components full-width because its table needs space.

## Cross-Page Recommendations

1. Add a standard empty-state component for Queues, Failures, and Logs.
2. Add a standard long-value component for paths, URLs, queue names, and DSN-like values: wrap, copy, or expand.
3. Avoid unlimited auto-fit grids on large screens. Use max columns and reasonable `minmax()` widths.
4. Keep Rules as grouped cards, but reduce nested-box visual noise over time.
5. Add screenshot review to the frontend gate for `1366x768` and `1920x1080` before handoff.

## Priority

| Priority | Item |
| --- | --- |
| P1 | Sync Config read-only grid: stop cards getting narrower on larger screens and improve long-value handling. |
| P1 | Queues / Failures / Logs empty states: replace large blank areas with explicit operational states. |
| P2 | Overview long path display and action grouping. |
| P2 | Rules wide-screen rhythm and enabled-state alignment. |
| P3 | Manual max reading width. |
| P3 | Settings Exit button visual weight. |

## Follow-Up Implementation

Implemented at `2026-05-22 18:48` by `frontend-ai`.

- Config read-only values now keep wider section cards, and long URL/path-like values wrap instead of truncating.
- Overview path/process values wrap, and start/refresh versus stop/restart actions are visually grouped.
- Shared empty/error states have stronger structure and avoid duplicate error detail text.
- Failures disables batch retry when there are no failed events.
- Rules read-only status chips have a fixed rhythm and better alignment.
- Manual content width is capped for 16:9 Full HD reading.
- Settings keeps the restored section structure and reduces the visual weight of the Exit action.

Retest evidence:

- Screenshot directory: `.cache/frontend-visual-review-16x9-after/`
- Audit file: `.cache/frontend-visual-review-16x9-after/audit.json`
- Viewports: `1366x768`, `1600x900`, `1920x1080`
- Pages: Overview, Sync Config, Rules, Queues, Failures, Logs, Manual, Settings
- Result: no page-level horizontal overflow detected in the 24 retested combinations.
