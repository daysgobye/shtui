# Shtui Linux User Installer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install Shtui for the current Linux user and register CSV/XLSX file-browser opens to launch it in a terminal.

**Architecture:** `scripts/install.sh` builds the existing Go command into `~/.local/bin/shtui`, writes a user-level freedesktop desktop entry under `${XDG_DATA_HOME:-$HOME/.local/share}`, then uses `xdg-mime` to set CSV and XLSX defaults. A self-contained shell test exercises the installer in isolated XDG directories, verifies defaults with the real MIME utility, and uses stubs only for explicit failure cases.

**Tech Stack:** Bash, Go, freedesktop `.desktop` entries, `xdg-mime`, `update-desktop-database`.

**Spec:** `docs/superpowers/specs/2026-10-20-shtui-linux-installer-design.md`

## Global Constraints

- Target a freedesktop-compatible Linux desktop, including Omarchy/Hyprland.
- Install for the current user only; do not require `sudo` or modify system directories.
- Build the executable from the current checkout using the existing Go command package.
- Shtui is a terminal UI, so desktop launches must open a terminal.
- One opened file per invocation, matching the existing CLI contract.
- Set Shtui as the default for CSV and XLSX as requested; this intentionally may replace the user's existing CSV default.
- Do not add XLSM association in this change; user requested CSV and XLSX.
- Missing optional `update-desktop-database` does not fail installation; missing `xdg-mime` does.

## Review Focus

- Home/XDG paths containing spaces must produce a correctly quoted absolute `Exec` command. Test with a temporary home path containing spaces.
- A failed Go build must preserve an existing installed binary. Test with a failing `go` stub and compare the old binary contents.
- An `xdg-mime` failure must stop installation and must not print a success message. Test with a failing `xdg-mime` stub.
- Re-running installation must not duplicate MIME declarations or leave duplicate desktop entries. Run twice and inspect the generated entry and target paths.
- A missing `update-desktop-database` must remain non-fatal while `xdg-mime` defaults are still set. Run with a controlled PATH that omits it and verify success.

---

### Task 1: Add and verify the user-local installer

**Files:**
- Create: `scripts/install.sh`
- Create: `scripts/install_test.sh`
- Modify: `docs/usage.md` (add installer instructions and note CSV/XLSX defaults)

**Interfaces:**
- Installer entry point: `bash scripts/install.sh`, run from any working directory.
- Installer outputs: `~/.local/bin/shtui` and `${XDG_DATA_HOME:-$HOME/.local/share}/applications/shtui.desktop`.
- Desktop command: quoted absolute installed executable followed by `%f`; `Terminal=true`.
- Defaults: `text/csv` and `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` map to `shtui.desktop`.

- [x] **Step 1: Write `scripts/install_test.sh` first.** Make it create a temporary home and XDG directories whose paths include spaces, run the real installer/build and actual `xdg-mime`, and query both defaults from the isolated config directory. Assert the installed binary is executable; the desktop file has `Type=Application`, `Terminal=true`, the escaped absolute `Exec` plus `%f`, and exactly the two MIME types. Run installation a second time and verify one desktop entry and no duplicate MIME declaration.
- [x] **Step 2: Run `bash scripts/install_test.sh` before adding the installer.** Expected: fail because `scripts/install.sh` is absent, confirming the test exercises the missing behavior.
- [x] **Step 3: Implement `scripts/install.sh`.** Resolve the checkout root from the script path; require `go` and `xdg-mime`; build to a temporary file in `~/.local/bin` and atomically rename it after a successful build; write the desktop entry atomically under `${XDG_DATA_HOME:-$HOME/.local/share}/applications`; safely quote the executable path; set both MIME defaults; run `update-desktop-database` if available; print paths and configured types only after success; clean temporary files on exit.
- [x] **Step 4: Extend `scripts/install_test.sh` for the Review Focus cases.** Test failed Go build preserves an existing binary, failed `xdg-mime` exits without a success message, and absence of `update-desktop-database` remains non-fatal while real `xdg-mime` defaults are set. Use stubs only for external-command failure cases; inspect the generated desktop entry and actual installed file behavior.
- [x] **Step 5: Run `bash scripts/install_test.sh`.** Expected: PASS for install, rerun, path escaping, actual MIME defaults, failed build, failed MIME registration, and optional database refresh behavior.
- [x] **Step 6: Document use in `docs/usage.md`.** Add a Linux installation section showing `bash scripts/install.sh`, explaining that it builds into `~/.local/bin`, launches in a terminal, and sets CSV/XLSX defaults (including the CSV default replacement consequence).
- [x] **Step 7: Verify the installed app.** Run `desktop-file-validate` on the generated entry when installed, run `go test ./...` and `go build -o <temporary-path> ./cmd/omasheet`, then launch the installed executable with CSV and XLSX fixtures from a terminal smoke test and confirm each opens the requested workbook.
- [x] **Step 8: Commit the installer, tests, and usage documentation.**

**Commit:** `feat: add user-local Linux installer`
