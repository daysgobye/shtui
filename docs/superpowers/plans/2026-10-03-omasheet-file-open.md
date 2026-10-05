# File Open Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The user can open a CSV or TSV file with `Ctrl+O`, see its name in the window title, and get a clear error when a file cannot be read.

**Architecture:** `Ctrl+O` spawns an `rfd` async dialog and stores the returned future in `OmaApp`. Each frame `body` polls that future with a no-op waker; when it resolves, the path is read through the existing `io::csv_read` reader and loaded into the workbook. Nothing blocks the UI thread.

**Tech Stack:** Rust 2024, `rfd = "=0.17.2"`, the existing `eframe`/`egui` 0.35.0 and `csv` 1.4.0.

**Spec:** `docs/superpowers/specs/2026-10-03-omasheet-m1-design.md`

## Global Constraints

- `eframe`, `egui` pinned `=0.35.0`; add `rfd = "=0.17.2"`. Do not unpin. This machine has **rustc 1.94.0**.
- `workbook/` and `io/` must have **zero** egui dependency. Only `src/ui/` and `src/app/` may import egui or rfd.
- No `unwrap()`/`expect()` in library code. Tests may use them.
- `cargo clippy --all-targets` must produce no output. Repo is `cargo fmt`-clean.

## Verified facts about `rfd` 0.17.2 — do not re-derive

Established by compiling and reading source:

- Builds clean on rustc 1.94.0; clippy silent.
- Default features are `xdg-portal` + `wayland`. **No GTK.** `org.freedesktop.portal.Desktop` is running on this session, so the portal backend can show a dialog.
- `FileDialog::pick_file()` is **blocking** and would freeze the window for as long as the dialog is open.
- `AsyncFileDialog::pick_file()` returns `impl Future<Output = Option<FileHandle>>`, and `FileHandle` derefs to the path. Polling it from the egui frame loop keeps us on the main thread, which is what the async backend requires.
- Therefore: **spawn async, poll per frame.** Never call the blocking variant.

---

## Task 1: Async dialog plumbing

**Files:**
- Modify: `Cargo.toml`
- Create: `src/app/open_dialog.rs`
- Create: `tests/open_dialog.rs`

**Interfaces:**
- Consumes: `std::future::Future`, `std::pin::Pin`.
- Produces: `pub struct OpenDialog { pending: Option<Pin<Box<dyn Future<Output = Option<PathBuf>>>>> }` with `pub fn spawn(&mut self)`, `pub fn is_open(&self) -> bool`, `pub fn poll(&mut self) -> Option<Option<PathBuf>>`, and `pub type PollResult = Option<Option<PathBuf>>`.

`poll` returning `None` means "still open"; `Some(None)` means the user cancelled; `Some(Some(path))` means they chose a file. That nesting is awkward — document it, and consider a small enum instead if it reads better:

```rust
pub enum OpenOutcome { Chosen(PathBuf), Cancelled }
/// `None` while the dialog is still open.
pub fn poll(&mut self) -> Option<OpenOutcome>
```

Prefer the enum. The whole point is that `OmaApp` can match on it without nesting `Option`s.

**Steps**

1. Write a failing test that `OpenDialog::poll` returns `None` before anything is spawned, and that spawning twice does not leak the first future.
2. `spawn` builds the dialog with filters for CSV and TSV, and stores the boxed future. If one is already pending, do nothing — a second `Ctrl+O` while a dialog is open must not orphan the first.
3. `poll` takes the future out of the slot, polls it once with a no-op waker, and puts it back if still pending. Implement the waker locally — about ten lines:

```rust
struct NoopWaker;
impl std::task::Wake for NoopWaker {
    fn wake(self: std::sync::Arc<Self>) {}
}
```

4. Tests, clippy, fmt, commit.

**Filters:** offer only what can actually be read. `csv` and `tsv`. **Not `xlsx`** — xlsx reading is a later milestone, and offering a format that opens into an error is worse than not offering it.

---

## Task 2: Wire it into the app, and set the window title

**Files:**
- Modify: `src/app/app.rs`
- Modify: `src/main.rs`
- Modify: `tests/formula_bar.rs` or a new `tests/open_file.rs`

**Interfaces:**
- Consumes: `OpenDialog`, `OpenOutcome`, `Workbook::load_sheet`, `io::csv_read::read_csv_file`, `OmaApp::error`.
- Produces: `OmaApp::open_dialog: OpenDialog`, `OmaApp::path: Option<PathBuf>`, and `pub fn window_title(&self) -> String`.

**Behaviour**

- `Ctrl+O` spawns the dialog. Read the chord with `ui.input(|i| i.pressed(Key::O) && i.modifiers.ctrl)`.
- Each frame, poll the dialog. On `Chosen(path)`, read the file and `load_sheet` it; on failure set `self.error` to the same legible message `open_csv` already produces.
- Set `self.path` so the title can show it.
- The window title becomes the file's **file name**, not its full path — a title bar is not a place for a 90-character path. Before a file is open, show `Omasheet`. Prefix with `●` when `Workbook::dirty()`.

```rust
/// What the window manager shows. The file name rather than the full path: a title
/// bar is not somewhere to put a ninety-character path, and the full path is still
/// reachable from the open dialog.
pub fn window_title(&self) -> String {
    let name = self
        .path
        .as_ref()
        .and_then(|p| p.file_name())
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| "Omasheet".to_owned());
    if self.wb.dirty() { format!("● {name}") } else { name }
}
```

Push it with `ui.ctx().send_viewport_cmd(egui::ViewportCommand::Title(title))`. Set it in `body`, not once in `main`, because it changes as files are opened and the workbook becomes dirty.

**Opening over unsaved changes.** If `Workbook::dirty()` and the user picks a file, their unsaved work is discarded silently. This is the first destructive action the app can take, so confirm it first: keep `OpenOutcome::Chosen` staged in `OpenDialog` until the app has decided, and while a confirmation is showing, do not spawn another dialog. Keep this simple — a boolean plus the staged path is enough. Do not build a modal dialog framework for it.

**Tests:** driving the *portal* is not possible headlessly, so test everything up to the dialog boundary: that `Ctrl+O` spawns, that a `Chosen` outcome loads the file and sets `path`, that a bad path sets `error`, that `window_title` reflects the file name and the dirty dot, and that `Ctrl+O` while dirty stages a confirmation rather than loading.

---

## Non-Goals

- xlsx reading (a later milestone)
- Recent files, a File menu, drag-and-drop
- Save. `Ctrl+S` and the xlsx writer are a later milestone.

## Known Risks

- The portal dialog cannot be exercised headlessly, so **the one thing most likely to be wrong — that the dialog actually appears — is verified by hand only.** Run the app and press `Ctrl+O`. Do not report this task done on tests alone.
- `rfd`'s portal backend shows the dialog as a separate window owned by the portal, so it may not follow the parent window's position. Acceptable; note it if observed.
