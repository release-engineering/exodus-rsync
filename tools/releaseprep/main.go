// Package main implements release preparation helpers for GitHub Actions.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const pinnedModver = "github.com/bobg/modver/v2/cmd/modver@v2.14.1"

var semverTagPattern = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

type releaseMetadata struct {
	Version          string `json:"version"`
	Tag              string `json:"tag"`
	PreviousTag      string `json:"previous_tag"`
	TargetCommit     string `json:"target_commit"`
	Bump             string `json:"bump"`
	ModverSuggestion string `json:"modver_suggestion"`
	PreparedAt       string `json:"prepared_at"`
}

type semver struct {
	major int
	minor int
	patch int
}

func main() {
	if len(os.Args) < 2 {
		die("missing subcommand: prepare or notes")
	}

	var err error
	switch os.Args[1] {
	case "prepare":
		err = runPrepare(os.Args[2:])
	case "notes":
		err = runNotes(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		die(err.Error())
	}
}

func die(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func runPrepare(args []string) error {
	fs := flag.NewFlagSet("prepare", flag.ExitOnError)
	bumpInput := fs.String("bump", "auto", "release bump: auto, patch, minor, or major")
	dryRun := fs.Bool("dry-run", false, "compute release details without writing files")
	baseRef := fs.String("base-ref", "origin/main", "git ref to release from")
	changelogPath := fs.String("changelog", "CHANGELOG.md", "path to changelog")
	metadataPath := fs.String("metadata", ".release/next-release.json", "path to release metadata")
	preparedAt := fs.String("prepared-at", time.Now().UTC().Format(time.RFC3339), "release preparation timestamp")
	modverCmd := fs.String("modver", pinnedModver, "pinned modver go run target")
	if err := fs.Parse(args); err != nil {
		return err
	}

	bump := strings.ToLower(strings.TrimSpace(*bumpInput))
	if !validBumpInput(bump) {
		return fmt.Errorf("invalid bump %q", *bumpInput)
	}

	tagsOut, err := gitOutput("tag", "--list", "v*")
	if err != nil {
		return err
	}
	latestTag, latestVersion, err := latestSemverTag(strings.Fields(tagsOut))
	if err != nil {
		return err
	}

	targetCommit, err := gitOutput("rev-parse", *baseRef)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", *baseRef, err)
	}
	targetCommit = strings.TrimSpace(targetCommit)

	commitCountRaw, err := gitOutput("rev-list", "--count", latestTag+".."+*baseRef)
	if err != nil {
		return fmt.Errorf("count commits since %s: %w", latestTag, err)
	}
	commitCount, err := strconv.Atoi(strings.TrimSpace(commitCountRaw))
	if err != nil {
		return fmt.Errorf("parse commit count %q: %w", strings.TrimSpace(commitCountRaw), err)
	}

	if commitCount == 0 {
		fmt.Printf("No commits found between %s and %s; no release preparation needed.\n", latestTag, *baseRef)
		return writeActionOutputs(map[string]string{
			"has_changes":   "false",
			"dry_run":       strconv.FormatBool(*dryRun),
			"previous_tag":  latestTag,
			"target_commit": targetCommit,
		})
	}

	modverSuggestion := "not_run"
	if bump == "auto" {
		modverSuggestion, err = suggestWithModver(*modverCmd, latestTag, *baseRef)
		if err != nil {
			return err
		}
		bump = maxBump("patch", modverSuggestion)
	}

	nextVersion := bumpVersion(latestVersion, bump)
	tag := "v" + nextVersion.String()

	changelogContentBytes, err := os.ReadFile(*changelogPath)
	if err != nil {
		return err
	}
	changelogContent := string(changelogContentBytes)

	generatedEntries, err := generatedChangelogEntries(latestTag, *baseRef)
	if err != nil {
		return err
	}
	entries := releaseEntries(changelogContent, generatedEntries)
	nextChangelog, err := updateChangelog(changelogContent, nextVersion.String(), releaseDate(*preparedAt), entries)
	if err != nil {
		return err
	}

	meta := releaseMetadata{
		Version:          nextVersion.String(),
		Tag:              tag,
		PreviousTag:      latestTag,
		TargetCommit:     targetCommit,
		Bump:             bump,
		ModverSuggestion: modverSuggestion,
		PreparedAt:       *preparedAt,
	}

	if !*dryRun {
		if err := os.MkdirAll(filepath.Dir(*metadataPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*changelogPath, []byte(nextChangelog), 0o644); err != nil {
			return err
		}
		metadataJSON, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}
		metadataJSON = append(metadataJSON, '\n')
		if err := os.WriteFile(*metadataPath, metadataJSON, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(*metadataPath), "release-pr.md"), []byte(preparationPRBody(meta, generatedEntries)), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("Prepared release %s from %s to %s using %s bump.\n", tag, latestTag, targetCommit, bump)
	if *dryRun {
		fmt.Println("Dry run enabled; CHANGELOG.md and release metadata were not modified.")
	}

	return writeActionOutputs(map[string]string{
		"has_changes":       "true",
		"dry_run":           strconv.FormatBool(*dryRun),
		"version":           meta.Version,
		"tag":               meta.Tag,
		"previous_tag":      meta.PreviousTag,
		"target_commit":     meta.TargetCommit,
		"bump":              meta.Bump,
		"modver_suggestion": meta.ModverSuggestion,
	})
}

func runNotes(args []string) error {
	fs := flag.NewFlagSet("notes", flag.ExitOnError)
	changelogPath := fs.String("changelog", "CHANGELOG.md", "path to changelog")
	metadataPath := fs.String("metadata", ".release/next-release.json", "path to release metadata")
	outputPath := fs.String("output", "release-notes.md", "path to generated release notes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	metadataBytes, err := os.ReadFile(*metadataPath)
	if err != nil {
		return err
	}
	var meta releaseMetadata
	if err := json.Unmarshal(metadataBytes, &meta); err != nil {
		return err
	}
	if err := validateMetadata(meta); err != nil {
		return err
	}

	changelogBytes, err := os.ReadFile(*changelogPath)
	if err != nil {
		return err
	}
	section, err := extractReleaseSection(string(changelogBytes), meta.Version)
	if err != nil {
		return err
	}

	var notes strings.Builder
	notes.WriteString(section)
	notes.WriteString("\n\n## Assets\n\n")
	notes.WriteString(fmt.Sprintf("- `exodus-rsync`: Linux amd64 binary built with `BUILDVERSION=%s`.\n", meta.Tag))
	notes.WriteString("- `exodus-rsync.sha256`: SHA-256 checksum for downstream verification.\n")

	if err := os.WriteFile(*outputPath, []byte(notes.String()), 0o644); err != nil {
		return err
	}

	return writeActionOutputs(map[string]string{
		"version":       meta.Version,
		"tag":           meta.Tag,
		"previous_tag":  meta.PreviousTag,
		"target_commit": meta.TargetCommit,
		"bump":          meta.Bump,
	})
}

func validBumpInput(bump string) bool {
	switch bump {
	case "auto", "patch", "minor", "major":
		return true
	default:
		return false
	}
}

func validateMetadata(meta releaseMetadata) error {
	missing := []string{}
	if meta.Version == "" {
		missing = append(missing, "version")
	}
	if meta.Tag == "" {
		missing = append(missing, "tag")
	}
	if meta.PreviousTag == "" {
		missing = append(missing, "previous_tag")
	}
	if meta.TargetCommit == "" {
		missing = append(missing, "target_commit")
	}
	if meta.Bump == "" {
		missing = append(missing, "bump")
	}
	if meta.PreparedAt == "" {
		missing = append(missing, "prepared_at")
	}
	if len(missing) > 0 {
		return fmt.Errorf("release metadata missing required fields: %s", strings.Join(missing, ", "))
	}
	if _, err := parseSemverTag(meta.Tag); err != nil {
		return err
	}
	return nil
}

func latestSemverTag(tags []string) (string, semver, error) {
	versions := make([]struct {
		tag string
		ver semver
	}, 0, len(tags))
	for _, tag := range tags {
		ver, err := parseSemverTag(tag)
		if err == nil {
			versions = append(versions, struct {
				tag string
				ver semver
			}{tag: tag, ver: ver})
		}
	}
	if len(versions) == 0 {
		return "", semver{}, errors.New("no vX.Y.Z release tag found")
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i].ver, versions[j].ver) < 0
	})
	latest := versions[len(versions)-1]
	return latest.tag, latest.ver, nil
}

