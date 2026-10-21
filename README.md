<div align="center">
  <img src="assets/shtui-wordmark.svg" alt="SHTUI — a calmer way to work with spreadsheets" width="790">

  <p><strong>A spreadsheet for your terminal.</strong></p>
  <p>Open CSV and XLSX workbooks, edit cells and formulas, style sheets, and keep your hands on the keyboard.</p>
</div>

## See it in action

Start Shtui, open a workbook, browse its sheets, and quit safely:

![Shtui start screen, workbook picker, multi-sheet workbook, and quit confirmation](assets/demos/01-start-and-open.gif)

<table>
  <tr>
    <td width="50%">
      <strong>Build a sheet with formulas</strong><br>
      <img src="assets/demos/02-build-with-formulas.gif" alt="Entering spreadsheet data, formulas, and formatting">
    </td>
    <td width="50%">
      <strong>Open a styled workbook</strong><br>
      <img src="assets/demos/03-workbook-styling.gif" alt="A workbook with colored headers, merged title, varied column widths, and multiple sheets">
    </td>
  </tr>
  <tr>
    <td width="50%">
      <strong>Apply conditional formatting</strong><br>
      <img src="assets/demos/04-live-conditional-formatting.gif" alt="Applying a conditional color rule to matching spreadsheet cells">
    </td>
    <td width="50%">
      <strong>Navigate a 25,000 × 20 CSV</strong><br>
      <img src="assets/demos/05-large-sheet.gif" alt="Opening a 25,000-row, 20-column CSV and jumping to its final row">
    </td>
  </tr>
</table>

## A real spreadsheet in the terminal

Shtui is a compiled Go terminal spreadsheet for working with CSV and XLSX files. Use familiar cell navigation and editing, a formula bar, a searchable command palette, and keyboard or mouse controls.

- **Workbooks:** open and save CSV/XLSX files; create new CSV or XLSX workbooks; move between sheets in XLSX files.
- **Formulas:** arithmetic, cell references and ranges, plus `SUM`, `AVERAGE`, `COUNT`, `COUNTA`, `MIN`, `MAX`, `IF`, `COUNTIF`, and `IFERROR`.
- **Formatting:** fills, font colors, bold, borders, merged cells, column resizing, and conditional formatting.
- **Data tools:** find, sort, filter, undo, and redo.
- **Large grids:** a virtualized grid targets workbooks up to roughly 25,000 rows by 20 columns per sheet. This is a target, not a guarantee for larger files or every machine.

Formula calculation is intentionally limited; Shtui is not full Excel. CSV supports one sheet and cannot retain workbook-only features such as formatting, merges, or multiple sheets. See [usage and format details](docs/usage.md).

## Install

From a checkout, run the installer:

```sh
bash scripts/install.sh
```

It builds `shtui` into `~/.local/bin`, creates a user-level desktop entry, and registers CSV/XLSX file associations. It requires Go and `xdg-mime`; it does not use `sudo`. Registering CSV as the default may replace your current CSV handler.

Or build and launch directly:

```sh
go build -o shtui ./cmd/omasheet
./shtui [workbook.csv|workbook.xlsx]
```

With no workbook path, choose **Open file**, **New CSV workbook**, or **New XLSX workbook** from the start screen. For shortcuts and complete file-format behavior, see the [usage guide](docs/usage.md).
