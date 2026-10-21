#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'shtui installer: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

[ -n "${HOME:-}" ] || fail 'HOME is not set'
case "$HOME" in
  /*) ;;
  *) fail 'HOME must be an absolute path' ;;
esac

xdg_data_home=${XDG_DATA_HOME:-"$HOME/.local/share"}
case "$xdg_data_home" in
  /*) ;;
  *) fail 'XDG_DATA_HOME must be an absolute path' ;;
esac

require_command go
require_command xdg-mime

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd -- "$script_dir/.." && pwd -P)
bin_dir="$HOME/.local/bin"
applications_dir="$xdg_data_home/applications"
binary="$bin_dir/shtui"
desktop_file="$applications_dir/shtui.desktop"

mkdir -p -- "$bin_dir" "$applications_dir"
binary_tmp=$(mktemp "$bin_dir/.shtui.XXXXXX")
desktop_tmp=''
cleanup() {
  [ -z "$binary_tmp" ] || rm -f -- "$binary_tmp"
  [ -z "$desktop_tmp" ] || rm -f -- "$desktop_tmp"
}
trap cleanup EXIT

if ! (cd -- "$repo_root" && go build -o "$binary_tmp" ./cmd/shtui); then
  fail 'Go build failed; existing installation was left unchanged'
fi
chmod +x -- "$binary_tmp"
mv -f -- "$binary_tmp" "$binary"
binary_tmp=''

escape_exec_arg() {
  local input=$1 escaped='' char
  while [ -n "$input" ]; do
    char=${input:0:1}
    input=${input:1}
    case "$char" in
      %)
        escaped+='%%'
        ;;
      \\|\"|\$|\`)
        escaped+="\\$char"
        ;;
      *)
        escaped+="$char"
        ;;
    esac
  done
  printf '"%s"' "$escaped"
}

exec_binary=$(escape_exec_arg "$binary")
desktop_tmp=$(mktemp "$applications_dir/.shtui.desktop.XXXXXX")
printf '%s\n' \
  '[Desktop Entry]' \
  'Type=Application' \
  'Name=Shtui' \
  "Exec=$exec_binary %f" \
  'Terminal=true' \
  'MimeType=text/csv;application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;' \
  > "$desktop_tmp"
mv -f -- "$desktop_tmp" "$desktop_file"
desktop_tmp=''

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$applications_dir"
fi

xdg-mime default shtui.desktop text/csv
xdg-mime default shtui.desktop application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

printf 'Installation complete.\nBinary: %s\nDesktop entry: %s\nMIME defaults: text/csv, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\n' \
  "$binary" "$desktop_file"
