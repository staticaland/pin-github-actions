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
pin-github-actions [--expand-major] [--policy <policy>] [--yes] [--dry-run] <workflow-file>

# Example
pin-github-actions --policy same-major --yes .github/workflows/release.yml
```

What it does:

- detect all `uses: owner/repo@ref` entries
- resolve a version based on your policy (see Options below). By default, it uses the latest GitHub release if available; otherwise it falls back to the highest semantic version tag; if no semver tags exist, it falls back to the newest tag returned by the API
- replace `@ref` with the exact commit SHA and keep the chosen version as a trailing comment

Flow:

- prints discovered actions
- resolves versions and SHAs in parallel
- shows a "Planned updates" preview (from → to) with line/column hints
- prompts for confirmation before writing: `Apply changes? [y/N]` (skipped when `--yes` is provided)
  - answering no leaves the file unchanged
  - answering yes writes the updated workflow file in place

Example replacement: `uses: actions/checkout@11bd... # v4.2.2`.

### Options

- `--expand-major`: When the input ref is a moving major tag like `v4` or `4`, the tool will resolve the commit and then attempt to discover the exact full semver tag (e.g., `v4.2.2`) that points to that commit. The comment will use this full version instead of the major tag. This only affects the version shown in the comment; the pinned ref is still the immutable commit SHA.
- `--policy`: Controls how versions are selected relative to what's in your workflow. Defaults to `major`.
  - `major` (default): bump to the latest available version across all majors (Renovate-like "latest" behavior)
  - `same-major`: stay within the requested major and pick the latest tag for that major
  - `requested`: pin exactly the requested ref (e.g., resolve `v4` to the commit it currently points to)
- `--yes`: Apply updates non-interactively by skipping the confirmation prompt.
- `--dry-run`: Perform a non-destructive preview. Prints the planned updates and exits without prompting or writing to disk.
  - Exit code 0 when no changes are needed, 2 when changes would be made (useful in CI)
  - Mutually exclusive with `--yes`

## Authentication

Requires a GitHub token with public repo read access. The token is discovered in this order:

- `GH_TOKEN`
- `GITHUB_TOKEN`
- token from `gh` (via `gh auth login`) discovered via:
  - OS keychain entry `gh:github.com`
  - `~/.config/gh/hosts.yml` (`github.com.oauth_token`)

If no token is found, the program exits with an error.

## Similar tools & related resources

- [Renovate](https://github.com/renovatebot/renovate)
- [Dependabot](https://github.com/dependabot/dependabot-core)
- [stacklok/frizbee](https://github.com/stacklok/frizbee)
- [Pin your GitHub Actions (Michael Heap)](https://michaelheap.com/pin-your-github-actions/)
- [GitHub Actions: Security Risk (Julien Renaux)](https://julienrenaux.fr/2019/12/20/github-actions-security-risk/)
- [mheap/pin-github-action](https://github.com/mheap/pin-github-action)

## License

MIT
