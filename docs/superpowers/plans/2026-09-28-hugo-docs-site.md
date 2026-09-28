# Hugo Documentation Site and Diátaxis Completion Implementation Plan

> **For agentic workers:** Execute this plan with an **Agent Team** of named agents (Agent tool with `name`, coordinated by the lead through SendMessage). Do NOT use git worktrees, do NOT use unnamed parallel subagents as the execution method, and do NOT use the `superpowers:executing-plans` skill. Roster and dependencies are in "Execution Model". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `docs/` as a Hextra-themed Hugo site on GitHub Pages via a docs-only-triggered workflow, and complete the Diátaxis documentation set.

**Architecture:** The site lives in `site/` with its own Hugo module `go.mod`, mounts `../docs` to `content/docs`, and gets `type: docs` by cascade from section landing pages. `docs/superpowers/` is excluded with cascaded build options. Two workflows build and deploy on docs changes only. The docs plugin's inventory and writer agents fill the remaining quadrant gaps.

**Tech Stack:** Hugo 0.166.0 extended, Hextra v0.12.3 (Hugo module), GitHub Pages via `actions/deploy-pages@v4`, GNU Make, bash.

**Spec:** `docs/superpowers/specs/2026-09-28-hugo-docs-site-design.md`. Read it first.

---

## Execution Model

### Team roster

| Agent name | Type / model | Owns | Skills and subagents to use |
|---|---|---|---|
| `site` | `hugo-repo:hugo-site-architect`, model **sonnet** | Tasks 1, 3, 4, 5 (`site/`, front-matter migration, Makefile, checkdocs, workflows, .gitignore) | `hugo-repo:hugo-fundamentals`, `hugo-repo:hugo-module-mounts`, `hugo-repo:hugo-github-actions`, `gnu-make:makefile-fundamentals` |
| `docs` | `general-purpose`, model **sonnet** | Tasks 2, 6, 7, 8 (landing pages, inventory, new pages, validation, README) | `diataxis-docs:doc-inventory`, `diataxis-docs:doc-tutorial-writer`, `diataxis-docs:doc-howto-writer`, `diataxis-docs:doc-reference-gen`, `diataxis-docs:doc-explanation-writer`, `diataxis-docs:doc-crosslink-validator` |
| lead (this session) | — | Task 9 (Pages enablement, push, deploy watch), Task 10 (final gate) | — |

### Dependency graph

```
Task 1 (front matter) ──┐
Task 2 (landing pages) ─┼─→ Task 3 (site scaffold, needs both to verify sidebar)
                        │       └─→ Task 4 (Makefile, checkdocs) ─→ Task 5 (workflows)
Task 6 (inventory) ─────┴─→ Task 7 (new pages) ─→ Task 8 (validate, README)
Tasks 5 and 8 ─────────────→ Task 9 (push, deploy) ─→ Task 10 (final gate)
```

Start order: spawn `site` on Task 1 and `docs` on Task 2 at the same time. When both report, `site` continues to Tasks 3–5 and `docs` to Task 6. `docs` reports the reconciled page list after Task 6 and waits for the lead's go-ahead before Task 7.

### Rules for every agent

1. Work in `/Users/bill/Projects/gojev` on `main`. No worktrees, no other branches.
2. Touch only the files listed under your task. Ask the lead for anything else.
3. Stage by explicit path only. Never `git add -A` or `git add .`. Retry once after two seconds on `index.lock`.
4. Commit with `git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "..."`. No "Authored by" lines. Do not push; the lead pushes.
5. No duration or timeline estimates anywhere in pages or front matter.
6. `TYPESAFE_API_KEY` is in the environment for real runs. Never print, log, or write it.
7. Scratch programs go under `/private/tmp/claude-501/-Users-bill-Projects-gojev/b96adec7-3da2-45f5-b6b1-95fdebc40eb3/scratchpad/`, never in the repo.
8. Report after each task: task number, commit hash, verification output tail, deviations.

---

## File map

| Path | Responsibility | Task |
|---|---|---|
| `docs/**/*.md` (17 existing pages) | front matter `diataxis:` and `weight:` | 1 |
| `docs/superpowers/_index.md` | build-option exclusion | 1 |
| `docs/_index.md`, `docs/{tutorials,how-to,reference,explanation}/_index.md` | landing pages with cascade | 2 |
| `site/hugo.toml`, `site/go.mod`, `site/go.sum`, `site/content/_index.md` | site | 3 |
| `.gitignore` | site build outputs | 3 |
| `Makefile`, `tools/checkdocs.sh` | targets and link checker | 4 |
| `.github/workflows/hugo-deploy.yml`, `.github/workflows/hugo-pr.yml` | CI | 5 |
| new pages under `docs/` | Diátaxis gaps | 7 |
| `README.md` | site link | 8 |