func parseSemverTag(tag string) (semver, error) {
	matches := semverTagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return semver{}, fmt.Errorf("invalid semver tag %q", tag)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return semver{major: major, minor: minor, patch: patch}, nil
}

func compareSemver(left, right semver) int {
	if left.major != right.major {
		return left.major - right.major
	}
	if left.minor != right.minor {
		return left.minor - right.minor
	}
	return left.patch - right.patch
}

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func bumpVersion(version semver, bump string) semver {
	switch bump {
	case "major":
		return semver{major: version.major + 1}
	case "minor":
		return semver{major: version.major, minor: version.minor + 1}
	default:
		return semver{major: version.major, minor: version.minor, patch: version.patch + 1}
	}
}

func suggestWithModver(modverCmd, previousTag, baseRef string) (string, error) {
	cmd := exec.Command("go", "run", modverCmd, "-git", ".git", previousTag, baseRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("modver failed: %w\n%s", err, string(out))
	}
	suggestion, err := normalizeModverSuggestion(string(out))
	if err != nil {
		return "", err
	}
	return suggestion, nil
}

func normalizeModverSuggestion(output string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(output))
	normalized = strings.TrimSuffix(normalized, ".")
	switch normalized {
	case "", "none", "no change":
		return "none", nil
	case "patch", "patchlevel", "patch-level":
		return "patch", nil
	case "minor":
		return "minor", nil
	case "major":
		return "major", nil
	default:
		return "", fmt.Errorf("unrecognized modver suggestion %q", strings.TrimSpace(output))
	}
}

