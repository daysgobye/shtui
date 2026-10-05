# Omasheet — Milestone 1 Design

**Date:** 2026-10-03
**Status:** Approved design, ready for implementation planning
**Scope:** First end-to-end slice — Excel-first vertical slice

---

## 1. What Milestone 1 Is

Omasheet is a fast, native, Linux-first spreadsheet inspired by Omawrite: open a file,
work with your data, get out of the way. The full product includes a Lua formula engine
(`LUA=`), conditional formatting, CSV/Markdown export, autosave recovery, and multiple
windows. That is roughly six independent subsystems and will be delivered as separate
spec-and-plan cycles.

**Milestone 1 is the Excel-first vertical slice.** It delivers the complete philosophy
end to end using only Excel formulas:

- Open a `.csv` or `.xlsx`
- See it in a real, fast spreadsheet grid
- Edit cells, multiple sheets, formulas, formatting
- Save as `.xlsx`
- Export to CSV or Markdown

The Lua engine is Milestone 2. Everything in Milestone 1 is designed so that Milestone 2
is additive rather than a rewrite.

### Non-Goals for Milestone 1

Lua formulas, conditional formatting, multi-line cell content, charts, pivot tables,
VBA, cloud features, collaboration.

---

## 2. Locked Decisions

| Area | Decision | Rationale |
|---|---|---|
| Slice shape | Excel-first vertical slice | Proves the riskiest unknowns (formula engine, xlsx fidelity) while delivering the whole philosophy end to end |
| Save behaviour | `Ctrl+S` always writes `.xlsx`; a CSV that was opened is never modified | One save path, no ambiguity, nothing silently destroyed |
| Fill & copy | Copy/paste, drag-fill, and `$` absolute references in Milestone 1 | Settling reference-translation semantics now is cheap; retrofitting them touches the parser, every fill path, and undo |
| Undo/redo | Log of inverse operations, one entry per user gesture | O(1) memory per edit, undo cannot desync because recalculation is always derived fresh |
| Formatting | Number/money/percent/text/date, font colour, background colour, bold, italic, underline, alignment — set via a small **collapsible** ribbon | User requirement; collapsed by default so the no-chrome mode stays reachable |
| Scale target | 10,000 populated cells at 60 fps, full recalc within one frame | Correctness before scale; the ceiling is known and monitored |
| Recalculation | Full recalc on every commit. No dependency graph in Milestone 1 | Simpler and faster to reach correctness at 10k cells; the recalc entry point is kept narrow so a dirty-set engine can replace its body later |
| Crate layout | Single crate, modules under `src/` | Fastest path to a working engine; a workspace can come when the engine earns its own manifest |
| Concurrency | Recalculation is synchronous and single-threaded | At 10k cells it is instant; async recalc introduces a class of stale-UI bugs for no benefit |
| xlsx style import | Gap accepted. Warn when opening a file whose styling cannot be imported | `calamine` does not expose fonts/fills/alignment. Hand-rolling an OOXML style reader is a Milestone 2 target |
| Performance | Asserted by a test, not assumed | A 10k-cell fixture with a hard 16 ms recalc budget |

---

## 3. Module Layout

### 3.0 Verified toolchain and API constraints

These were verified against source on 2026-10-03, not assumed. They are load-bearing, and
egui's API has been changing quickly enough that writing them down is cheaper than
rediscovering them.

| Fact | Value |
|---|---|
| `eframe` / `egui` | **0.35.0**, pinned with `=` |
| Why not 0.36 | 0.36.2 declares `rust-version = "1.95"`; this machine has **rustc 1.94.0** and does not build it |
| Renderer | `wgpu` (an eframe default feature) — confirmed running under Wayland |
| `App` trait method | `fn ui(&mut self, ui: &mut egui::Ui, frame: &mut Frame)` — **not** `update(ctx, frame)`; renamed in 0.35 |
| `AppCreator` | returns `Result<Box<dyn App>, DynError>` |
| `ui.add(widget)` | returns `egui::Response` in 0.35 (0.36 returns `TextEditOutput`) |
| Headless UI tests | `egui::Context::run_ui(RawInput, impl FnMut(&mut Ui)) -> FullOutput`. `egui_kittest` is 0.36 and unusable here |
| Scroll introspection | `ScrollArea::show` returns `ScrollAreaOutput` with `.state.offset: Vec2` and `.content_size: Vec2` |

