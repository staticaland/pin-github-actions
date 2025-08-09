# Feature: Improved CLI help and usage (no deps)

Make the help output more informative and consistent using only the Go standard library `flag` package.

- Behavior: enhance `flag.Usage` to print a short description, `Usage:` line, `flag.PrintDefaults()` for options, an `Examples:` block, an `Environment:` block (`GH_TOKEN`, `GITHUB_TOKEN`), and a brief `Exit codes:` note; print to stdout on explicit help and to stderr on usage errors.
- Flags: add `--version` to print `pin-github-actions <version> (<commit>)` and exit 0; keep `-h` working and accept `--help` by normalizing `os.Args` (convert leading `--name` to `-name`) before `flag.Parse`.
- Notes: preserve single-dash long options (`-policy`) for compatibility, but allow `--policy` via normalization; avoid external CLI frameworks or dependencies.
- Rationale: improves discoverability and aligns with common Unix conventions without increasing binary size or complexity.