---

## Task 1: Front-matter migration and superpowers exclusion

**Agent:** `site`

**Files:**
- Modify: every `docs/**/*.md` except `docs/superpowers/**` (17 files)
- Create: `docs/superpowers/_index.md`

- [x] **Step 1: Migrate `type:` to `diataxis:` and add weights**

Run this script from the repo root. It rewrites only the front matter block and fails loudly if a page lacks `type:`.

```bash
python3 - <<'EOF'
import re, pathlib, sys
weights = {
  "tutorials/first-judgment-in-go.md": 10,
  "tutorials/jev-from-the-command-line.md": 20,
  "how-to/handle-rate-limits-and-retries.md": 10,
  "how-to/decode-answers-into-your-own-struct.md": 20,
  "how-to/call-through-a-gateway.md": 30,
  "how-to/trace-calls-and-send-to-honeycomb.md": 40,
  "how-to/drive-jev-from-a-script-or-agent.md": 50,
  "reference/client-options-and-environment.md": 10,
  "reference/question-types.md": 20,
  "reference/response-types.md": 30,
  "reference/errors-and-exit-codes.md": 40,
  "reference/jev-cli.md": 50,
  "reference/span-attributes.md": 60,
  "explanation/why-the-core-is-stdlib-only.md": 10,
  "explanation/retries-and-budgets.md": 20,
  "explanation/why-content-is-not-traced-by-default.md": 30,
  "explanation/mapping-from-the-python-sdk.md": 40,
}
root = pathlib.Path("docs")
for rel, w in weights.items():
    p = root / rel
    s = p.read_text()
    m = re.match(r"---\n(.*?)\n---\n", s, re.S)
    if not m: sys.exit(f"no front matter: {p}")
    fm = m.group(1)
    if not re.search(r"^type: ", fm, re.M): sys.exit(f"no type key: {p}")
    fm = re.sub(r"^type: (.+)$", r"diataxis: \1", fm, count=1, flags=re.M)
    fm = re.sub(r"^weight: .*\n?", "", fm, flags=re.M)
    fm = fm + f"\nweight: {w}"
    p.write_text(s[:m.start(1)] + fm + s[m.end(1):])
    print("migrated", rel)
EOF
```
Expected: 17 `migrated` lines. Then check:

```bash
grep -L '^diataxis: ' $(find docs -name '*.md' -not -path 'docs/superpowers/*' -not -name '_index.md') ; echo "---"; grep -l '^type: ' $(find docs -name '*.md' -not -path 'docs/superpowers/*')
```
Expected: both lists empty.

- [x] **Step 2: Create `docs/superpowers/_index.md`**

```markdown
---
title: "Design documents"
description: "Specs and implementation plans. Not published to the documentation site."
build:
  render: never
  list: never
cascade:
  build:
    render: never
    list: never
---

This directory holds the design specs and implementation plans used to build
the project. They are kept in the repository for history and are excluded
from the published documentation site.
```

- [x] **Step 3: Verify the link checker still passes and commit**

```bash
./tools/checkdocs.sh && echo "checkdocs ok"
git add $(git ls-files -m docs) docs/superpowers/_index.md
git commit -m "Move Diátaxis kind to a diataxis key, add sidebar weights, exclude design docs from the site"
```
Expected: `checkdocs ok`. (If the checker reports `docs/superpowers/_index.md` as unlinked, it is already excluded by path; Task 4 makes `_index.md` handling explicit.)

---

## Task 2: Section landing pages

**Agent:** `docs` (runs in parallel with Task 1; touches only new files)

**Files:**
- Create: `docs/_index.md`, `docs/tutorials/_index.md`, `docs/how-to/_index.md`, `docs/reference/_index.md`, `docs/explanation/_index.md`

- [x] **Step 1: Write `docs/_index.md`**

```markdown
---
title: "Documentation"
description: "Tutorials, how-to guides, reference, and explanation for the typesafe-go library and the jev command-line tool."
type: docs
---

typesafe-go is a Go client for the [TypeSafe](https://typesafe.ai) System One
API, plus `jev`, a command-line tool built on it. This documentation follows
the [Diátaxis](https://diataxis.fr) model, so each page has one job:

| | Practical | Theoretical |
|---|---|---|
| **Learning** | [Tutorials](tutorials/_index.md): lessons that take you from nothing to a working result | [Explanation](explanation/_index.md): why the library is shaped the way it is |
| **Working** | [How-to guides](how-to/_index.md): recipes for a task you already understand | [Reference](reference/_index.md): facts about every option, type, flag, and exit code |

New here? Start with [Your first judgment in Go](tutorials/first-judgment-in-go.md).
Looking for a specific option or flag? Go straight to the reference.
```

