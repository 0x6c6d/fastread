#!/usr/bin/env bash
# Fails when a protected file (README.md, prompts/, .claude/) or the "Delegation workflow" section
# of CLAUDE.md differs from scripts/protected.sha256. No git history needed.
# Rewrite the manifest only on purpose (user instruction): scripts/check-protected.sh --update
set -euo pipefail

cd "$(dirname "$0")/.."
manifest=scripts/protected.sha256

compute() {
  find README.md prompts .claude -type f ! -name '*.swp' -print0 | LC_ALL=C sort -z | xargs -0 sha256sum
  awk '/^## Delegation workflow/ {f = 1} f && /^## / && !/^## Delegation workflow/ {f = 0} f' CLAUDE.md \
    | sha256sum | sed 's|-$|CLAUDE.md#delegation-workflow|'
}

if [ "${1:-}" = "--update" ]; then
  compute > "$manifest"
  echo "wrote $manifest" >&2
  exit 0
fi

if [ ! -f "$manifest" ]; then
  echo "missing $manifest" >&2
  exit 1
fi

if ! diff <(compute) "$manifest" > /dev/null; then
  echo "protected files changed (diff of current vs $manifest):" >&2
  diff <(compute) "$manifest" >&2 || true
  exit 1
fi
