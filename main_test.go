package main

import (
	"strings"
	"testing"
)

func TestUpdateContent(t *testing.T) {
	input := `name: Test Workflow
on:
  push:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5.0.0 # existing comment
      - name: Run tests
        uses: actions/cache@v3
      - uses: docker/setup-buildx-action@v2 # some comment
        with:
          driver: docker-container
      - uses: actions/upload-artifact@v4.0.0`

	actionInfos := []ActionInfo{
		{
			Owner:   "actions",
			Repo:    "checkout",
			Version: "v4.1.0",
			SHA:     "8ade135a41bc03ea155e62e844d188df1ea18608",
		},
		{
			Owner:   "actions",
			Repo:    "setup-go",
			Version: "v5.0.1",
			SHA:     "93397bea11091df50f3d7e59dc26a7711a8bcfbe",
		},
		{
			Owner:   "actions",
			Repo:    "cache",
			Version: "v4.0.2",
			SHA:     "ab5e6d0c87105b4c9c2047343972218f562e4319",
		},
	}

	result := updateContent(input, actionInfos)

	expected := `name: Test Workflow
on:
  push:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8ade135a41bc03ea155e62e844d188df1ea18608 # v4.1.0
      - uses: actions/setup-go@93397bea11091df50f3d7e59dc26a7711a8bcfbe # v5.0.1
      - name: Run tests
        uses: actions/cache@ab5e6d0c87105b4c9c2047343972218f562e4319 # v4.0.2
      - uses: docker/setup-buildx-action@v2 # some comment
        with:
          driver: docker-container
      - uses: actions/upload-artifact@v4.0.0`

	if result != expected {
		t.Errorf("updateContent() failed")
		t.Logf("Expected:\n%s", expected)
		t.Logf("Got:\n%s", result)

		expectedLines := strings.Split(expected, "\n")
		resultLines := strings.Split(result, "\n")

		maxLines := len(expectedLines)
		if len(resultLines) > maxLines {
			maxLines = len(resultLines)
		}

		for i := 0; i < maxLines; i++ {
			var expectedLine, resultLine string
			if i < len(expectedLines) {
				expectedLine = expectedLines[i]
			}
			if i < len(resultLines) {
				resultLine = resultLines[i]
			}

			if expectedLine != resultLine {
				t.Logf("Diff at line %d:", i+1)
				t.Logf("  Expected: %q", expectedLine)
				t.Logf("  Got:      %q", resultLine)
			}
		}
	}
}