func maxBump(left, right string) string {
	order := map[string]int{
		"none":  0,
		"patch": 1,
		"minor": 2,
		"major": 3,
	}
	if order[right] > order[left] {
		return right
	}
	return left
}

func generatedChangelogEntries(previousTag, baseRef string) ([]string, error) {
	out, err := gitOutput("log", "--first-parent", "--format=- %s (%h)", previousTag+".."+baseRef)
	if err != nil {
		return nil, fmt.Errorf("generate changelog entries: %w", err)
	}
	entries := nonEmptyLines(out)
	if len(entries) == 0 {
		return nil, errors.New("commit batch is non-empty but no first-parent changelog entries were generated")
	}
	return entries, nil
}

func releaseEntries(changelogContent string, generated []string) []string {
	unreleased := extractUnreleasedEntries(changelogContent)
	if hasCuratedEntries(unreleased) {
		return unreleased
	}
	return generated
}

func extractUnreleasedEntries(changelogContent string) []string {
	lines := splitLines(changelogContent)
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Unreleased" {
			start = i + 1
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	return trimBlankLines(lines[start:end])
}

func hasCuratedEntries(entries []string) bool {
	trimmed := trimBlankLines(entries)
	if len(trimmed) == 0 {
		return false
	}
	if len(trimmed) == 1 && strings.EqualFold(strings.TrimSpace(trimmed[0]), "- n/a") {
		return false
	}
	return true
}

func updateChangelog(content, version, date string, entries []string) (string, error) {
	if _, err := extractReleaseSection(content, version); err == nil {
		return "", fmt.Errorf("CHANGELOG.md already contains a section for %s", version)
	}

	lines := splitLines(content)
	unreleasedLine := -1
	nextSectionLine := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Unreleased" {
			unreleasedLine = i
			continue
		}
		if unreleasedLine >= 0 && strings.HasPrefix(line, "## ") {
			nextSectionLine = i
			break
		}
	}
	if unreleasedLine < 0 {
		return "", errors.New("CHANGELOG.md is missing an ## Unreleased section")
	}
	if nextSectionLine < 0 {
		nextSectionLine = len(lines)
	}

	out := make([]string, 0, len(lines)+len(entries)+6)
	out = append(out, lines[:unreleasedLine+1]...)
	out = append(out, "", "- n/a", "", fmt.Sprintf("## %s - %s", version, date), "")
	out = append(out, trimBlankLines(entries)...)
	out = append(out, "")
	out = append(out, lines[nextSectionLine:]...)
	return strings.Join(out, "\n") + "\n", nil
}

func extractReleaseSection(content, version string) (string, error) {
	lines := splitLines(content)
	headerPrefix := "## " + version + " - "
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, headerPrefix) {
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("CHANGELOG.md is missing a section for %s", version)
	}
	return strings.Join(trimBlankLines(lines[start:end]), "\n"), nil
}

func releaseDate(preparedAt string) string {
	timestamp, err := time.Parse(time.RFC3339, preparedAt)
	if err != nil {
		return time.Now().UTC().Format(time.DateOnly)
	}
	return timestamp.UTC().Format(time.DateOnly)
}

func preparationPRBody(meta releaseMetadata, generatedEntries []string) string {
	var body strings.Builder
	body.WriteString("## Release Preparation\n\n")
	body.WriteString(fmt.Sprintf("- Previous tag: `%s`\n", meta.PreviousTag))
	body.WriteString(fmt.Sprintf("- Proposed tag: `%s`\n", meta.Tag))
	body.WriteString(fmt.Sprintf("- Bump: `%s`\n", meta.Bump))
	body.WriteString(fmt.Sprintf("- modver suggestion: `%s`\n", meta.ModverSuggestion))
	body.WriteString(fmt.Sprintf("- Target commit: `%s`\n\n", meta.TargetCommit))
	body.WriteString("## Generated Commit Summary\n\n")
	for _, entry := range generatedEntries {
		body.WriteString(entry)
		body.WriteString("\n")
	}
	body.WriteString("\n## Publish Step\n\n")
	body.WriteString("Merging this PR triggers the release publishing workflow, which creates the tag, GitHub Release, binary, checksum, and configured downstream update flows.\n")
	return body.String()
}

func splitLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	return strings.Split(content, "\n")
}

func nonEmptyLines(content string) []string {
	raw := splitLines(content)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func trimBlankLines(lines []string) []string {
	start := 0
	end := len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return append([]string{}, lines[start:end]...)
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func writeActionOutputs(values map[string]string) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	for _, key := range keys {
		out.WriteString(key)
		out.WriteString("=")
		out.WriteString(values[key])
		out.WriteString("\n")
	}

	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(out.String())
	return err
}
