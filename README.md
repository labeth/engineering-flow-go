# engflow

`engflow` is a Spec Kit extension CLI.
Its primary purpose is to add engineering-model-go support and requirement traceability
to Spec Kit-driven projects.

It provides:

- Spec Kit-first project setup with engineering-model inputs
- single-source canonical engineering artifacts
- verify generation/tests
- detect drift
- show next action status

Language support note:
- Full engineering-model feature support is currently focused on Go, Rust, and TypeScript.
- For new feature work, prefer Go/Rust/TypeScript unless constraints require another language.
- Deployment-side support is currently focused on Terraform and Flux.
- For deployment/infrastructure work, prefer Terraform + Flux unless constraints require another stack.

## Build and run

```bash
go run ./cmd/engflow help
```

## Project config

Defaults are loaded from `.engflow/config.yml` (or `--config <path>` per command).

Current repo config is in:

- `.engflow/config.yml`

Example:

```yaml
feature: engflow-mvp
paths:
  spec: spec.md
  requirements: requirements.yml
  design: design.yml
  architecture: architecture.yml
  architecture_ai: ARCHITECTURE.ai.json
commands:
  test: go test ./...
verify:
  watch: requirements.yml,design.yml,architecture.yml,ARCHITECTURE.ai.json
```

## Commands

```bash
engflow init --project-dir /path/to/new-project --feature initial-feature
engflow gate --feature <feature-id>
engflow trace-query --id <REQ-ID>
engflow verify --feature <feature-id> [--regen-cmd "..."] [--test-cmd "..."]
engflow drift --feature <feature-id> [--spec spec.md] [--requirements requirements.yml]
engflow status --feature <feature-id>
```

Each command also accepts:

```bash
--config .engflow/config.yml
```

## Output layout

- `.engflow/status/latest.{md,json}`
- `.engflow/reports/verify.{md,json}`
- `.engflow/reports/drift.{md,json}`
- `.engflow/state/`

## Current scope

This is an MVP extension layer on top of Spec Kit for engineering-model-go workflows.

## OpenCode assets

- Commands:
  - `.opencode/command/engflow.*.md`
  - `.opencode/command/engmod.*.md`
- Skills:
  - `.opencode/skills/engflow-trace-review/SKILL.md`
  - `.opencode/skills/engflow-verification-gate/SKILL.md`
- Tools:
  - `.opencode/tools/README.md` (native CLI mappings)

## Spec Kit integration

See `integrations/speckit/` for:

- preset contract: `preset/engflow.preset.yml`
- hook mapping: `hooks/README.md`

How engflow extends Spec Kit:

- `engflow init` runs `specify init` by default.
- engflow writes `.specify/templates/overrides/spec-template.md`.
- The override enforces `REQ-*` IDs and replaces default Spec Kit `FR-*` requirement ID usage in this flow.
- Canonical requirements are authored in `requirements.yml` (single source of truth).

This is currently an integration package/extension, not a released upstream Spec Kit plugin.

## One-command gate

Run the local gate:

```bash
engflow gate --feature <feature-id> --config .engflow/config.yml
```

It runs `verify`, `drift`, and `status` in sequence.

Enforced behavior:
- `verify`/`gate` require engineering-model regeneration to be configured (`commands.regen` or `ENGMODEL_GENERATE_CMD`).
- `verify`/`gate` fail if `ARCHITECTURE.ai.json` is not present after regeneration.

GitHub Actions gate is defined in:

- `.github/workflows/engflow-gate.yml`

## New Project Scaffold

Native command (recommended):

```bash
engflow init --project-dir /path/to/new-project --feature initial-feature --regen-cmd "make engmodel-generate"
```

This creates:

- `spec.md`
- `requirements.yml`
- `design.yml`
- `architecture.yml`
- `.engflow/config.yml`
- `AGENTS.md` (engflow default instructions section)
- `.specify/extensions.yml` (engflow-enforced hook wiring template)
- `.specify/templates/overrides/spec-template.md` (REQ-* single-source override)
- `.github/workflows/engflow-gate.yml`
- `.opencode/command/engflow.{status,verify,drift,gate,trace-query}.md`
- `.opencode/command/engmod.{generate,req-text,paths}.md`

Note:
- `ARCHITECTURE.ai.json` is treated as generated output and is not scaffolded.

Default behavior includes Spec Kit init and requires `specify` installed.
If you need to scaffold without Spec Kit:

```bash
engflow init --project-dir /path/to/new-project --feature initial-feature --no-speckit-init
```

Init also runs output generation by default and expects a real engineering generation command.
Command resolution order:

1. `--regen-cmd`
2. `ENGMODEL_GENERATE_CMD` environment variable
3. `.engflow/config.yml` -> `commands.regen`

If you want scaffold-only setup without running generation:

```bash
engflow init --project-dir /path/to/new-project --feature initial-feature --no-generate-outputs
```

## engineering-model-go integration

See `integrations/engineering-model-go/` for adapter contract and rollout notes.
