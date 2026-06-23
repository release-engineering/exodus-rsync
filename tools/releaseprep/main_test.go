package main

import (
	"strings"
	"testing"
)

func TestLatestSemverTag(t *testing.T) {
	tag, version, err := latestSemverTag([]string{
		"v1.9.7",
		"ignore-me",
		"v1.12.3",
		"v1.10.0",
	})
	if err != nil {
		t.Fatalf("latestSemverTag failed: %v", err)
	}
	if tag != "v1.12.3" {
		t.Fatalf("tag = %s, want v1.12.3", tag)
	}
	if version.String() != "1.12.3" {
		t.Fatalf("version = %s, want 1.12.3", version.String())
	}
}

func TestBumpVersion(t *testing.T) {
	base := semver{major: 1, minor: 12, patch: 3}
	cases := map[string]string{
		"patch": "1.12.4",
		"minor": "1.13.0",
		"major": "2.0.0",
	}

	for bump, want := range cases {
		t.Run(bump, func(t *testing.T) {
			if got := bumpVersion(base, bump).String(); got != want {
				t.Fatalf("bumpVersion(%s) = %s, want %s", bump, got, want)
			}
		})
	}
}

func TestNormalizeModverSuggestion(t *testing.T) {
	cases := map[string]string{
		"None\n":       "none",
		"Patchlevel":   "patch",
		"patch-level.": "patch",
		"Minor":        "minor",
		"Major":        "major",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			got, err := normalizeModverSuggestion(input)
			if err != nil {
				t.Fatalf("normalizeModverSuggestion failed: %v", err)
			}
			if got != want {
				t.Fatalf("normalizeModverSuggestion(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestMaxBumpTreatsChangesAsPatchMinimum(t *testing.T) {
	if got := maxBump("patch", "none"); got != "patch" {
		t.Fatalf("maxBump(patch, none) = %s, want patch", got)
	}
	if got := maxBump("patch", "minor"); got != "minor" {
		t.Fatalf("maxBump(patch, minor) = %s, want minor", got)
	}
}

func TestReleaseEntriesPreferCuratedUnreleased(t *testing.T) {
	changelog := strings.Join([]string{
		"# Changelog",
		"",
		"## Unreleased",
		"",
		"- Carefully written note",
		"",
		"## 1.0.0 - 2024-01-01",
		"",
		"- Previous",
		"",
	}, "\n")

	got := releaseEntries(changelog, []string{"- Generated"})
	if len(got) != 1 || got[0] != "- Carefully written note" {
		t.Fatalf("releaseEntries = %#v, want curated entry", got)
	}
}

func TestReleaseEntriesFallbackToGenerated(t *testing.T) {
	changelog := strings.Join([]string{
		"# Changelog",
		"",
		"## Unreleased",
		"",
		"- n/a",
		"",
		"## 1.0.0 - 2024-01-01",
		"",
		"- Previous",
		"",
	}, "\n")

	got := releaseEntries(changelog, []string{"- Generated"})
	if len(got) != 1 || got[0] != "- Generated" {
		t.Fatalf("releaseEntries = %#v, want generated entry", got)
	}
}

func TestUpdateChangelog(t *testing.T) {
	changelog := strings.Join([]string{
		"# Changelog",
		"",
		"## Unreleased",
		"",
		"- n/a",
		"",
		"## 1.0.0 - 2024-01-01",
		"",
		"- Previous",
		"",
	}, "\n")

	got, err := updateChangelog(changelog, "1.0.1", "2024-02-03", []string{"- New entry"})
	if err != nil {
		t.Fatalf("updateChangelog failed: %v", err)
	}

	wantContains := []string{
		"## Unreleased\n\n- n/a",
		"## 1.0.1 - 2024-02-03\n\n- New entry",
		"## 1.0.0 - 2024-01-01",
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Fatalf("updated changelog does not contain %q:\n%s", want, got)
		}
	}
}

func TestExtractReleaseSection(t *testing.T) {
	changelog := strings.Join([]string{
		"# Changelog",
		"",
		"## Unreleased",
		"",
		"- n/a",
		"",
		"## 1.0.1 - 2024-02-03",
		"",
		"- New entry",
		"",
		"## 1.0.0 - 2024-01-01",
		"",
		"- Previous",
		"",
	}, "\n")

	got, err := extractReleaseSection(changelog, "1.0.1")
	if err != nil {
		t.Fatalf("extractReleaseSection failed: %v", err)
	}
	want := "## 1.0.1 - 2024-02-03\n\n- New entry"
	if got != want {
		t.Fatalf("section = %q, want %q", got, want)
	}
}
