package engflow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type scaffoldTemplateData struct {
	ProjectName  string
	ProjectDir   string
	Feature      string
	SystemID     string
	ProjectKey   string
	SystemIDHint string
}

type scaffoldTemplateFile struct {
	Path     string
	Template string
	Mode     os.FileMode
}

func runInit(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("init")
	projectDir := fs.String("project-dir", "", "target project directory")
	feature := fs.String("feature", "initial-feature", "initial feature identifier")
	force := fs.Bool("force", false, "overwrite existing scaffolded files")
	noGit := fs.Bool("no-git", false, "skip git initialization")
	runSpecKitInit := fs.Bool("run-speckit-init", true, "run Spec Kit initialization in target project (default: true)")
	noSpecKitInit := fs.Bool("no-speckit-init", false, "skip Spec Kit initialization")
	runGenerateOutputs := fs.Bool("generate-outputs", true, "run engineering generation command and produce generated outputs (default: true)")
	noGenerateOutputs := fs.Bool("no-generate-outputs", false, "skip generation of output artifacts during init")
	regenCmdFlag := fs.String("regen-cmd", "", "engineering generation command override (used by init generation)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "init: %v\n", err)
		return 2
	}
	if *noSpecKitInit {
		*runSpecKitInit = false
	}
	if *noGenerateOutputs {
		*runGenerateOutputs = false
	}

	if strings.TrimSpace(*projectDir) == "" && fs.NArg() > 0 {
		*projectDir = fs.Arg(0)
	}
	if strings.TrimSpace(*projectDir) == "" {
		fmt.Fprintln(errOut, "init: project directory is required (--project-dir or positional argument)")
		return 2
	}
	if strings.TrimSpace(*feature) == "" {
		fmt.Fprintln(errOut, "init: --feature cannot be empty")
		return 2
	}

	targetAbs, err := filepath.Abs(*projectDir)
	if err != nil {
		fmt.Fprintf(errOut, "init: resolve project directory: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		fmt.Fprintf(errOut, "init: create project directory: %v\n", err)
		return 1
	}

	projectName := filepath.Base(targetAbs)
	projectKey := sanitizeUpperIdentifier(projectName, 12, "PROJECT")
	data := scaffoldTemplateData{
		ProjectName:  projectName,
		ProjectDir:   targetAbs,
		Feature:      safeFeatureName(*feature),
		SystemID:     "SYS-" + projectKey,
		ProjectKey:   projectKey,
		SystemIDHint: "REQ-" + projectKey + "-001",
	}

	if !*noGit {
		if err := ensureGitRepo(targetAbs); err != nil {
			fmt.Fprintf(errOut, "init: git initialization failed: %v\n", err)
			return 1
		}
	}

	templateFiles := []scaffoldTemplateFile{
		{Path: "spec.md", Template: tmplSpec, Mode: 0o644},
		{Path: "requirements.yml", Template: tmplRequirements, Mode: 0o644},
		{Path: "design.yml", Template: tmplDesign, Mode: 0o644},
		{Path: "architecture.yml", Template: tmplArchitecture, Mode: 0o644},
		{Path: ".engflow/config.yml", Template: tmplConfig, Mode: 0o644},
		{Path: "AGENTS.md", Template: tmplAgents, Mode: 0o644},
		{Path: ".specify/extensions.yml", Template: tmplSpecifyExtensions, Mode: 0o644},
		{Path: ".specify/templates/overrides/spec-template.md", Template: tmplSpecTemplateOverride, Mode: 0o644},
		{Path: ".github/workflows/engflow-gate.yml", Template: tmplGateWorkflow, Mode: 0o644},
		{Path: ".opencode/command/engflow.status.md", Template: tmplOpenCodeEngflowStatus, Mode: 0o644},
		{Path: ".opencode/command/engflow.verify.md", Template: tmplOpenCodeEngflowVerify, Mode: 0o644},
		{Path: ".opencode/command/engflow.drift.md", Template: tmplOpenCodeEngflowDrift, Mode: 0o644},
		{Path: ".opencode/command/engflow.gate.md", Template: tmplOpenCodeEngflowGate, Mode: 0o644},
		{Path: ".opencode/command/engflow.trace-query.md", Template: tmplOpenCodeEngflowTraceQuery, Mode: 0o644},
		{Path: ".opencode/command/engmod.generate.md", Template: tmplOpenCodeEngmodGenerate, Mode: 0o644},
		{Path: ".opencode/command/engmod.req-text.md", Template: tmplOpenCodeEngmodReqText, Mode: 0o644},
		{Path: ".opencode/command/engmod.paths.md", Template: tmplOpenCodeEngmodPaths, Mode: 0o644},
	}

	written := make([]string, 0, len(templateFiles))
	skipped := make([]string, 0, len(templateFiles))
	for _, tf := range templateFiles {
		result, err := writeScaffoldTemplate(targetAbs, tf, data, *force)
		if err != nil {
			fmt.Fprintf(errOut, "init: write %s: %v\n", tf.Path, err)
			return 1
		}
		if result {
			written = append(written, tf.Path)
		} else {
			skipped = append(skipped, tf.Path)
		}
	}

	if *runSpecKitInit {
		if err := runSpecKitBootstrap(targetAbs); err != nil {
			fmt.Fprintf(errOut, "init: Spec Kit init failed: %v\n", err)
			return 1
		}
	}

	generationSummary := ""
	if *runGenerateOutputs {
		genInfo, err := runInitGeneration(targetAbs, strings.TrimSpace(*regenCmdFlag))
		if err != nil {
			fmt.Fprintf(errOut, "init: generation failed: %v\n", err)
			return 1
		}
		generationSummary = genInfo
	}

	fmt.Fprintf(out, "init complete: %s\n", targetAbs)
	if len(written) > 0 {
		fmt.Fprintf(out, "written files:\n")
		for _, p := range written {
			fmt.Fprintf(out, "  - %s\n", p)
		}
	}
	if len(skipped) > 0 {
		fmt.Fprintf(out, "skipped existing files:\n")
		for _, p := range skipped {
			fmt.Fprintf(out, "  - %s\n", p)
		}
	}
	if generationSummary != "" {
		fmt.Fprintln(out, generationSummary)
	}
	fmt.Fprintf(out, "next steps:\n")
	fmt.Fprintf(out, "  1) set .engflow/config.yml commands.regen for engineering-model generation\n")
	fmt.Fprintf(out, "  2) edit spec.md and requirements/design artifacts\n")
	fmt.Fprintf(out, "  3) ensure Spec Kit uses override template at .specify/templates/overrides/spec-template.md\n")
	fmt.Fprintf(out, "  4) run: engflow gate --config .engflow/config.yml --feature %s\n", data.Feature)
	return 0
}

func ensureGitRepo(root string) error {
	if fileExists(filepath.Join(root, ".git")) {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeScaffoldTemplate(projectRoot string, tf scaffoldTemplateFile, data scaffoldTemplateData, force bool) (bool, error) {
	target := filepath.Join(projectRoot, filepath.Clean(tf.Path))
	if !force && fileExists(target) {
		return false, nil
	}
	tmpl, err := template.New(tf.Path).Parse(tf.Template)
	if err != nil {
		return false, err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return false, err
	}
	if err := mustEnsureDir(filepath.Dir(target)); err != nil {
		return false, err
	}
	if err := os.WriteFile(target, b.Bytes(), tf.Mode); err != nil {
		return false, err
	}
	return true, nil
}

func sanitizeUpperIdentifier(raw string, maxLen int, fallback string) string {
	raw = strings.ToUpper(raw)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = fallback
	}
	if maxLen > 0 && len(s) > maxLen {
		s = strings.ReplaceAll(s[:maxLen], "-", "")
	}
	if s == "" {
		s = fallback
	}
	return s
}

func runSpecKitBootstrap(projectRoot string) error {
	if _, err := exec.LookPath("specify"); err != nil {
		return errors.New("specify command not found; install Spec Kit CLI or rerun with --no-speckit-init")
	}

	// Prefer the current Spec Kit flags; fall back when optional flags are unsupported.
	primaryArgs := []string{"init", "--here", "--force", "--ai", "opencode", "--ignore-agent-tools"}
	if err := runSpecify(projectRoot, primaryArgs); err == nil {
		return nil
	}

	fallbackArgs := []string{"init", "--here", "--force", "--ai", "opencode"}
	if err := runSpecify(projectRoot, fallbackArgs); err == nil {
		return nil
	}

	return runSpecify(projectRoot, primaryArgs)
}

func runSpecify(projectRoot string, args []string) error {
	cmd := exec.Command("specify", args...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runInitGeneration(projectRoot, regenOverride string) (string, error) {
	cfgPath := filepath.Join(projectRoot, ".engflow", "config.yml")
	cfg, err := loadRunConfig(cfgPath)
	if err != nil {
		return "", fmt.Errorf("load config %q: %w", cfgPath, err)
	}

	regenCmd := strings.TrimSpace(regenOverride)
	if regenCmd == "" {
		regenCmd = strings.TrimSpace(os.Getenv("ENGMODEL_GENERATE_CMD"))
	}
	if regenCmd == "" {
		regenCmd = strings.TrimSpace(cfg.RegenCmd)
	}
	if regenCmd == "" {
		return "", errors.New("generation is enabled but no regen command is configured; set --regen-cmd, ENGMODEL_GENERATE_CMD, or commands.regen in .engflow/config.yml")
	}

	cmd := exec.Command("bash", "-lc", regenCmd)
	cmd.Dir = projectRoot
	env := os.Environ()
	env = append(env, "ENGMODEL_REPO_ROOT="+projectRoot)
	env = append(env, "ENGMODEL_REQUIREMENTS_PATH="+cfg.RequirementsPath)
	env = append(env, "ENGMODEL_DESIGN_PATH="+cfg.DesignPath)
	env = append(env, "ENGMODEL_ARCHITECTURE_PATH="+cfg.ArchitecturePath)
	env = append(env, "ENGMODEL_ARCH_AI_PATH="+cfg.ArchitectureAI)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %q failed: %w (%s)", regenCmd, err, strings.TrimSpace(string(out)))
	}

	aiPath := cfg.ArchitectureAI
	if strings.TrimSpace(aiPath) == "" {
		aiPath = "ARCHITECTURE.ai.json"
	}
	absAIPath := aiPath
	if !filepath.IsAbs(absAIPath) {
		absAIPath = filepath.Join(projectRoot, aiPath)
	}
	if !fileExists(absAIPath) {
		return "", fmt.Errorf("generation command succeeded but expected output was not produced: %s", aiPath)
	}
	return fmt.Sprintf("generated outputs:\n  - %s", aiPath), nil
}

const tmplSpec = `# Product Spec: {{ .ProjectName }}

## User stories
### Story 1 - initial capability
As a user, I want <capability> so that <outcome>.

Acceptance criteria:
- <criterion-1>
- <criterion-2>
`

const tmplRequirements = `system:
  id: {{ .SystemID }}
  name: "{{ .ProjectName }}"
  purpose: "Initial engineering-model-go requirement set."
requirements:
  - id: {{ .SystemIDHint }}
    title: "Initial requirement"
    statement: "Define the first end-to-end feature behavior."
    priority: must
`

const tmplDesign = `system:
  id: {{ .SystemID }}
  name: "{{ .ProjectName }}"

scope:
  summary: "Describe the first capability and boundaries."

architecture_notes:
  components:
    - "Capture major components and interactions."

verification_notes:
  checks:
    - "List tests and checks that prove the requirement."
`

const tmplArchitecture = `system:
  id: {{ .SystemID }}
  name: "{{ .ProjectName }}"

functional_units:
  - id: FU-{{ .ProjectKey }}-001
    name: "core-flow"
    purpose: "Primary functional unit for initial feature flow."
    supports_requirements:
      - {{ .SystemIDHint }}

runtime_evidence:
  - id: RT-{{ .ProjectKey }}-001
    name: "initial-runtime-path"
    linked_units:
      - FU-{{ .ProjectKey }}-001

code_evidence:
  - id: CODE-{{ .ProjectKey }}-001
    name: "initial-implementation-placeholder"
    linked_runtime:
      - RT-{{ .ProjectKey }}-001

verification:
  - id: VER-{{ .ProjectKey }}-001
    name: "initial-verification-placeholder"
    verifies:
      - {{ .SystemIDHint }}
      - CODE-{{ .ProjectKey }}-001
`

const tmplConfig = `feature: {{ .Feature }}

paths:
  spec: spec.md
  requirements: requirements.yml
  design: design.yml
  architecture: architecture.yml
  architecture_ai: ARCHITECTURE.ai.json
  repo_root: .

commands:
  # Required for enforced verify/gate flow. Example: "make engmodel-generate"
  regen: ""
  test: ""

verify:
  watch: requirements.yml,design.yml,architecture.yml,ARCHITECTURE.ai.json
`

const tmplGateWorkflow = `name: engflow-gate

on:
  workflow_dispatch:

jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Ensure engflow is available
        run: |
          if ! command -v engflow >/dev/null 2>&1; then
            echo "engflow binary not found in PATH"
            echo "Install engflow in CI before enabling this gate workflow."
            exit 1
          fi

      - name: Run engflow gate
        run: engflow gate --config .engflow/config.yml --feature {{ .Feature }}
`

const tmplAgents = `# AGENTS.md

Default workflow for this repository uses engflow + engineering-model-go artifacts.

<!-- ENGFLOW:START -->
Rules:
- Canonical artifacts: requirements.yml, design.yml, architecture.yml
- Generated artifact target: ARCHITECTURE.ai.json
- Use .engflow/config.yml defaults for paths/commands
- Full engmod feature support is currently only for Go, Rust, and TypeScript
- Prefer Go/Rust/TypeScript for new feature implementations unless user constraints require another stack
- Deployment-side support is currently focused on Terraform and Flux
- Prefer Terraform + Flux for deployment/infrastructure work unless user constraints require another stack
- Speckit lifecycle is mandatory for feature work:
  1. /speckit.specify
  2. /speckit.plan
  3. /speckit.tasks
  4. /speckit.implement
- Engflow checkpoints are mandatory through .specify/extensions.yml hooks
- Engflow quality gate order: verify -> drift -> status (or gate)
- Keep REQ-* identifiers stable; do not silently rename accepted IDs
- Add REQ-* references in code/tests/verification artifacts for traceability
- verify/gate must fail if commands.regen is missing or ARCHITECTURE.ai.json is missing

Before /speckit.specify, collect required feature input:
- Feature description (user-facing behavior)
- Scope boundaries and constraints
- Feature depth: full artifacts vs lightweight pass
- Feature naming/branching preference (or confirm repo defaults)
- Whether optional git auto-commit hooks should be used

Input collection policy:
- If required inputs are already inferable from user prompt/repo context, proceed without extra questions
- Ask follow-up questions only for missing or ambiguous inputs that materially affect scope/plan/tasks

If required input is still missing:
- Ask only for the missing items before continuing
- If user asks to proceed without detail, state assumptions explicitly and continue with repo defaults

Quick commands:
- /speckit.specify "<feature description>"
- /speckit.plan
- /speckit.tasks
- /speckit.implement
- /engmod.generate
- engflow gate --config .engflow/config.yml --feature {{ .Feature }}
<!-- ENGFLOW:END -->
`

const tmplSpecifyExtensions = `# Enforced Spec Kit hook wiring for engflow.
# If you initialized with engflow, these hooks are enabled and mandatory by default.
# To relax enforcement, set enabled: false and/or optional: true per hook.

installed: []
settings:
  auto_execute_hooks: true
hooks:
  before_plan:
    - extension: engflow
      command: engflow.status
      description: Enforce canonical artifact readiness before planning
      optional: false
      enabled: true
  after_plan:
    - extension: engflow
      command: engflow.status
      description: Enforce workflow status checkpoint after plan generation
      optional: false
      enabled: true
  before_implement:
    - extension: engflow
      command: engflow.status
      description: Enforce readiness checkpoint before implementation
      optional: false
      enabled: true
  after_implement:
    - extension: engflow
      command: engflow.gate
      description: Enforce verify, drift, and status gate after implementation
      optional: false
      enabled: true
`

const tmplSpecTemplateOverride = `# Feature Specification: [FEATURE NAME]

**Feature Branch**: [###-feature-name]  
**Created**: [DATE]  
**Status**: Draft  
**Input**: User description: "$ARGUMENTS"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - [Brief Title] (Priority: P1)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]
2. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 2 - [Brief Title] (Priority: P2)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 3 - [Brief Title] (Priority: P3)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

### Edge Cases

- What happens when [boundary condition]?
- How does system handle [error scenario]?

## Requirements *(single source: REQ IDs)*

Canonical source of truth is requirements.yml.
List requirement IDs here for traceability and planning context.

### Functional Requirements

Generate requirement text from requirements.yml with:

    yq -o=json '.requirements' requirements.yml | jq -r '.[] | "- **\(.id)**: \(.statement)"'

Paste the generated lines here and keep them aligned with requirements.yml.

Do not use FR-* IDs in this repository.

### Key Entities *(include if feature involves data)*

- **[Entity 1]**: [What it represents, key attributes without implementation]
- **[Entity 2]**: [What it represents, relationships to other entities]

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: [Measurable metric]
- **SC-002**: [Measurable metric]
- **SC-003**: [User/business metric]

## Assumptions

- [Assumption about target users]
- [Assumption about scope boundaries]
- [Assumption about dependencies]
`

const tmplOpenCodeEngflowStatus = `# /engflow.status

Show current engflow status and next action.

## Usage

/engflow.status [feature-id]

## Behavior

Run:
engflow status --config .engflow/config.yml --feature {{ .Feature }}
`

const tmplOpenCodeEngflowVerify = `# /engflow.verify

Run verification checks and produce verify report outputs.

## Usage

/engflow.verify [feature-id]

## Behavior

Run:
engflow verify --config .engflow/config.yml --feature {{ .Feature }}

Surface:
- .engflow/reports/verify.md
- .engflow/reports/verify.json
`

const tmplOpenCodeEngflowDrift = `# /engflow.drift

Run drift checks and summarize blocking/warning findings.

## Usage

/engflow.drift [feature-id]

## Behavior

Run:
engflow drift --config .engflow/config.yml --feature {{ .Feature }}

Surface:
- .engflow/reports/drift.md
- .engflow/reports/drift.json
`

const tmplOpenCodeEngflowGate = `# /engflow.gate

Run the full quality gate (verify -> drift -> status).

## Usage

/engflow.gate [feature-id]

## Behavior

Run:
engflow gate --config .engflow/config.yml --feature {{ .Feature }}
`

const tmplOpenCodeEngflowTraceQuery = `# /engflow.trace-query

Find repository references for a requirement ID.

## Usage

/engflow.trace-query <REQ-ID>

## Behavior

Run:
engflow trace-query --config .engflow/config.yml --id <REQ-ID>
`

const tmplOpenCodeEngmodGenerate = `# /engmod.generate

Run engineering-model-go generation using project configuration.

## Usage

/engmod.generate

## Behavior

1. Resolve generation command from:
   - .engflow/config.yml -> commands.regen
   - ENGMODEL_GENERATE_CMD (fallback)
2. If no command is configured, stop with an actionable error.
3. Run the generation command from the repository root.
4. Verify ARCHITECTURE.ai.json exists after generation.
`

const tmplOpenCodeEngmodReqText = `# /engmod.req-text

Render requirement IDs and statements from requirements.yml.

## Usage

/engmod.req-text

## Behavior

Run:
yq -o=json '.requirements' requirements.yml | jq -r '.[] | "- **\(.id)**: \(.statement)"'

Return generated markdown lines for spec/plan context.
`

const tmplOpenCodeEngmodPaths = `# /engmod.paths

Show canonical engineering-model-go path bindings for this repo.

## Usage

/engmod.paths

## Behavior

Return canonical paths from .engflow/config.yml:
- requirements path
- design path
- architecture path
- architecture AI output path

And show equivalent environment variable mapping:
- ENGMODEL_REQUIREMENTS_PATH
- ENGMODEL_DESIGN_PATH
- ENGMODEL_ARCHITECTURE_PATH
- ENGMODEL_ARCH_AI_PATH
`
