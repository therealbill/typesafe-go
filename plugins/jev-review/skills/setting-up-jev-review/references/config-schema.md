# jev-review.json

The config `jev review` reads. The two field tables are checked against the Go structs by `tools/checkplugin.sh` in the typesafe-go repository: keep every field on a row that starts with its backticked name, and start no other table row that way.

## Top level

| Field | Type | Required | Meaning |
|---|---|---|---|
| `description` | string | no | Free text. |
| `spec` | string | unless every unit sets its own | Path to the spec file, relative to this config file. |
| `units` | array | yes, at least one | The units below. |

## Unit

| Field | Type | Required | Meaning |
|---|---|---|---|
| `name` | string | yes, unique | Names the unit in the report and in `--only`. |
| `spec` | string | no | Spec file for this unit only; overrides the top-level one. Relative to the config file. |
| `spec_heading` | string | yes | A heading line matched exactly, `#` characters included: `"## Retry policy"`. |
| `implementation` | array of string | yes, at least one | Files that implement the section. |
| `tests` | array of string | no | Files that test it. Empty is allowed and reads as untested. |
| `behaviors` | array of string | yes, at least one | One checkable claim per entry. They become `covers_00`, `covers_01`, and so on, in order. |
| `accepted` | array of string | no | Flags reviewed and allowed: a behavior's exact text, or `contradicts_spec`, `thoroughness`, `weakest_area`. A `covers_NN` entry is rejected at load. |
| `notes` | array of string | no | Why each acceptance was granted. Copied into the report. |

## Paths

Every path, including `spec`, resolves relative to the directory containing the config file, from wherever `jev review` runs. An absolute path is used as is. Use forward slashes.

`--units` defaults to `jev-review.json` in the working directory. A config written anywhere else needs `--units` on every run.

## Starter

`jev review init` writes this file. It refuses to overwrite an existing one and exits 1:

```json
{
  "description": "Units for jev review. Paths are relative to this file. Each unit names one spec section, the files that implement it, the files that test it, and the behaviors the section requires, one concrete claim per line. A unit may set its own \"spec\" to override the top-level one.",
  "spec": "docs/design.md",
  "units": [
    {
      "name": "example",
      "spec_heading": "## Retry policy",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "A 429 response is retried after the Retry-After delay",
        "Retries stop after MaxRetries and the last error is returned"
      ],
      "accepted": [],
      "notes": []
    }
  ]
}
```

## Checking a config

`jev review --dry-run --units <path>` loads the config, extracts every unit's section, reads and bundles every file, and stops before the API call. No key, no network, no report file. Exit 0 when every unit resolved; 8 when a heading or file was missing, with the unit's error under the table; 1 when the config is invalid. A `--only` name that matches no unit exits 1. Add `--json` for the report as JSON.

## Running the review

`jev review --units <path>` needs `TYPESAFE_API_KEY` and sends the spec section and every listed file to the TypeSafe API. Exit 0 clean, 8 flagged, 1 bad config, 3 to 7 API or transport failure. It writes `jev-review-report.json` next to where it runs; add that file to `.gitignore`.

Flags: `--only NAME`, `--json`, `--report PATH`, `--min-cover 0.6`, `--max-contradict 0.4`, `--min-thorough 2`, `--max-state-bytes 100000`, `--unit-timeout 2m`.

## Installing jev

- `go install github.com/therealbill/typesafe-go/cmd/jev@latest`, with Go 1.25 or newer.
- A binary from https://github.com/therealbill/typesafe-go/releases.
- `--dry-run` arrived in the first release after v0.1.1. Check with `jev review --help`.

Source: https://therealbill.github.io/typesafe-go/docs/reference/jev-review/
