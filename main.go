package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	semver "github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v57/github"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

var (
	version = "dev"
	commit  = "none"
)

type ActionInfo struct {
	Owner   string
	Repo    string
	Version string
	SHA     string
	Error   error
}

type GitHubHosts struct {
	GitHubCom struct {
		OAuthToken string `yaml:"oauth_token"`
	} `yaml:"github.com"`
}

// UpdatePolicy defines how versions should be selected relative to the requested reference.
// - UpdatePolicyMajor: bump to the latest available version across all majors (default)
// - UpdatePolicySameMajor: stay within the requested major, pick the latest tag for that major
// - UpdatePolicyRequested: pin exactly the requested ref (useful for moving majors like v4)
type UpdatePolicy int

const (
	UpdatePolicyMajor UpdatePolicy = iota
	UpdatePolicySameMajor
	UpdatePolicyRequested
)

type Config struct{}

func parsePolicy(policyStr string) (UpdatePolicy, error) {
	switch strings.ToLower(strings.TrimSpace(policyStr)) {
	case "", "major", "latest-major", "latest":
		return UpdatePolicyMajor, nil
	case "same-major", "stay-major", "minor", "patch":
		// We treat minor/patch as staying within the same major for this tool's scope
		return UpdatePolicySameMajor, nil
	case "requested", "exact", "pin-requested":
		return UpdatePolicyRequested, nil
	default:
		return UpdatePolicyMajor, fmt.Errorf("unknown policy: %s", policyStr)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func bold(text string) string {
	return "\u001b[1m" + text + "\u001b[0m"
}

// prettyRef formats a ref for human-friendly output.
// - If empty, returns (none)
// - If it looks like a full 40-char SHA, abbreviates to 12 chars with an ellipsis
// - Otherwise returns the ref unchanged
func prettyRef(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return "(none)"
	}
	if isFullSHA(ref) {
		return ref[:12] + "…"
	}
	return ref
}

func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < 40; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// printPlannedChanges prints a concise from → to mapping for each action that will change.
func printPlannedChanges(requestedRefs map[string]string, actionInfos []ActionInfo) {
	fmt.Println(bold("Planned updates:\n"))
	hadChange := false
	for _, info := range actionInfos {
		if info.Error != nil {
			continue
		}
		action := fmt.Sprintf("%s/%s", info.Owner, info.Repo)
		oldRef := requestedRefs[action]
		newRef := info.SHA
		if oldRef == newRef {
			// No change for this action; skip in the plan
			continue
		}
		// Example: "  - actions/checkout: v4 → 5e2f1c1…  (v4.2.2)"
		fmt.Printf("  - %s: %s → %s  (%s)\n", action, prettyRef(oldRef), prettyRef(newRef), info.Version)
		hadChange = true
	}
	if !hadChange {
		fmt.Println("  No changes needed. All actions already pinned to the latest commits.")
	}
}

func getGitHubToken() (string, error) {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	token, err := keyring.Get("gh:github.com", "")
	if err == nil {
		return token, nil
	}

	token, err = getGitHubTokenFromHostsFile()
	if err == nil {
		return token, nil
	}

	return "", fmt.Errorf("no GitHub token found. Set GH_TOKEN or GITHUB_TOKEN environment variable, or use 'gh auth login'")
}

func getGitHubTokenFromHostsFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	hostsFile := filepath.Join(homeDir, ".config", "gh", "hosts.yml")
	data, err := os.ReadFile(hostsFile)
	if err != nil {
		return "", err
	}

	var hosts GitHubHosts
	err = yaml.Unmarshal(data, &hosts)
	if err != nil {
		return "", err
	}

	if hosts.GitHubCom.OAuthToken != "" {
		return hosts.GitHubCom.OAuthToken, nil
	}

	return "", fmt.Errorf("no oauth_token found in hosts file")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <workflow-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s .github/workflows/update_cli_docs.yml\n", os.Args[0])
	}
	// Toggle: when a moving major tag (e.g., v4 or 4) is detected, expand the displayed version
	// comment to the full semver tag (e.g., v4.2.2) that the major tag currently points to.
	expandMajorFlag := flag.Bool("expand-major", false, "Expand moving major tags (vN or N) to full semver in the version comment")
	policyFlag := flag.String("policy", "major", "Update policy: major (default), same-major, requested")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	workflowFile := flag.Arg(0)

	if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not found\n", workflowFile)
		os.Exit(1)
	}

	fmt.Printf("\n%s %s\n\n", bold("Scanning workflow"), workflowFile)

	content, err := os.ReadFile(workflowFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	actions := extractActions(string(content))
	requestedRefs := extractActionRefs(string(content))
	if len(actions) == 0 {
		fmt.Printf("%s No GitHub Actions references found in %s\n", bold("No actions:"), workflowFile)
		os.Exit(1)
	}

	fmt.Println(bold("Discovered actions:\n"))
	for _, action := range actions {
		fmt.Printf("  - %s\n", action)
	}
	fmt.Println()

	// Determine effective update policy (default to latest major) from flag only
	effectivePolicy := UpdatePolicyMajor
	if p, err := parsePolicy(*policyFlag); err == nil {
		effectivePolicy = p
	}

	fmt.Println(bold("Resolving latest versions and SHAs (parallel)...\n"))

	token, err := getGitHubToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client := github.NewTokenClient(ctx, token)

	actionInfos := getActionInfos(ctx, client, actions, requestedRefs, *expandMajorFlag, effectivePolicy)

	if len(actionInfos) == 0 {
		fmt.Println(bold("No action information retrieved."))
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s %s\n", bold("Updating file"), workflowFile)

	updatedContent := updateContent(string(content), actionInfos)

	// Always show planned updates for a clear from → to view
	fmt.Println()
	printPlannedChanges(requestedRefs, actionInfos)

	if string(content) == updatedContent {
		fmt.Println()
		fmt.Println(bold("\nUp to date:"), "All actions are already pinned to the latest versions.")
		return
	}

	fmt.Println()
	if !promptConfirmation(bold("Apply changes?") + " [y/N] ") {
		fmt.Println(bold("\nNo changes applied."))
		return
	}

	err = os.WriteFile(workflowFile, []byte(updatedContent), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s %s\n", bold("\nUpdated file"), workflowFile)
	fmt.Println()
	fmt.Println(bold("Pinned actions:\n"))
	for _, info := range actionInfos {
		if info.Error == nil {
			fmt.Printf("  %s/%s@%s # %s\n", info.Owner, info.Repo, info.SHA, info.Version)
		}
	}
}

func extractActions(content string) []string {
	re := regexp.MustCompile(`uses:\s+([^@/]+/[^@\s]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	actionSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			actionSet[match[1]] = true
		}
	}

	var actions []string
	for action := range actionSet {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	return actions
}

func extractActionRefs(content string) map[string]string {
	// Matches: uses: owner/repo@ref (ignores trailing comments)
	re := regexp.MustCompile(`uses:\s+([^@/]+/[^@\s]+)@([^\s#]+)`) // group1: owner/repo, group2: ref
	matches := re.FindAllStringSubmatch(content, -1)

	refs := make(map[string]string)
	for _, m := range matches {
		if len(m) >= 3 {
			action := m[1]
			ref := m[2]
			refs[action] = ref
		}
	}
	return refs
}

func isMovingMajorTag(ref string) bool {
	// v4 or 4
	re := regexp.MustCompile(`^v?\d+$`)
	return re.MatchString(ref)
}

func resolveTagToCommitSHA(ctx context.Context, client *github.Client, owner, repo, tagName string) (string, string, error) {
	// Resolve a tag ref to a commit SHA, dereferencing annotated tags
	ref, resp, err := client.Git.GetRef(ctx, owner, repo, "tags/"+tagName)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", "", fmt.Errorf("tag not found: %s", tagName)
		}
		return "", "", err
	}
	sha := ref.GetObject().GetSHA()
	if ref.GetObject().GetType() == "tag" {
		tagObj, _, tagErr := client.Git.GetTag(ctx, owner, repo, sha)
		if tagErr == nil && tagObj != nil && tagObj.GetObject().GetType() == "commit" && tagObj.GetObject().GetSHA() != "" {
			sha = tagObj.GetObject().GetSHA()
		}
	}
	if sha == "" {
		return "", "", fmt.Errorf("no SHA found for tag %s", tagName)
	}
	return sha, tagName, nil
}

func selectTagBySemverOrNewest(ctx context.Context, client *github.Client, owner, repo string) (string, string, error) {
	// List tags and pick highest semver; if none parsable, pick newest (first page ordering)
	opts := &github.ListOptions{PerPage: 100}
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil || len(tags) == 0 {
		if err == nil {
			err = fmt.Errorf("no tags found")
		}
		return "", "", err
	}

	var bestVersion *semver.Version
	var bestTagName string

	for _, t := range tags {
		name := t.GetName()
		v, parseErr := semver.NewVersion(name)
		if parseErr != nil {
			continue
		}
		if bestVersion == nil || v.GreaterThan(bestVersion) {
			bestVersion = v
			bestTagName = name
		}
	}

	chosen := ""
	if bestVersion != nil {
		chosen = bestTagName
	} else {
		// Fallback to newest tag as returned by API (assumed newest first)
		chosen = tags[0].GetName()
	}

	sha, tagName, err := resolveTagToCommitSHA(ctx, client, owner, repo, chosen)
	if err != nil {
		return "", "", err
	}
	return sha, tagName, nil
}

// parseMajor extracts the major version number from a ref string.
// Accepts forms like "v4", "4", or full semver tags like "v4.2.2".
func parseMajor(ref string) (int, bool) {
	if isMovingMajorTag(ref) {
		r := strings.TrimPrefix(ref, "v")
		v, err := strconv.Atoi(r)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	if v, err := semver.NewVersion(ref); err == nil {
		return int(v.Major()), true
	}
	return 0, false
}

// selectTagBySameMajor finds the highest semver tag within the specified major.
func selectTagBySameMajor(ctx context.Context, client *github.Client, owner, repo string, major int) (string, string, error) {
	page := 1
	var bestVersion *semver.Version
	var bestTagName string

	for {
		opts := &github.ListOptions{PerPage: 100, Page: page}
		tags, resp, err := client.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return "", "", err
		}
		for _, t := range tags {
			name := t.GetName()
			v, parseErr := semver.NewVersion(name)
			if parseErr != nil {
				continue
			}
			if int(v.Major()) != major {
				continue
			}
			if bestVersion == nil || v.GreaterThan(bestVersion) {
				bestVersion = v
				bestTagName = name
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	if bestVersion == nil || bestTagName == "" {
		return "", "", fmt.Errorf("no tags found for major %d", major)
	}
	sha, tagName, err := resolveTagToCommitSHA(ctx, client, owner, repo, bestTagName)
	if err != nil {
		return "", "", err
	}
	return sha, tagName, nil
}

// findFullSemverTagForMajorCommit attempts to find the exact full semver tag (e.g., v4.2.2)
// that currently corresponds to the provided moving major ref (e.g., v4 or 4), by matching
// the resolved commit SHA of the major tag against all semver tags with the same major.
func findFullSemverTagForMajorCommit(ctx context.Context, client *github.Client, owner, repo, majorRef, resolvedCommitSHA string) (string, error) {
	// Parse major number from ref (strip optional leading 'v')
	ref := majorRef
	if strings.HasPrefix(ref, "v") {
		ref = strings.TrimPrefix(ref, "v")
	}
	majorInt, err := strconv.Atoi(ref)
	if err != nil {
		return "", fmt.Errorf("not a major ref: %s", majorRef)
	}

	page := 1
	for {
		opts := &github.ListOptions{PerPage: 100, Page: page}
		tags, resp, listErr := client.Repositories.ListTags(ctx, owner, repo, opts)
		if listErr != nil {
			return "", listErr
		}
		for _, t := range tags {
			name := t.GetName()
			v, parseErr := semver.NewVersion(name)
			if parseErr != nil {
				continue
			}
			if int(v.Major()) != majorInt {
				continue
			}
			sha, _, resolveErr := resolveTagToCommitSHA(ctx, client, owner, repo, name)
			if resolveErr != nil {
				continue
			}
			if sha == resolvedCommitSHA {
				return name, nil
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return "", fmt.Errorf("no matching full tag found for %s", majorRef)
}

func normalizeMajorRef(ref string) string {
	// Ensure we try with leading 'v' first; many repos use that form
	if strings.HasPrefix(ref, "v") {
		return ref
	}
	return "v" + ref
}

func getActionInfos(ctx context.Context, client *github.Client, actions []string, requestedRefs map[string]string, expandMajor bool, policy UpdatePolicy) []ActionInfo {
	var wg sync.WaitGroup
	actionInfos := make([]ActionInfo, len(actions))

	for i, action := range actions {
		wg.Add(1)
		go func(idx int, actionName string) {
			defer wg.Done()

			parts := strings.Split(actionName, "/")
			if len(parts) != 2 {
				actionInfos[idx] = ActionInfo{Error: fmt.Errorf("invalid action format: %s", actionName)}
				return
			}

			owner, repo := parts[0], parts[1]

			requestedRef := requestedRefs[actionName]

			// Policy: Requested
			if policy == UpdatePolicyRequested {
				if requestedRef != "" {
					// If moving major, resolve to the commit that major points to
					if isMovingMajorTag(requestedRef) {
						candidates := []string{requestedRef}
						if !strings.HasPrefix(requestedRef, "v") {
							candidates = append(candidates, normalizeMajorRef(requestedRef))
						}
						var sha, tagName string
						var err error
						for _, c := range candidates {
							sha, tagName, err = resolveTagToCommitSHA(ctx, client, owner, repo, c)
							if err == nil {
								break
							}
						}
						if err == nil {
							resolvedVersion := tagName
							if expandMajor {
								if fullTag, ferr := findFullSemverTagForMajorCommit(ctx, client, owner, repo, requestedRef, sha); ferr == nil && fullTag != "" {
									resolvedVersion = fullTag
								}
							}
							actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: resolvedVersion, SHA: sha}
							fmt.Printf("  %s: %s -> %s\n", actionName, resolvedVersion, sha)
							return
						}
					}
					// Else try resolve as an exact tag
					if sha, tagName, err := resolveTagToCommitSHA(ctx, client, owner, repo, requestedRef); err == nil {
						actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}
						fmt.Printf("  %s: %s -> %s\n", actionName, tagName, sha)
						return
					}
					// If ref already a SHA, keep it
					if isFullSHA(requestedRef) {
						actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: requestedRef, SHA: requestedRef}
						fmt.Printf("  %s: %s -> %s\n", actionName, requestedRef, requestedRef)
						return
					}
				}
				// Fall back to major policy if nothing matched
			}

			// Policy: Same major
			if policy == UpdatePolicySameMajor && requestedRef != "" {
				if major, ok := parseMajor(requestedRef); ok {
					if sha, tagName, err := selectTagBySameMajor(ctx, client, owner, repo, major); err == nil {
						actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}
						fmt.Printf("  %s: %s -> %s\n", actionName, tagName, sha)
						return
					}
				}
				// If we failed to parse major or resolve, continue to major policy below
			}

			// Policy: Major (default) - latest release, else highest semver, else newest
			release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
			if err == nil && release != nil {
				version := release.GetTagName()
				sha, tagName, err := resolveTagToCommitSHA(ctx, client, owner, repo, version)
				if err == nil {
					actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}
					fmt.Printf("  %s: latest %s -> %s\n", actionName, tagName, sha)
					return
				}
				// fall back to tags below if resolving tag failed
			} else if resp != nil && resp.StatusCode != http.StatusNotFound {
				// Unexpected error (not 404). Record and stop for this action.
				fmt.Fprintf(os.Stderr, "%s Could not get latest release for %s: %v\n", bold("WARN:"), actionName, err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Error: err}
				return
			}

			sha, tagName, err := selectTagBySemverOrNewest(ctx, client, owner, repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s Could not resolve tag for %s: %v\n", bold("WARN:"), actionName, err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Error: err}
				return
			}
			actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}
			fmt.Printf("  %s: %s -> %s\n", actionName, tagName, sha)
		}(i, action)
	}

	wg.Wait()

	var validInfos []ActionInfo
	for _, info := range actionInfos {
		if info.Error == nil {
			validInfos = append(validInfos, info)
		}
	}

	return validInfos
}

func updateContent(content string, actionInfos []ActionInfo) string {
	result := content

	for _, info := range actionInfos {
		if info.Error != nil {
			continue
		}

		actionName := fmt.Sprintf("%s/%s", info.Owner, info.Repo)

		pattern := fmt.Sprintf(`(uses:\s+%s)@[^\s]*(\s*#[^\n]*)?`, regexp.QuoteMeta(actionName))
		replacement := fmt.Sprintf("${1}@%s # %s", info.SHA, info.Version)

		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, replacement)
	}

	return result
}

// Diff preview removed

func promptConfirmation(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return response == "y" || response == "yes"
}
