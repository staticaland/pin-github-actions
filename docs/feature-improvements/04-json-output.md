# Feature: Machine-readable output (`--output json`)

Expose a structured JSON output mode for tooling.

- Behavior: `--output json` prints a single JSON object with fields: `file`, `occurrences` (array of `{action, line, column, requestedRef, resolvedVersion, sha, changed}`), and summary counts.
- Defaults: keep current human-readable output; `--output text` as explicit alias.
- Rationale: enables integrations and richer CI annotations.
