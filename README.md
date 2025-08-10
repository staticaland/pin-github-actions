# pin-github-actions

CLI to pin GitHub Actions in workflow files to immutable commit SHAs.

## Installation

### Homebrew (macOS/Linux)

First, tap the repository:

```bash
brew tap staticaland/pin-github-actions https://github.com/staticaland/pin-github-actions
```

Then, install the cask:

```bash
brew install --cask pin-github-actions
```

If the cask is unavailable on your Linux setup, use one of the alternatives below instead.

### From Source

```bash
go build -o pin-github-actions
```

Or install directly:

```bash
go install github.com/staticaland/pin-github-actions@latest
```

### Prebuilt binaries

Download a tarball for your OS/arch from the project releases and place the `pin-github-actions` binary on your `PATH`.

## Usage

```bash
pin-github-actions [--expand-major] [--policy <policy>] [--yes] [--dry-run] [--exclude <glob>]... <path|glob> [<path|glob> ...]

# Examples
pin-github-actions --policy same-major --yes .github/workflows/release.yml
pin-github-actions --dry-run .github/workflows/*.y{a,}ml
pin-github-actions --yes --exclude "**/legacy/*.yml" .github/workflows ./.github/ci
```

What it does:

- detect all `uses: owner/repo@ref` entries
- resolves versions and SHAs in parallel
- shows a "Planned updates" preview (from → to) with line/column hints
  - prompts for confirmation before writing: `Apply changes? [y/N]` (skipped when `--yes` is provided)
  - answering no leaves the file unchanged
  - answering yes writes the updated workflow file in place

Example replacement: `uses: actions/checkout@11bd... # v4.2.2`.

### Flags
- `--expand-major`: When the input ref is a moving major tag like `v4` or `4`, the tool will resolve the commit and then attempt to discover the exact full semver tag (e.g., `v4.2.2`) that points to that commit. The comment will use this full version instead of the major tag. This only affects the version shown in the comment; the pinned ref is still the immutable commit SHA.
- `--policy`: Controls how versions are selected relative to what's in your workflow. Defaults to `major`.
  - `major` (default): bump to the latest available version across all majors (Renovate-like "latest" behavior)
  - `same-major`: stay within the requested major and pick the latest tag for that major
  - `requested`: pin exactly the requested ref (e.g., resolve `v4` to the commit it currently points to)
- `--yes`: Apply updates non-interactively by skipping the confirmation prompt.
- `--dry-run`: Perform a non-destructive preview. Prints the planned updates and exits without prompting or writing to disk.
  - Exit code 0 when no changes are needed, 2 when changes would be made (useful in CI)
  - Mutually exclusive with `--yes`
- `--exclude <glob>`: Exclude files by glob pattern (repeatable). Works with glob and directory inputs; directories recurse by default.

## Authentication

- OS keychain entry `gh:github.com`
- `~/.config/gh/hosts.yml` (`github.com.oauth_token`)
- env var `GH_TOKEN` or `GITHUB_TOKEN`

## Related
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet)
- [GitHub Docs: Secure use of actions in workflows](https://docs.github.com/en/actions/reference/security/secure-use)

## License

MIT
