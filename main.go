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
	fmt.Println(bold("Planned updates:"))
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

	fmt.Printf("%s %s\n", bold("Scanning workflow"), workflowFile)

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

	fmt.Println(bold("Discovered actions:"))
	for _, action := range actions {
		fmt.Printf("  - %s\n", action)
	}
	fmt.Println()

	fmt.Println(bold("Resolving latest versions and SHAs (parallel)..."))

	token, err := getGitHubToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client := github.NewTokenClient(ctx, token)

	actionInfos := getActionInfos(ctx, client, actions, requestedRefs, *expandMajorFlag)

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
		fmt.Println(bold("Up to date:"), "All actions are already pinned to the latest versions.")
		return
	}

	fmt.Println()
	if !promptConfirmation(bold("Apply changes?") + " [y/N] ") {
		fmt.Println(bold("No changes applied."))
		return
	}

	err = os.WriteFile(workflowFile, []byte(updatedContent), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s %s\n", bold("Updated file"), workflowFile)
	fmt.Println()
	fmt.Println(bold("Pinned actions:"))
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

func getActionInfos(ctx context.Context, client *github.Client, actions []string, requestedRefs map[string]string, expandMajor bool) []ActionInfo {
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

			// If the user requested a moving major tag like v4, resolve that directly
			if ref, ok := requestedRefs[actionName]; ok && isMovingMajorTag(ref) {
				// Try exact ref, then try normalized with 'v'
				candidates := []string{ref}
				if !strings.HasPrefix(ref, "v") {
					candidates = append(candidates, normalizeMajorRef(ref))
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
						if fullTag, ferr := findFullSemverTagForMajorCommit(ctx, client, owner, repo, ref, sha); ferr == nil && fullTag != "" {
							resolvedVersion = fullTag
						}
					}
					actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: resolvedVersion, SHA: sha}
					fmt.Printf("  %s: %s -> %s\n", actionName, resolvedVersion, sha)
					return
				}
				// If failed to resolve requested moving tag, continue to normal resolution below
			}

			// Try latest release first
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

			// Fallback to tags: highest semver, else newest
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
