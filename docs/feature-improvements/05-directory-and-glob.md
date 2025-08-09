# Feature: Directory/glob mode

Allow processing many workflow files in one run.

- Behavior: accept directory paths and glob patterns (e.g., `.github/workflows/*.y{a,}ml`); recurse by default for directories; support `--exclude` for globs.
- Output: per-file planned updates and a final summary; respect `--dry-run`/`--yes` consistently.
- Rationale: monorepos and multi-workflow repos benefit from bulk pinning.
