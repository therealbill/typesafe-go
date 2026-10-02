# Language conventions for pairing

Where tests live and how to find definitions and references, per language. Use with `pairing-files.md`. Every command is read-only. Replace `Symbol` with the name from the spec.

## Any language

- Inventory: `git ls-files`
- Tests referencing a symbol: `grep -rln --exclude-dir=.git 'Symbol' <test locations>`
- Sizes: `wc -c <files>`
- Read one existing test before searching, to learn the naming the repository uses.
- Exclude dependency and build trees from every recursive grep below: `--exclude-dir={.git,node_modules,vendor,target,dist,build}`. A match under one of them is not evidence for a pairing.

## Go

- Layout: one package per directory. Tests are `_test.go` siblings in the same directory. A file declaring `package foo_test` is an external test package for `foo`.
- Files per package: `go list -f '{{.Dir}}: {{.GoFiles}} | {{.TestGoFiles}} {{.XTestGoFiles}}' ./...`
- Definition of a symbol: `grep -rn --include='*.go' --exclude='*_test.go' -E '^func (\([^)]*\) )?Symbol\(|^type Symbol\b|^\s*Symbol\s+=' .`
- Tests referencing it: `grep -rln --include='*_test.go' '\bSymbol\b' .`
- A method's receiver type names the file to look in first. Behaviors exercised through the client (retries, cancellation, budgets) are often asserted in the client's tests, not the helper's.
- Sizes for a package: `wc -c $(go list -f '{{range .GoFiles}}{{$.Dir}}/{{.}} {{end}}' ./pkg)`

## Python

- Tests: `tests/`, `test_*.py`, `*_test.py`; pytest.
- Definition: `grep -rn --include='*.py' -E '^\s*(def|class) Symbol\b' .`
- Tests referencing it: `grep -rln --include='test_*.py' --include='*_test.py' '\bSymbol\b' .`
- The import lines at the top of a test file name the modules it exercises.

## TypeScript and JavaScript

- Tests: `*.test.ts`, `*.spec.ts`, `__tests__/`, `test/`; Jest, Vitest, Mocha.
- Definition: `grep -rn --include='*.ts' --include='*.tsx' --include='*.js' -E '(export )?(default )?(async )?(function|class|const|let|interface|type|enum) Symbol\b' .`
- Tests referencing it: `grep -rln --include='*.test.ts' --include='*.spec.ts' --include='*.test.js' '\bSymbol\b' .`
- The `import` lines in a test name the modules under test.

## Rust

- Unit tests live in the implementation file under `#[cfg(test)] mod tests`. List that file under both `implementation` and `tests`.
- Integration tests: `tests/*.rs`, which use the crate's public API.
- Definition: `grep -rn --include='*.rs' -E '^\s*(pub(\([a-z]+\))? )?(fn|struct|enum|trait|type|const) Symbol\b' src`
- Test files referencing it: `grep -rl --include='*.rs' -E '#\[(test|tokio::test)\]' . | xargs grep -l '\bSymbol\b' /dev/null`. The `/dev/null` operand keeps `xargs` from running `grep` on standard input when the first command matches nothing.

## Other languages

Apply the "Any language" commands with the test naming the repository uses.
