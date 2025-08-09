# Feature: GitHub Enterprise support (`--api-base` / env)

Add support for GitHub Enterprise by making the API base configurable.

- Behavior: detect `GITHUB_API_URL` or new `--api-base` flag; construct `go-github` client with custom BaseURL and UploadURL.
- Auth: prefer `GH_TOKEN`/`GITHUB_TOKEN` as today; allow host-scoped tokens from `gh hosts.yml` for enterprise domains.
- Rationale: many orgs run GHE; improves adoption.
