#!/bin/bash
set -euxo pipefail
if [ -f tigerbeetle ]; then
  exit 0
fi
git clone https://github.com/tigerbeetledb/tigerbeetle.git tb-repo || true
(cd tb-repo && scripts/install.sh)
mv tb-repo/tigerbeetle tigerbeetle
touch needs-update
