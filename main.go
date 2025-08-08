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

	actionInfos := getActionInfos(ctx, client, actions, requestedRefs)

	if len(actionInfos) == 0 {
		fmt.Println(bold("No action information retrieved."))
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s %s\n", bold("Updating file"), workflowFile)

	updatedContent := updateContent(string(content), actionInfos)

	if string(content) == updatedContent {
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

func normalizeMajorRef(ref string) string {
	// Ensure we try with leading 'v' first; many repos use that form
	if strings.HasPrefix(ref, "v") {
		return ref
	}
	return "v" + ref
}

func getActionInfos(ctx context.Context, client *github.Client, actions []string, requestedRefs map[string]string) []ActionInfo {
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
					actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}
					fmt.Printf("  %s: %s -> %s\n", actionName, tagName, sha)
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
