#!/bin/bash

set -Eeuo pipefail
trap cleanup SIGINT SIGTERM ERR EXIT

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd -P)

usage() {
  cat <<EOF
Usage: $(basename "${BASH_SOURCE[0]}") [-h] path/to/gphotobackup baseDir

Downloads the latest run of the binary and executes it with common parameters

Available options:

-h, --help      Print this help and exit
-v, --verbose   Print script debug info
EOF
  exit
}

cleanup() {
  trap - SIGINT SIGTERM ERR EXIT
  # script cleanup here
}

msg() {
  echo >&2 -e "${1-}"
}

die() {
  local msg=$1
  local code=${2-1} # default exit status 1
  msg "$msg"
  exit "$code"
}

parse_params() {
  while :; do
    case "${1-}" in
    -h | --help) usage ;;
    -v | --verbose) set -x ;;
    -?*) die "Unknown option: $1" ;;
    *) break ;;
    esac
    shift
  done

  args=("$@")

  [[ ${#args[@]} -ne 2 ]] && die "Missing script arguments"

  return 0
}

parse_params "$@"

BIN_DIR=$(dirname "${args[0]}")
BIN="${args[0]}"

if [[ -f "$BIN" ]]; then
 msg "Removing old binary"
 rm $BIN
fi

RUNID=`gh run list --repo ttomsu/gphotobackup --workflow go.yml --limit 1 --json databaseId --jq .[0].databaseId`
msg "Downloading binary from run ID $RUNID"
gh run download --repo ttomsu/gphotobackup $RUNID --name gphotobackup-linux --dir $BIN_DIR

msg "Starting gphotobackup"
$BIN print --out "${args[1]}"/albums/albumIDs.jsonl
$BIN backup \
--sinceDays 90 \
--albums \
--favorites \
--out "${args[1]}" \
--workers=6 \
--verbose=false
