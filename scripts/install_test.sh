#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
real_go=$(command -v go) || { printf 'go is required for this test\n' >&2; exit 1; }
real_xdg_mime=$(command -v xdg-mime) || { printf 'xdg-mime is required for this test\n' >&2; exit 1; }
real_gomodcache=$(go env GOMODCACHE)
real_gocache=$(go env GOCACHE)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/shtui install test.XXXXXX")
cleanup() {
  chmod -R u+w -- "$tmp" 2>/dev/null || true
  rm -rf -- "$tmp"
}
trap cleanup EXIT

export HOME="$tmp/home with spaces"
export GOMODCACHE="$real_gomodcache"
export GOCACHE="$real_gocache"
export XDG_DATA_HOME="$HOME/xdg data"
export XDG_CONFIG_HOME="$HOME/xdg config"
mkdir -p "$HOME" "$XDG_DATA_HOME" "$XDG_CONFIG_HOME"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_file_contains() {
  grep -Fqx -- "$2" "$1" || fail "$1 lacks: $2"
}

assert_default() {
  local mime_type=$1
  local actual
  actual=$(xdg-mime query default "$mime_type")
  [ "$actual" = shtui.desktop ] || fail "$mime_type default is '$actual'"
}

run_install() {
  (cd "$tmp" && /bin/bash "$repo_root/scripts/install.sh")
}

binary="$HOME/.local/bin/shtui"
desktop_dir="$XDG_DATA_HOME/applications"
desktop_file="$desktop_dir/shtui.desktop"

run_install
[ -x "$binary" ] || fail "installed binary is not executable"
assert_file_contains "$desktop_file" 'Type=Application'
assert_file_contains "$desktop_file" 'Terminal=true'
assert_file_contains "$desktop_file" "Exec=\"$binary\" %f"
assert_file_contains "$desktop_file" 'MimeType=text/csv;application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;'
if command -v desktop-file-validate >/dev/null 2>&1; then
  desktop-file-validate "$desktop_file"
fi
assert_default text/csv
assert_default application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

run_install
[ "$(find "$desktop_dir" -maxdepth 1 -name 'shtui.desktop' -type f | wc -l)" -eq 1 ] || fail "installer created duplicate desktop entries"
[ "$(grep -c '^MimeType=' "$desktop_file")" -eq 1 ] || fail "desktop entry has duplicate MIME declarations"
assert_default text/csv
assert_default application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

# A failed build must not replace the installed executable.
printf 'existing binary marker\n' > "$binary"
chmod +x "$binary"
failing_go="$tmp/failing-go"
mkdir -p "$failing_go"
printf '#!/bin/sh\nexit 41\n' > "$failing_go/go"
chmod +x "$failing_go/go"
if PATH="$failing_go:$PATH" run_install >"$tmp/build-failure.log" 2>&1; then
  fail "installer succeeded with a failing Go build"
fi
[ "$(cat "$binary")" = 'existing binary marker' ] || fail "failed build replaced the installed binary"

# A failed MIME registration must fail the install without claiming success.
failing_mime="$tmp/failing-mime"
mkdir -p "$failing_mime"
printf '#!/bin/sh\nexit 42\n' > "$failing_mime/xdg-mime"
chmod +x "$failing_mime/xdg-mime"
if PATH="$failing_mime:$PATH" run_install >"$tmp/mime-failure.log" 2>&1; then
  fail "installer succeeded with a failing xdg-mime"
fi
if grep -Fq 'Installation complete' "$tmp/mime-failure.log"; then
  fail "installer reported success after MIME registration failed"
fi

# Omit update-desktop-database from PATH while retaining real Go and xdg-mime.
no_update_path="$tmp/path without update-desktop-database"
mkdir -p "$no_update_path"
for utility in dirname mkdir mktemp mv chmod rm; do
  ln -s "$(command -v "$utility")" "$no_update_path/$utility"
done
cat > "$no_update_path/go" <<'EOF'
#!/bin/bash
PATH="$REAL_PATH" exec "$REAL_GO" "$@"
EOF
cat > "$no_update_path/xdg-mime" <<'EOF'
#!/bin/bash
PATH="$REAL_PATH" exec "$REAL_XDG_MIME" "$@"
EOF
chmod +x "$no_update_path/go" "$no_update_path/xdg-mime"
if ! REAL_PATH="$PATH" REAL_GO="$real_go" REAL_XDG_MIME="$real_xdg_mime" PATH="$no_update_path" run_install >"$tmp/no-update.log" 2>&1; then
  cat "$tmp/no-update.log" >&2
  fail "installer failed when optional update-desktop-database was unavailable"
fi
assert_default text/csv
assert_default application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

printf 'PASS: install, MIME defaults, rerun, paths with spaces, failure safety, optional refresh\n'
