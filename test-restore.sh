#!/bin/bash
# Restore all repos: fetch from remote then reset to upstream head.
# Undoes test-reset.sh.
set -euo pipefail

WS_HOME="$(cd "$(dirname "$0")" && pwd)"
export WS_HOME

echo "Fetching all repos..."
ws fetch

echo ""
echo "Resetting to upstream..."
ws git reset --hard '@{u}' 2>&1 | grep "HEAD is now"

echo ""
echo "Done."