### 3.1 Modules

```
src/
  main.rs
  app/
    mod.rs
    app.rs          Application state, top-level egui layout, wiring
    undo.rs         Undo/redo stacks of Operations
    clipboard.rs    Internal cell-block clipboard + system clipboard bridge
  workbook/
    mod.rs
    workbook.rs     Workbook, sheet registry, file origin and save path
    sheet.rs        Sparse Sheet, rows, used range
    cell.rs         Cell, CellValue, CellRef, CellFormat, NumberFormat
  formula/
    mod.rs
    lexer.rs        Excel formula text -> tokens
    parser.rs       Tokens -> Expr
    ast.rs          Expr, Ref, operators, translate, to_src
    eval.rs         Expr -> CellValue, errors, cycle detection
    functions.rs    SUM, AVERAGE, MIN, MAX, COUNT, COUNTA, IF, ABS, ROUND, AND, OR, NOT
    refedit.rs      Range normalisation, translation on copy/fill
    recalc.rs       Full-recalc pass behind one entry point
  io/
    mod.rs
    csv_read.rs     Delimiter sniffing, RFC-4180 parsing, number inference
    xlsx_read.rs    calamine-backed import
    xlsx_write.rs   rust_xlsxwriter-backed save
    export_csv.rs   One file per sheet of displayed values
    export_md.rs    Single Markdown document
  ui/
    mod.rs
    grid.rs         Virtualized grid, headers, selection, in-cell editing
    formula_bar.rs  Mirror of the active cell's input
    ribbon.rs       Collapsible formatting ribbon
    sheet_tabs.rs   Sheet list, add/rename/delete/reorder
    dialogs.rs      Open, save, export, recent files, import warning
```

Each module has one clear purpose and is testable in isolation. The `formula/`, `workbook/`,
and `io/` modules have no egui dependency, so the entire engine is testable headlessly.

---

## 4. Data Model

### 4.1 Cell references

```rust
struct CellRef { sheet: SheetId, row: u32, col: u32 }
```

**Always sheet-qualified**, even for same-sheet references. This makes `Sheet2!A1` and
cross-sheet ranges such as `SUM(Sheet1!B3:B8)` ordinary cases rather than special cases,
and there is no second reference representation to convert between.

Rows and columns are zero-indexed internally; display names (`A1`) are converted at the
UI and parser boundary only.

### 4.2 Cell

```rust
struct Cell {
    input:  CellInput,   // added with the formula engine; absent until then
    value:  CellValue,
    format: CellFormat,
}

enum CellInput {
    Literal(CellValue),
    Formula { ast: Expr, src: String },
}
```

Input and value are stored separately: `input` is what the user typed, `value` is what
recalculation produced. The parsed `ast` is stored beside the source `src` so a formula
is parsed **once on edit**, not on every recalculation. A full recalc is therefore pure
evaluation with no allocation of parse structures.

### 4.3 Values

```rust
enum CellValue {
    Empty,
    Number(f64),
    Text(String),
    Bool(bool),
    Error(ErrorKind),
}
```

`ErrorKind` covers `#DIV/0!`, `#VALUE!`, `#NAME?`, `#REF!`, `#CIRCULAR!`, and `#NUM!`.

**Errors are values, not panics.** They propagate through operators and render inside the
cell, exactly as a user expects from a spreadsheet.

### 4.4 Storage

```rust
struct Sheet {
    id:   SheetId,
    name: String,
    rows: BTreeMap<u32, Row>,
    used: Option<CellRange>,   // cached bounding box of populated cells; None when empty
}

struct Row { cells: BTreeMap<u32, Cell> }
```

Sparse, because a 10k-cell sheet must not allocate a dense grid, and because the same
structure handles a 10M-row sheet later without change. `used` is maintained on every
mutation so scroll bounds and export never require a full scan.

### 4.5 Formatting

