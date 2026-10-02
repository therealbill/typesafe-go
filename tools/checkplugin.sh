#!/bin/zsh
# Checks the Claude Code marketplace and plugin files: both manifests parse,
# every plugin source exists with its own manifest and matching name, every
# skill, command, and agent carries the frontmatter Claude Code needs, and the
# config-schema reference names exactly the fields of the review package's
# Config and Unit structs. Runs `claude plugin validate --strict` when the
# claude CLI is on PATH.
set -euo pipefail
cd "${0:A:h}/.."
rc=0
fail() { print -u2 -- "checkplugin: $*"; rc=1 }

json_ok() { python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$1" 2>/dev/null }

json_field() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$1" "$2" 2>/dev/null }

# frontmatter_has FILE KEY: true when the file starts with a YAML frontmatter
# block that has KEY at the top level.
frontmatter_has() {
  local file=$1 key=$2
  [[ "$(head -n1 "$file")" == "---" ]] || return 1
  sed -n '2,/^---$/p' "$file" | grep -qE "^${key}:"
}

market=.claude-plugin/marketplace.json
[[ -f $market ]] || { fail "$market is missing"; exit 1 }
json_ok $market || { fail "$market is not valid JSON"; exit 1 }

typeset -a sources
sources=(${(f)"$(python3 -c 'import json; [print(p["source"], p["name"]) for p in json.load(open(".claude-plugin/marketplace.json"))["plugins"]]' 2>/dev/null)"}) || true
(( ${#sources} )) || { fail "$market has no readable plugins list"; exit 1 }
for entry in $sources; do
  src=${entry%% *}
  name=${entry#* }
  [[ -d $src ]] || { fail "plugin source $src does not exist"; continue }
  manifest=$src/.claude-plugin/plugin.json
  [[ -f $manifest ]] || { fail "$manifest is missing"; continue }
  json_ok $manifest || { fail "$manifest is not valid JSON"; continue }
  [[ "$(json_field $manifest name)" == "$name" ]] || fail "$manifest name does not match marketplace entry $name"
  for d in $src/skills/*(N/); do
    [[ -f $d/SKILL.md ]] || { fail "$d has no SKILL.md"; continue }
    frontmatter_has $d/SKILL.md name || fail "$d/SKILL.md frontmatter lacks name"
    frontmatter_has $d/SKILL.md description || fail "$d/SKILL.md frontmatter lacks description"
  done
  for f in $src/commands/*.md(N) $src/agents/*.md(N); do
    frontmatter_has $f description || fail "$f frontmatter lacks description"
  done
done

# The config-schema reference must name exactly the json fields of Config and
# Unit. Only its field-table rows start with a backticked lowercase name. The
# grep stages carry `|| true` so a run with no matches reports the drift
# instead of ending the script.
schema=plugins/jev-review/skills/setting-up-jev-review/references/config-schema.md
[[ -f $schema ]] || { fail "$schema is missing"; exit 1 }
typeset -a tags documented
tags=(${(f)"$(awk '/^type (Config|Unit) struct/,/^}/' internal/review/review.go | { grep -o 'json:"[a-z_]*' || true } | sed 's/json:"//' | sort -u)"})
documented=(${(f)"$({ grep -oE '^\| `[a-z_]+`' $schema || true } | sed -E 's/^\| `([a-z_]+)`/\1/' | sort -u)"})
(( ${#tags} )) || fail "found no json tags on Config or Unit in internal/review/review.go"
for t in $tags; do
  (( ${documented[(Ie)$t]} )) || fail "config-schema.md does not document field $t"
done
for d in $documented; do
  (( ${tags[(Ie)$d]} )) || fail "config-schema.md documents $d, which is not a field of Config or Unit"
done

if command -v claude >/dev/null 2>&1; then
  claude plugin validate --strict . >/dev/null || fail "claude plugin validate --strict . failed"
  for entry in $sources; do
    src=${entry%% *}
    claude plugin validate --strict $src >/dev/null || fail "claude plugin validate --strict $src failed"
  done
fi

exit $rc
