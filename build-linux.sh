#!/usr/local/bin/bash

set -eo pipefail

GOOS=linux go build -o gphotosync-linux gphotosync.go