- [x] **Step 2: Write the four section pages**

`docs/tutorials/_index.md`:
```markdown
---
title: "Tutorials"
description: "Learning-oriented lessons. Follow them in order to go from nothing to a working program."
weight: 1
cascade:
  type: docs
---

Tutorials are lessons. Each one takes you through building something that
works, with a checkpoint after every step so you can tell whether you are on
track. They do not explain every option along the way; that is what the
[reference](../reference/_index.md) and [explanation](../explanation/_index.md)
sections are for.

- [Your first judgment in Go](first-judgment-in-go.md)
- [jev from the command line](jev-from-the-command-line.md)
```

`docs/how-to/_index.md`:
```markdown
---
title: "How-to guides"
description: "Goal-oriented recipes for readers who already know the basics."
weight: 2
cascade:
  type: docs
---

How-to guides solve one problem each. They assume you have done the
[tutorials](../tutorials/_index.md) and want to get a specific thing done,
so they skip background and go straight to the steps.

- [Handle rate limits and retries](handle-rate-limits-and-retries.md)
- [Decode answers into your own struct](decode-answers-into-your-own-struct.md)
- [Call through a gateway](call-through-a-gateway.md)
- [Trace calls and send them to Honeycomb](trace-calls-and-send-to-honeycomb.md)
- [Drive jev from a script or agent](drive-jev-from-a-script-or-agent.md)
```

`docs/reference/_index.md`:
```markdown
---
title: "Reference"
description: "Facts about every option, type, flag, attribute, and exit code, generated from the source."
weight: 3
cascade:
  type: docs
---

Reference pages describe what exists. They state facts, defaults, and shapes
without advice; when a page tells you how to choose, it belongs in the
[how-to guides](../how-to/_index.md) instead.

- [Client options and environment](client-options-and-environment.md)
- [Question types](question-types.md)
- [Response types](response-types.md)
- [Errors and exit codes](errors-and-exit-codes.md)
- [jev CLI](jev-cli.md)
- [Span attributes](span-attributes.md)
```

`docs/explanation/_index.md`:
```markdown
---
title: "Explanation"
description: "Why the library is designed the way it is, and how its parts fit together."
weight: 4
cascade:
  type: docs
---

Explanation pages step back from the code to discuss design choices, trade-offs,
and mental models. Nothing here is a step to follow; read them when a
[how-to guide](../how-to/_index.md) leaves you wondering why.

- [Why the core is stdlib-only](why-the-core-is-stdlib-only.md)
- [Retries and budgets](retries-and-budgets.md)
- [Why content is not traced by default](why-content-is-not-traced-by-default.md)
- [Mapping from the Python SDK](mapping-from-the-python-sdk.md)
```

Task 7 appends the new pages to these lists.

- [x] **Step 3: Commit**

```bash
git add docs/_index.md docs/tutorials/_index.md docs/how-to/_index.md docs/reference/_index.md docs/explanation/_index.md
git commit -m "Add Diátaxis section landing pages"
```

---

## Task 3: Site scaffold

**Agent:** `site` (after Tasks 1 and 2 are committed)

**Files:**
- Create: `site/hugo.toml`, `site/go.mod`, `site/go.sum`, `site/content/_index.md`
- Modify: `.gitignore`

- [x] **Step 1: Initialize the Hugo module and fetch Hextra**

```bash
mkdir -p site/content && cd site
hugo mod init github.com/therealbill/typesafe-go/site
hugo mod get github.com/imfing/hextra@v0.12.3
cd ..
grep hextra site/go.mod
```
Expected: `github.com/imfing/hextra v0.12.3`. Do not run `go mod tidy` at the repo root; the nested module is invisible to it, but confirm with `go build ./... && git status --short go.mod` showing no change to the root `go.mod`.

- [x] **Step 2: Write `site/hugo.toml`**

