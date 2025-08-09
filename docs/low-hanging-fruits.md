# Low-hanging fruits

Quick, high-impact improvements that are easy to implement. Grouped by area with suggested actions.

## Code & CLI UX
- Add `--version` flag
  - Why: Surface build metadata (`version`, `commit`) already present in `main.go`.
  - How: Add a flag that prints `<name> <version> (<commit>)` and exits 0.
  - Effort: Very low.

- Add `--dry-run` alias
  - Why: There is an interactive preview by default; an explicit non-writing mode helps scripting.
  - How: `--dry-run` prints planned updates and exits without prompting or writing.
  - Effort: Very low.

- Standardize exit codes
  - Why: CLI consumers benefit from predictable codes (e.g., 0=success/no changes, 2=input error, 3=auth error).
  - How: Map common failure paths (no token, file not found, invalid YAML) to well-documented codes.
  - Effort: Low.

## Tests
- Add unit tests for extraction
  - Why: `extractOccurrences` is core and currently untested; easy to cover with table-driven cases and YAML fixtures.
  - How: Add `main_extract_test.go` with edge cases (spacing, inline comments, no `@`, multiple occurrences).
  - Effort: Low.

- Add integration tests with mocked GitHub API
  - Why: Validate tag/release resolution and policy behavior without network.
  - How: Use `httptest.Server` and a `go-github` client with custom BaseURL; cover annotated vs lightweight tags, latest release fallback, same-major selection, caching behavior.
  - Effort: Low–medium.

- Race/coverage in CI
  - Why: Catch data races (e.g., concurrent resolution cache) and track coverage trends.
  - How: Run `go test -race -cover ./...` in CI; optionally upload coverage artifact.
  - Effort: Very low.

## CI/CD
- Speed up test job
  - Why: Faster feedback.
  - How: Use `actions/setup-go` with `go-version-file: go.mod` and `cache: true`.
  - Effort: Very low.

- Add linting step
  - Why: Surface static issues early.
  - How: Add `golangci/golangci-lint-action` (pinned) with a minimal config; keep it non-blocking initially if preferred.
  - Effort: Low.

- Add `go vet` step in tests workflow
  - Why: Ensure static vet checks run in CI.
  - How: Add a step `go vet ./...` before tests.
  - Effort: Very low.

- Consider a "pin sanity" job (optional)
  - Why: Ensure workflow action refs remain pinned.
  - How: Run this tool in `--dry-run` mode against `.github/workflows/*.yml` and fail if any ref is unpinned or outdated.
  - Effort: Medium.

## Dependencies
- Bump safe updates
  - Why: Keep dependencies current; several updates available.
  - How: Update direct deps (e.g., `github.com/Masterminds/semver/v3` to `v3.4.0`); run `go mod tidy`, tests, vet.
  - Effort: Low.

- Track Go version in CI via go.mod
  - Why: Avoid drift.
  - How: Switch `go-tests.yml` to `go-version-file: go.mod` instead of hard-coded `1.21`.
  - Effort: Very low.

## Release/Packaging
- Expand builds
  - Why: Wider audience.
  - How: Add `windows` to `.goreleaser.yaml` `goos`; consider `arm64` where applicable.
  - Effort: Low.

- Homebrew formula (optional)
  - Why: Typical CLIs ship as formula, not just cask; enables `brew install` without `--cask`.
  - How: Add `brews` section in GoReleaser or maintain a tap formula.
  - Effort: Medium.

## Documentation
- Add `LICENSE` file (MIT)
  - Why: README states MIT; include the standard license text at repo root.
  - How: Add `LICENSE` with MIT template.
  - Effort: Very low.

- Add `CONTRIBUTING.md`
  - Why: Guides PRs/issues and local dev (tests, lint, release process).
  - How: Outline justfile tasks, testing with `-race`, and release automation.
  - Effort: Low.

- Clarify README
  - Why: Improve first-run experience.
  - How: Document exit codes, examples for each `--policy`, `--expand-major` behavior, and non-interactive usage with `--yes`.
  - Effort: Very low.

## Developer ergonomics
- Enhance `justfile`
  - Why: One-liners for common flows.
  - How: Add `test-race`, `lint`, `vet`, `update-deps` targets.
  - Effort: Very low.

- Add `.editorconfig`
  - Why: Consistent editors across contributors.
  - How: Basic UTF-8, LF, 2 spaces for YAML/MD, tabs for Go.
  - Effort: Very low.

---

If helpful, I can open PR(s) to:
- Update CI (`go-version-file`, caching, `-race`, `-cover`, `vet`).
- Add `--version` and `--dry-run` flags.
- Add tests for `extractOccurrences` and a mocked-API suite.
- Add `LICENSE`, `CONTRIBUTING.md`, and `justfile` targets.
- Bump direct dependencies and run `go mod tidy`.