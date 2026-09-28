# Hugo Documentation Site and Diátaxis Completion — Design

Date: 2026-09-28
Repository: `github.com/therealbill/typesafe-go` (renamed from `gojev` so the
Go module path and the repository agree)
Site URL: `https://therealbill.github.io/typesafe-go/`

## Goal

Publish the repository's Diátaxis documentation as a Hugo site on GitHub
Pages, built by a GitHub Actions workflow that runs only when documentation
changes, and complete the documentation set so every Diátaxis quadrant covers
what the code actually does.

Out of scope: publishing `docs/superpowers/` (specs and plans), versioned
docs, a custom domain, analytics, comments, and any change to the Go code.

## Constraints discovered

- The root `go.mod` belongs to the Go library. Hugo modules also use
  `go.mod`, and `go mod tidy` would strip a theme requirement from the root
  file. The site therefore lives in `site/` with its own nested `go.mod`,
  which Go ignores.
- Hugo can mount a parent directory: a site in `site/` mounting `../docs`
  builds correctly, and Hugo's default link render hook resolves the existing
  `../reference/x.md` cross-links to page URLs (verified 2026-09-28 with Hugo
  0.166.0).
- Hextra selects its sidebar layouts by Hugo `type: docs`. The pages' current
  `type: how-to` style key was this project's own convention, not the
  Diátaxis plugin's (its validator asks for `title`, `summary`,
  `prerequisites`, `est_time`, `roles`, `stability`). The kind moves to a
  `diataxis:` key and `type: docs` is cascaded from the section landing pages.
- Hextra's sidebar spans the whole tree only when the docs sit under one
  section, so `docs/` mounts to `content/docs`, giving URLs of the form
  `/docs/how-to/<page>/`.

## Layout

```
site/
  hugo.toml                 configuration (below)
  go.mod, go.sum            Hugo module github.com/therealbill/typesafe-go/site;
                            requires github.com/imfing/hextra v0.12.3
  content/_index.md         landing page, layout hextra-home
  layouts/_markup/render-link.html   only if Hextra's own hook does not
                            resolve .md links; otherwise absent
docs/
  _index.md                 "Documentation": the Diátaxis compass
  tutorials/_index.md       weight 1, cascade type: docs
  how-to/_index.md          weight 2, cascade type: docs
  reference/_index.md       weight 3, cascade type: docs
  explanation/_index.md     weight 4, cascade type: docs
  <pages>                   front matter: title, description, diataxis, weight
```

`hugo.toml` essentials:

```toml
baseURL      = "https://therealbill.github.io/typesafe-go/"
title        = "typesafe-go"
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
    excludeFiles = ["superpowers/**"]

[markup.goldmark.renderHooks.link]
  enableDefault = true
[markup.goldmark.renderer]
  unsafe = true            # Hextra shortcodes emit HTML
[markup.highlight]
  noClasses = false

[params]
  description = "Go client and jev CLI for the TypeSafe System One API"
  [params.navbar]
    displayTitle = true
    displayLogo  = false
  [params.theme]
    default = "system"
    displayToggle = true
  [params.search]
    enable = true
    type   = "flexsearch"
  [params.editURL]
    enable = true
    base   = "https://github.com/therealbill/typesafe-go/edit/main/site/content"
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

The edit link base above points at `site/content`, which is right for the
landing page but wrong for mounted pages; the plan verifies what Hextra
renders for mounted files and either sets a base that works for both or
disables the edit link. Whichever is chosen is recorded in the plan, not left
open.

`.gitignore` gains:

```
site/public/
site/resources/
site/.hugo_build.lock
```

## Front matter

Each documentation page:

```yaml
---
title: "How to Call Through a Gateway"
description: "One sentence."
diataxis: how-to           # tutorial | how-to | reference | explanation
weight: 30                 # sidebar order within its section
---
```

Each section landing page (`docs/<section>/_index.md`):

```yaml
---
title: "How-to guides"
description: "Goal-oriented recipes for readers who already know the basics."
weight: 2
cascade:
  type: docs
---
```

followed by one paragraph stating what the quadrant is for and what it is
not, and Hextra's `{{< cards >}}` or a plain list linking every page in the
section. `docs/_index.md` carries `type: docs` itself (so it renders with the
sidebar), describes the four quadrants in a two-by-two framing, and links each
section. Front matter `type: docs` never appears on individual pages; it
arrives by cascade.

## Landing page

`site/content/_index.md` uses `layout: hextra-home` and Hextra's hero and
feature-grid shortcodes: a headline, one sentence of description, two
buttons (Get started → the first tutorial, GitHub), the `go get` line and a
five-line Go example in a code block, and four feature cards, one per
quadrant, each linking to that section's landing page. No timing or
"minutes to read" text anywhere.

## Makefile and link checker

New targets:

```make
site: ## Build the documentation site into site/public
	cd site && hugo --gc --minify

site-serve: ## Serve the documentation site locally with live reload
	cd site && hugo server --buildDrafts --navigateToChanged