```toml
baseURL = "https://therealbill.github.io/typesafe-go/"
title = "typesafe-go"
languageCode = "en-us"
enableRobotsTXT = true

[module]
  [[module.imports]]
    path = "github.com/imfing/hextra"
  [[module.mounts]]
    source = "content"
    target = "content"
  [[module.mounts]]
    source = "../docs"
    target = "content/docs"

[markup]
  [markup.goldmark.renderer]
    unsafe = true
  [markup.highlight]
    noClasses = false

[params]
  description = "Go client and jev CLI for the TypeSafe System One API"
  [params.navbar]
    displayTitle = true
    displayLogo = false
  [params.theme]
    default = "system"
    displayToggle = true
  [params.search]
    enable = true
    type = "flexsearch"
  [params.editURL]
    enable = false
  [params.footer]
    displayCopyright = false
    displayPoweredBy = false

[menu]
  [[menu.main]]
    name = "Docs"
    pageRef = "/docs"
    weight = 1
  [[menu.main]]
    name = "GitHub"
    url = "https://github.com/therealbill/typesafe-go"
    weight = 2
    [menu.main.params]
      icon = "github"
  [[menu.main]]
    name = "Search"
    weight = 3
    [menu.main.params]
      type = "search"
```

- [x] **Step 3: Write the landing page `site/content/_index.md`**

```markdown
---
title: "typesafe-go"
layout: hextra-home
---

{{< hextra/hero-badge link="https://github.com/therealbill/typesafe-go/releases" >}}
  Releases
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Typed judgments from Jev, in Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  A stdlib-only Go client for the TypeSafe System One API, plus the `jev` command-line tool for scripts and agents.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get started" link="docs/tutorials/first-judgment-in-go" >}}
</div>

```bash
go get github.com/therealbill/typesafe-go
```

```go
client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
res, err := client.SystemOne(ctx, "I was charged twice.", typesafe.Questions{
    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
    "tone":    typesafe.Choice{Instructions: "What is the tone?", Criteria: map[string]typesafe.JSONContent{"calm": nil, "angry": nil}},
})
fmt.Println(res.Nouls()["billing"].Noul, res.Choices()["tone"].Choice)
```

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Tutorials"
    subtitle="Build something that works, step by step, with a checkpoint after each one."
    link="docs/tutorials"
  >}}
  {{< hextra/feature-card
    title="How-to guides"
    subtitle="Recipes for retries, typed decoding, gateways, tracing, and driving jev from scripts."
    link="docs/how-to"
  >}}
  {{< hextra/feature-card
    title="Reference"
    subtitle="Every option, type, flag, span attribute, and exit code, generated from the source."
    link="docs/reference"
  >}}
  {{< hextra/feature-card
    title="Explanation"
    subtitle="Why the core is stdlib-only, how retries and budgets interact, what is and is not traced."
    link="docs/explanation"
  >}}
{{< /hextra/feature-grid >}}
```

- [x] **Step 4: Add build outputs to `.gitignore`**

Append:
```
site/public/
site/resources/
site/.hugo_build.lock
```

- [x] **Step 5: Build and verify**

```bash
cd site && hugo --gc --minify 2>&1 | tail -3; cd ..
echo "--- pages ---"; find site/public -name index.html | wc -l
echo "--- superpowers excluded (0) ---"; find site/public -path '*superpowers*' | wc -l
echo "--- link resolution ---"; grep -o 'href="/typesafe-go/docs/reference/[^"]*"' site/public/docs/how-to/handle-rate-limits-and-retries/index.html | sort -u | head -3
echo "--- raw .md hrefs (0) ---"; grep -c 'href="\.\./[a-z-]*/[a-z-]*\.md"' site/public/docs/how-to/handle-rate-limits-and-retries/index.html
echo "--- sidebar sections ---"; grep -o '>Tutorials<\|>How-to guides<\|>Reference<\|>Explanation<' site/public/docs/how-to/handle-rate-limits-and-retries/index.html | sort -u
echo "--- search index ---"; ls site/public/*.search-data.json && grep -c 'superpowers' site/public/en.search-data.json
```
Expected: no WARN or ERROR lines; 23 or more pages; 0 superpowers; at least one resolved reference href; 0 raw `.md` hrefs; all four section names present; search index exists with 0 superpowers hits. If the build reports a deprecation warning for any key in `hugo.toml`, fix the key and note it in the report.

- [x] **Step 6: Smoke the dev server**

```bash
(cd site && hugo server --port 1313 >/tmp/hugo-server.log 2>&1 &) ; sleep 4; curl -fsS http://localhost:1313/typesafe-go/ | grep -o '<title>[^<]*</title>'; curl -fsS http://localhost:1313/typesafe-go/docs/how-to/ | grep -o '<title>[^<]*</title>'; pkill -f 'hugo server --port 1313'
```
Expected: two `<title>` lines.

- [x] **Step 7: Commit**

```bash
git add site/hugo.toml site/go.mod site/go.sum site/content/_index.md .gitignore
git commit -m "Add Hextra Hugo site mounting docs/"
```

---

## Task 4: Makefile targets and link checker

**Agent:** `site`