```rust
struct CellFormat {
    number_format: NumberFormat,
    font_color: Option<Color>,
    bg_color: Option<Color>,
    bold: bool,
    italic: bool,
    underline: bool,
    align: Align,      // Default | Left | Center | Right
}

enum NumberFormat {
    General,
    Integer,
    Decimal(u32),
    Currency { symbol: char, places: u32 },
    Percent(u32),
    Date,
    Time,
    Text,
}
```

Currency and percent are expressed as parameters rather than raw strings, and the
format code (`$#,##0.00`, `0.0%`) is generated from those parameters. xlsx write is
therefore a string emit, and xlsx read is a string parse.

---

## 5. Formula Engine

### 5.1 Parsing

Formula text is lexed, then parsed by a Pratt parser into an `Expr`.

**Operator precedence**, tightest to loosest — Excel's order, deliberately not Rust's:

| Precedence | Operator | Example |
|---|---|---|
| 1 | `:` range | `A2:A5` |
| 2 | postfix `%` | `50%` |
| 3 | unary `-` / `+` | `-A1` |
| 4 | `^` | `2^10` |
| 5 | `*` `/` | `A1*B1` |
| 6 | `+` `-` | `A1-B1` |
| 7 | `&` concatenation | `"a"&"b"` |
| 8 | `= <> < > <= >=` | `A1>100` |

```rust
enum Expr {
    Number(f64),
    Text(String),
    Bool(bool),
    Ref(Ref),                          // optional sheet name, optional $ flags
    Range(Ref, Ref),
    Unary(UnOp, Box<Expr>),
    Binary(BinOp, Box<Expr>, Box<Expr>),
    Call(String, Vec<Expr>),
}
```

**Parse failure** does not discard the user's work. The cell stores the original text
with an error value: the formula bar shows exactly what was typed, and the cell reads
`#NAME?`.

### 5.2 Reference translation

Translation is a pure data transformation — `Expr::translate(drow, dcol)` rewrites every
`Ref` and `Range` node — followed by `Expr::to_src()` to regenerate the formula-bar text
after a fill or paste.

| Written | Filled down one row | Rule |
|---|---|---|
| `B3` | `B4` | both axes relative |
| `$B$3` | `$B$3` | both axes absolute |
| `B$3` | `B$4` | column absolute, row relative |
| `$B3` | `$B4` | column relative, row absolute |
| `B3:B8` | `B4:B9` | both endpoints translate together |

**Sheet names never change.** Filling does not move a reference to another sheet.

### 5.3 Evaluation

```rust
fn eval(expr: &Expr, ctx: &EvalCtx) -> CellValue
```

Evaluation is a depth-first walk that resolves references through the workbook. It is
short-circuiting where Excel is: `IF` evaluates only the branch it takes, so
`IF(A1=0,0,A1/A1)` does not divide by zero.

**Value semantics:**

- An empty cell reads as `0` in arithmetic.
- Aggregates skip empty cells: `SUM`, `AVERAGE`, `MIN`, `MAX`.
- `COUNT` counts numeric cells only. `COUNTA` counts any non-empty cell.
- Division by zero yields `#DIV/0!`; the error propagates outward.

### 5.4 Function set

`SUM`, `AVERAGE` (aliased `AVG`), `MIN`, `MAX`, `COUNT`, `COUNTA`, `IF`, `ABS`, `ROUND`,
`AND`, `OR`, `NOT`.

`AND`, `OR`, and `NOT` are included because they cost almost nothing and are firmly
inside the 80/20. `AVG` is an alias for `AVERAGE` because the project's own vocabulary
uses that spelling.

**Deferred to Milestone 2:** `LOWER`, `UPPER`, `LEN`, `LEFT`, `RIGHT`, `CONCAT`, `TODAY`,
`NOW`. `TODAY` and `NOW` additionally require a volatility concept that the full-recalc
model does not otherwise need.

### 5.5 Recalculation

```rust
pub fn recalc(wb: &mut Workbook)
```

One synchronous pass over every cell. For each formula cell, evaluation depth-first
resolves dependencies using a per-pass memo and a **visiting set**. Re-entering a cell
already on the current evaluation stack is a cycle: every cell on that stack is marked
`#CIRCULAR!`, not just one arbitrary cell.

