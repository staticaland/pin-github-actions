# Conventional Commits

- ALWAYS write commit messages and PR titles using Conventional Commits.
- Format: `type(scope)!: short, imperative summary`
  - `type`: one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
  - `scope` (optional): area/package/component, e.g. `cli`, `parser`, `docs`, `release`. Prefer kebab-case.
  - `!` (optional): use when introducing a breaking change.
  - `summary`: imperative, present tense, no trailing period, ideally ≤ 72 chars.
- Body (optional but recommended): why the change was made and notable details.
- Footer (as needed):
  - `BREAKING CHANGE: <description>` when breaking behavior.
  - `Closes: #123` / `Refs: #123` to link issues.

We use Conventional Commits to drive automated changelogs and versioning. Ensure squash-merge PRs produce a final title/body that conforms.

## Examples

- `feat(cli): add --config flag`
- `fix(parser): handle empty input without panic`
- `refactor: simplify validation flow`
- `perf(cache)!: drop LRU in favor of ARC for better hit rate`

Body example:

```
feat(release): enable release-please manifest

This integrates release-please to automate version bumps and changelog
entries based on commit types. No behavioral changes at runtime.

Refs: #42
```