**Files:**
- Modify: `Makefile`, `tools/checkdocs.sh`

- [x] **Step 1: Makefile**

Add `site site-serve` to `.PHONY`, make `docs` depend on `site`, and add the two targets after `docs`:

```make
docs: site ## Build the site and check that every docs/ page is reachable and links resolve
	@./tools/checkdocs.sh

site: ## Build the documentation site into site/public
	cd site && hugo --gc --minify

site-serve: ## Serve the documentation site locally with live reload
	cd site && hugo server --buildDrafts --navigateToChanged
```
Also add `site/public` and `site/resources` to the `clean` target's `rm -rf`.

- [x] **Step 2: checkdocs.sh**

Two changes in `tools/checkdocs.sh`:

1. In the unlinked-page loop, `_index.md` files count as pages and must be linked from `README.md` or another page by their path (for example `tutorials/_index.md`) or by their directory link form; treat a match on either `$(basename "$page")` or `$(basename "$(dirname "$page")")/` as linked.
2. In the link-extraction pipeline, drop lines containing `{{<` or `{{%` before extracting links, so Hugo shortcode arguments are not parsed as Markdown links.

Concretely, replace the unlinked check with:

```bash
while IFS= read -r page; do
  name=$(basename "$page")
  dirlink="$(basename "$(dirname "$page")")/"
  if [ "$name" = "_index.md" ]; then
    grep -rqE --include='*.md' -e "${dirlink}_index\.md\)|${dirlink}\)" README.md docs && continue
    echo "unlinked: $page"; status=1
  elif ! grep -rq --include='*.md' -F "$name" README.md docs; then
    echo "unlinked: $page"; status=1
  fi
done < <(find docs -name '*.md' -not -path 'docs/superpowers/*' | sort)
```
(Hugo-style directory links such as `[x](how-to/)` and file links such as `[x](how-to/_index.md)` both satisfy it; `docs/_index.md` itself is linked from README in Task 8.)

And change the link extraction to strip shortcode lines:

```bash
  done < <(grep -v -e '{{<' -e '{{%' "$file" | grep -oE '\]\([^)]+\)' | sed -E 's/^\]\((.*)\)$/\1/' || true)
```
(Keep the existing `strip_code` fence handling that was added earlier; apply the shortcode filter after it.)

- [x] **Step 3: Verify and commit**

```bash
make docs && echo "docs ok"
make help | grep -E 'site|docs'
git add Makefile tools/checkdocs.sh
git commit -m "Add site build targets and teach the link checker about landing pages"
```
Expected: `docs ok`; help lists `docs`, `site`, `site-serve`. If `make docs` reports `docs/_index.md` unlinked, that is expected until Task 8 adds the README link; report it and proceed.

---

## Task 5: Deploy and PR workflows

**Agent:** `site`

**Files:**
- Create: `.github/workflows/hugo-deploy.yml`, `.github/workflows/hugo-pr.yml`

- [x] **Step 1: Write `hugo-deploy.yml`**

```yaml
name: Deploy documentation site

on:
  push:
    branches: [main]
    paths:
      - "docs/**"
      - "site/**"
      - ".github/workflows/hugo-deploy.yml"
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  build:
    runs-on: ubuntu-latest
    env:
      HUGO_VERSION: "0.166.0"
      HUGO_ENVIRONMENT: production
    steps:
      - name: Install Hugo
        run: |
          wget -O "${{ runner.temp }}/hugo.deb" "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.deb"
          sudo dpkg -i "${{ runner.temp }}/hugo.deb"
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Configure Pages
        id: pages
        uses: actions/configure-pages@v5
      - name: Cache Hugo modules
        uses: actions/cache@v4
        with:
          path: ~/.cache/hugo_cache
          key: ${{ runner.os }}-hugo-${{ hashFiles('site/go.sum') }}
          restore-keys: ${{ runner.os }}-hugo-
      - name: Build
        run: hugo --gc --minify -s site --baseURL "${{ steps.pages.outputs.base_url }}/"
      - uses: actions/upload-pages-artifact@v3
        with:
          path: site/public

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - id: deployment
        uses: actions/deploy-pages@v4
```
`actions/setup-go` is needed because Hugo modules shell out to `go` to fetch the theme.

- [x] **Step 2: Write `hugo-pr.yml`**

