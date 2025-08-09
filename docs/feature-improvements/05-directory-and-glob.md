# Feature: Directory/glob mode

Enable processing many workflow files in one run.

- Behavior: accept one or more positional arguments that may be files, directories, or glob patterns (e.g., `.github/workflows/*.y{a,}ml`); for directories, recurse by default and match `**/*.y{a,}ml`; deduplicate results and process in stable, sorted order; warn on no matches.
- Flags: support repeatable `--exclude <glob>` filters applied after expansion; combine with existing flags; no interactivity changes.
- Output: print a per-file header and planned updates table for each match, then a final cross-file summary; respect `--dry-run`/`--yes` consistently.
- Exit codes: 0 when no changes are needed; 2 when changes would be or were applied (aggregate across files); non-zero on errors.
- Rationale: monorepos and multi-workflow repos benefit from bulk pinning with reproducible, scriptable results.
