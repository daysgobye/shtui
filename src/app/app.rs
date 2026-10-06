use std::path::PathBuf;

use eframe::egui::{self, Key, Rect, Ui, UiBuilder, pos2};

use crate::app::open_dialog::{OpenDialog, OpenOutcome};
use crate::ui::formula_bar::{self, FormulaBar, Navigate};
use crate::ui::grid::{self, GridState, Selection, Step};
use crate::workbook::Workbook;
use crate::workbook::sheet::Sheet;

pub struct OmaApp {
    pub wb: Workbook,
    pub grid: GridState,
    pub bar: FormulaBar,
    pub error: Option<String>,
    /// The file that is open, for the window title. `None` until the first successful
    /// open, and left alone by a failed one.
    pub path: Option<PathBuf>,
    pub open_dialog: OpenDialog,
    /// The title last handed to the window manager, so an unchanged one is not pushed
    /// again. `None` until the first frame has sent one.
    pub last_title: Option<String>,
}

impl Default for OmaApp {
    fn default() -> Self {
        Self {
            wb: Workbook::new(),
            grid: GridState::new(),
            bar: FormulaBar::new(),
            error: None,
            path: None,
            open_dialog: OpenDialog::new(),
            last_title: None,
        }
    }
}

impl OmaApp {
    /// Point every sheet-qualified reference the UI holds at the active sheet.
    ///
    /// The grid's selection carries a `CellRef`, which is sheet-qualified by design so
    /// cross-sheet references need no special case. That makes it go stale the moment the
    /// active sheet changes: a highlight would stop painting because it is compared
    /// against a freshly qualified `CellRef`, and `remove_sheet` re-stamps every surviving
    /// sheet id, so a stale reference can silently come to mean a different sheet. One
    /// call here requalifies it, and the formula bar follows by comparison: it reloads
    /// from whatever cell it is handed, so the two cannot disagree.
    ///
    /// This is called once per frame from [`OmaApp::body`] rather than from each of
    /// `Workbook`'s three mutators, so no route to a changed active sheet can be missed.
    /// The cost is two integer writes. The trade-off it makes: selection position is not
    /// remembered per sheet. When sheet tabs land and per-sheet selection is wanted, this
    /// is the one line to replace — save the old corners keyed by sheet id before calling.
    pub fn retarget_selection_to_active_sheet(&mut self) {
        let sheet = self.wb.active_id();
        let Selection {
            mut active,
            mut anchor,
        } = self.grid.selection;
        active.sheet = sheet;
        anchor.sheet = sheet;
        self.grid.selection = Selection { active, anchor };
    }

    /// Replace the active sheet's contents, preserving its identity so that
    /// selection and scroll position stay meaningful.
    pub fn load_sheet(&mut self, sheet: Sheet) {
        let id = self.wb.active_id();
        self.wb.replace_sheet(id, sheet);
    }

    /// Read a CSV into the active sheet and record it as the file that is open.
    ///
    /// A parse failure surfaces in `error` rather than aborting, so a bad file never
    /// closes the window, and it leaves `path` alone: a file that was never read is not
    /// the file that is open, and the title should keep naming what is.
    pub fn open_csv(&mut self, path: &std::path::Path) {
        match crate::io::csv_read::read_csv_file(path) {
            Ok(sheet) => {
                self.load_sheet(sheet);
                // The field has to be loaded here, because nothing downstream can see
                // that the open happened. `formula_bar::show`'s reload gate fires on a
                // changed `CellRef` or on an untouched field, and neither is true across
                // an open: `replace_sheet` keeps the sheet id, the selection keeps its row
                // and column, and a workbook dirty enough to have been asked about is dirty
                // because the user typed — which is exactly the state that leaves `touched`
                // set. So the gate stays shut and a half-typed value sits in the field over
                // a cell that has just changed underneath it, where the next `Esc` writes it
                // back (`Esc` restores `original`) or the next keystroke appends to it. Both
                // land in the file the user just opened. `load_from` touches no focus state,
                // so the caret survives this.
                self.bar.load_from(&self.wb, self.grid.selection.active);
                // `replace_sheet` marks the workbook dirty because putting new contents
                // into a workbook is a change. Opening a file is the other thing: what is
                // on screen now *is* what is on disk, so it is not unsaved work. Left
                // marked, the title would show the modified dot from the instant the file
                // appeared, and the next `Ctrl+O` would ask to discard changes nobody made.
                self.wb.clear_dirty();
                self.error = None;
                self.path = Some(path.to_owned());
            }
            Err(e) => self.error = Some(format!("could not read {}: {e}", path.display())),
        }
    }