There is no dependency graph, no incremental invalidation, and no iteration on
circular references.

**Two deliberate escape hatches**, so that adopting a dirty-set engine in Milestone 2 is a
single-module change rather than a cross-cutting one:

1. Every caller reaches recalculation through the one `recalc(&mut Workbook)` function.
   Replacing its body is confined to `formula/recalc.rs`.
2. The workbook maintains a formula-cell count, logged once when it exceeds 5,000. That
   log line is the signal that full recalc is becoming the bottleneck, rather than the
   symptom being discovered later as unexplained input lag.

---

## 6. Editing, Undo, and Clipboard

### 6.1 Modes

The grid is in exactly one of two modes.

- **Navigating** — selection, movement, clipboard, commands.
- **Editing** — the formula bar holds a real text cursor for one cell.

**The formula bar is the only input surface.** Typing a printable character on the grid,
pressing `F2`, or double-clicking does not open an in-cell editor; it focuses the formula bar
and loads the target cell into it. One caret-bearing widget means `Backspace`, `Delete`,
`Home`/`End`, arrow keys within the text and mouse selection are egui's problem rather than
ours, and there is no second editor that could disagree with the first.

**Typing writes the cell immediately.** There is no pending commit: every keystroke updates
the model, so `Enter` and `Tab` only *move*. `Esc` restores a snapshot taken when the field
was first touched in this visit.

The bar's height follows its content, so a long value or a multi-line Lua chunk stays
readable. It grows up to a cap and scrolls past it, rather than eating the window.

| Key | Action |
|---|---|
| `Enter` | Move down |
| `Shift+Enter` | Insert a newline |
| `Tab` | Move right |
| `Shift+Tab` | **Not wired.** egui's multiline filter excludes `Tab` from the widget entirely, so it never reaches the field; the app does not handle it either. Inert |
| `Esc` | Restore the pre-edit snapshot, drop focus |

Moving up is the up arrow. `Shift+Enter` no longer commits and moves up: a multi-line field
in which one chord both inserts a newline and moves the cursor is incoherent, and newlines are
what Milestone 2's `LUA=` chunks need first. This is the one behaviour traded away here.

### 6.2 Operations and undo

```rust
struct Op {
    label: &'static str,
    changes: Vec<(CellRef, Cell, Cell)>,   // (ref, before, after)
}
```

Before/after stores the **whole** `Cell` — input, value, and format — so a formatting
change and a value change are the same operation type and nothing special-cases them.

**One operation per gesture.** A drag-fill across 500 rows produces a single `Op`
containing 500 changes, so one `Ctrl+Z` undoes the entire fill rather than one cell.

Applying an operation in either direction sets the affected cells and then triggers one
full recalculation. Because recalculation is always derived from the current cell inputs,
undo cannot leave cached values out of sync with formulas.

The undo stack holds 200 entries; the oldest is dropped beyond that.

### 6.3 Clipboard

The **internal clipboard** holds a rectangular block of whole cells, so copy and paste
are lossless in both directions — formulas, values, and formatting all travel.

**System clipboard** integration is best-effort on top of it:

- **Copy** emits TSV of *displayed* values.
- **Paste** parses TSV into cells, translating any leading `=` formula relative to the
  paste origin.

A formula does **not** round-trip through the system clipboard; doing so would lose
formatting. Internal copy/paste is the lossless path.

---

## 7. User Interface

```
┌──────────────────────────────────────────────┐
│ Sheet1 │ Sheet2 │ +                          │
├──────────────────────────────────────────────┤
│    │ A     │ B       │ C                   │
│  1 │ Name  │ Value   │ 3× Price            │
│  2 │ Rent  │ 1200    │ 3600                │
│  … │  …    │  …      │  …    ← grid        │
├──────────────────────────────────────────────┤
│ A1 ● │ =VALUE*3                             │  ← formula bar, grows with its content
└──────────────────────────────────────────────┘
```