```yaml
name: Documentation site build check

on:
  pull_request:
    paths:
      - "docs/**"
      - "site/**"
      - ".github/workflows/hugo-pr.yml"

jobs:
  build:
    runs-on: ubuntu-latest
    env:
      HUGO_VERSION: "0.166.0"
    steps:
      - name: Install Hugo
        run: |
          wget -O "${{ runner.temp }}/hugo.deb" "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.deb"
          sudo dpkg -i "${{ runner.temp }}/hugo.deb"
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build
        run: hugo --gc --minify -s site --baseURL "https://therealbill.github.io/typesafe-go/"
      - name: Summary
        run: |
          echo "## Documentation site build" >> "$GITHUB_STEP_SUMMARY"
          echo "Pages: $(find site/public -name index.html | wc -l)" >> "$GITHUB_STEP_SUMMARY"
          echo "Size: $(du -sh site/public | cut -f1)" >> "$GITHUB_STEP_SUMMARY"
```

- [x] **Step 3: Validate YAML and commit**

```bash
python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in ['.github/workflows/hugo-deploy.yml','.github/workflows/hugo-pr.yml']]; print('yaml ok')"
git add .github/workflows/hugo-deploy.yml .github/workflows/hugo-pr.yml
git commit -m "Add GitHub Pages deploy and PR build workflows for the docs site"
```
Expected: `yaml ok`. Report "Task 5 done" to the lead.

---
## Task 6: Diátaxis inventory and gap reconciliation

**Agent:** `docs` (after Task 2 is committed)

- [x] **Step 1: Run the inventory**

Dispatch `diataxis-docs:doc-inventory` with this brief:

> Inventory `/Users/bill/Projects/gojev/docs/`, excluding `docs/superpowers/`. The Diátaxis kind of each page is in the `diataxis:` front-matter key (values tutorial, how-to, reference, explanation); `_index.md` files are section landing pages, not content. Classify each page by its actual content, flag any page whose content leaks into another quadrant (a how-to that teaches concepts, a reference that gives advice, an explanation with steps), and list gaps: things the code does that no page covers. The code is in the repository root (`*.go`), `otel/`, `internal/cli/`, `tools/selfreview/`, and the Makefile; read `go doc -all .` and `./bin/jev --help` (run `make build` first) to see the surface. Write `inventory.json` to `/private/tmp/claude-501/-Users-bill-Projects-gojev/b96adec7-3da2-45f5-b6b1-95fdebc40eb3/scratchpad/inventory.json`, not into the repo, and return the leakage findings and gap list in your report.

- [x] **Step 2: Reconcile with the candidate list**

Compare the inventory's gaps with the spec's candidate table (14 pages). For each candidate, confirm the behavior exists in the code by reading the source; drop any candidate that does not. Keep any inventory gap that the code supports and the candidates missed. For each leakage finding on an existing page, decide: move the leaking paragraph to the right quadrant's page (existing or new), or leave it with a one-line justification.

- [x] **Step 3: Report and wait**

Send the lead: the final page list per quadrant with a one-line scope each and the intended file name, the leakage decisions, and the inventory's classification of the 17 existing pages. Do not start Task 7 until the lead replies.

---

## Task 7: Write the new pages

**Agent:** `docs` (after the lead approves the Task 6 list)

**Files:**
- Create: the approved pages under `docs/tutorials/`, `docs/how-to/`, `docs/reference/`, `docs/explanation/`
- Modify: the four section `_index.md` lists

