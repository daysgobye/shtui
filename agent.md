# Agent Guide

Shtui is a Go terminal spreadsheet for CSV and XLSX workbooks. Keep changes focused and preserve existing behavior unless the task asks to change it.

## Project map

- `cmd/shtui/`: executable entry point.
- `internal/app/`: application startup and coordination.
- `internal/ui/`: terminal interface and interaction logic.
- `internal/workbook/`: workbook and file-format handling.
- `docs/usage.md`: user-facing usage and file-format details.
- `scripts/`: installation and installer checks.

## Development

- Use the Go version declared in `go.mod` (currently Go 1.26).
- Format changed Go files with `gofmt`.
- Add or update tests alongside the affected package; keep tests deterministic and avoid changing workbook fixtures unless the task requires it.
- Run the relevant package tests, then the full suite with `go test ./...`.
- Verify the executable builds with `go build -o /tmp/shtui ./cmd/shtui`.
- Run `/tmp/shtui [workbook.csv|workbook.xlsx]` to launch the binary built by the command above; use a terminal for interactive UI checks.
- Update `README.md` or `docs/usage.md` when user-visible behavior or instructions change.

## Change practices

- Follow the existing package boundaries and patterns; avoid adding dependencies for work the standard library or current dependencies already support.
- Keep CSV-specific behavior separate from XLSX-only workbook features where appropriate; CSV files support one sheet and cannot retain workbook-only formatting or multiple sheets.
- Do not claim full Excel compatibility: formula calculation is intentionally limited. Consult `docs/usage.md` before changing file-format behavior.
- Do not commit generated binaries or unrelated changes.
