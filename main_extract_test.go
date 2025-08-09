package main

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestExtractOccurrences_Spacing(t *testing.T) {
    content, err := os.ReadFile(filepath.Join("testdata", "extract", "spacing.yaml"))
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }

    occs := extractOccurrences(string(content))
    if len(occs) != 3 {
        t.Fatalf("expected 3 occurrences, got %d", len(occs))
    }

    want := []struct{
        owner string
        repo  string
        ref   string
    }{
        {"actions", "checkout", "v4"},
        {"actions", "setup-go", "v5.0.0"},
        {"actions", "cache", "v4"},
    }

    for i, oc := range occs {
        if oc.Owner != want[i].owner || oc.Repo != want[i].repo || oc.RequestedRef != want[i].ref {
            t.Fatalf("occ[%d] = %s/%s@%s, want %s/%s@%s", i, oc.Owner, oc.Repo, oc.RequestedRef, want[i].owner, want[i].repo, want[i].ref)
        }
        if int(oc.ReplaceStart) < 0 || int(oc.ReplaceEnd) > len(content) || oc.ReplaceEnd != oc.MatchEnd {
            t.Fatalf("occ[%d] invalid replace bounds: [%d,%d) match=[%d,%d)", i, oc.ReplaceStart, oc.ReplaceEnd, oc.MatchStart, oc.MatchEnd)
        }
        if content[oc.ReplaceStart] != '@' {
            t.Fatalf("occ[%d] replace should start at '@', got byte %q", i, content[oc.ReplaceStart])
        }
    }
}

func TestExtractOccurrences_InlineComments(t *testing.T) {
    content, err := os.ReadFile(filepath.Join("testdata", "extract", "inline_comments.yaml"))
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }

    occs := extractOccurrences(string(content))
    if len(occs) != 2 {
        t.Fatalf("expected 2 occurrences, got %d", len(occs))
    }

    // Validate trailing comments are within the match (replace end should be match end)
    for i, oc := range occs {
        if oc.ReplaceEnd != oc.MatchEnd {
            t.Fatalf("occ[%d] ReplaceEnd != MatchEnd: %d != %d", i, oc.ReplaceEnd, oc.MatchEnd)
        }
        seg := string(content[oc.ReplaceStart:oc.ReplaceEnd])
        if !strings.Contains(seg, "#") {
            t.Fatalf("occ[%d] replacement segment should include trailing comment, got %q", i, seg)
        }
    }

    if occs[0].Owner != "actions" || occs[0].Repo != "setup-node" || occs[0].RequestedRef != "v4" {
        t.Fatalf("first occurrence mismatch: %+v", occs[0])
    }
    if occs[1].Owner != "docker" || occs[1].Repo != "setup-buildx-action" || occs[1].RequestedRef != "v3" {
        t.Fatalf("second occurrence mismatch: %+v", occs[1])
    }
}

func TestExtractOccurrences_NoAt(t *testing.T) {
    content, err := os.ReadFile(filepath.Join("testdata", "extract", "no_at.yaml"))
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }

    occs := extractOccurrences(string(content))
    if len(occs) != 0 {
        t.Fatalf("expected 0 occurrences, got %d: %+v", len(occs), occs)
    }
}

func TestExtractOccurrences_Multiple(t *testing.T) {
    content, err := os.ReadFile(filepath.Join("testdata", "extract", "multiple.yaml"))
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }

    occs := extractOccurrences(string(content))
    if len(occs) != 3 {
        t.Fatalf("expected 3 occurrences, got %d", len(occs))
    }

    got := make([]string, 0, len(occs))
    for _, oc := range occs {
        got = append(got, oc.Owner+"/"+oc.Repo+"@"+oc.RequestedRef)
    }
    want := []string{
        "actions/checkout@v4",
        "actions/cache@v4",
        "github/super-linter@v6.0.0",
    }
    if strings.Join(got, ";") != strings.Join(want, ";") {
        t.Fatalf("got %v, want %v", got, want)
    }
}