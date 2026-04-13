package engflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type driftIssue struct {
	Severity        string   `json:"severity"`
	Code            string   `json:"code"`
	Message         string   `json:"message"`
	Evidence        []string `json:"evidence,omitempty"`
	SuggestedAction string   `json:"suggested_action,omitempty"`
}

type driftReport struct {
	Feature     string         `json:"feature"`
	GeneratedAt string         `json:"generated_at"`
	Counts      map[string]int `json:"counts"`
	Issues      []driftIssue   `json:"issues"`
}

func runDrift(args []string, out, errOut io.Writer) int {
	configPath := resolveConfigPath(args)
	cfg, err := loadRunConfig(configPath)
	if err != nil {
		fmt.Fprintf(errOut, "drift: load config %q: %v\n", configPath, err)
		return 1
	}

	fs := newFlagSet("drift")
	_ = fs.String("config", configPath, "path to config file")
	feature := fs.String("feature", cfg.Feature, "feature identifier")
	specPath := fs.String("spec", cfg.SpecPath, "feature spec path")
	requirementsPath := fs.String("requirements", cfg.RequirementsPath, "canonical requirements path")
	repoRoot := fs.String("repo-root", cfg.RepoRoot, "repository root to scan")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "drift: %v\n", err)
		return 2
	}

	issues := make([]driftIssue, 0)

	specInfo, specErr := os.Stat(*specPath)
	reqInfo, reqErr := os.Stat(*requirementsPath)
	if specErr != nil {
		issues = append(issues, driftIssue{
			Severity:        "blocking",
			Code:            "DRIFT-SPEC-MISSING",
			Message:         fmt.Sprintf("spec file missing: %s", *specPath),
			SuggestedAction: "Create or point to a valid spec.md before running drift checks.",
		})
	}
	if reqErr != nil {
		issues = append(issues, driftIssue{
			Severity:        "blocking",
			Code:            "DRIFT-REQ-MISSING",
			Message:         fmt.Sprintf("requirements file missing: %s", *requirementsPath),
			SuggestedAction: "Create/update canonical requirements.yml before running drift.",
		})
	}
	if specErr == nil && reqErr == nil && specInfo.ModTime().After(reqInfo.ModTime()) {
		issues = append(issues, driftIssue{
			Severity:        "blocking",
			Code:            "DRIFT-SPEC-AHEAD",
			Message:         "spec is newer than requirements; canonical artifacts may be out of date",
			Evidence:        []string{*specPath, *requirementsPath},
			SuggestedAction: "Update canonical requirements/design/architecture to match current spec intent.",
		})
	}

	canonicalIDs := []string{}
	canonicalSet := map[string]struct{}{}
	if reqErr == nil {
		ids, err := parseRequirementIDsFromFile(*requirementsPath)
		if err == nil {
			canonicalIDs = ids
			for _, id := range ids {
				canonicalSet[id] = struct{}{}
			}
		}
		if len(canonicalIDs) == 0 {
			issues = append(issues, driftIssue{
				Severity:        "warning",
				Code:            "DRIFT-NO-REQ-IDS",
				Message:         "no REQ-* IDs found in canonical requirements",
				SuggestedAction: "Ensure requirements.yml contains stable REQ-* identifiers.",
			})
		}
	}

	mentions, err := collectIDMentions(*repoRoot)
	if err != nil {
		fmt.Fprintf(errOut, "drift: scan repo: %v\n", err)
		return 1
	}

	for _, id := range canonicalIDs {
		paths := mentions[id]
		filtered := make([]string, 0, len(paths))
		for _, p := range paths {
			if filepath.Clean(p) == filepath.Clean(*requirementsPath) {
				continue
			}
			filtered = append(filtered, p)
		}
		if len(filtered) == 0 {
			issues = append(issues, driftIssue{
				Severity:        "warning",
				Code:            "DRIFT-MISSING-EVIDENCE",
				Message:         fmt.Sprintf("no evidence links found for %s outside requirements", id),
				SuggestedAction: "Reference requirement IDs in code comments, tests, or verification artifacts.",
			})
		}
	}

	for id, paths := range mentions {
		if _, ok := canonicalSet[id]; ok {
			continue
		}
		if hasTestPath(paths) {
			issues = append(issues, driftIssue{
				Severity:        "blocking",
				Code:            "DRIFT-UNKNOWN-ID-IN-TEST",
				Message:         fmt.Sprintf("test references unknown requirement ID %s", id),
				Evidence:        paths,
				SuggestedAction: "Add the missing requirement to canonical artifacts or update test references.",
			})
		}
	}

	counts := map[string]int{"blocking": 0, "warning": 0, "informational": 0}
	for _, it := range issues {
		counts[it.Severity]++
	}
	sort.Slice(issues, func(i, j int) bool {
		return severityRank(issues[i].Severity) < severityRank(issues[j].Severity)
	})

	report := driftReport{
		Feature:     safeFeatureName(*feature),
		GeneratedAt: nowRFC3339(),
		Counts:      counts,
		Issues:      issues,
	}
	jsonPath := filepath.Join(".engflow/reports", "drift.json")
	mdPath := filepath.Join(".engflow/reports", "drift.md")
	if err := writeJSON(jsonPath, report); err != nil {
		fmt.Fprintf(errOut, "drift: write json report: %v\n", err)
		return 1
	}
	if err := writeTextFile(mdPath, renderDriftMarkdown(report)); err != nil {
		fmt.Fprintf(errOut, "drift: write markdown report: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "drift report written: %s\n", mdPath)
	fmt.Fprintf(out, "blocking=%d warning=%d info=%d\n", counts["blocking"], counts["warning"], counts["informational"])
	if counts["blocking"] > 0 {
		return 1
	}
	return 0
}

func severityRank(s string) int {
	switch s {
	case "blocking":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func hasTestPath(paths []string) bool {
	for _, p := range paths {
		base := strings.ToLower(filepath.Base(p))
		low := strings.ToLower(filepath.ToSlash(p))
		if strings.HasSuffix(base, "_test.go") || strings.Contains(low, "/test/") || strings.Contains(low, "/tests/") || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, ".spec.ts") {
			return true
		}
	}
	return false
}

func collectIDMentions(root string) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".engflow" {
				if filepath.Clean(path) == filepath.Clean(root) {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !scanEligibleFile(path) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if len(b) > 1_000_000 {
			return nil
		}
		ids := parseRequirementIDs(string(b))
		for _, id := range ids {
			out[id] = append(out[id], filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for id, paths := range out {
		out[id] = uniqueStrings(paths)
	}
	return out, nil
}

func scanEligibleFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".md", ".txt", ".yml", ".yaml", ".py", ".sh", ".js", ".ts", ".tsx", ".rs", ".java":
		return true
	default:
		return false
	}
}

func renderDriftMarkdown(r driftReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Drift Report: %s\n\n", r.Feature)
	fmt.Fprintf(&b, "Generated at: %s\n\n", r.GeneratedAt)
	fmt.Fprintf(&b, "## Summary\n")
	fmt.Fprintf(&b, "- blocking: %d\n", r.Counts["blocking"])
	fmt.Fprintf(&b, "- warning: %d\n", r.Counts["warning"])
	fmt.Fprintf(&b, "- informational: %d\n", r.Counts["informational"])
	fmt.Fprintf(&b, "\n## Issues\n")
	if len(r.Issues) == 0 {
		fmt.Fprintf(&b, "- none\n")
		return b.String()
	}
	for _, it := range r.Issues {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", it.Severity, it.Message, it.Code)
		if it.SuggestedAction != "" {
			fmt.Fprintf(&b, "  action: %s\n", it.SuggestedAction)
		}
		for _, ev := range it.Evidence {
			fmt.Fprintf(&b, "  evidence: %s\n", ev)
		}
	}
	return b.String()
}
