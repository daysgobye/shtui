//! Sparse sheet storage.
//!
//! Rows are stored in a `BTreeMap` keyed by row index, and each row holds a
//! `BTreeMap` of cells. A 10k-cell sheet allocates nothing for the cells it does
//! not have, and the same structure handles a far larger sheet unchanged.

use std::collections::BTreeMap;

use super::cell::{Cell, CellRange, SheetId};

/// One row of cells, keyed by column index.
#[derive(Debug, Default)]
pub struct Row {
    cells: BTreeMap<u32, Cell>,
}

impl Row {
    pub fn get(&self, col: u32) -> Option<&Cell> {
        self.cells.get(&col)
    }

    pub fn get_mut(&mut self, col: u32) -> Option<&mut Cell> {
        self.cells.get_mut(&col)
    }

    pub fn set(&mut self, col: u32, cell: Cell) {
        self.cells.insert(col, cell);
    }

    pub fn remove(&mut self, col: u32) -> Option<Cell> {
        self.cells.remove(&col)
    }

    pub fn is_empty(&self) -> bool {
        self.cells.is_empty()
    }

    pub fn iter(&self) -> impl Iterator<Item = (u32, &Cell)> {
        self.cells.iter().map(|(c, cell)| (*c, cell))
    }

    pub fn min_col(&self) -> u32 {
        self.cells.keys().copied().min().unwrap_or(0)
    }

    pub fn max_col(&self) -> u32 {
        self.cells.keys().copied().max().unwrap_or(0)
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
        self.rows.entry(row).or_default().set(col, cell);
    }

    /// Remove a cell, shrinking the used range only if the removed cell was on
    /// its boundary. An empty sheet reports `None` rather than a degenerate range.
    pub fn remove(&mut self, row: u32, col: u32) -> Option<Cell> {
        let removed = {
            let row_entry = self.rows.get_mut(&row)?;
            let removed = row_entry.remove(col);
            if row_entry.is_empty() {
                self.rows.remove(&row);
            }
            removed
        }?;

        if self.used.is_some() {
            let mut min_row = u32::MAX;
            let mut max_row = 0u32;
            let mut min_col = u32::MAX;
            let mut max_col = 0u32;
            for (r, row_entry) in &self.rows {
                if row_entry.is_empty() {
                    continue;
                }
                min_row = min_row.min(*r);
                max_row = max_row.max(*r);
                min_col = min_col.min(row_entry.min_col());
                max_col = max_col.max(row_entry.max_col());
            }
            self.used = if self.rows.is_empty() {
                None
            } else {
                Some(CellRange {
                    min_row,
                    min_col,
                    max_row,
                    max_col,
                })
            };
        }
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
