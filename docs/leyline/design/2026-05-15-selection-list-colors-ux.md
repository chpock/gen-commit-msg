# Selection List Color Styling - UX spec
Date: 2026-05-15
Product spec: docs/leyline/specs/2026-05-15-selection-list-colors-design.md
Surfaces: single-screen-ui

## Surfaces enumerated
- **Message selection view**: Inline list of generated commit subjects. One row is
  selected at a time. Selected marker and selected row text are colorized with
  distinct colors. Non-selected rows remain in terminal default color.
  - Marker: ANSI 39 + bold
  - Selected text: ANSI 14
  - Selected Conventional-like punctuation: `:` `(` `)` in ANSI 8 and `!` in
    ANSI 9

## User flows

### Flow 1 - Navigate and select
1. User reaches the message selection view with multiple generated subjects.
2. First item is selected by default.
3. User navigates with arrow keys.
4. The selected marker renders with ANSI 39 + bold and selected subject text
   renders with ANSI 14.
5. User confirms with Enter.
6. Selection is accepted; program exits with the chosen message.

Failure path: If `NO_COLOR` is set, `GCM_TUI_SELECTION_COLORS=0`, or terminal
color support is unavailable, added colorization is skipped. Selection remains
functional via marker character and row position.

Fallback invariant: In all color-disabled paths, selected row keeps a plain-text
`> ` marker prefix with unchanged spacing.

Precedence rule: If both `NO_COLOR` and `GCM_TUI_SELECTION_COLORS` are present,
`NO_COLOR` wins and selected-row rendering remains plain text.

### Flow 2 - Conventional Commit-like selected subject
1. User navigates to a selected subject matching one anchored prefix at subject
   start:
   - `^[a-z]+:`
   - `^[a-z]+\([a-z0-9-]+\):`
   - `^[a-z]+\([a-z0-9-]+\)!:`
2. Within selected text only, punctuation tokens render as follows:
   - `:` ANSI 8
   - `(` ANSI 8
   - `)` ANSI 8
   - `!` ANSI 9
3. User can still read the full subject and choose it normally.

Failure path: If the subject does not match one of these forms, punctuation
token coloring is skipped and the full selected subject uses selected-text color.

## State matrix

| Surface | Empty | Loading | Error | Success | Permission-denied | Offline |
|---------|-------|---------|-------|---------|-------------------|---------|
| Message selection view | N/A - zero messages do not enter this view | N/A - this view is shown after generation | N/A - errors handled in progress/error flow before this view | Selected row has ANSI 39 bold marker and ANSI 14 text when colorization is enabled; non-selected rows use terminal default; punctuation highlighting applies only on selected row when pattern matches; if `NO_COLOR` or `GCM_TUI_SELECTION_COLORS=0`, added colorization is disabled | N/A - no permission-gated action in this view | N/A - no network action in this view |

## Voice and tone
Reference strings:
- Error: `Error: no commit messages generated`
- Success: *(silent)* selected message is printed to stdout without banner text
- Empty state: *(same as error path for this surface context)* `Error: no commit messages generated`

Tone remains direct, technical, and English-only.

## Accessibility targets
- **WCAG level**: N/A (terminal TUI text surface); color independence is
  required.
- **Keyboard flow**: Arrow keys move selection; Enter confirms; Esc exits.
- **Screen reader**: Full selected row remains plain text content (no truncation
  of subject tokens); marker remains a visible character prefix.
- **Motion**: No added motion for this change.
- **Color independence**: Current item is identified by marker and row position,
  not color alone. Token semantics are additive and do not carry required state.
  Fallback modes (`NO_COLOR`, `GCM_TUI_SELECTION_COLORS=0`, or no-color
  terminals) preserve complete operability and unambiguous current-row
  identification with the `> ` prefix unchanged.

## Platform / harness constraints
- Implemented in Bubble Tea list delegate rendering.
- Must preserve one-line row height and current spacing.
- Must work on varied terminal themes by leaving non-selected rows unstyled.
- Must honor `NO_COLOR` and `GCM_TUI_SELECTION_COLORS=0` as runtime
  color-disable controls.
- Capability acceptance matrix includes three deterministic classes:
  - `ansi_capable` -> selected-row colorization enabled
  - `no_color` -> plain-text fallback (`disabled_capability` behavior)
  - `degraded_or_partial` -> plain-text fallback (`disabled_capability`
    behavior)

## Operational mitigation
- Runtime disable control: `GCM_TUI_SELECTION_COLORS=0`
- Global no-color control: `NO_COLOR`
- Triage starts in `internal/tui/tui_test.go` with render fixtures.
- Diagnostics are mode-only; they must not log commit subject text or full row
  render content.
- Incident trigger for rollback triage: at least 3 confirmed styling regressions
  across at least 2 capability classes within 24 hours.
- Mitigation SLA: runtime disable decision within 4 hours of trigger.
- Escalation path: unresolved regressions after 24 hours escalate to repository
  maintainer review.

## Non-goals
- Changing selection keybindings or list behavior
- Styling non-selected rows
- Parsing or validating commit message format beyond display tokenization

## Approvals
- UX spec approved - round 2 - 2026-05-15
- UX spec approved - round 3 - 2026-05-15
- UX spec approved - round 4 - 2026-05-15
- UX spec approved - round 5 - 2026-05-15
- UX spec approved - round 6 - 2026-05-15
- design-interrogation skipped - scope: single-screen-ui with one surface;
  state matrix has one surface row and round 6 findings were non-material
  `(R)/(E)` refinements
