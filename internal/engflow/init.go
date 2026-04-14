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
		{Path: "catalog.yml", Template: tmplCatalog, Mode: 0o644},
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
	enforcedSpecTemplates, err := enforceSpecTemplateTemplates(targetAbs, data)
	if err != nil {
		fmt.Fprintf(errOut, "init: enforce spec template override failed: %v\n", err)
		return 1
	}
	for _, p := range enforcedSpecTemplates {
		if !containsString(written, p) {
			written = append(written, p)
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
	cfg, err := loadRunConfig(filepath.Join(targetAbs, ".engflow", "config.yml"))
	if err != nil {
		fmt.Fprintf(errOut, "init: load scaffold config failed: %v\n", err)
		return 1
	}
	if err := writeScaffoldBaseline(targetAbs, cfg, *force); err != nil {
		fmt.Fprintf(errOut, "init: write scaffold baseline failed: %v\n", err)
		return 1
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
	fmt.Fprintf(out, "  1) review .engflow/config.yml commands.regen for engineering-model generation\n")
	fmt.Fprintf(out, "  2) edit spec.md and catalog/requirements/design/architecture artifacts\n")
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

func enforceSpecTemplateTemplates(projectRoot string, data scaffoldTemplateData) ([]string, error) {
	targets := []string{
		".specify/templates/spec-template.md",
		".specify/templates/overrides/spec-template.md",
	}
	written := make([]string, 0, len(targets))
	for _, rel := range targets {
		tf := scaffoldTemplateFile{
			Path:     rel,
			Template: tmplSpecTemplateOverride,
			Mode:     0o644,
		}
		ok, err := writeScaffoldTemplate(projectRoot, tf, data, true)
		if err != nil {
			return nil, err
		}
		if ok {
			written = append(written, rel)
		}
	}
	return written, nil
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
	env = append(env, "ENGMODEL_CATALOG_PATH="+cfg.CatalogPath)
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

const tmplCatalog = `catalog:
  systems:
    - id: {{ .SystemID }}
      name: "{{ .ProjectName }} system"
      definition: "System-of-interest for initial engflow scaffold."
      aliases: ["the {{ .ProjectName }} system"]

  functionalGroups:
    - id: FG-{{ .ProjectKey }}-CORE
      name: "core flow"
      definition: "Primary functional group for the first end-to-end capability."
      aliases: ["core domain"]

  functionalUnits:
    - id: FU-{{ .ProjectKey }}-CORE
      name: "core flow"
      definition: "Primary functional unit responsible for initial request handling."
      aliases: ["core unit"]

  referencedElements:
    - id: REF-{{ .ProjectKey }}-EXTERNAL-SERVICE
      name: "external service endpoint"
      definition: "Placeholder external dependency used by the initial scaffold."
      aliases: ["external endpoint"]

  actors:
    - id: ACT-{{ .ProjectKey }}-USER
      name: "user"
      definition: "Primary actor interacting with the initial feature flow."
      aliases: ["operator"]

  attackVectors:
    - id: AV-{{ .ProjectKey }}-INVALID-INPUT
      name: "invalid input payload"
      definition: "Malformed or hostile input attempting to break request handling."
      aliases: ["malformed request"]

  events:
    - id: EVT-{{ .ProjectKey }}-INITIAL-REQUEST
      name: "initial request is received"
      definition: "Event emitted when the first feature request enters the system."
      aliases: ["request received"]

  states:
    - id: STATE-{{ .ProjectKey }}-ACTIVE
      name: "{{ .ProjectName }} is active"
      definition: "System state indicating the primary flow is ready to execute."
      aliases: ["active state"]

  features:
    - id: FEAT-{{ .ProjectKey }}-CORE-FLOW
      name: "core flow is enabled"
      definition: "Feature flag indicating primary feature flow is enabled."
      aliases: ["core flow enabled"]

  modes:
    - id: MODE-{{ .ProjectKey }}-NORMAL
      name: "normal mode is active"
      definition: "Default operating mode for the scaffolded system."
      aliases: ["normal mode"]

  conditions:
    - id: COND-{{ .ProjectKey }}-DEPENDENCIES-READY
      name: "dependencies are ready"
      definition: "Condition indicating required upstream dependencies are available."
      aliases: ["deps ready"]

  dataTerms:
    - id: DATA-{{ .ProjectKey }}-REQUEST-ID
      name: "request id"
      definition: "Identifier used to trace the request through the system."
      aliases: ["trace id"]
`

const tmplRequirements = `lintRun:
  id: {{ .ProjectKey }}-lint
  mode: guided
  commaAsAnd: true
  catalogRef: ./catalog.yml

requirements:
  - id: {{ .SystemIDHint }}
    text: "When initial request is received, the {{ .ProjectName }} system shall execute core flow."
    notes: "Initial end-to-end requirement tied to scaffolded catalog terms."
    appliesTo:
      - FU-{{ .ProjectKey }}-CORE
`

const tmplDesign = `design:
  id: {{ .ProjectKey }}-design
  title: "{{ .ProjectName }} Initial Design"

  functionalGroups:
    - id: FG-{{ .ProjectKey }}-CORE
      views:
        architecture_intent:
          title: "Core Flow Architecture Intent"
          narrative: |
            This initial design defines the primary functional boundary for the first capability.
            Expand this narrative as architecture intent and responsibilities become clearer.

  functionalUnits:
    - id: FU-{{ .ProjectKey }}-CORE
      group: FG-{{ .ProjectKey }}-CORE
      views:
        architecture_intent:
          title: "Core Flow Unit Architecture Intent"
          narrative: |
            This unit handles the primary request lifecycle for the scaffolded feature.
            Add communication, deployment, security, and traceability narratives during planning.
`

const tmplArchitecture = `model:
  id: {{ .ProjectKey }}-architecture
  title: "{{ .ProjectName }} Architecture Model"
  introduction: |
    This is the initial engineering-model-go architecture for {{ .ProjectName }}.
    Extend authored architecture and mappings as feature scope becomes concrete.
  baseCatalogRef: ./catalog.yml

authoredArchitecture:
  functionalGroups:
    - id: FG-{{ .ProjectKey }}-CORE
      name: Core Flow
      description: Primary capability group for the initial scaffold.
      prose: |
        Core flow owns the initial end-to-end behavior described in requirements.yml.
        Expand the prose with business and system boundaries as planning progresses.

  functionalUnits:
    - id: FU-{{ .ProjectKey }}-CORE
      name: Core Flow Unit
      group: FG-{{ .ProjectKey }}-CORE
      prose: |
        Core flow unit executes the first request lifecycle and orchestration behavior.
        Add detailed behavior, constraints, and ownership during implementation planning.

  actors:
    - id: ACT-{{ .ProjectKey }}-USER
      name: User
      description: Primary actor interacting with the first feature flow.

  attackVectors:
    - id: AV-{{ .ProjectKey }}-INVALID-INPUT
      name: Invalid Input Payload
      description: Malformed or hostile request aiming to disrupt primary flow.

  referencedElements:
    - id: REF-{{ .ProjectKey }}-EXTERNAL-SERVICE
      kind: external_service_endpoint
      layer: runtime
      name: External Service Endpoint

  mappings:
    - type: contains
      from: FG-{{ .ProjectKey }}-CORE
      to: FU-{{ .ProjectKey }}-CORE
    - type: interacts_with
      from: ACT-{{ .ProjectKey }}-USER
      to: FU-{{ .ProjectKey }}-CORE
      description: Initiates the primary request flow.
    - type: depends_on
      from: FU-{{ .ProjectKey }}-CORE
      to: REF-{{ .ProjectKey }}-EXTERNAL-SERVICE
      description: Uses external dependency for initial orchestration.
    - type: targets
      from: AV-{{ .ProjectKey }}-INVALID-INPUT
      to: FU-{{ .ProjectKey }}-CORE

inferenceHints:
  runtimeSources:
    - ./infra/terraform
    - ./infra/flux
  codeSources:
    - ./src
  expectedRuntimeKinds:
    - workload
  ownershipResolutionOrder:
    - explicit_annotation
    - path_convention
    - unresolved_warning

views:
  - id: VIEW-ARCH-INTENT
    kind: architecture-intent
    roots:
      - FG-{{ .ProjectKey }}-CORE
    authoredStatus: partial
    authoredStatusExplanation: "Initial scaffold view; expand roots and details with feature scope."
`

const tmplConfig = `feature: {{ .Feature }}

paths:
  spec: spec.md
  catalog: catalog.yml
  requirements: requirements.yml
  design: design.yml
  architecture: architecture.yml
  architecture_ai: ARCHITECTURE.ai.json
  repo_root: .

commands:
  # Required for enforced verify/gate flow.
  regen: "engdoc --model architecture.yml --requirements requirements.yml --design design.yml --ai-json-out ARCHITECTURE.ai.json"
  test: ""

verify:
  watch: catalog.yml,requirements.yml,design.yml,architecture.yml,ARCHITECTURE.ai.json
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
- Policy precedence:
  1. This AGENTS.md is authoritative for this repository.
  2. If any default assistant behavior conflicts with this file, this file MUST win.
- Canonical artifacts: catalog.yml, requirements.yml, design.yml, architecture.yml
- Generated artifact target: ARCHITECTURE.ai.json
- Use .engflow/config.yml defaults for paths/commands
- Architecture root-of-trust policy:
  - engdoc (via commands.regen) is the only trusted producer of ARCHITECTURE.ai.json.
  - ARCHITECTURE.ai.json is generated-only and MUST NOT be manually edited.
  - After changing catalog.yml, requirements.yml, design.yml, or architecture.yml, run /engmod.generate before architecture-dependent planning or implementation.
  - If ARCHITECTURE.ai.json is missing or stale, block architecture conclusions until regeneration succeeds.
  - For non-trivial feature implementation, update at least one canonical model file (catalog.yml, requirements.yml, design.yml, architecture.yml).
  - If implementation files change while canonical model files stay unchanged, consider the feature incomplete and fail verify/gate.
- Implementation language hard gate:
  - New implementation code MUST use one of: Go, Rust, TypeScript.
  - Python and other runtimes MUST NOT be used for new implementation code unless the user explicitly requests an exception in the current conversation.
  - If language is ambiguous, default MUST be TypeScript.
- Deployment stack hard gate:
  - Deployment/infrastructure work MUST use Terraform + Flux unless the user explicitly requests an exception in the current conversation.
- Speckit lifecycle is mandatory for non-trivial feature work:
  1. /speckit.specify
  2. /speckit.plan
  3. /speckit.tasks
  4. Update canonical model files for the feature (catalog.yml, requirements.yml, design.yml, architecture.yml).
  5. /engmod.generate (pre-implementation regeneration; must refresh ARCHITECTURE.ai.json using engdoc).
  6. /speckit.implement (implementation happens here, after model updates and pre-regeneration).
  7. /engmod.generate (final regeneration after implementation).
  8. engflow gate --config .engflow/config.yml --feature {{ .Feature }}
- Treat implementation as incomplete until steps 7 and 8 pass.
- The agent MUST NOT skip Speckit lifecycle steps unless user explicitly says "skip speckit workflow".
- Engflow checkpoints are mandatory through .specify/extensions.yml hooks
- Engflow quality gate order: verify -> drift -> status (or gate)
- Keep REQ-* identifiers stable; do not silently rename accepted IDs
- Add REQ-* references in code/tests/verification artifacts for traceability
- verify/gate must fail if commands.regen is missing or ARCHITECTURE.ai.json is missing
- Manual test-result documents are forbidden.
- Do not create ad-hoc files like test-results.md, verification-notes.md, qa-report.md, or docs/test-results/*.
- Test evidence must come from command output and generated reports only:
  - .engflow/reports/verify.{md,json}
  - .engflow/reports/drift.{md,json}

Preflight output (must appear before edits):
- Selected language/runtime
- Why it matches policy
- Intended project path
- Whether Speckit lifecycle will be executed

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
- Ask exactly one blocking question that covers all missing items before continuing
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

    yq -o=json '.requirements' requirements.yml | jq -r '.[] | "- **\(.id)**: \(.text)"'

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
5. Treat ARCHITECTURE.ai.json as generated-only; do not manually edit it.
6. If generation fails, stop and fix generation before architecture-dependent implementation work.
`

const tmplOpenCodeEngmodReqText = `# /engmod.req-text

Render requirement IDs and statements from requirements.yml.

## Usage

/engmod.req-text

## Behavior

Run:
yq -o=json '.requirements' requirements.yml | jq -r '.[] | "- **\(.id)**: \(.text)"'

Return generated markdown lines for spec/plan context.
`

const tmplOpenCodeEngmodPaths = `# /engmod.paths

Show canonical engineering-model-go path bindings for this repo.

## Usage

/engmod.paths

## Behavior

Return canonical paths from .engflow/config.yml:
- catalog path
- requirements path
- design path
- architecture path
- architecture AI output path

And show equivalent environment variable mapping:
- ENGMODEL_CATALOG_PATH
- ENGMODEL_REQUIREMENTS_PATH
- ENGMODEL_DESIGN_PATH
- ENGMODEL_ARCHITECTURE_PATH
- ENGMODEL_ARCH_AI_PATH
`
