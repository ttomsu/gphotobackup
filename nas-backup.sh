#!/bin/bash

set -eo pipefail

/volume1/homes/ttomsu/bin/gphotosync-linux \
backup \
--sinceDays 21 \
--out /volume1/GooglePhotosBackup/travis.tomsu/
