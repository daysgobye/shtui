//! Sparse sheet storage.
//!
//! Rows are stored in a `BTreeMap` keyed by row index, and each row holds a
//! `BTreeMap` of cells. A 10k-cell sheet allocates nothing for the cells it does
//! not have, and the same structure handles a far larger sheet unchanged.

use std::collections::BTreeMap;

use super::cell::{Cell, CellRange, SheetId};

/// One row of cells, keyed by column index.
#[derive(Debug)]
pub struct Row {
    cells: BTreeMap<u32, Cell>,
    /// Cached column bounds, kept in step with `cells`.
    ///
    /// Without these, recomputing a sheet's used range walks every populated cell in
    /// the sheet rather than every row, which makes repeated single-cell clears
    /// quadratic. `min_col`/`max_col` are O(1).
    min_col: u32,
    max_col: u32,
}

impl Row {
    fn new() -> Self {
        Self {
            cells: BTreeMap::new(),
            min_col: u32::MAX,
            max_col: 0,
        }
    }

    pub fn get(&self, col: u32) -> Option<&Cell> {
        self.cells.get(&col)
    }

    pub fn get_mut(&mut self, col: u32) -> Option<&mut Cell> {
        self.cells.get_mut(&col)
    }

    pub fn set(&mut self, col: u32, cell: Cell) {
        if self.cells.is_empty() {
            self.min_col = col;
            self.max_col = col;
        } else {
            self.min_col = self.min_col.min(col);
            self.max_col = self.max_col.max(col);
        }
        self.cells.insert(col, cell);
    }

    pub fn remove(&mut self, col: u32) -> Option<Cell> {
        let removed = self.cells.remove(&col)?;
        if !self.cells.is_empty() && (col == self.min_col || col == self.max_col) {
            // A boundary column was removed, so the cached bounds are stale. Rebuilding
            // costs one pass over this row's keys, bounded by the row's width.
            self.min_col = self.cells.keys().copied().min().unwrap_or(u32::MAX);
            self.max_col = self.cells.keys().copied().max().unwrap_or(0);
        }
        Some(removed)
    }

    pub fn is_empty(&self) -> bool {
        self.cells.is_empty()
    }

    pub fn iter(&self) -> impl Iterator<Item = (u32, &Cell)> {
        self.cells.iter().map(|(c, cell)| (*c, cell))
    }

    pub fn min_col(&self) -> u32 {
        self.min_col
    }

    pub fn max_col(&self) -> u32 {
        self.max_col
    }
}

/// A single sheet: sparse rows plus a cached bounding box of populated cells.
#[derive(Debug)]
pub struct Sheet {
    pub(crate) id: SheetId,
    name: String,
    rows: BTreeMap<u32, Row>,
    used: Option<CellRange>,
}

impl Sheet {
    pub fn new(id: SheetId, name: impl Into<String>) -> Self {
        Self {
            id,
            name: name.into(),
            rows: BTreeMap::new(),
            used: None,
        }
    }

    pub fn id(&self) -> SheetId {
        self.id
    }

    pub fn set_id(&mut self, id: SheetId) {
        self.id = id;
    }

    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn set_name(&mut self, name: impl Into<String>) {
        self.name = name.into();
    }

    pub fn get(&self, row: u32, col: u32) -> Option<&Cell> {
        self.rows.get(&row).and_then(|r| r.get(col))
    }

    pub fn get_mut(&mut self, row: u32, col: u32) -> Option<&mut Cell> {
        self.rows.get_mut(&row).and_then(|r| r.get_mut(col))
    }

    /// Store a cell and extend the used range to cover it.
    pub fn set(&mut self, row: u32, col: u32, cell: Cell) {
        self.used = Some(match self.used {
            Some(r) => r.union(&CellRange::new(row, col, row, col)),
            None => CellRange::new(row, col, row, col),
        });
        self.rows.entry(row).or_insert_with(Row::new).set(col, cell);
    }

    /// Remove a cell and keep the used range correct.
    ///
    /// Removing an interior cell cannot change the bounding box, so that case returns
    /// without touching it. Only a removal on the box's boundary triggers a rebuild,
    /// and because [`Row`] caches its own column bounds the rebuild costs one pass over
    /// the sheet's rows rather than over every populated cell. An emptied sheet reports
    /// `None` rather than a degenerate range.
    pub fn remove(&mut self, row: u32, col: u32) -> Option<Cell> {
        let removed = {
            let row_entry = self.rows.get_mut(&row)?;
            let removed = row_entry.remove(col);
            if row_entry.is_empty() {
                self.rows.remove(&row);
            }
            removed
        }?;

        let Some(used) = self.used else {
            return Some(removed);
        };
        let touched_boundary = row == used.min_row
            || row == used.max_row
            || col == used.min_col
            || col == used.max_col;
        if !touched_boundary {
            return Some(removed);
        }

        if self.rows.is_empty() {
            self.used = None;
            return Some(removed);
        }

        let mut min_row = u32::MAX;
        let mut max_row = 0u32;
        let mut min_col = u32::MAX;
        let mut max_col = 0u32;
        for (r, row_entry) in &self.rows {
            min_row = min_row.min(*r);
            max_row = max_row.max(*r);
            min_col = min_col.min(row_entry.min_col());
            max_col = max_col.max(row_entry.max_col());
        }
        self.used = Some(CellRange {
            min_row,
            min_col,
            max_row,
            max_col,
        });
        Some(removed)
    }

    /// Remove every cell.
    pub fn clear(&mut self) {
        self.rows.clear();
        self.used = None;
    }

    /// The bounding box of populated cells, or `None` when the sheet is empty.
    pub fn used(&self) -> Option<CellRange> {
        self.used
    }

    /// One past the highest populated row index, or 0 when empty.
    pub fn row_count(&self) -> u32 {
        self.used.map(|r| r.max_row + 1).unwrap_or(0)
    }

    /// One past the highest populated column index, or 0 when empty.
    pub fn col_count(&self) -> u32 {
        self.used.map(|r| r.max_col + 1).unwrap_or(0)
    }

    pub fn iter_row(&self, row: u32) -> impl Iterator<Item = (u32, &Cell)> {
        self.rows.get(&row).into_iter().flat_map(|r| r.iter())
    }

    /// Every populated cell, in row-major order. Used by save and export.
    pub fn iter_cells(&self) -> impl Iterator<Item = (u32, u32, &Cell)> {
        self.rows
            .iter()
            .flat_map(|(r, row)| row.iter().map(move |(c, cell)| (*r, c, cell)))
    }
}
