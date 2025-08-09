## Test Plan for pin-github-actions

This plan proposes unit, integration (with mocked GitHub API), and light CLI tests to cover core behavior, edge cases, and regressions for the CLI that pins GitHub Actions to commit SHAs.

### Goals
- Validate YAML parsing and occurrence extraction.
- Verify version/commit resolution across policies and tag shapes.
- Ensure content rewriting is correct, deterministic, and idempotent.
- Exercise error paths and token discovery behavior.
- Provide structure for fast, hermetic tests (no real network).

### Unit tests (pure/local functions)
- **parsePolicy**
  - Accepts: "", "major", "latest-major", "latest" -> UpdatePolicyMajor, no error.
  - Accepts: "same-major", "stay-major", "minor", "patch" -> UpdatePolicySameMajor, no error.
  - Accepts: "requested", "exact", "pin-requested" -> UpdatePolicyRequested, no error.
  - Unknown string returns error (and default value is UpdatePolicyMajor).
  - Implemented: see `main_policy_test.go` with a table-driven test covering valid synonyms and error on unknown policy. Verified with `go test` passing.
- **prettyRef / isFullSHA**
  - Empty -> "(none)".
  - 40-hex -> abbreviated to first 12 + ellipsis; mixed-case hex accepted; any non-hex or wrong length -> unchanged.
- **isMovingMajorTag**
  - Matches: `v4`, `4`.
  - Non-matches: `v4.2`, `main`, SHA.
- **parseMajor**
  - `v4`, `4` -> 4.
  - `v4.2.2` -> 4.
  - Invalid strings -> (0,false).
- **normalizeMajorRef**
  - `4` -> `v4`; `v4` -> `v4`.
- **computeLineCol**
  - Start-of-file, mid-file, end-of-file positions produce correct 1-based line/column.
  - Multi-line content with varying line lengths.
- **extractActions**
  - De-duplicates, sorts alphabetically.
  - Handles various spacings: `uses:owner/repo@v1`, `uses:  owner/repo@v1`, `uses: owner/repo` (no @).
  - Ignores comments-only lines and non-`uses:` yaml keys.
- **extractOccurrences**
  - Finds `uses: owner/repo@ref` with/without trailing inline comments.
  - Correctly populates `MatchStart/End`, `ReplaceStart/End`, `RequestedRef`, `Action`, and 1-based `Line/Column`.
  - Multiple occurrences across jobs/steps.
- **updateContent**
  - Rewrites only targeted `@ref…` spans to `@<sha> # <version>`.
  - Preserves unrelated content and YAML formatting.
  - Skips occurrences when resolved SHA equals current ref.
  - Multiple non-overlapping replacements in order.
  - Idempotency: applying twice yields identical content.
  - Cases with existing inline comments (replace entire post-@ segment up to end-of-match).

### Integration tests (mocked GitHub API)
Use `httptest.Server` to emulate the GitHub API and a `go-github` client with custom BaseURL for tests. Cover:

- **resolveTagToCommitSHA**
  - Lightweight tag: `GET /repos/:owner/:repo/git/ref/tags/:name` returns object type `commit`.
  - Annotated tag: initial object type `tag` -> `GET /repos/:owner/:repo/git/tags/:sha` dereferences to commit.
  - 404 -> error `tag not found`.
  - Missing/empty SHA -> error.
- **selectTagBySemverOrNewest**
  - Tag list includes semver and non-semver; chooses highest semver when available.
  - No parsable semver -> chooses newest (first returned tag).
  - No tags -> error.
- **selectTagBySameMajor**
  - Multiple paginated pages with mixed majors; ensures only requested major considered.
  - Early-stop heuristic: after first page with matches, stop once a later page returns no matches.
  - No tags for major -> error.
- **findFullSemverTagForMajorCommit**
  - Finds match by comparing lightweight tag SHAs first; if not found, dereferences annotated tags.
  - No match -> error.
- **resolveActionForPolicy**
  - requested: ref is moving major (`v4` or `4`) -> resolves to commit of major ref; with `expandMajor` true, comment becomes full semver tag corresponding to resolved commit.
  - requested: ref is exact semver tag -> returns its SHA.
  - requested: ref is SHA -> returns unchanged.
  - same-major: input ref has major N -> picks highest `vN.x.y` tag.
  - major (default): latest release path happy case; fallback to tags when release exists but cannot be resolved; unexpected non-404 error from releases bubbles up as error.
- **getActionInfosForOccurrences**
  - Parallel resolution over multiple occurrences.
  - Cache hit: same `owner/repo|policy|requestedRef` triggers only one API sequence; assert by counting server hits.

### CLI/behavioral tests (no network)
- `main` with no args: prints usage, exits code 1.
- Non-existent file: prints error and exits code 1.
- File with no `uses:` lines: prints "No actions" and exits code 1 (current behavior).
- Confirm prompt flow using stdin:
  - Answer `n`: no write; content unchanged.
  - Answer `y`: writes updated content; verify on disk.

Note: For end-to-end pinning, consider adding an env-controlled base URL (e.g., `GITHUB_API_URL`) so CLI can target a test server. This will make full CLI integration tests hermetic without real network.

### Token discovery tests
- `GH_TOKEN` set: used and preferred over `GITHUB_TOKEN`.
- `GITHUB_TOKEN` set only: used.
- `gh hosts.yml` fallback: set HOME to a temp dir with `.config/gh/hosts.yml` including `oauth_token`; ensure it is used when env vars unset.
- No token anywhere -> error message instructing how to set token.
- Keyring path is difficult to test portably; prefer to keep untested or behind an interface.

### Edge-case YAML fixtures
Include small fixtures to drive extraction and rewriting:

- Inline comments and spacing variations:
  ```yaml
  steps:
    - uses:actions/checkout@v4 # comment
    - uses:  actions/setup-node@v4.0.0
    - uses: owner/repo@v1-alpha.1  # pre-release
  ```
- Multiple occurrences of the same action with different refs.
- Uses without `@` (should be ignored by `extractOccurrences`, still listed by `extractActions`).
- Very long files to test line/column and stability.

### Non-functional tests
- Run tests with `-race` to catch data races in concurrency (notably the cache in `getActionInfosForOccurrences`).

### Potential defects surfaced by scan (tests should expose these)
- `printPlannedChanges` references `ActionInfo.Line` and `.Column`, but `ActionInfo` lacks these fields.
- `updateContent` indexes `actionInfos` like a map and calls `occurrenceKey(occ)`, which does not exist; current signature accepts `[]ActionInfo`. Implementing tests around `updateContent` will not compile until this is corrected (e.g., pass a map keyed by occurrence identity or align types).
- `loadConfig` and `Config` are currently unused.

### Proposed test file layout
- Unit tests:
  - `main_extract_test.go` (extractActions, extractOccurrences, computeLineCol)
  - `main_util_test.go` (policy parsing, ref helpers)
  - `main_update_test.go` (updateContent cases)
- Integration tests with mock API:
  - `main_resolve_test.go` (tag/release resolution, policies, caching)
- CLI behavior tests:
  - `main_cli_test.go` (arg parsing, prompts, no-network paths)

### Coverage target
- Aim for ≥85% line coverage on `main.go`, with meaningful branch coverage across policy and API error paths.