    /// What the window manager shows.
    ///
    /// The file's name rather than its path: a title bar is not somewhere to put a
    /// ninety-character path, and the full path is one click away in the dialog the user
    /// just came from. `Omasheet` before anything is open. The leading dot marks
    /// [`Workbook::dirty`], which is workbook-wide — so it appears on an unsaved sheet
    /// with no file behind it too, because it reports the workbook and not the file.
    pub fn window_title(&self) -> String {
        let name = self
            .path
            .as_ref()
            .and_then(|p| p.file_name())
            .map(|n| n.to_string_lossy().into_owned())
            // A path with no file name in it — the filesystem root, or a trailing slash —
            // would otherwise leave the title bar blank, which reads as a broken window.
            .unwrap_or_else(|| "Omasheet".to_owned());
        if self.wb.dirty() {
            format!("\u{25cf} {name}")
        } else {
            name
        }
    }

    /// Open the staged path, discarding whatever the workbook holds now.
    ///
    /// The only route that replaces unsaved work, and reachable only once the user has
    /// answered the question. A no-op with nothing staged, so there is no way to call it
    /// into discarding a workbook by accident.
    pub fn confirm_open(&mut self) {
        if let Some(path) = self.open_dialog.take_staged() {
            self.open_csv(&path);
        }
    }

    /// Leave the workbook alone and drop the question. The unsaved work stays unsaved,
    /// which is what "keep editing" means.
    pub fn decline_open(&mut self) {
        self.open_dialog.take_staged();
    }

    /// `Ctrl+O`, the dialog's answer, and the title.
    ///
    /// Split out of [`OmaApp::body`] because it is the app's only input that is not the
    /// grid or the bar, and because the ordering inside it is the whole of the open
    /// route: poll, then spawn, then ask, then title.
    fn open(&mut self, ui: &Ui) {
        // The answer before the request. Applying last frame's outcome before reading
        // this frame's chord is what lets a `Ctrl+O` land on the frame a dialog closes
        // and open the next one, rather than being swallowed by the dialog that was
        // already up when it was pressed.
        match self.open_dialog.poll() {
            None | Some(OpenOutcome::Cancelled) => {}
            Some(OpenOutcome::Chosen(path)) => {
                if self.wb.dirty() {
                    // Held rather than opened: this is the app's first destructive act,
                    // and a file chosen over unsaved work is a question, not a command.
                    self.open_dialog.stage(path);
                } else {
                    self.open_csv(&path);
                }
                // The frame after this one, asked for here rather than left to the
                // interval below. That interval only runs while a dialog is up, and this
                // frame is the one that ends it, so nothing else would schedule a
                // follow-up — and there is work a further frame settles: `body` reads the
                // bar's height at the top to place the grid and `formula_bar::show`
                // writes it at the bottom, so the field was laid out for the value it
                // used to hold.
                ui.ctx().request_repaint();
            }
        }

        // Refused rather than queued while the route is busy. A second dialog over a
        // question already on screen would leave two questions about one workbook with
        // no way to answer the first.
        // `key_pressed`, not the `pressed` the plan names: egui 0.35 spells it this way,
        // and the rest of the app reads keys the same way, so the chord is not the one
        // place in the app that has to know which of the two this egui version has.
        let asked = ui.input(|i| i.key_pressed(Key::O) && i.modifiers.ctrl);
        if asked && !self.open_dialog.is_busy() {
            self.open_dialog.spawn();
        }

        self.show_open_confirmation(ui);

        if self.open_dialog.is_open() {
            // Nothing will wake egui when the portal answers. The dialog's future is
            // polled with a no-op waker — see `open_dialog`'s module docs — and egui
            // only draws a frame when something asks it to, so without this the answer
            // would sit in the future until the user happened to move the mouse.
            //
            // `request_repaint_after`, not `request_repaint`: the latter means "draw
            // another frame right after this one", so it is 60 Hz for as long as the user
            // is looking at the portal's window, re-tessellating a grid that nobody is
            // watching. The only thing a shorter interval buys is how long the answer can
            // sit before it is noticed, and 50 ms is below the threshold at which that is
            // visible — so twenty polls a second against sixty, for the same result.
            ui.ctx()
                .request_repaint_after(std::time::Duration::from_millis(50));
        }

        // Pushed when it changes rather than set once in `main`: the title changes when a
        // file is opened and when the workbook becomes dirty, and neither happens on a
        // frame the app gets to choose. Compared against the last one pushed because a
        // `ViewportCommand::Title` is a message to the compositor, and one per frame for
        // as long as the window is open — the same sixty a second as above — to say
        // something that did not change.
        let title = self.window_title();
        if self.last_title.as_deref() != Some(title.as_str()) {
            self.last_title = Some(title.clone());
            ui.ctx()
                .send_viewport_cmd(egui::ViewportCommand::Title(title));
        }
    }

