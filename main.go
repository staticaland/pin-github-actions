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

// ActionOccurrence represents a single occurrence of a `uses: owner/repo@ref` entry
// in the workflow content. It tracks the exact byte offsets for safe in-place replacement
// and also provides human-friendly line/column for output.
type ActionOccurrence struct {
	Owner        string
	Repo         string
	Action       string // owner/repo
	RequestedRef string

	// Byte offsets in the original file content
	MatchStart   int // start of the entire `uses: ...` match
	MatchEnd     int // end of the entire match
	ReplaceStart int // start of the replacement span (the '@' character before the ref)
	ReplaceEnd   int // end of the replacement span (end of match)

	// 1-based positions for display
	Line   int
	Column int
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

// printPlannedChanges prints a concise from → to mapping for each occurrence that will change.
func printPlannedChanges(occurrences []ActionOccurrence, actionInfos []ActionInfo) {
	fmt.Println(bold("Planned updates:\n"))
	hadChange := false

	for i, occ := range occurrences {
		if i >= len(actionInfos) {
			continue
		}
		info := actionInfos[i]
		if info.Error != nil {
			continue
		}
		oldRef := occ.RequestedRef
		newRef := info.SHA
		if oldRef == newRef || strings.TrimSpace(newRef) == "" {
			continue
		}
		action := fmt.Sprintf("%s/%s", occ.Owner, occ.Repo)
		// Example: "  - actions/checkout (L12:C9): v4 → 5e2f1c1…  (v4.2.2)"
		fmt.Printf("  - %s (L%d:C%d): %s → %s  (%s)\n", action, occ.Line, occ.Column, prettyRef(oldRef), prettyRef(newRef), info.Version)
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
		fmt.Fprintf(os.Stderr, "Usage: %s [--expand-major] [--policy <policy>] [--yes|--write] [--dry-run] <workflow-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s --policy same-major --yes .github/workflows/update_cli_docs.yml\n", os.Args[0])
	}
	// Toggle: when a moving major tag (e.g., v4 or 4) is detected, expand the displayed version
	// comment to the full semver tag (e.g., v4.2.2) that the major tag currently points to.
	expandMajorFlag := flag.Bool("expand-major", false, "Expand moving major tags (vN or N) to full semver in the version comment")
	policyFlag := flag.String("policy", "major", "Update policy: major (default), same-major, requested")
	yesFlag := flag.Bool("yes", false, "Apply changes without confirmation prompt")
	writeFlag := flag.Bool("write", false, "Apply changes without confirmation prompt (alias of --yes)")
	dryRunFlag := flag.Bool("dry-run", false, "Preview planned updates and exit without writing")
	flag.Parse()

	nonInteractiveApply := *yesFlag || *writeFlag

	if *dryRunFlag && nonInteractiveApply {
		fmt.Fprintf(os.Stderr, "Error: --dry-run cannot be used with --yes/--write\n")
		os.Exit(1)
	}

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
	occurrences := extractOccurrences(string(content))
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

	actionInfos := getActionInfosForOccurrences(ctx, client, occurrences, *expandMajorFlag, effectivePolicy)

	if len(actionInfos) == 0 {
		fmt.Println(bold("No action information retrieved."))
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s %s\n", bold("Updating file"), workflowFile)

	updatedContent := updateContent(string(content), occurrences, actionInfos)

	// Always show planned updates for a clear from → to view
	fmt.Println()
	printPlannedChanges(occurrences, actionInfos)

	// Dry-run: exit after preview without prompting or writing. Exit code 2 if changes would be made.
	if *dryRunFlag {
		if string(content) == updatedContent {
			return
		}
		os.Exit(2)
	}

	if string(content) == updatedContent {
		fmt.Println()
		fmt.Println(bold("\nUp to date:"), "All actions are already pinned to the latest versions.")
		return
	}

	fmt.Println()
	// If --yes is set, skip the prompt and apply immediately
	if !nonInteractiveApply {
		if !promptConfirmation(bold("Apply changes?") + " [y/N] ") {
			fmt.Println(bold("\nNo changes applied."))
			return
		}
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
	// Preserve order of first appearance while de-duplicating
	re := regexp.MustCompile(`uses:\s+([^@/]+/[^@\s]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	seen := make(map[string]bool)
	actions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) <= 1 {
			continue
		}
		action := match[1]
		if seen[action] {
			continue
		}
		seen[action] = true
		actions = append(actions, action)
	}
	return actions
}

// extractOccurrences finds each `uses: owner/repo@ref` occurrence along with positions.
func extractOccurrences(content string) []ActionOccurrence {
	re := regexp.MustCompile(`uses:\s+([^@/]+/[^@\s]+)@([^\s#]+)(\s*#[^\n]*)?`)
	indices := re.FindAllStringSubmatchIndex(content, -1)
	occurrences := make([]ActionOccurrence, 0, len(indices))

	for _, idxs := range indices {
		if len(idxs) < 6 {
			continue
		}
		matchStart, matchEnd := idxs[0], idxs[1]
		ownerRepoStart, ownerRepoEnd := idxs[2], idxs[3]
		refStart, refEnd := idxs[4], idxs[5]
		action := content[ownerRepoStart:ownerRepoEnd]
		parts := strings.SplitN(action, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, repo := parts[0], parts[1]
		requestedRef := content[refStart:refEnd]
		// '@' should be right after ownerRepoEnd
		replaceStart := ownerRepoEnd
		replaceEnd := matchEnd

		line, col := computeLineCol(content, ownerRepoStart)

		occurrences = append(occurrences, ActionOccurrence{
			Owner:        owner,
			Repo:         repo,
			Action:       action,
			RequestedRef: requestedRef,
			MatchStart:   matchStart,
			MatchEnd:     matchEnd,
			ReplaceStart: replaceStart,
			ReplaceEnd:   replaceEnd,
			Line:         line,
			Column:       col,
		})
	}
	return occurrences
}

// computeLineCol returns 1-based line and column for the given byte offset.
func computeLineCol(content string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	line := 1
	col := 1
	lastNL := -1
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lastNL = i
		}
	}
	col = offset - lastNL
	return line, col
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
	foundMatchInPriorPages := false

	for {
		opts := &github.ListOptions{PerPage: 100, Page: page}
		tags, resp, err := client.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return "", "", err
		}

		// Track whether this page contained any tags matching the requested major.
		// If none are found on a page and tags are ordered newest-first by the API,
		// we can stop early once we have already seen at least one match in prior pages.
		foundMatchOnCurrentPage := false

		for _, t := range tags {
			name := t.GetName()
			v, parseErr := semver.NewVersion(name)
			if parseErr != nil {
				continue
			}
			if int(v.Major()) != major {
				continue
			}
			foundMatchOnCurrentPage = true
			if bestVersion == nil || v.GreaterThan(bestVersion) {
				bestVersion = v
				bestTagName = name
			}
		}

		// Early stop heuristic: only stop when the current page has no matches AND we
		// previously saw at least one matching tag on an earlier page.
		if !foundMatchOnCurrentPage && foundMatchInPriorPages {
			break
		}
		if foundMatchOnCurrentPage {
			foundMatchInPriorPages = true
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

	// First pass: collect candidate tags by major and compare lightweight tag SHAs directly
	candidates := make([]string, 0, 100)
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
			candidates = append(candidates, name)
			// Compare the SHA provided by ListTags (lightweight tags) before dereferencing annotated ones
			if t.GetCommit() != nil {
				if sha := t.GetCommit().GetSHA(); sha != "" && sha == resolvedCommitSHA {
					return name, nil
				}
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	// Second pass: dereference annotated tags only
	for _, name := range candidates {
		sha, _, resolveErr := resolveTagToCommitSHA(ctx, client, owner, repo, name)
		if resolveErr != nil {
			continue
		}
		if sha == resolvedCommitSHA {
			return name, nil
		}
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

// resolveActionForPolicy resolves a single occurrence according to the chosen policy.
func resolveActionForPolicy(ctx context.Context, client *github.Client, owner, repo, requestedRef string, expandMajor bool, policy UpdatePolicy) (ActionInfo, error) {

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
					return ActionInfo{Owner: owner, Repo: repo, Version: resolvedVersion, SHA: sha}, nil
				}
			}
			// Else try resolve as an exact tag
			if sha, tagName, err := resolveTagToCommitSHA(ctx, client, owner, repo, requestedRef); err == nil {
				return ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}, nil
			}
			// If ref already a SHA, keep it
			if isFullSHA(requestedRef) {
				return ActionInfo{Owner: owner, Repo: repo, Version: requestedRef, SHA: requestedRef}, nil
			}
		}
		// Fall back to major policy if nothing matched
	}

	// Policy: Same major
	if policy == UpdatePolicySameMajor && requestedRef != "" {
		if major, ok := parseMajor(requestedRef); ok {
			if sha, tagName, err := selectTagBySameMajor(ctx, client, owner, repo, major); err == nil {
				return ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}, nil
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
			return ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}, nil
		}
		// fall back to tags below if resolving tag failed
	} else if resp != nil && resp.StatusCode != http.StatusNotFound {
		// Unexpected error (not 404). Record and stop for this action.
		return ActionInfo{Owner: owner, Repo: repo, Error: err}, err
	}

	sha, tagName, err := selectTagBySemverOrNewest(ctx, client, owner, repo)
	if err != nil {
		return ActionInfo{Owner: owner, Repo: repo, Error: err}, err
	}
	return ActionInfo{Owner: owner, Repo: repo, Version: tagName, SHA: sha}, nil
}

// getActionInfosForOccurrences resolves each occurrence independently.
func getActionInfosForOccurrences(ctx context.Context, client *github.Client, occurrences []ActionOccurrence, expandMajor bool, policy UpdatePolicy) []ActionInfo {
	var wg sync.WaitGroup
	infos := make([]ActionInfo, len(occurrences))
	// Collect per-occurrence messages for deterministic output after wg.Wait()
	messages := make([]string, len(occurrences))

	// Simple cache to avoid duplicate network calls when resolution is identical.
	type cacheEntry struct {
		info ActionInfo
		ok   bool
	}
	cache := make(map[string]cacheEntry)
	var mu sync.Mutex
	cacheKey := func(owner, repo string, policy UpdatePolicy, requestedRef string) string {
		return fmt.Sprintf("%s/%s|%d|%s", owner, repo, policy, requestedRef)
	}

	for i, occ := range occurrences {
		wg.Add(1)
		go func(idx int, o ActionOccurrence) {
			defer wg.Done()
			key := cacheKey(o.Owner, o.Repo, policy, o.RequestedRef)
			mu.Lock()
			if ce, exists := cache[key]; exists && ce.ok {
				mu.Unlock()
				infos[idx] = ce.info
				return
			}
			mu.Unlock()

			info, err := resolveActionForPolicy(ctx, client, o.Owner, o.Repo, o.RequestedRef, expandMajor, policy)
			if err == nil {
				messages[idx] = fmt.Sprintf("  %s: %s -> %s", o.Action, info.Version, info.SHA)
			}
			infos[idx] = info

			mu.Lock()
			cache[key] = cacheEntry{info: info, ok: info.Error == nil}
			mu.Unlock()
		}(i, occ)
	}

	wg.Wait()
	// Print buffered messages in the original order
	for _, m := range messages {
		if strings.TrimSpace(m) == "" {
			continue
		}
		fmt.Println(m)
	}
	return infos
}

func updateContent(content string, occurrences []ActionOccurrence, actionInfos []ActionInfo) string {
	// Build replacements for occurrences with successful resolutions
	type repl struct {
		start int
		end   int
		text  string
	}
	repls := make([]repl, 0)
	for i, occ := range occurrences {
		if i >= len(actionInfos) {
			continue
		}
		info := actionInfos[i]
		if info.Error != nil || strings.TrimSpace(info.SHA) == "" || occ.ReplaceStart < 0 || occ.ReplaceEnd <= occ.ReplaceStart {
			continue
		}
		// If the target SHA equals the current ref, skip
		if occ.RequestedRef == info.SHA {
			continue
		}
		repls = append(repls, repl{
			start: occ.ReplaceStart,
			end:   occ.ReplaceEnd,
			text:  fmt.Sprintf("@%s # %s", info.SHA, info.Version),
		})
	}
	if len(repls) == 0 {
		return content
	}
	// Sort by start ascending to rebuild content
	sort.Slice(repls, func(i, j int) bool { return repls[i].start < repls[j].start })
	var b strings.Builder
	prev := 0
	for _, r := range repls {
		if r.start < prev {
			// overlapping/unsorted; skip defensively
			continue
		}
		b.WriteString(content[prev:r.start])
		b.WriteString(r.text)
		prev = r.end
	}
	b.WriteString(content[prev:])
	return b.String()
}

// Diff preview removed

func promptConfirmation(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return response == "y" || response == "yes"
}
