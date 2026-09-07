# Wails Default Size Visual Review

Date: 2026-05-22
Identity: frontend-ai

## Default Size

Wails native window size is configured in `internal/datasyncui/app.go`:

- Width: `1100`
- Height: `720`
- MinWidth / MinHeight: not configured

Screenshots were captured with a browser viewport of `1100x720`, matching the configured Wails default content size as closely as the browser smoke can.

Evidence:

- Screenshot directory: `.cache/wails-default-1100x720-review/`
- Audit file: `.cache/wails-default-1100x720-review/audit.json`

## Summary

The current design language is not fundamentally wrong for NodeBridge. A dense dark industrial console is appropriate for a local sync management tool.

The problem is that several page layouts still behave like wide desktop dashboards while the Wails default window is only `1100x720`. At this size, the product needs a more deliberate compact-window layout:

- Fewer same-row columns.
- More vertical grouping.
- Stronger empty states for operational pages.
- Rules should not use a four-column read-only grid at default width.
- Manual and empty pages should use the space intentionally, not leave a large dead lower half.

## Page Findings

| Page | Result | Notes |
| --- | --- | --- |
| Overview | Acceptable | No horizontal overflow. Dense but usable. Lower Agent process area is partly below the fold, which is acceptable for a status page. |
| Sync Config | Mixed | No overflow, but two large columns plus uneven card heights make the page feel heavy. Read-only config is usable, but the right side can feel empty depending on config state. |
| Rules | Poor at default size | Internal horizontal overflow exists: `.content-area` scroll width `1257` vs client width `1095`; `.rule-readonly-grid` scroll width around `1230`. The user sees a horizontal scrollbar and clipped right-side content. |
| Queues | Too sparse | Empty state is clear, but the page leaves most of the window as unused black space. |
| Failures | Too sparse | Same issue as Queues. Toolbar is okay, but empty state does not use the window enough. |
| Logs | Better than Queues/Failures | Filters and diagnostic hint help, but the lower half is still mostly empty when no logs exist. |
| Manual | Acceptable but sparse | Left chapter list works. Content is readable. The first chapter leaves unused lower space, but this is less harmful for documentation. |
| Settings | Mixed | Sectioning is understandable, but the top error banner is too large and the first viewport is visually heavy. It scrolls correctly. |

## Quantitative Notes

At `1100x720`:

- No document-level horizontal overflow was detected on most pages.
- Rules has internal horizontal overflow inside content containers.
- Content coverage is low on empty operational pages:
  - Overview: `0.157`
  - Queues: `0.187`
  - Failures: `0.187`
  - Manual: `0.138`
  - Logs: `0.241`
- Settings and Config occupy more of the viewport:
  - Config: `0.480`
  - Settings: `0.555`

The low coverage is not automatically wrong, but for Queues / Failures / Logs it reads as unfinished because these are operational pages.

## Design Language Assessment

Keep:

- Dark industrial terminal style.
- Compact typography.
- Thin borders and status colors.
- Read-only locked mode.
- Page-level nav and status bar.

Adjust:

- The default `1100x720` size should be treated as the primary design target, not a reduced breakpoint.
- Cards should not be arranged in grids that only look good at `1366px+`.
- Empty states should explain the operational state and next action in a structured panel.
- Rules needs a default-width layout, not a compressed wide-grid layout.
- Settings error states should be less dominant unless the error blocks the entire page.

## Recommended Fix Plan

P1:

- Rules read-only layout: change from four-column grid to two-column or stacked sections at widths below about `1280px`.
- Remove Rules internal horizontal scroll at `1100x720`.

P2:

- Queues / Failures / Logs: replace single empty card with a structured operational empty panel containing status, reason, next action, and relevant disabled/enabled controls.
- Settings: shrink the top error banner and make it closer to a compact inline status unless it is a fatal page load error.

P3:

- Config: cap read-only section heights or balance section placement so the first viewport does not look like uneven blocks.
- Manual: keep current structure, but optionally show one more quick reference panel in the first viewport.

## Conclusion

The design language is suitable, but the layout system is not yet tuned for the actual Wails default size. The next UI iteration should optimize `1100x720` first, then scale up to `1366x768` and Full HD.

## Follow-Up Implementation

Implemented at `2026-05-22 23:50` by `frontend-ai`.

- Rules read-only layout now switches to two columns below `1280px`, removing the internal horizontal overflow at `1100x720`.
- Queues, Failures, and Logs empty states now use operational panels with status, next action, scope/filter/diagnostic context.
- Settings page errors now use a compact inline error row instead of a large empty-state block.
- Config read-only sections use two balanced columns at default Wails width.

Retest evidence:

- Screenshot directory: `.cache/wails-default-1100x720-after-iteration/`
- Audit file: `.cache/wails-default-1100x720-after-iteration/audit.json`
- Result: no document-level or internal content horizontal overflow detected across all 8 pages at `1100x720`.
