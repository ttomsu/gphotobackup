#!/bin/bash

set -Euo pipefail
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

if [[ -h "$BIN" ]]; then
  if ! RUN_ID=$(gh run list --repo ttomsu/gphotobackup --workflow go.yml --limit 1 --json databaseId --jq '.[0].databaseId'); then
    echo "Failed to get latest run ID"
  elif [[ ! -f "$BIN-$RUN_ID" ]]; then
    echo "Downloading binary from run ID $RUN_ID"
    if ! gh run download --repo ttomsu/gphotobackup "$RUN_ID" --name gphotobackup-linux --dir "$BIN_DIR"; then
       echo "Failed to download binary"
    else
      echo "Symlinking new binary"
      chmod 755 "$BIN-$RUN_ID"
      rm "$BIN"
      ln -s "$BIN-$RUN_ID" "$BIN"
    fi
  fi
fi


msg "Starting gphotobackup"
$BIN print --out "${args[1]}"/albums/albumIDs.jsonl
$BIN backup \
--sinceDays 30 \
--albums \
--favorites \
--out "${args[1]}" \
--workers=10 \
--verbose=false
