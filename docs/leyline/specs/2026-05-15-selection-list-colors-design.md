# Selection List Color Styling - product spec
Date: 2026-05-15
Author: human partner
Surfaces: single-screen-ui

Deep-discovery round 1 classification:
- (S) Undefined color contract for marker/text/token colors
- (S) Conventional format detection grammar under-specified
- (O) Success criteria not deterministic enough for regression safety
- (O) No explicit rollback/disable path for rendering regressions
- (O) Ownership/triage expectations are implicit

Deep-discovery round 2 classification:
- (O) Runtime toggle semantics and precedence are under-specified
- (O) Terminal color-capability detection contract is missing
- (O) Accessibility fallback acceptance is not deterministic enough in tests
- (O) Observability for rollback/triage mode decisions is missing
- (R) Rejected grammar variants are not explicitly documented

Deep-discovery round 3 classification:
- (O) Env parsing/normalization for `GCM_TUI_SELECTION_COLORS` lacks strict rules
- (O) Mode-decision logging lacks deterministic acceptance criteria
- (O) Capability-fallback testing seam is under-specified
- (O) Fallback accessibility marker invariant is not strict enough

Deep-discovery round 4 classification:
- (O) Invalid-env telemetry branch needs deterministic mode taxonomy
- (O) Capability detection requires explicit profile matrix acceptance
- (O) Logging safety constraint must forbid subject-content emission
- (O) Performance and rollback decision boundaries are under-specified
- (O) Ownership needs response process checkpoints

Deep-discovery round 5 classification:
- (O) Performance acceptance criteria were non-deterministic
- (O) Rollback trigger thresholds were not measurable
- (O) Capability contract missed partial/degraded profile behavior
- (O) Ownership lacked time-bound accountability
- (O) Observability fields were too thin for incident reconstruction

## Problem
The commit message selection list is readable but visually flat. The currently
selected item is not emphasized enough, and Conventional Commit-like subjects
(`fix:`, `fix(scope):`, `fix(scope)!:`) are not tokenized for fast scanning.

## Goals
- Make the selection list more visually expressive without adding noise
- Use different colors for the selected marker and selected item text
- Keep non-selected items in the terminal's default foreground color
- For Conventional Commit-like selected subjects, color punctuation tokens:
  `:`, `(`, `)` in gray and `!` in red

## Non-goals
- No changes to item ordering, keyboard behavior, or selection logic
- No syntax highlighting for non-selected items
- No validation or parsing changes for commit message generation

## Constraints
- Must be implemented in the existing Bubble Tea + lipgloss rendering path
- Must preserve current one-line list layout and spacing
- Must not degrade readability on terminals with varied themes

### Color contract and fallback
- Marker color uses ANSI 39 (default foreground) with bold style
- Selected text color uses ANSI 14 (bright cyan)
- Punctuation tokens use ANSI 8 (gray) for `:`, `(`, `)` and ANSI 9 (red) for
  `!`
- `NO_COLOR` has highest precedence: if set, disable all added colorization and
  render plain text regardless of `GCM_TUI_SELECTION_COLORS`
- `GCM_TUI_SELECTION_COLORS=0` disables added colorization when `NO_COLOR` is
  not set
- `GCM_TUI_SELECTION_COLORS` unset enables colorization; any non-empty value
  other than `0` keeps colorization enabled
- `GCM_TUI_SELECTION_COLORS` normalization rules:
  - Leading/trailing ASCII whitespace is trimmed before evaluation
  - Exact normalized value `0` disables colorization
  - Empty/unset value enables colorization
  - Any other normalized value enables colorization and emits a WARN log noting
    an unrecognized toggle value
- On terminals without color support, keep marker and spacing behavior unchanged
  and render plain text without added token colors

### Terminal capability contract
- Renderer checks terminal capability through the Bubble Tea/lipgloss color
  profile exposed at runtime
- If runtime profile indicates no ANSI color support, behavior must match
  `NO_COLOR` fallback output for selected rows
- Capability detection is sourced through a testable seam (injectable capability
  provider/profile) so no-color fallback behavior is deterministic in tests
- Acceptance matrix includes at least one ANSI-capable profile and one no-color
  profile, each with deterministic expected mode outcomes
- Capability classes and expected mode mapping:
  - `ansi_capable` -> colorization enabled (`enabled` or `enabled_invalid_env`)
  - `no_color` -> `disabled_capability`
  - `degraded_or_partial` -> `disabled_capability` (conservative fallback)

### Logging safety
- Mode-decision diagnostics must not include commit subject text or full rendered
  row content

## Approaches considered
### Approach A - Style only selected row with lightweight parser
Keep non-selected rows unstyled. For selected row, use one color for marker,
another color for title text, and apply token coloring only when the subject
matches a Conventional Commit-like prefix (`type:`, `type(scope):`,
`type(scope)!:`).

Trade-offs: Low complexity, predictable behavior, minimal visual noise,
easy to test with delegate render output.

### Approach B - Style all rows with reduced intensity
Apply dim styling to non-selected rows and richer styling to selected row.

Trade-offs: More visual noise; conflicts with requirement to preserve terminal
default color for non-selected items.

### Approach C - Full regex-driven highlighting for all commit patterns
Add broader syntax highlighting for many commit variants in all rows.

Trade-offs: Highest complexity and maintenance cost for limited UX gain.

## Recommendation
Approach A. It matches the requested behavior exactly, keeps list readability
high, and limits complexity to selected-row rendering.

Operational rollback: guard the styling behind an environment toggle
`GCM_TUI_SELECTION_COLORS` (default: enabled). Setting it to `0` disables the
new marker/text/token coloring at runtime without changing selection logic.

