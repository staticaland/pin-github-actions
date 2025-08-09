### pin-github-actions vs. Renovate

This document compares `pin-github-actions` with Renovate, highlights ideas we can learn from Renovate, and explores whether Renovate can be used to achieve similar results.


## What this tool does

`pin-github-actions` is a focused CLI that:
- Pins all `uses: owner/repo@ref` entries in a workflow file to immutable commit SHAs.
- Resolves versions via the GitHub API based on an update policy:
  - `major` (default): pick the latest across all majors
  - `same-major`: stay within the current major
  - `requested`: resolve exactly the requested ref (e.g., `v4` → current commit)
- Optionally expands moving majors (`vN` or `N`) to a full semver for the trailing comment via `--expand-major`.
- Prints a planned updates preview with line/column hints, prompts for confirmation, then updates in-place (or uses `--yes` non-interactively).

Scope: a single repository file at a time (today). It does not open PRs, schedule runs, or orchestrate across repos.


## How it differs from Renovate

- Purpose and scope
  - pin-github-actions: single-purpose CLI to pin Actions to SHAs with clear, deterministic in-file edits.
  - Renovate: full-featured dependency update bot for many ecosystems, with scheduling, grouping, PR automation, and policy management.

- Output and UX
  - pin-github-actions: local preview, in-place edits, optional interactivity.
  - Renovate: automated branches/PRs with rich metadata, labels, and dashboards.

- Configuration model
  - pin-github-actions: command-line flags; minimal/no repo-wide configuration.
  - Renovate: extensive JSON-based configuration, presets, and inheritance for org-wide policy.

- Update strategy for GitHub Actions
  - pin-github-actions: explicitly converts any `@ref` to an immutable commit SHA and optionally retains the resolved version as a comment (e.g., `# v4.2.2`).
  - Renovate: manages `uses:` references as a package manager (“github-actions”). It focuses on updating refs to newer versions/tags and opening PRs. Pinning to SHAs is not the default behavior; Renovate’s core strength is managing tags/versions and PR workflows across repos.

- Execution environment
  - pin-github-actions: runs locally or in CI as a one-shot command (requires a GitHub token).
  - Renovate: runs as a bot (hosted or self-hosted) and continuously monitors repositories.


## Ideas we can learn from Renovate

- Rich policy + presets
  - Central, shareable configuration with presets to standardize policy across many repos.
- Scheduling and noise reduction
  - Time windows, rate limits, grouping, and stability days to reduce PR noise.
- PR ergonomics
  - Descriptive PR titles/bodies, labels, changelog links, and clear upgrade rationale.
- Safety/merge strategies
  - Automerge rules gated by tests and policies; confidence indicators.
- Org-wide rollout
  - Onboarding PRs and dashboards that explain and track what the bot will do.

These ideas can influence future enhancements like repo-level config, directory/glob support, CI-friendly JSON output, and optional PR creation.


## Can Renovate do the same things?

Short answer: partially, with trade-offs.

- Renovate can already update GitHub Actions refs to newer versions and open PRs automatically. If your goal is “keep Actions up to date,” Renovate does this very well.
- If your goal is “pin all Actions to immutable SHAs and keep them pinned,” Renovate does not, by default, rewrite tag-based refs to commit SHAs. Its native flow is tag-oriented. You can approximate pinning outcomes with a hybrid approach (below).


## Recommended hybrid workflows

1) Use this CLI within Renovate PRs (post-upgrade task)
- Let Renovate detect and stage updates for `github-actions`.
- Add a post-upgrade step that runs `pin-github-actions` to convert updated refs to SHAs before the PR is finalized.
- Benefits: Renovate handles discovery, PRs, scheduling, grouping, and policy; this tool ensures the in-repo state is pinned to SHAs.

Example `renovate.json` snippet:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:base"],
  "enabledManagers": ["github-actions"],
  "packageRules": [
    {
      "matchManagers": ["github-actions"],
      "postUpgradeTasks": {
        "commands": [
          "bash -lc 'shopt -s globstar nullglob; for f in .github/workflows/**/*.yml .github/workflows/**/*.yaml; do /usr/local/bin/pin-github-actions --policy same-major --yes "$f" || exit 1; done'"
        ],
        "fileFilters": [".github/workflows/**"]
      }
    }
  ]
}
```

Notes:
- Ensure the `pin-github-actions` binary is available in the Renovate runtime (e.g., via a custom Renovate image or pre-step).
- Provide a GitHub token via environment (`GH_TOKEN`/`GITHUB_TOKEN`).

2) Let Renovate drive PRs; run this CLI in CI on renovate branches
- Configure Renovate normally for `github-actions`.
- Add a CI job that runs on `renovate/*` branches to pin files to SHAs, commit the changes back to the PR branch.
- Pros: avoids custom Renovate image; Cons: requires CI write permissions and careful loop-avoidance.

3) Use Renovate’s Regex Manager to orchestrate version discovery, this CLI for pinning
- If you need advanced matching or custom mapping of tags → SHAs, you can use Renovate’s Regex Manager to annotate matches and then run this CLI as a post step.
- Pros: maximum flexibility; Cons: more configuration complexity.


## When to prefer each tool

- Prefer `pin-github-actions` when:
  - You want deterministic, local control to pin workflows to SHAs with a clear preview and minimal configuration.
  - You need to enforce a specific version-selection policy at the file level.
  - You do not need PR orchestration or cross-repo scheduling.

- Prefer Renovate when:
  - You want automated, continuous dependency updates with PRs, labels, grouping, and schedules across many repos.
  - Tag-based management is acceptable, or you pair Renovate with this CLI to enforce SHA pinning.


## Limitations and open questions

- Renovate’s native `github-actions` manager focuses on tag-based updates; it does not automatically convert tags to SHAs. A hybrid approach is typically required to keep refs pinned to SHAs while leveraging Renovate’s PR automation.
- If you already use SHAs, Renovate may not infer which upstream tag/version that SHA corresponds to for upgrade logic without additional context or a helper step. The hybrid patterns above address this by invoking this CLI during PR creation.


## Conclusion

- Use `pin-github-actions` to guarantee immutable SHAs and a predictable, auditable on-disk state.
- Use Renovate to scale scheduling, policy, and PR workflows across repositories.
- Combine them to get the best of both: Renovate orchestrates, this CLI enforces SHA pinning.