General rules for every page, in every brief:
- Front matter: `title`, `description`, `diataxis`, `weight` (continue each section's numbering in steps of 10).
- Every command output is captured from a real run against `./bin/jev` (run `make build` first) or the live API through a scratch Go module under the scratchpad with `replace github.com/therealbill/typesafe-go => /Users/bill/Projects/gojev`; the page shows the plain `go get` form.
- Every Go snippet passes `go vet` in that scratch module.
- Facts are verified against source before the brief is dispatched; the docs agent reads the relevant files itself and includes the verified facts in the brief.
- Cross-link with relative `.md` paths as the existing pages do.
- No attribution lines, no duration estimates.

Briefs for the candidate pages (adjust to the approved list):

**Tutorial `docs/tutorials/build-a-ticket-router.md`** (`diataxis-docs:doc-tutorial-writer`, weight 30): the reader builds a program that takes a support ticket, sends one `SystemOne` request with four independent questions (a Noul "about billing", a Choice of department with a `none` label, a Score of urgency, and a speculative Noul "would a human need to review this before replying"), then routes: high urgency or review-needed goes to a human queue, otherwise to the chosen department, with the Choice confidence below 0.6 also routed to a human. Checkpoints after each step show the real answers for two or three sample tickets. Ends by pointing to the confidence and thresholds how-to and the TypeSafe confidence documentation at https://docs.typesafe.ai/confidence.

**How-to `docs/how-to/ask-several-questions-in-one-request.md`** (`doc-howto-writer`, weight 60): a `Questions` map with mixed types; identifiers are for code and are not sent to the model; questions are independent and cannot see each other's answers; when a second request is warranted (an answer is needed to build new state); `RawQuestion` can be mixed in. Verified facts: `countQuestions` in client.go, the `Questions` type doc.

**How-to `docs/how-to/act-on-probabilities-and-confidence.md`** (weight 70): what `NoulAnswer.Noul`, `ChoiceAnswer.Confidence` and `.Probabilities`, `ScoreAnswer.Score` and `.Probabilities` mean; pick thresholds on your own data; a Noul near 0.5 means undecided, not medium; treat `none` labels explicitly. Links to https://docs.typesafe.ai/confidence.

**How-to `docs/how-to/send-fields-the-sdk-does-not-model-yet.md`** (weight 80): `RawQuestion` requires a string `type` and is sent as-is; `WithExtraBody` merges top-level fields and rejects `state`, `model`, `questions` with a `ValidationError` at path `extra_body.<key>`; in `jev ask` JSON mode, unknown top-level fields pass through and unknown question types pass through, but unknown fields inside `noul`/`choice`/`score` are rejected. Verified against client.go and internal/cli/request.go, with a real `jev ask` run showing the pass-through.

**How-to `docs/how-to/configure-logging.md`** (weight 90): `WithLogger`, `NewLogger(w, level)`, `TYPESAFE_LOG_LEVEL` accepting debug, info, warning or warn, error, with any other value (including `off` and unset) discarding; what each level logs (debug: each success; warn: retries and budget exhaustion and 4xx final failures; error: 5xx and transport final failures); the Authorization header and bodies are never logged; `jev --log-level` and the env fallback write to jev's stderr. Verified against client.go `NewLogger` and transport.go log calls.

**How-to `docs/how-to/run-the-self-review.md`** (weight 100): `make selfreview`, what it sends (spec section, implementation, tests), reading the table and flags, adding a real test versus recording an acceptance with a note, the stale-acceptance line, and the drift caveat. Verified by a real run (the report file is gitignored; quote the table).

**How-to `docs/how-to/cut-a-release.md`** (weight 110): tag `vX.Y.Z`, push the tag, `release.yml` runs goreleaser for darwin and linux on amd64 and arm64, binaries and `checksums.txt` attach to the GitHub release, version and commit are injected through ldflags and shown by `jev version`. Do not create a tag while writing; show the commands and cite the workflow file. Verified against `.goreleaser.yaml`, `.github/workflows/release.yml`, and `Makefile`.

**Reference `docs/reference/jev-ask-wire-format.md`** (`doc-reference-gen`, weight 70): the input document (`state`, `questions`, optional `model`, extra fields), each question's JSON shape, the output document (`model`, `answers` keyed by id with per-type fields, `usage`, `request_id`), the error envelope, and the exit-code link. Use `testdata/request.json` and `testdata/systemone_ok.json` as the canonical examples; state that they were captured from the live API.

**Reference `docs/reference/self-review-tool.md`** (weight 80): flags from `go run ./tools/selfreview -h`; `units.json` schema (spec, units[].name, spec_heading, implementation, tests, behaviors, accepted, notes); default thresholds; report JSON fields from the `unitReport` struct; exit codes 0, 1, 2 and what each means; the Markdown table columns.

**Reference `docs/reference/makefile-and-repository-layout.md`** (weight 90): the `make help` output verbatim; one line per top-level directory and package; the workflows and what triggers each.

**Explanation `docs/explanation/how-answers-are-decoded.md`** (`doc-explanation-writer`, weight 50): two-pass decoding on the `type` discriminator, `UnknownAnswer` preservation and round trip, deterministic field paths, the typed decode through `encoding/json` and why a type mismatch names types rather than the struct field, and the raw body as the escape hatch.

**Explanation `docs/explanation/the-instrumentation-hook.md`** (weight 60): why the core exposes a hook instead of importing OpenTelemetry, what `RequestInfo` and `RequestResult` carry, that the hook runs before validation and still fires on panics, how the otel package implements it, and what a different implementation (metrics, logging) would look like.

**Explanation `docs/explanation/the-agent-facing-cli-contract.md`** (weight 70): why `jev` mirrors the HTTP body on stdin, why every failure writes a JSON envelope to stdout and a sanitized line to stderr, the exit-code design including 130 for interrupts, the terminal-stdin rule, the input cap, and how a plugin is expected to consume it.

**Explanation `docs/explanation/what-the-self-review-measures.md`** (weight 80): how the tool turns the spec into Noul, Score, and Choice questions, why thresholds sit inside the model's drift band, what acceptances record, the retry unit-boundary lesson, and why it is a development tool rather than a CI gate.

- [x] **Step 1: Write and commit per quadrant**

After each quadrant's pages pass the docs agent's own source check, append them to that section's `_index.md` list and commit:

```bash
git add docs/tutorials && git commit -m "Add ticket-router tutorial"
git add docs/how-to && git commit -m "Add how-to guides for composition, confidence, forward compatibility, logging, self-review, and releases"
git add docs/reference && git commit -m "Add reference pages for the wire format, self-review tool, and repository layout"
git add docs/explanation && git commit -m "Add explanation pages for decoding, the instrumentation hook, the CLI contract, and the self-review"
```
Adjust messages to the approved list. Report all hashes.

---

## Task 8: Validation and README link

**Agent:** `docs`

**Files:**
- Modify: `README.md`, any page the validator flags

- [x] **Step 1: README**

In the `## Documentation` section, add as the first line:

```markdown
Published at <https://therealbill.github.io/typesafe-go/>. Sources are under [docs/](docs/_index.md).
```
Then add the new pages to the existing per-quadrant link lines.

- [x] **Step 2: Validate**

Dispatch `diataxis-docs:doc-crosslink-validator` over `docs/` (excluding `superpowers/`) and `README.md` with this note in the brief:

> This project's front matter uses `title`, `description`, `diataxis` (the kind), and `weight`. It deliberately does not use `summary`, `prerequisites`, `est_time`, `roles`, or `stability`; do not report their absence. `_index.md` files are section landing pages.

Fix everything else it reports. Then:

```bash
make docs && echo "docs ok"
```
Expected: `docs ok` with no output from the checker.

- [x] **Step 3: Commit**

```bash
git add README.md docs
git commit -m "Link the published site from the README and validate documentation"
```
Report "Task 8 done" with the validator's summary counts.

---

## Task 8b: Prose edit for plain, direct writing

**Agent:** `docs` (after Task 8; added during execution at the user's request)

The user reviewed the pages and found model mannerisms throughout: contrast-reveal sentences, narrated reasoning, staccato declaratives, em dashes, and prose that explains itself to the reader. Every page under `docs/` (excluding `superpowers/`), `README.md`, and `site/content/_index.md` gets an editing pass against Strunk and White: omit needless words, active voice, positive form, concrete language, one topic per paragraph, imperatives for instructions and plain declaratives for facts, third person in explanations, no em dashes, no rhetorical framing, no first-person narration. Facts, code, outputs, links, tables, headings, and front matter stay exactly as they are.

- [x] **Step 1: Dispatch one editing subagent per quadrant plus one for README and the landing page**, each with the rubric verbatim.
- [x] **Step 2: Read every edited page against the rubric; run the em-dash and banned-phrase greps before and after and report both counts.**
- [x] **Step 3: `make docs` passes; commit as "Edit documentation for plain, direct prose".**

---

## Task 9: Enable Pages, push, watch the deploy

**Agent:** lead (after Tasks 5 and 8 are committed)

- [x] **Step 1: Enable GitHub Pages from Actions (idempotent)**

```bash
gh api repos/therealbill/typesafe-go/pages >/dev/null 2>&1 && echo "pages already enabled" || gh api -X POST repos/therealbill/typesafe-go/pages -f build_type=workflow -q '.html_url'
```
Expected: `https://therealbill.github.io/typesafe-go/`.

- [x] **Step 2: Push and watch**

```bash
git push origin main
sleep 10
gh run list --workflow "Deploy documentation site" --limit 1
gh run watch $(gh run list --workflow "Deploy documentation site" --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
```
Expected: the run completes with conclusion success.

- [x] **Step 3: Check the live site**

```bash
curl -fsS https://therealbill.github.io/typesafe-go/ | grep -o '<title>[^<]*</title>'
curl -fsS https://therealbill.github.io/typesafe-go/docs/how-to/handle-rate-limits-and-retries/ | grep -o '<title>[^<]*</title>'
curl -fsS -o /dev/null -w '%{http_code}\n' https://therealbill.github.io/typesafe-go/docs/superpowers/
```
Expected: two titles; the third returns 404. Pages can take a minute after the deploy job to serve; retry once after 60 seconds if needed.

---

## Task 10: Final gate

**Agent:** lead

- [x] **Step 1: Local gate**

```bash
make lint && make test && make docs && git status --short && echo "(clean)"
```
Expected: all pass, working tree clean.

- [x] **Step 2: Tick the plan and report**

Tick every step in this plan, commit the plan, push, and report: commit count, page count on the site, the deploy run URL, and anything the user should check by hand (search, dark mode, phone width).
