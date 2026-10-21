# Shtui Linux User Installer

## Goal

Let a Linux desktop user install Shtui from a source checkout and open CSV and XLSX files in it from the file browser.

## Audience and constraints

- Target a freedesktop-compatible Linux desktop, including Omarchy/Hyprland.
- Install for the current user only; do not require `sudo` or modify system directories.
- Build the executable from the current checkout using the existing Go command package.
- Shtui is a terminal UI, so desktop launches must open a terminal.
- One opened file per invocation, matching the existing CLI contract.
- Set Shtui as the default for CSV and XLSX as requested; this intentionally may replace the user's existing CSV default.

## Architecture decisions

### One user-local installer

Add `scripts/install.sh`. Resolve the repository root relative to the script, build `./cmd/shtui` with Go, and install the resulting `shtui` executable to `~/.local/bin/shtui`. Fail clearly if Go or the required `xdg-mime` command is unavailable. Build to a temporary file before replacing the installed executable so a failed build does not damage an existing installation.

### Desktop entry

Write a `Type=Application` desktop entry named `Shtui` under `${XDG_DATA_HOME:-$HOME/.local/share}/applications/shtui.desktop`. Set `Terminal=true`, pass one selected local file using `%f`, and register `text/csv` and `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`. Quote/escape the absolute executable path according to desktop-entry `Exec` syntax so home paths containing spaces work. Do not add XLSM association in this change; user requested CSV and XLSX.

### MIME defaults and refresh

Use `xdg-mime default shtui.desktop` for each advertised MIME type. Refresh the applications database with `update-desktop-database` when that command is available. Print the installed binary/entry paths and the MIME types configured. Re-running the script replaces the same user-local files and reapplies the same associations without creating duplicates.

## Error behavior

- Stop on build, directory creation, or file-write failure and report the failing step.
- A failed build must leave the previously installed executable intact.
- Missing optional `update-desktop-database` does not fail installation; missing `xdg-mime` does.
- Do not silently fall back to system-wide installation or claim success if MIME-default registration fails.

## Acceptance checks

- Run the installer from the repository checkout using an isolated temporary home and XDG data directory; verify it builds a runnable binary, writes a valid desktop entry with `Terminal=true`, the absolute executable path, `%f`, and both MIME types, and sets both defaults.
- Verify paths containing spaces are escaped correctly in the desktop entry.
- Verify rerunning the installer updates the same entry and does not duplicate MIME types.
- Verify failed builds do not replace a pre-existing installed binary.
- Run `desktop-file-validate` on the generated entry when available, and launch the installed executable with a CSV/XLSX fixture in a terminal smoke test.
- Document the installer invocation and the defaults it changes in `docs/usage.md`.

## Alternatives considered

1. **Build and register user-locally (selected):** one command installs the project and wires file associations without elevated privileges; predictable for a source checkout.
2. **Register an already-installed binary only:** smaller script, but leaves executable installation and path management to the user, so it does not deliver the requested click-to-open flow end to end.
3. **System-wide package/install:** integrates all users but requires privileged writes and package-maintenance decisions outside the request.

## Out of scope

- XLSM, legacy XLS, and other file types.
- Uninstaller, system-wide installation, package-manager integration, icons, and multi-file opening.
- GUI spreadsheet behavior; Shtui remains a terminal application.
