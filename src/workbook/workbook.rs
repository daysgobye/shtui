//! Workbook container: the sheet registry, the active sheet, and the dirty flag.

use super::cell::{Cell, CellFormat, CellRef, CellValue, SheetId};
use super::sheet::Sheet;

/// A workbook. Always has at least one sheet.
#[derive(Debug)]
pub struct Workbook {
    sheets: Vec<Sheet>,
    active: SheetId,
    dirty: bool,
}

impl Default for Workbook {
    fn default() -> Self {
        Self::new()
    }
}

impl Workbook {
    /// A workbook with one sheet named `Sheet1`.
    pub fn new() -> Self {
        Self {
            sheets: vec![Sheet::new(0, "Sheet1")],
            active: 0,
            dirty: false,
        }
    }

    pub fn sheet_count(&self) -> usize {
        self.sheets.len()
    }

    pub fn sheet(&self, id: SheetId) -> &Sheet {
        &self.sheets[id]
    }

    pub fn sheet_mut(&mut self, id: SheetId) -> &mut Sheet {
        &mut self.sheets[id]
    }

    pub fn sheets(&self) -> &[Sheet] {
        &self.sheets
    }

    pub fn sheet_name(&self, id: SheetId) -> &str {
        self.sheets[id].name()
    }

    pub fn active_id(&self) -> SheetId {
        self.active
    }

    pub fn active(&self) -> &Sheet {
        &self.sheets[self.active]
    }

    pub fn active_mut(&mut self) -> &mut Sheet {
        &mut self.sheets[self.active]
    }

    pub fn set_active(&mut self, id: SheetId) {
        if id < self.sheets.len() {
            self.active = id;
        }
    }

    /// Add a sheet named `name`. Names must be unique and non-empty.
    pub fn add_sheet(&mut self, name: &str) -> bool {
        if name.is_empty() || self.sheets.iter().any(|s| s.name() == name) {
            return false;
        }
        let id = self.sheets.len();
        self.sheets.push(Sheet::new(id, name));
        self.active = id;
        self.dirty = true;
        true
    }

    /// Rename a sheet. Refuses empty names, duplicates, and unknown ids.
    pub fn rename_sheet(&mut self, id: SheetId, name: &str) -> bool {
        if id >= self.sheets.len() || name.is_empty() {
            return false;
        }
        if self
            .sheets
            .iter()
            .enumerate()
            .any(|(i, s)| i != id && s.name() == name)
        {
            return false;
        }
        self.sheets[id].set_name(name);
        self.dirty = true;
        true
    }

    /// Remove a sheet, never the last one. The active sheet becomes the first
    /// remaining sheet.
    pub fn remove_sheet(&mut self, id: SheetId) -> bool {
        if id >= self.sheets.len() || self.sheets.len() == 1 {
            return false;
        }
        self.sheets.remove(id);
        for (i, s) in self.sheets.iter_mut().enumerate() {
            s.id = i;
        }
        self.active = 0;
        self.dirty = true;
        true
    }

    pub fn cell(&self, at: CellRef) -> Option<&Cell> {
        self.sheets
            .get(at.sheet)
            .and_then(|s| s.get(at.row, at.col))
    }

    pub fn cell_mut(&mut self, at: CellRef) -> Option<&mut Cell> {
        self.sheets
            .get_mut(at.sheet)
            .and_then(|s| s.get_mut(at.row, at.col))
    }

    /// The value at `at`, treating a missing cell as empty. This is the accessor
    /// the grid and the formula engine use; they should not deal in `Option`.
    pub fn value_at(&self, at: CellRef) -> CellValue {
        self.cell(at)
            .map(|c| c.value.clone())
            .unwrap_or(CellValue::Empty)
    }

    pub fn set_cell(&mut self, at: CellRef, value: CellValue) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        let sheet = &mut self.sheets[at.sheet];
        let mut cell = sheet.get(at.row, at.col).cloned().unwrap_or_default();
        cell.value = value;
        sheet.set(at.row, at.col, cell);
        self.dirty = true;
    }

    pub fn set_cell_format(&mut self, at: CellRef, format: CellFormat) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        let sheet = &mut self.sheets[at.sheet];
        let mut cell = sheet.get(at.row, at.col).cloned().unwrap_or_default();
        cell.format = format;
        sheet.set(at.row, at.col, cell);
        self.dirty = true;
    }

    /// Clear one cell. A no-op clear leaves the dirty flag alone, so pressing Delete on
    /// an empty cell does not mark the workbook as needing a save.
    pub fn clear_cell(&mut self, at: CellRef) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        if self.sheets[at.sheet].remove(at.row, at.col).is_some() {
            self.dirty = true;
        }
    }

    pub fn dirty(&self) -> bool {
        self.dirty
    }

    pub fn mark_dirty(&mut self) {
        self.dirty = true;
    }

    pub fn clear_dirty(&mut self) {
        self.dirty = false;
    }

    /// Load a sheet produced by a reader, replacing the workbook's contents.
    /// Used by file open in a later plan; defined here so the seam exists.
    pub fn replace_sheet(&mut self, id: SheetId, mut sheet: Sheet) {
        if id < self.sheets.len() {
            sheet.set_id(id);
            self.sheets[id] = sheet;
            self.dirty = true;
        }
    }
}
