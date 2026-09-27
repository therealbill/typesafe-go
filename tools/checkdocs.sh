#!/usr/bin/env bash
# Checks that every page under docs/ (excluding docs/superpowers/) is linked
# from README.md or another docs page, and that every relative markdown link
# in README.md and docs/ resolves to an existing file.
#
# Fenced code blocks and inline code spans are stripped before links are
# extracted, so a Go snippet such as SystemOneAs[T](ctx, c, state) is not
# mistaken for a markdown link.
set -euo pipefail
cd "$(dirname "$0")/.."
status=0

# strip_code removes fenced code blocks and inline code spans from a file.
strip_code() {
  awk '/^[[:space:]]*```/ { fenced = !fenced; next } !fenced' "$1" | sed 's/`[^`]*`//g'
}

while IFS= read -r page; do
  name=$(basename "$page")
  if ! grep -rq --include='*.md' -F "$name" README.md docs; then
    echo "unlinked: $page"; status=1
  fi
done < <(find docs -name '*.md' -not -path 'docs/superpowers/*' | sort)
while IFS= read -r file; do
  dir=$(dirname "$file")
  while IFS= read -r link; do
    link=${link%%#*}
    [ -z "$link" ] && continue
    case "$link" in http://*|https://*|mailto:*) continue;; esac
    if [ ! -e "$dir/$link" ]; then
      echo "broken link in $file: $link"; status=1
    fi
  done < <(strip_code "$file" | grep -oE '\]\([^)]+\)' | sed -E 's/^\]\((.*)\)$/\1/' || true)
done < <({ echo README.md; find docs -name '*.md' -not -path 'docs/superpowers/*'; } | sort)
exit $status