```

`docs` becomes `docs: site` so the site build runs as part of the local docs
gate; the existing `tools/checkdocs.sh` still runs after it.

`tools/checkdocs.sh` changes: treat `_index.md` files as pages that must be
reachable; skip lines that contain Hugo shortcode syntax (`{{<` or `{{%`)
when extracting links; keep excluding `docs/superpowers/`.

## Workflows

`.github/workflows/hugo-deploy.yml`:

- Trigger: push to `main` with paths `docs/**`, `site/**`, and
  `.github/workflows/hugo-deploy.yml`; plus `workflow_dispatch`. Because
  `docs/superpowers/**` is under `docs/**`, spec and plan commits also
  trigger a build; that is accepted (the build excludes those files and the
  cost is one short run).
- Permissions: `contents: read`, `pages: write`, `id-token: write`.
  Concurrency group `pages`, `cancel-in-progress: false`.
- Build job: install Hugo extended `0.166.0` from the release `.deb`;
  checkout with `fetch-depth: 0`; `actions/configure-pages@v5`; cache
  `~/.cache/hugo_cache` keyed on `hashFiles('site/go.sum')`; run
  `hugo --gc --minify -s site --baseURL "${{ steps.pages.outputs.base_url }}/"`;
  `actions/upload-pages-artifact@v3` with `path: site/public`.
- Deploy job: `actions/deploy-pages@v4`, environment `github-pages`, needs
  build.

`.github/workflows/hugo-pr.yml`: on `pull_request` with the same paths,
build only with the production `baseURL`, and append page count and total
size to `$GITHUB_STEP_SUMMARY`.

One-time setup, performed by the lead during implementation:
`gh api -X POST repos/therealbill/typesafe-go/pages -f build_type=workflow`
(idempotent check first with `GET`).

`ci.yml` is unchanged.

## Diátaxis completion

Process:

1. Run `diataxis-docs:doc-inventory` over `docs/` (excluding `superpowers/`)
   to classify existing pages, flag quadrant leakage, and list gaps.
2. Reconcile the inventory against the candidate list below; drop any
   candidate the code does not support; keep any gap the inventory finds that
   the code does support.
3. Write the pages with the plugin's writer agents. Every command output is
   captured from a real run and every code snippet passes `go vet` in a
   scratch module outside the repository; facts are verified against source
   by the docs agent before any brief is dispatched. No attribution lines, no
   duration estimates.
4. Run `diataxis-docs:doc-crosslink-validator` with the `diataxis` key
   declared as the kind field and the plugin's `summary`/`est_time`/`roles`/
   `stability` fields declared as not used by this project; fix what it
   reports.
5. `make docs` must pass.

Candidate pages (final list decided by step 2):

| Quadrant | Page |
|---|---|
| Tutorial | Build a ticket router: several independent questions in one request, thresholds on probability and confidence, a human-review branch for uncertain cases |
| How-to | Ask several questions in one request |
| How-to | Act on probabilities and confidence thresholds |
| How-to | Send fields the SDK does not model yet (`RawQuestion`, `WithExtraBody`) |
| How-to | Configure logging (`WithLogger`, `TYPESAFE_LOG_LEVEL`, what is redacted) |
| How-to | Run the self-review tool and read its report |
| How-to | Cut a release (tag, goreleaser, binaries) |
| Reference | `jev ask` wire format: input document, output document, error envelope |
| Reference | Self-review tool: flags, `units.json` schema, thresholds, report fields, exit codes |
| Reference | Makefile targets and repository layout |
| Explanation | How answers are decoded: two-pass decoding, unknown types, typed decoding |
| Explanation | The instrumentation hook: why a hook, what it receives, how otel implements it |
| Explanation | The agent-facing CLI contract: why exit codes, the error envelope, stdin rules |
| Explanation | What the self-review measures and why its readings drift |

Plus the five landing pages from the Layout section. The README's
Documentation section gains a first line linking to the published site; the
per-page links remain.

## Verification

Local, before push:

- `make site` exits 0 with no warnings.
- `make docs` exits 0.
- A built page under `site/public/docs/how-to/` contains an `href` to
  `/typesafe-go/docs/reference/…/` where the source has a `../reference/….md`
  link.
- `hugo server` starts and serves the landing page (agent smoke check).

Remote, after push:

- The deploy workflow run completes green (`gh run watch`).
- `curl -fsS https://therealbill.github.io/typesafe-go/` contains the site
  title; one deep page URL returns 200 and its title.
- Manual by the user: search returns results, dark mode toggles, phone width
  reads well.

## Execution

Agent team of two plus the lead, disjoint paths:

- `site` agent (`hugo-repo:hugo-site-architect`): `site/`, the two workflows,
  Makefile targets, `.gitignore`, `tools/checkdocs.sh`, and the front-matter
  migration of existing pages (mechanical `type:` → `diataxis:` plus
  `weight`), because that migration is what the site build depends on.
- `docs` agent (`general-purpose`, driving the Diátaxis agents): the five
  landing pages, the inventory, the new pages, validation, README link.
- Lead: Pages enablement, push, deploy watch, final check.

The site agent's front-matter migration lands first; the docs agent's landing
pages are needed for `make site` to produce sidebar sections, so the two
coordinate through the lead on that one dependency.
