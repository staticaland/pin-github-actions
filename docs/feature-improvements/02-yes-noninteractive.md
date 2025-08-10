# Feature: Non-interactive apply (`--yes`/`--write`)

Allow automatic application of updates without the confirmation prompt.

- Behavior: when set, skip `Apply changes?` and write updates immediately.
- Combine with: `--dry-run` forbidden together; exit non-zero on write errors.
- Rationale: supports batch scripts and CI automation.
