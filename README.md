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

### From Source

```bash
go build -o pin-github-actions
```

Or install directly:

```bash
go install github.com/staticaland/pin-github-actions@latest
```

## Usage

```bash
pin-github-actions [--expand-major] [--policy <policy>] <workflow-file>

# Example
pin-github-actions --policy same-major .github/workflows/release.yml
```

The tool will:

- detect all `uses: owner/repo@ref` entries
- resolve the latest release tag via the GitHub API
- replace `@ref` with the exact commit SHA and keep the version as a comment

Example replacement: `uses: actions/checkout@11bd... # v4.2.2`.

### Options

- `--expand-major`: When the input ref is a moving major tag like `v4` or `4`, the tool will resolve the commit and then attempt to discover the exact full semver tag (e.g. `v4.2.2`) that points to that commit. The comment will use this full version instead of the major tag. This only affects the version shown in the comment; the pinned ref is still the immutable commit SHA.
- `--policy`: Controls how versions are selected relative to what's in your workflow. Defaults to `major`.
  - `major` (default): bump to the latest available version across all majors (Renovate-like "latest" behavior)
  - `same-major`: stay within the requested major and pick the latest tag for that major
  - `requested`: pin exactly the requested ref (e.g., resolve `v4` to the commit it currently points to)

## Authentication

Requires a GitHub token with public repo read access. The token is discovered in this order:

- `GH_TOKEN`
- `GITHUB_TOKEN`
- token from `gh` (via `gh auth login`)

If no token is found, the program exits with an error.

## License

MIT
