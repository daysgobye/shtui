### Task 7: Formula Bar as the Only Input Surface

**Files:**
- Create: `src/ui/formula_bar.rs`
- Modify: `src/ui/grid.rs` — remove the in-cell editor, add `focus_requested`
- Modify: `src/app/app.rs` — own the bar, lay it out under the grid
- Modify: `tests/grid_keys.rs` — remove editing tests, add focus-request tests
- Create: `tests/formula_bar.rs`

**Interfaces:**
- Consumes: `Workbook`, `CellValue::from_text`, `CellRef`, `Selection`, `CellAction`, `cell_at_pos`, the existing `OmaApp::retarget_selection_to_active_sheet`.
- Produces: `pub struct FormulaBar { … }` with `load_from`, `request_focus`, `focused`, and `pub fn show(bar: &mut FormulaBar, wb: &mut Workbook, target: CellRef, ui: &mut egui::Ui)`; `GridState::focus_requested: bool`.

Replaces spec §6.1 as amended. This task deletes the in-cell editor that Task 6 built.

---

## The egui behaviour this design rests on

These were **verified by running a probe against egui 0.35.0**, not read off the docs. Do not
re-derive them; do not "simplify" past them.

| Fact | Consequence |
|---|---|
| `TextEdit::multiline(...).return_key(KeyboardShortcut::new(Modifiers::SHIFT, Key::Enter))` makes **Shift+Enter insert `\n`** and leaves **plain Enter completely unhandled** — the buffer is byte-identical afterwards | We get the newline for free and can claim plain Enter for "move down" |
| **Tab in multiline with default `lock_focus(false)` inserts nothing and surrenders focus** | Tab is "move right", and focus comes back to the bar so you can type `a<Tab>b<Tab>c` |
| **Tab in multiline with `lock_focus(true)` inserts a literal `"\t"`** | **Do not call `lock_focus`.** Note that `TextEdit::code_editor()` sets it, so do not use that preset either |
| `Shift+Tab` in multiline **decreases indentation** | We claim Tab first, so `Shift+Tab` never reaches the widget |
| Typing arrives as `Event::Text`, not as a key event | `Key::A` inserts nothing; that is egui, not a bug |
| `TextEdit::show` returns `TextEditOutput` with `galley: Arc<Galley>`; `ui.add` returns a bare `Response` with no galley | Auto-grow must use `TextEdit::show`, not `ui.add` |

## Design

```
┌────────────────────────────────────────────┐
│  grid (frozen headers, selection)          │
├────────────────────────────────────────────┤
│ A1 │ value being edited                  ▓ │  ← grows with the text
└────────────────────────────────────────────┘
```

**State**

```rust
pub struct FormulaBar {
    /// The cell the field is currently bound to.
    target: CellRef,
    /// The field's text. Owned here so it survives focus changes.
    text: String,
    /// Pre-edit text, snapshotted when the user first types in this visit.
    original: String,
    /// Whether the user has touched the field since it was loaded.
    touched: bool,
    /// Cached content height, so the bar can grow.
    height: f32,
}
```

**Flow, in `show`**

1. If `target` differs from the caller's current cell, `load_from` it: `text` becomes
   `wb.value_at(cell).to_input_string()`, `original` is set to the same, `touched` is cleared,
   and the cached height resets.
2. `TextEdit::multiline(&mut text).return_key(SHIFT+Enter).desired_rows(rows_from_height)`
   with `rows` derived from the cached `height`. **No `lock_focus`.**
3. If `output.changed()`: snapshot `original` on the first change, set `touched`, and
   `wb.set_cell(target, CellValue::from_text(&text))`.
4. Measure `output.galley.size().y` and store the next frame's `height`, clamped to
   `[MIN_BAR_H, max_h]` where `max_h` is about 40% of the UI height.
5. If plain `Enter` was pressed this frame: move the caller's selection down.
   If `Tab` was pressed: move right. In both cases `request_focus` the field again so
   typing continues across cells.
6. If `Esc` was pressed and `touched`: restore `original` into `text`, write it to the cell,
   clear `touched`, surrender focus.

Key events are read with `ui.input(|i| i.key_pressed(..))`. Because plain Enter and Tab are
left unhandled by the widget, reading them *after* `show` is safe.

**Grid side**

`GridState` gains `pub focus_requested: bool`. When the user types a printable character,
presses `F2`, or double-clicks, `show` sets it and consumes the gesture. The grid does not
know the bar exists. `OmaApp::body` clears it by calling `bar.request_focus()`.

**Deletions.** Remove `EditState`, `begin_edit`, `commit_edit`, `cancel_edit`,
`GridState::edit`, the entire editing branch in `show`'s key loop, the `CellAction::Edit`
handling, and the editor overlay painting in `paint_body`. `parse_input` is already gone.
`CellAction` collapses to `Select` / `None`; `click_action` keeps its shape but returns `None`
for a double-click, because a double-click now only asks for focus rather than opening an
editor at a specific cell.

`OmaApp::retarget_selection_to_active_sheet` loses its `edit` arm.

**Layout.** `OmaApp::body` gives the grid `ui.available_height() - bar.min_height()` and draws
the bar beneath it, with the address gutter on the left showing `format_a1(row, col)`.

## Tests

`tests/formula_bar.rs`, driven through `OmaApp::body` with `Context::run_ui`:

- selecting a cell loads its value into the field
- typing writes the cell on the **same** keystroke, not on a later commit
- `Enter` moves down and the field reloads from the new cell
- `Tab` moves right, inserts no tab character into the value, and focus stays in the field
- typing `a`, `Tab`, `b` lands `a` and `b` in two adjacent cells
- `Esc` restores the pre-edit snapshot and leaves the cell as it was
- `Esc` on an untouched field is a no-op
- a long multi-line value grows the bar, and the height is capped
- no `\t` ever appears in a cell value

`tests/grid_keys.rs`: delete the editing tests that asserted on `GridState::edit` and
`commit_edit`, keep everything about navigation, selection, virtualisation and `click_action`,
and add: typing on the grid sets `focus_requested` exactly once; `F2` sets it; a click does not.

## Global constraints

Unchanged: egui pinned `=0.35.0`; `workbook/` and `io/` must have **zero** egui dependency;
rows and columns zero-indexed; no `unwrap`/`expect` in library code; `cargo clippy --all-targets`
silent; repo is `cargo fmt`-clean.

New: `src/ui/formula_bar.rs` may import egui; nothing outside `src/ui/` and `src/app/` may.

## Build order

1. Write `tests/formula_bar.rs` against the new API; confirm it fails to compile.
2. Add `FormulaBar` with `load_from` / `request_focus` / `focused`.
3. Wire `show` with the TextEdit config above, live write, Enter/Tab/Esc, and auto-grow.
4. Add `GridState::focus_requested`; delete the in-cell editor.
5. Lay the bar out in `OmaApp::body`.
6. Full suite, clippy, fmt, `cargo run`, and a screenshot of a multi-line value with the bar grown.

## Known constraints, stated up front

- **Headless tests cannot drive the TextEdit's caret.** `Context::run_ui` gives each pass a
  fresh `Context`, so focus does not persist between passes. Tests that need the field focused
  must call `bar.request_focus()` and let the next pass pick it up, or assert through
  `FormulaBar`'s own methods rather than through synthesised keystrokes. If a test appears to
  pass for the wrong reason, say so rather than keeping it.
- Real focus persistence works in the running app — one `Context` for the process lifetime —
  so this is a test-harness limitation, not a product one. Verify by hand.