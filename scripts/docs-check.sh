#!/bin/sh
# Docs integrity — the checks .github/workflows/ci.yml runs in its `docs` job,
# here so `make docs-check` (part of `make test`) fails the same way a PR would.
#
# 1. Every [Rn] citation in code or docs resolves to a research note.
# 2. Every research note says what it changes in the code.
# 3. Both locales carry the same keys (ADR-004) — web-ui/scripts/check-locales.mjs.
set -u
cd "$(dirname "$0")/.."
status=0

missing=0
for id in $(grep -rhoE '\[R[0-9]+\]' docs .claude/PLAN.md CLAUDE.md backend web-ui/src 2>/dev/null \
            | tr -d '[]R' | sort -un); do
  n=$(printf '%02d' "$id")
  if ! ls .claude/research/"${n}"-*.md >/dev/null 2>&1; then
    echo "docs-check: cited [R$id] but .claude/research/${n}-*.md does not exist" >&2
    missing=1
  fi
done
[ "$missing" -eq 0 ] || status=1

bad=0
for f in .claude/research/[0-9][0-9]-*.md; do
  case "$f" in *00-source-plan.md) continue ;; esac
  grep -qi 'What this changes in our code' "$f" || {
    echo "docs-check: $f is missing its 'What this changes in our code' section" >&2
    bad=1
  }
done
[ "$bad" -eq 0 ] || status=1

if command -v node >/dev/null 2>&1; then
  node web-ui/scripts/check-locales.mjs || status=1
else
  echo "docs-check: node not found — locale parity not checked" >&2
  status=1
fi

# 4. Design tokens meet WCAG and illustrations stay under 2 KB (ADR-059).
if command -v python3 >/dev/null 2>&1; then
  python3 scripts/validate_palette.py >/dev/null 2>&1 || {
    echo "docs-check: scripts/validate_palette.py failed — run it for the table" >&2
    status=1
  }
  python3 scripts/check_art.py >/dev/null 2>&1 || {
    echo "docs-check: scripts/check_art.py failed — run it for the list" >&2
    status=1
  }
else
  echo "docs-check: python3 not found — palette contrast not checked" >&2
  status=1
fi

[ "$status" -eq 0 ] && echo "docs-check: ok"
exit "$status"
