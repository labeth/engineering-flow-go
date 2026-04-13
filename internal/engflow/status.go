package engflow

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type statusSnapshot struct {
	Feature       string   `json:"feature"`
	GeneratedAt   string   `json:"generated_at"`
	InputsReady   bool     `json:"inputs_ready"`
	MissingInputs []string `json:"missing_inputs,omitempty"`
	DriftBlocking int      `json:"drift_blocking"`
	DriftWarning  int      `json:"drift_warning"`
	VerifyStatus  string   `json:"verify_status"`
	NextAction    string   `json:"next_action"`
}

func runStatus(args []string, out, errOut io.Writer) int {
	configPath := resolveConfigPath(args)
	cfg, err := loadRunConfig(configPath)
	if err != nil {
		fmt.Fprintf(errOut, "status: load config %q: %v\n", configPath, err)
		return 1
	}

	fs := newFlagSet("status")
	_ = fs.String("config", configPath, "path to config file")
	feature := fs.String("feature", cfg.Feature, "feature identifier")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "status: %v\n", err)
		return 2
	}

	featureSafe := safeFeatureName(*feature)
	missingInputs := missingCanonicalInputs(cfg)
	inputsReady := len(missingInputs) == 0

	driftBlocking, driftWarning := readDriftCounts(filepath.Join(".engflow/reports", "drift.json"))
	verifyStatus := readVerifyStatus(filepath.Join(".engflow/reports", "verify.json"))

	nextAction := "ready for review"
	switch {
	case !inputsReady:
		nextAction = fmt.Sprintf("create missing canonical files: %s", strings.Join(missingInputs, ", "))
	case driftBlocking > 0:
		nextAction = fmt.Sprintf("resolve blocking drift issues and rerun `engflow drift --feature %s`", featureSafe)
	case verifyStatus == "missing":
		nextAction = fmt.Sprintf("run `engflow verify --feature %s`", featureSafe)
	case verifyStatus != "pass":
		nextAction = fmt.Sprintf("fix verify failures and rerun `engflow verify --feature %s`", featureSafe)
	}

	snap := statusSnapshot{
		Feature:       featureSafe,
		GeneratedAt:   nowRFC3339(),
		InputsReady:   inputsReady,
		MissingInputs: missingInputs,
		DriftBlocking: driftBlocking,
		DriftWarning:  driftWarning,
		VerifyStatus:  verifyStatus,
		NextAction:    nextAction,
	}

	md := renderStatusMarkdown(snap)
	if err := writeTextFile(filepath.Join(".engflow/status", "latest.md"), md); err != nil {
		fmt.Fprintf(errOut, "status: write markdown: %v\n", err)
		return 1
	}
	if err := writeJSON(filepath.Join(".engflow/status", "latest.json"), snap); err != nil {
		fmt.Fprintf(errOut, "status: write json: %v\n", err)
		return 1
	}

	fmt.Fprint(out, md)
	return 0
}

func readDriftCounts(path string) (int, int) {
	if !fileExists(path) {
		return 0, 0
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	var payload struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return 0, 0
	}
	return payload.Counts["blocking"], payload.Counts["warning"]
}

func readVerifyStatus(path string) string {
	if !fileExists(path) {
		return "missing"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return "unknown"
	}
	if strings.TrimSpace(payload.Status) == "" {
		return "unknown"
	}
	return payload.Status
}

func renderStatusMarkdown(s statusSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Status: %s\n\n", s.Feature)
	fmt.Fprintf(&b, "Generated at: %s\n\n", s.GeneratedAt)
	fmt.Fprintf(&b, "## Summary\n")
	fmt.Fprintf(&b, "- canonical inputs ready: %t\n", s.InputsReady)
	if len(s.MissingInputs) > 0 {
		fmt.Fprintf(&b, "- missing inputs: %s\n", strings.Join(s.MissingInputs, ", "))
	}
	fmt.Fprintf(&b, "- drift blocking: %d\n", s.DriftBlocking)
	fmt.Fprintf(&b, "- drift warning: %d\n", s.DriftWarning)
	fmt.Fprintf(&b, "- verify status: %s\n", s.VerifyStatus)
	fmt.Fprintf(&b, "\n## Next action\n")
	fmt.Fprintf(&b, "%s\n", s.NextAction)
	return b.String()
}

func missingCanonicalInputs(cfg runConfig) []string {
	type required struct {
		label string
		path  string
	}
	checks := []required{
		{label: "spec", path: cfg.SpecPath},
		{label: "requirements", path: cfg.RequirementsPath},
		{label: "design", path: cfg.DesignPath},
		{label: "architecture", path: cfg.ArchitecturePath},
	}
	missing := make([]string, 0, len(checks))
	for _, c := range checks {
		if strings.TrimSpace(c.path) == "" || !fileExists(c.path) {
			missing = append(missing, c.label)
		}
	}
	return missing
}