No title strip and no ribbon in this slice: the ribbon arrives with the formatting UI, and a
heading that says only "Omasheet" costs a row of grid height to say nothing the window manager
does not already say. The modified marker rides in the formula bar's name box, next to the
address it refers to.

The ribbon itself departs from the original Omawrite-inspired sketch, which specified no
ribbon. When it arrives with the formatting UI it will be **collapsible and collapsed by
default**, toggled by `Ctrl+Shift+R`, so the low-chrome mode stays one keystroke away. It
is not built yet, and §7.3 lists `Ctrl+Shift+R` as not-yet-built; the paragraph above is
the reason it will be collapsible rather than permanent, not a claim that it exists.

### 7.1 Grid

- Renders only the visible rows and columns plus a small overscan. This is what keeps
  60fps at the scale target. The visible window is pure arithmetic extracted from the
  painter, so virtualisation is unit-tested rather than only observable by eye.
- A drag-fill handle on the selection's bottom-right corner. Dragging it in either axis
  fills the swept rectangle as a single undo operation.
- Frozen row and column headers. The header gutters are reserved *outside* the scrolling
  body, so the letters and numbers stay put while the cells scroll beneath them.
- The selection cannot be moved outside the scrollable extent, so the highlight is always
  reachable. **Keeping it scrolled into view is deferred**: the selection may sit off-screen
  until the user scrolls, rather than the view following the selection automatically.
- Uniform default column width, with per-column overrides; autosize on double-clicking
  a header edge.
- Editing happens in the formula bar, which is the only input surface. Typing on the grid,
  `F2` and double-click all focus it rather than opening an in-cell editor — see §6.1.

### 7.2 Ribbon

Exposes exactly what the project requires of Milestone 1 formatting:

- Number format: General, Number, Money, Percent, Text, Date
- Font colour
- Background colour
- Bold, italic, underline
- Alignment: Left, Center, Right

Every field of `CellFormat` is reachable from the ribbon. A format the UI cannot set is a
format the model should not carry.

### 7.3 Keyboard

| Key | Action |
|---|---|
| Arrows | Move selection |
| `Ctrl+↑` / `Ctrl+↓` | Jump to data edge |
| `Ctrl+Home` / `Ctrl+End` | Corners of the used range |
| `PageUp` / `PageDown` | Move by viewport |
| `Delete` / `Backspace` | Clear contents |
| `F2` | Focus the formula bar, seeded with the cell |
| `Enter` / `Tab` | Move down / right; the cell is already written |
| `Esc` | Restore the pre-edit snapshot and drop focus |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Clipboard |
| `Ctrl+Z` / `Ctrl+Y` | Undo / redo |
| `Ctrl+D` / `Ctrl+R` | Fill down / fill right |
| `Ctrl+S` | Save as `.xlsx` |
| `Ctrl+O` | Open |
| `Ctrl+E` | Export |
| `Ctrl+B` / `Ctrl+I` / `Ctrl+U` | Bold / italic / underline |
| `Ctrl+Shift+R` | Toggle ribbon |

**Built in the first core slice:** arrows, `Shift`+arrows, `Home` (column only),
`Delete`, `Backspace`, `F2`, `Enter`, `Tab`, `Shift+Enter`, `Esc`.
**Not yet built:** every `Ctrl` binding, `PageUp`/`PageDown`, `Ctrl+Arrow`,
`Ctrl+Home`/`End`, and `Shift+Tab` — these arrive with the formula, clipboard, and sheet-tab
work. The table above describes the finished application, not this slice. `Shift+Tab` is the
one that needs a ruling: §6.1 once promised it as a newline, but a newline at the caret needs
caret access the bar does not own, and `return_key` accepts only one chord per key. It is
inert for now rather than pretending otherwise. Note that nothing has to *claim* it: egui's
multiline `TextEdit` filters the `Tab` key out of its own event list, so `Tab` and `Shift+Tab`
reach only the app. The app uses `Tab` to move right and ignores the shifted form.

---

## 8. File I/O

### 8.1 Opening a CSV

