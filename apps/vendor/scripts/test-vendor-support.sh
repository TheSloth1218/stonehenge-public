#!/bin/sh
set -eu

REPOSITORY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec python3 -m unittest discover \
    -s "$REPOSITORY_ROOT/deploy/vendor-support/tests" \
    -p 'test_*.py' \
    -v
