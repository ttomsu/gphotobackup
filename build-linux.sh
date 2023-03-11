#!/usr/local/bin/bash

set -eo pipefail

GOOS=linux go build -o gphotobackup-linux gphotobackup.go