- Delimiter sniffed from `,` `;` `\t` `|`. The `csv` crate (1.4) performs the quoting,
  escaping and delimiter handling; Omasheet owns sniffing and number inference, because
  both need the raw field text the crate returns verbatim. Sniff by parsing the first
  20 rows with each candidate and scoring each on `(total fields, agreements)`.
  **Total fields is primary** — a delimiter that does not appear splits every line into
  one field — while agreement is only a tie-break, because genuinely ragged files have
  inconsistent widths and must not lose to a wrong delimiter. Exact ties fall back to
  candidate order, so a comma wins them.
- UTF-8, with or without BOM.
- RFC-4180 quoting, including escaped `""` inside quoted fields.
- Both LF and CRLF line endings.
- **Row 1 is data, not a header.** Omasheet has no header concept, which keeps the
  CSV-to-grid mapping lossless.

**Type inference: numbers only, and only unambiguous ones.** A field becomes a `Number`
if and only if all three conditions hold:

1. It parses as `f64`.
2. The result is finite — which rejects `NaN`, `inf`, and `-inf`, all of which Rust's
   `f64::from_str` otherwise accepts.
3. Its integer part has no redundant leading zero, so `0` and `0.5` qualify but `00123`
   does not.

Everything else stays `Text`. There is no date inference and no boolean inference.

Condition 3 is the whole point. A bare "parses as a number" rule would satisfy the
literal wording above while silently turning `00123` into `123` — which is exactly how a
spreadsheet eats data, and it becomes permanent the moment the file is saved as xlsx.
The three conditions together give the behaviour the wording was reaching for: `3.50`
becomes `3.5`, while a leading-zero identifier stays text.

### 8.2 Opening an xlsx

Verified against `calamine` 0.36.1 source:

| Capability | Available | How |
|---|---|---|
| Cell values | Yes | `Data` enum |
| Formulas | Yes | `worksheet_formula()` returns `Range<String>`; shared formulas expanded |
| Number formats | Yes | `read_styles` parses `numFmtId` per `xf` |
| Merged regions | Yes | `load_merged_regions()` |
| Font colour | **No** | absent from the parse |
| Background colour | **No** | absent from the parse |
| Bold / italic / underline | **No** | absent from the parse |
| Alignment | **No** | absent from the parse |

`calamine`'s `Data` enum has no `Formula` variant; formulas are read through the separate
`worksheet_formula()` reader.

**Accepted Milestone 1 gap:** styling imported from another application's xlsx files is
not displayed and is not preserved on save. When a file is opened and its styles could not
be fully imported, a single non-blocking line appears in the status area saying so. Colors
and fonts that Omasheet itself applies are written correctly and survive a round trip.

Writing a minimal OOXML style reader to recover font colour, fill colour, bold, italic,
and alignment from `styles.xml` is a **Milestone 2 target**.

### 8.3 Saving

- **xlsx origin** — write in place, with no confirmation, because the dirty indicator
  already told the user something changed.
- **CSV origin** — write `<name>.xlsx` beside the original CSV. If a file already exists
  at that path and it is **not** the workbook that was opened, prompt for confirmation
  before overwriting. Overwriting a file this session did not open is the one destructive
  action the application takes, so it is never silent.
- **Never opened or from a `Save As` destination** — same confirmation rule as above.

The originally opened CSV is never modified, in any case. There is exactly one save path
and no lossy-write warning to understand.

`rust_xlsxwriter` is used for output: sheets, values, formulas **with cached values** so
that a reader which does not calculate still displays numbers, number formats, fonts,
fills, and alignment.

### 8.4 Export

- **CSV** — one file per sheet, containing displayed (formatted) values. For a workbook
  with `Sheet1`, `Sheet2`, `Sheet3`, the export produces `Sheet1.csv`, `Sheet2.csv`,
  `Sheet3.csv` in a chosen directory.
- **Markdown** — a single document, one `# SheetName` heading per sheet followed by a
  table, with `|` escaped as `\|`.

---

## 9. Testing

TDD from the first module onward. The `formula/`, `workbook/`, and `io/` modules have no
egui dependency and are tested headlessly.

