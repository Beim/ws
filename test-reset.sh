#!/bin/bash
# Reset all repos N commits back and rewind remote tracking refs to match.
# This makes `ws fetch` actually download objects.
# Usage: ./test-reset.sh [N]  (default: 2)
set -euo pipefail

N="${1:-2}"
WS_HOME="$(cd "$(dirname "$0")" && pwd)"
export WS_HOME

echo "Resetting all repos ${N} commits back..."
ws git reset --hard "HEAD~${N}" 2>&1 | grep "HEAD is now"

echo ""
echo "Rewinding remote tracking refs..."
for repo in $(ws list 2>/dev/null | awk 'NR>2 && $NF=="yes" {print $1}'); do
  d="$(dirname "$WS_HOME")/$repo"
  tracking=$(git -C "$d" rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "")
  if [ -n "$tracking" ]; then
    git -C "$d" update-ref "refs/remotes/$tracking" HEAD 2>/dev/null
    echo "  $repo -> rewound $tracking"
  fi
done

echo ""
echo "Done. Now run: ws fetch"
