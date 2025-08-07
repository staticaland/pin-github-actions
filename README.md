# pin-github-actions

CLI to pin GitHub Actions in workflow files to immutable commit SHAs.

## Install

### Homebrew (macOS/Linux)

```bash
brew install --cask staticaland/pin-github-actions/pin-github-actions
```

### Go install

```bash
go install github.com/staticaland/pin-github-actions@latest
```

## Usage

```bash
pin-github-actions <workflow-file>

# Example
pin-github-actions .github/workflows/release.yml
```

The tool will:
- detect all `uses: owner/repo@ref` entries
- resolve the latest release tag via the GitHub API
- replace `@ref` with the exact commit SHA and keep the version as a comment

Example replacement: `uses: actions/checkout@11bd... # v4.2.2`.

## Authentication

Requires a GitHub token with public repo read access. The token is discovered in this order:
- `GH_TOKEN`
- `GITHUB_TOKEN`
- token from `gh` (via `gh auth login`)

If no token is found, the program exits with an error.

## License

MIT