| Module | Coverage |
|---|---|
| `formula/lexer`, `formula/parser` | Tokenisation, operator precedence against Excel's table, `$` flag parsing, parse-failure preserves source |
| `formula/ast` | `translate` for all four flag combinations, range endpoints, `to_src` round-trip |
| `formula/eval` | Every function, empty-cell semantics, error propagation, `IF` short-circuit |
| `formula/recalc` | Cycle detection marks the whole cycle, memo correctness, error values persist |
| `workbook/` | Sparse storage invariants, `used` range maintenance, undo/redo round-trips over seeded random operation sequences |
| `io/` | Golden round-trips (write xlsx, read back, compare); CSV fixtures for quotes, CRLF, BOM, embedded delimiters, numbers-only inference |
| `app/` | Headless egui harness for keybindings where practical |
| **Performance** | A 10,000-cell CSV fixture asserting a full load-and-paint pass completes within one 16 ms frame. Measured warm at 0.12–0.23 ms; cold paint (font rasterisation, paid once at launch) is 3.6–8.0 ms in a debug build and is printed rather than asserted. The recalculation half of the original promise lands with the formula engine |

The performance assertion is a gate, not an aspiration. If it fails, the slice does not
ship.

---

## 10. Build Order

Each step produces working software.

1. **Skeleton** — `cargo init`, eframe window, empty 100x26 grid. Verifies the toolchain
   and Wayland display before any engine code exists.
2. **Storage** — sparse `Sheet`/`Cell`, `used` range maintenance, CSV read.
3. **Grid render and navigation** — a real CSV on screen with frozen headers and selection.
4. **Editing** — in-cell editing, commit keys, dirty-state tracking.
5. **Formulas** — lexer, parser, evaluation, full recalc, errors, cycle detection.
6. **Cross-sheet** — sheet tabs, add/rename/delete, `Sheet2!A1` references.
7. **Undo, clipboard, fill** — inverse operations, reference translation.
8. **xlsx write and save** — the `Ctrl+S` path, cached formula values.
9. **xlsx read, open dialog, recent files** — including the import-warning path.
10. **Formats and ribbon** — format rendering, `Ctrl+Shift+R` toggling, colour pickers.
11. **Export, find and replace, polish, performance gate.**

---

## 11. Deferred to Milestone 2 and Beyond

**Milestone 2**

- Scroll-to-selection: the view does not yet follow the active cell, so keyboard
  navigation can leave the highlight off-screen. The selection is clamped to the
  scrollable extent, which prevents losing it, but not needing to scroll to find it.
- Headless pointer-event testing. `egui::Context::run_ui` does not simulate pointer input —
  verified: a plain `ui.button` also fails to report `clicked()` under it — so
  `response.clicked()` cannot be asserted without a window or `egui_kittest`, which
  requires egui 0.36 and therefore rustc 1.95. The pointer-to-selection mapping is
  extracted into a pure function and tested; the egui call itself is not.

- Lua engine and sandbox (`LUA=` formulas, cell/column/row/cross-sheet context)
- Conditional formatting, in both Excel and Lua rule forms
- Multi-line cell content (required by `LUA=` chunks)
- Minimal OOXML style reader, to close the xlsx import fidelity gap
- `LOWER`, `UPPER`, `LEN`, `LEFT`, `RIGHT`, `CONCAT`, `TODAY`, `NOW`

**Later**

- Dirty-set recalculation via a dependency graph, when the formula-count log fires
- Row and column insert/delete
- Date type inference
- Autosave recovery, recent files, multiple windows
- Export formats beyond CSV and Markdown

---

## 12. Risks

| Risk | Mitigation |
|---|---|
| Full recalc degrades as workbooks grow | Formula count logged past 5,000; `recalc()` is already isolated so the swap is one module |
| egui immediate-mode grid is slow at the edges | Virtualised rendering plus an explicit 10k-cell / 16 ms test gate |
| xlsx import silently loses styling | Warning shown on open; gap documented; Milestone 2 target |
| Formula and value caches drift apart | Caches are always fully derived by `recalc()`; no incremental update path exists to drift |
| Drag-fill produces hundreds of undo entries | Operations are per gesture, verified by test |
| CSV round-trip loses type information | Numbers-only inference, and CSV is never written back by `Ctrl+S` |