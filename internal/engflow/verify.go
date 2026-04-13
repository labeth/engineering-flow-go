package engflow

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type verifyReport struct {
	Feature       string   `json:"feature"`
	GeneratedAt   string   `json:"generated_at"`
	RegenCmd      string   `json:"regen_cmd,omitempty"`
	TestCmd       string   `json:"test_cmd,omitempty"`
	ArchitectureAI string  `json:"architecture_ai,omitempty"`
	RegenChanged  []string `json:"regen_changed,omitempty"`
	RegenSuccess  bool     `json:"regen_success"`
	ArchitectureAIOk bool   `json:"architecture_ai_ok"`
	TestSuccess   bool     `json:"test_success"`
	Status        string   `json:"status"`
	NextAction    string   `json:"next_action"`
	OutputSnippet string   `json:"output_snippet,omitempty"`
}

func runVerify(args []string, out, errOut io.Writer) int {
	configPath := resolveConfigPath(args)
	cfg, err := loadRunConfig(configPath)
	if err != nil {
		fmt.Fprintf(errOut, "verify: load config %q: %v\n", configPath, err)
		return 1
	}
	defaultWatch := strings.Join(cfg.WatchPaths, ",")

	fs := newFlagSet("verify")
	_ = fs.String("config", configPath, "path to config file")
	feature := fs.String("feature", cfg.Feature, "feature identifier")
	regenCmd := fs.String("regen-cmd", cfg.RegenCmd, "artifact regeneration shell command")
	testCmd := fs.String("test-cmd", cfg.TestCmd, "test shell command")
	watch := fs.String("watch", defaultWatch, "comma-separated file/dir paths to detect deterministic regen drift")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "verify: %v\n", err)
		return 2
	}

	if strings.TrimSpace(*testCmd) == "" && fileExists("go.mod") {
		*testCmd = "go test ./..."
	}
	resolvedRegenCmd := strings.TrimSpace(*regenCmd)
	if resolvedRegenCmd == "" {
		resolvedRegenCmd = strings.TrimSpace(os.Getenv("ENGMODEL_GENERATE_CMD"))
	}
	archAIPath := strings.TrimSpace(cfg.ArchitectureAI)
	if archAIPath == "" {
		archAIPath = "ARCHITECTURE.ai.json"
	}

	watchPaths := parseWatchList(*watch)
	before, err := snapshotPaths(watchPaths)
	if err != nil {
		fmt.Fprintf(errOut, "verify: snapshot before: %v\n", err)
		return 1
	}

	report := verifyReport{
		Feature:      safeFeatureName(*feature),
		GeneratedAt:  nowRFC3339(),
		RegenCmd:     resolvedRegenCmd,
		TestCmd:      strings.TrimSpace(*testCmd),
		ArchitectureAI: archAIPath,
		RegenSuccess: false,
		ArchitectureAIOk: false,
		TestSuccess:  true,
	}
	combinedOutput := make([]string, 0, 2)

	if resolvedRegenCmd != "" {
		output, err := runShell(resolvedRegenCmd)
		combinedOutput = append(combinedOutput, trimOutput(output, 2000))
		if err == nil {
			report.RegenSuccess = true
		}
	}

	after, err := snapshotPaths(watchPaths)
	if err != nil {
		fmt.Fprintf(errOut, "verify: snapshot after: %v\n", err)
		return 1
	}
	if resolvedRegenCmd != "" {
		report.RegenChanged = changedPaths(before, after)
	}
	if fileExists(archAIPath) {
		report.ArchitectureAIOk = true
	}

	if strings.TrimSpace(*testCmd) != "" {
		report.TestSuccess = false
		output, err := runShell(*testCmd)
		combinedOutput = append(combinedOutput, trimOutput(output, 2000))
		if err == nil {
			report.TestSuccess = true
		}
	}

	if len(combinedOutput) > 0 {
		report.OutputSnippet = strings.Join(combinedOutput, "\n\n")
	}

	report.Status = "pass"
	report.NextAction = "No blockers detected."
	if strings.TrimSpace(report.RegenCmd) == "" {
		report.Status = "fail"
		report.NextAction = "Set commands.regen in .engflow/config.yml or ENGMODEL_GENERATE_CMD, then rerun verify."
	} else {
		if !report.RegenSuccess {
			report.Status = "fail"
			report.NextAction = "Fix regeneration command failure and rerun verify."
		}
		if report.RegenSuccess && len(report.RegenChanged) > 0 {
			report.Status = "fail"
			report.NextAction = "Regeneration changed watched artifacts; commit/update canonical outputs, then rerun verify."
		}
		if !report.ArchitectureAIOk {
			report.Status = "fail"
			report.NextAction = fmt.Sprintf("Run regeneration and ensure %s is produced.", archAIPath)
		}
	}
	if report.Status == "pass" && !report.TestSuccess {
		report.Status = "fail"
		report.NextAction = "Fix failing tests and rerun verify."
	}

	jsonPath := filepath.Join(".engflow/reports", "verify.json")
	mdPath := filepath.Join(".engflow/reports", "verify.md")
	if err := writeJSON(jsonPath, report); err != nil {
		fmt.Fprintf(errOut, "verify: write json report: %v\n", err)
		return 1
	}
	if err := writeTextFile(mdPath, renderVerifyMarkdown(report)); err != nil {
		fmt.Fprintf(errOut, "verify: write markdown report: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "verify report written: %s\n", mdPath)
	fmt.Fprintf(out, "status=%s regen_changed=%d tests_ok=%t\n", report.Status, len(report.RegenChanged), report.TestSuccess)
	if report.Status != "pass" {
		return 1
	}
	return 0
}

func parseWatchList(raw string) []string {
	items := strings.Split(raw, ",")
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		out = append(out, it)
	}
	if len(out) == 0 {
		return []string{"requirements.yml", "design.yml", "ARCHITECTURE.ai.json"}
	}
	return out
}

func runShell(cmd string) (string, error) {
	c := exec.Command("bash", "-lc", cmd)
	b, err := c.CombinedOutput()
	return string(b), err
}

func trimOutput(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

func renderVerifyMarkdown(r verifyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Verify Report: %s\n\n", r.Feature)
	fmt.Fprintf(&b, "Generated at: %s\n\n", r.GeneratedAt)
	fmt.Fprintf(&b, "## Summary\n")
	fmt.Fprintf(&b, "- status: %s\n", r.Status)
	fmt.Fprintf(&b, "- regen command: %s\n", valueOrNone(r.RegenCmd))
	fmt.Fprintf(&b, "- architecture ai path: %s\n", valueOrNone(r.ArchitectureAI))
	fmt.Fprintf(&b, "- tests command: %s\n", valueOrNone(r.TestCmd))
	fmt.Fprintf(&b, "- regen success: %t\n", r.RegenSuccess)
	fmt.Fprintf(&b, "- architecture ai present: %t\n", r.ArchitectureAIOk)
	fmt.Fprintf(&b, "- test success: %t\n", r.TestSuccess)
	if len(r.RegenChanged) > 0 {
		fmt.Fprintf(&b, "- changed watched paths:\n")
		for _, p := range r.RegenChanged {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	}
	fmt.Fprintf(&b, "\n## Next action\n")
	fmt.Fprintf(&b, "%s\n", r.NextAction)
	if r.OutputSnippet != "" {
		fmt.Fprintf(&b, "\n## Command output (trimmed)\n```")
		fmt.Fprintf(&b, "\n%s\n", r.OutputSnippet)
		fmt.Fprintf(&b, "```\n")
	}
	return b.String()
}

func valueOrNone(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "none"
	}
	return s
}
