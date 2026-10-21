# Shtui UI Interaction and Welcome Redesign

## Goal

Make Shtui's sheet navigation, color controls, and conditional-format dialog reliably mouse-operable; make cell fills visually meet grid boundaries; and replace the plain fresh-open screen with a polished, centered Charm-style launcher.

## Audience and constraints

- Keep Go as the application language and preserve native compiled startup.
- Retain Bubble Tea/Bubbles, Lip Gloss, the custom virtualized grid, and Excelize workbook operations; add no dependencies.
- Preserve supported keyboard workflows, terminal-theme colors, async open/save, CSV-loss warnings, and unknown XLSX feature round-tripping.
- Preserve workbook contents and formatting until an explicit action applies a change. Conditional-format drafts remain unapplied until Done.
- Keep the internal Go module and source paths (`shtui`) unchanged. User-facing brand and executable name become `Shtui` and `shtui`.
- No recent-file persistence or new spreadsheet functions are in scope.

## Architecture decisions

### One welcome-screen owner

`AppModel` becomes the single owner of the no-workbook welcome view and its keyboard/mouse action list. `Controller.View` delegates to that view rather than maintaining a second plain-text home screen. Existing controller actions continue to own file-picker, asynchronous load, and workbook creation behavior. Bubbles `list` provides the focused action selection; Ctrl+P remains available.

### Layout-derived mouse hit testing

Mouse targets derive from the actual rendered layout and popup geometry, not fixed assumptions about terminal rows. Modal priority is explicit: the color picker handles pointer events before its parent conditional-format dialog; the parent dialog handles its controls while open; the grid receives events only when no modal owns them. Keyboard navigation remains available for every action.

### Sheet tabs and add action

Keep sheet tabs in the existing workbook view. The hit region for each tab matches its rendered width and actual row. Append a visible `+` tab/button after the final sheet. Clicking it runs the existing Add Sheet action and activates the created sheet. Ctrl+PageUp/Down and command-palette actions remain unchanged.

### Fill-to-border rendering

Cell fill covers the cell interior through the terminal character used for its adjacent grid boundary; border glyph foreground and border presets remain visible. A terminal character can carry only one background color where differently formatted cells share a boundary. Resolve that collision consistently using the fill from the cell above for horizontal boundaries and from the cell to the left for vertical boundaries. Merged-cell boundaries remain suppressed as before.

### Color picker mouse behavior

The existing theme-first palette remains in the same order. Clicking a theme or custom color applies the same action as Enter; clicking Add custom RGB opens the existing input flow. Clicking outside dismisses without changing the workbook. The same pointer behavior applies when the palette is nested over conditional formatting.

### Conditional-format popup

Keep the existing conditional-rule and workbook APIs. Reorganize the popup into clearly labeled groups: Apply to range; rule type and criteria; formatting style and preview; current draft rules; Add another rule and Cancel/Done actions. Give sections more vertical separation on normal terminals and retain a compact, reachable layout on small terminals.

Mouse clicks focus or activate the range/value inputs, rule tabs, condition selection, color controls, scale controls, Add, Cancel, and Done. Existing keyboard focus order and Escape behavior remain. The list of rules already added to the draft is visible. Color choices continue to use terminal theme colors before custom RGB.

### Product identity

Use `Shtui` in the welcome screen, loading/empty states, and documentation. The user-facing executable is `shtui`; update usage text and build/run examples. Keep the Go module name, internal imports, and source command package directory unchanged to avoid unrelated source-path churn.

## Interaction contract

- Clicking a visible sheet tab activates that sheet and resets grid navigation as the keyboard sheet-switch path does.
- Clicking `+` creates and activates the next available sheet name without leaving the workbook view.
- Clicking each color option applies that option once; selecting a theme color resolves the current terminal color when selected. Custom RGB remains an explicit input step.
- Outside clicks close the color picker without applying a color; they do not leak to underlying grid or dialog controls.
- Conditional-dialog pointer actions target the visible control under the pointer, including when the dialog is overlaid or the color picker is nested. Done applies drafts once; Cancel leaves the workbook unchanged.
- The home action list supports both pointer and keyboard activation, maps to the existing controller actions, and fits short and wide terminal sizes without clipping.
- Existing keyboard actions, menu shortcuts, file handling, and workbook data semantics are unchanged.

## Error and edge behavior

- Derive hit testing from the rendered home, tab, and popup layouts so toolbar visibility and terminal resizing cannot shift controls away from their mouse targets.
- A color-picker click outside its bounds dismisses only the topmost picker; it must not activate a sheet, grid cell, or underlying conditional control.
- A conditional-format validation error remains visible in the dialog, with existing workbook drafts untouched until Done.
- If adjacent cell fills differ, only the shared boundary background uses the documented above/left tie-breaker; border foreground stays legible.
- On short terminals, prioritize editable range/rule controls and visible Cancel/Done actions; avoid clipping or placing controls outside the viewport.

## Acceptance checks

- `AppModel.Update` mouse tests click a non-active sheet tab and the `+` tab using actual rendered coordinates, with the toolbar both visible and hidden; verify active sheet, new-sheet creation, and selection reset.
- Grid render tests verify filled cells have no unfilled terminal-character gap at adjacent boundaries, preserve border foreground/styles, and behave deterministically for adjacent different fills and merged ranges.
- Color-picker mouse tests select theme colors, open custom RGB input, and dismiss by outside click without applying; include nested conditional-dialog usage.
- Conditional-format mouse tests activate the rule tabs, criteria/dropdowns, range/value inputs, color controls, scale controls, Add, Cancel, and Done. Assert that Cancel leaves workbook state unchanged and Done applies each listed draft once.
- Conditional-format render tests show range, rule, style, preview, existing drafts, and actions in a readable order at normal dimensions; small-terminal tests keep critical inputs and actions reachable.
- Home-screen tests show the Shtui wordmark, focused Open/New CSV/New XLSX actions, keyboard hints, and responsive layout; mouse and keyboard activation route through existing controller behavior.
- CLI usage and docs identify the application as Shtui / `shtui`, while internal Go import paths remain unchanged.
- Run `go test ./...`, build the native CLI, and smoke the actual TUI at normal and compact terminal sizes, including sheet clicks, `+`, palette clicks, fill borders, conditional-dialog clicks, and opening/creating a workbook.

## Alternatives considered

1. **Derive mouse targets from rendered geometry (selected):** resilient to toolbar visibility, compact layouts, and popup changes; tests can use the same coordinates a user sees.
2. **Keep fixed mouse rows and add offsets:** smaller initial edit but preserves the coordinate mismatch when the layout changes.
3. **Keep the plain controller-owned welcome screen:** avoids moving ownership but leaves two inconsistent no-workbook screens and cannot give the list a single mouse/keyboard event owner.

## Out of scope

- Renaming the internal Go module, import paths, or source directory.
- Adding recent-file history, cloud sync, macros, or additional Excel functions.
- Replacing the existing grid or changing supported workbook semantics.
- Removing or changing current keyboard bindings beyond the visible Shtui usage label.