    /// The question asked before unsaved work is discarded.
    ///
    /// Two buttons and a window, which is all this needs: there is one question, it is
    /// asked at most once, and a framework for dialogs would be a larger thing than the
    /// feature it serves.
    fn show_open_confirmation(&mut self, ui: &Ui) {
        let Some(path) = self.open_dialog.staged() else {
            return;
        };
        // The name rather than the path, for the same reason the title bar uses one.
        let name = path
            .file_name()
            .map(|n| n.to_string_lossy().into_owned())
            .unwrap_or_else(|| path.display().to_string());

        let mut discard = false;
        let mut keep = false;
        egui::Window::new("Unsaved changes")
            .collapsible(false)
            .resizable(false)
            .anchor(egui::Align2::CENTER_CENTER, egui::Vec2::ZERO)
            .show(ui.ctx(), |ui| {
                ui.label(format!(
                    "Opening {name} will discard the changes you have made."
                ));
                ui.horizontal(|ui| {
                    discard = ui.button("Discard changes").clicked();
                    keep = ui.button("Keep editing").clicked();
                });
            });

        // Read after the window is drawn, so the frame the button lands on is the frame
        // it is acted on. The window is an `Area`, not a focusable widget, so it takes
        // no keyboard and typing goes on reaching the bar while the question is up.
        if discard {
            self.confirm_open();
        } else if keep {
            self.decline_open();
        }
    }

    /// The UI body. Split from `eframe::App::ui` so headless tests can drive it.
    pub fn body(&mut self, ui: &mut egui::Ui) {
        // Before anything reads the selection this frame, including the grid's own
        // navigation and painting. See `retarget_selection_to_active_sheet`.
        self.retarget_selection_to_active_sheet();
        // Ahead of the error label and the grid, so a file opened this frame is the one
        // that gets drawn on it rather than the one that was there when the frame began.
        self.open(ui);

        // No title strip. "Omasheet" said nothing the window manager does not already say,
        // and a heading costs a row of vertical space the grid could use. The modified
        // marker lives in the formula bar's name box instead, which is where a spreadsheet
        // keeps it and where it sits next to the address it refers to.
        if let Some(err) = &self.error {
            ui.colored_label(egui::Color32::from_rgb(220, 90, 90), err);
        }

        // Whether the bar holds the keyboard has to be settled before the grid reads any
        // key: a `Backspace` must be either a deletion in the field or a deletion of the
        // cell, never both.
        let bar_owns_input = self.bar.focused(ui);

        // The two share the panel rather than being stacked by whatever order they were
        // added in, because the bar's height is not known until the text in it is: the
        // grid is given the remainder, so a long value shrinks the grid instead of
        // pushing the bar off the bottom of the screen.
        let full = ui.available_rect_before_wrap();
        let bar_top = (full.bottom() - self.bar.min_height()).max(full.top());
        let mut grid_ui = ui.new_child(
            UiBuilder::new()
                .id_salt("grid")
                .max_rect(Rect::from_min_max(full.min, pos2(full.right(), bar_top))),
        );
        grid::show(&mut self.grid, &mut self.wb, bar_owns_input, &mut grid_ui);

        // The grid's gesture is complete; where the focus goes is this layer's business.
        if std::mem::take(&mut self.grid.focus_requested) {
            self.bar.request_focus();
        }
        let bar_rect = Rect::from_min_max(pos2(full.left(), bar_top), full.max);
        let mut bar_ui = ui.new_child(
            UiBuilder::new()
                .id_salt("formula-bar")
                .max_rect(bar_rect)
                .layout(egui::Layout::top_down(egui::Align::LEFT)),
        );
        // Read before the borrow, not inline in the call: `show` takes `&mut wb` and
        // `dirty` reads it. `full` is passed as well as read here because the bar's cap is
        // a share of this rect, not of the window: `bar_top` above is derived from it, so
        // a cap measured from anywhere taller could outgrow the panel and leave the grid
        // no height.
        let dirty = self.wb.dirty();
        formula_bar::show(
            &mut self.bar,
            &mut self.wb,
            self.grid.selection.active,
            dirty,
            full,
            &mut bar_ui,
        );

        // Applied after the bar has been drawn, so a move made this frame is loaded into
        // the field on the next one rather than overwritten by this frame's text.
        if let Some(navigate) = self.bar.take_navigate() {
            let step = match navigate {
                Navigate::Down => Step::Down,
                Navigate::Right => Step::Right,
            };
            grid::step_active(&mut self.grid, &self.wb, step);
            ui.ctx().request_repaint();
        }
    }
}

impl eframe::App for OmaApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        self.body(ui);
    }
}
