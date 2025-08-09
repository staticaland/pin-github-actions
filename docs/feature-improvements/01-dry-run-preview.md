# Feature: Dry-run preview mode (`--dry-run`)

Provide a non-destructive mode that resolves and displays planned updates without prompting or writing to disk.

- Behavior: perform full scan and resolution; print the "Planned updates" table; do not modify files; exit code 0 when no changes, 2 when changes would be made (useful in CI).
- Flags: `--dry-run` mutually exclusive with `--yes`.
- Rationale: enables safe auditing and CI gating.
