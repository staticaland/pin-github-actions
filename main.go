package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v57/github"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
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

	fmt.Printf("Finding GitHub Actions in %s...\n", workflowFile)

	content, err := os.ReadFile(workflowFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	actions := extractActions(string(content))
	if len(actions) == 0 {
		fmt.Printf("No GitHub Actions found in %s\n", workflowFile)
		os.Exit(1)
	}

	fmt.Println("Found actions:")
	for _, action := range actions {
		fmt.Printf("  - %s\n", action)
	}
	fmt.Println()

	fmt.Println("Getting latest versions and SHAs (parallel processing)...")

	token, err := getGitHubToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client := github.NewTokenClient(ctx, token)

	actionInfos := getActionInfos(ctx, client, actions)

	if len(actionInfos) == 0 {
		fmt.Println("Failed to get action information")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Updating %s...\n", workflowFile)

	updatedContent := updateContent(string(content), actionInfos)

	if string(content) == updatedContent {
		fmt.Println("No updates needed - all actions are already at latest versions!")
		return
	}

	fmt.Println()
	fmt.Println("Changes to be made:")
	showDiff(workflowFile, string(content), updatedContent)

	fmt.Println()
	if !promptConfirmation("Apply these changes? [y/N] ") {
		fmt.Println("Changes not applied")
		return
	}

	err = os.WriteFile(workflowFile, []byte(updatedContent), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated %s successfully!\n", workflowFile)
	fmt.Println()
	fmt.Println("Summary of pinned actions:")
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

func getActionInfos(ctx context.Context, client *github.Client, actions []string) []ActionInfo {
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
			
			release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not get latest version for %s: %v\n", actionName, err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Error: err}
				return
			}
			
			version := release.GetTagName()
			if version == "" {
				err := fmt.Errorf("no tag name found for latest release")
				fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Error: err}
				return
			}
			
			ref, _, err := client.Git.GetRef(ctx, owner, repo, "tags/"+version)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not get SHA for %s@%s: %v\n", actionName, version, err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: version, Error: err}
				return
			}
			
			sha := ref.GetObject().GetSHA()
			if sha == "" {
				err := fmt.Errorf("no SHA found for tag %s", version)
				fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
				actionInfos[idx] = ActionInfo{Owner: owner, Repo: repo, Version: version, Error: err}
				return
			}
			
			actionInfos[idx] = ActionInfo{
				Owner:   owner,
				Repo:    repo,
				Version: version,
				SHA:     sha,
			}
			
			fmt.Printf("%s -> %s (%s)\n", actionName, version, sha)
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

func showDiff(filename, oldContent, newContent string) {
	oldFile, err := os.CreateTemp("", "old-*.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		return
	}
	defer os.Remove(oldFile.Name())
	defer oldFile.Close()
	
	newFile, err := os.CreateTemp("", "new-*.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		return
	}
	defer os.Remove(newFile.Name())
	defer newFile.Close()
	
	oldFile.WriteString(oldContent)
	newFile.WriteString(newContent)
	oldFile.Close()
	newFile.Close()
	
	cmd := exec.Command("diff", "-u", oldFile.Name(), newFile.Name())
	output, _ := cmd.Output()
	
	diffLines := strings.Split(string(output), "\n")
	if len(diffLines) > 2 {
		diffLines[0] = "--- " + filename + ".old"
		diffLines[1] = "+++ " + filename
		fmt.Println(strings.Join(diffLines, "\n"))
	}
}

func promptConfirmation(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return response == "y" || response == "yes"
}