Operational precedence:
- `NO_COLOR` overrides all other colorization controls
- `GCM_TUI_SELECTION_COLORS=0` disables colorization when `NO_COLOR` is absent

Rollback decision criteria:
- Trigger: at least 3 confirmed styling regressions across at least 2 capability
  classes within 24 hours
- Owner decision checkpoint: designated `internal/tui` maintainer acts as decider
- Action path: prefer runtime disable (`GCM_TUI_SELECTION_COLORS=0`) for immediate
  mitigation; escalate to code rollback when regression persists after patch
- Time to mitigation target: runtime disable decision within 4 hours of trigger

## Conventional format detection
Token coloring applies only when the selected subject begins with one of:
- `type:`
- `type(scope):`
- `type(scope)!:`

Detection grammar (anchored at subject start):
- `type` is ASCII lowercase letters `[a-z]+`
- `scope` is one or more ASCII lowercase letters, digits, or hyphens
  `[a-z0-9-]+`
- Accepted prefixes:
  - `^[a-z]+:`
  - `^[a-z]+\([a-z0-9-]+\):`
  - `^[a-z]+\([a-z0-9-]+\)!:`
- Matching applies only to the prefix. Remaining subject text is rendered with
  selected-text color and no extra token parsing.

Rejected variants (explicitly non-matching by design):
- Uppercase `type` letters (e.g., `FIX:`)
- Scope characters outside `[a-z0-9-]` (e.g., underscore or dot)

Rationale: restricting matching to a narrow ASCII subset keeps tokenization
deterministic across terminals and avoids ambiguous partial highlighting.

If the subject does not match these forms, render the selected row with plain
selected-text color (no punctuation token coloring).

## Visual behavior
- Selected marker (currently `> `) uses marker color
- Selected subject text uses selected-text color
- Inside selected subject and only for matched formats:
  - `:`, `(`, `)` use gray
  - `!` uses red
- Non-selected rows remain unstyled (terminal default color)

## Ownership and triage
- Owning component: `internal/tui`
- Triage path for regressions: reproduce with subject fixtures in
  `internal/tui/tui_test.go`, then disable via `GCM_TUI_SELECTION_COLORS=0` as
  immediate mitigation while patching.
- Logging requirement: emit a mode decision record for selected-row styling
  (`enabled`, `enabled_invalid_env`, `disabled_no_color`, `disabled_env`,
  `disabled_capability`) to support diagnosis of fallback/disable behavior.
- Mode decision log fields (minimum): `mode`, `source`, `selected_row_styling`
- Extended diagnostic fields: `capability_class`, `env_raw_present`,
  `env_normalized_value`, `env_recognized_toggle`
- Response process checkpoint: triage outcome documents whether runtime disable
  or code rollback was chosen and why
- Escalation: unresolved regressions after 24 hours are escalated to repository
  maintainer review

### Performance boundary
- Selected-row tokenization remains O(prefix length) with no full-line
  backtracking parser
- Test fixtures include a max-list render scenario to detect hot-path
  regressions
- Render-path acceptance budget: median delegate render duration across 1,000
  iterations on the max-list fixture must not exceed baseline by more than 10%

## Success criteria
- Selected marker color differs from selected subject color
- Non-selected items preserve terminal default foreground color
- `fix:` renders with gray `:` when selected
- `fix(scope):` renders with gray `(` `)` `:` when selected
- `fix(scope)!:` renders with gray `(` `)` `:` and red `!` when selected
- Non-matching selected subjects do not receive punctuation token coloring
- Render tests assert ANSI token placement for marker/text/punctuation on the
  selected row using deterministic fixture subjects
- Render tests assert non-selected rows remain unstyled
- `NO_COLOR` disables added colorization
- `GCM_TUI_SELECTION_COLORS=0` disables added colorization
- When both `NO_COLOR` and `GCM_TUI_SELECTION_COLORS` are present, `NO_COLOR`
  takes precedence
- Render tests assert fallback behavior for three paths: `NO_COLOR`,
  `GCM_TUI_SELECTION_COLORS=0`, and simulated no-color capability profile
- In each fallback path, selected-row identification remains unambiguous via
  marker and row position without relying on color
- In each fallback path, non-selected rows remain byte-for-byte unstyled
- In each fallback path, selected row keeps a plain-text `> ` marker prefix with
  unchanged spacing
- Render tests verify mode-decision logs for each path:
  - `NO_COLOR` -> `disabled_no_color`
  - `GCM_TUI_SELECTION_COLORS=0` -> `disabled_env`
  - no-color capability profile -> `disabled_capability`
- Invalid non-empty env values map to `enabled_invalid_env` and emit WARN logs
- Capability matrix tests assert deterministic outcomes for both ANSI-capable
  and no-color runtime profiles
- Capability matrix tests include a degraded/partial profile that must map to
  `disabled_capability`
- Render tests cover env normalization edge cases (`"0"`, `" 0 "`, unset,
  invalid non-empty values) with deterministic expected mode outcomes
- Mode-decision logs never include commit subject text or full rendered rows
- Mode-decision logs include `capability_class` and env normalization fields
  with deterministic values per test fixture
- Regression-response notes record runtime-disable vs rollback decision outcomes
- Performance tests enforce the 10% median render-duration budget on the
  max-list fixture
- Existing TUI tests pass and new/updated tests cover delegate rendering rules
- `make all` (fmt -> vet -> lint -> test -> build) passes

## Approvals
- Product spec approved - round 3 - 2026-05-15
- Product spec approved - round 4 - 2026-05-15
- Product spec approved - round 5 - 2026-05-15
- Product spec approved - round 6 - 2026-05-15
Deep-discovery pass complete - round 6 - 2026-05-15
