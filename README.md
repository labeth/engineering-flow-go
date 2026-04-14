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
  catalog: catalog.yml
  requirements: requirements.yml
  design: design.yml
  architecture: architecture.yml
  architecture_ai: ARCHITECTURE.ai.json
commands:
  regen: npm run -s arch:regen
  test: npm test
verify:
  watch: catalog.yml,requirements.yml,design.yml,architecture.yml,ARCHITECTURE.ai.json,ARCHITECTURE.adoc
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
- engflow enforces the REQ template in both:
  - `.specify/templates/spec-template.md`
  - `.specify/templates/overrides/spec-template.md`
- This replaces default Spec Kit `FR-*` requirement ID usage in this flow.
- Canonical engineering inputs are authored in `catalog.yml`, `requirements.yml`, `design.yml`, and `architecture.yml`.

This is currently an integration package/extension, not a released upstream Spec Kit plugin.

## One-command gate

Run the local gate:

```bash
engflow gate --feature <feature-id> --config .engflow/config.yml
```

It runs `verify`, `drift`, and `status` in sequence.
`verify` runs `commands.test` before regeneration, so engdoc consumes fresh executed test artifacts from `test-results/`.

Enforced behavior:
- `verify`/`gate` require engineering-model regeneration to be configured (`commands.regen` or `ENGMODEL_GENERATE_CMD`).
- `verify`/`gate` fail if `ARCHITECTURE.ai.json` is not present after regeneration.
- `init` writes `.engflow/state/scaffold-baseline.json` with canonical model hashes.
- `verify`/`gate` fail when implementation files changed since scaffold baseline but canonical model files (`catalog.yml`, `requirements.yml`, `design.yml`, `architecture.yml`) remain unchanged.
- Manual test-result documents are not allowed; use command output plus generated `.engflow/reports/verify.{md,json}` and `.engflow/reports/drift.{md,json}`.

## Architecture Root Of Trust

- `engdoc` (invoked via `.engflow/config.yml -> commands.regen`) is the only trusted producer of `ARCHITECTURE.ai.json`.
- `ARCHITECTURE.ai.json` is generated output and must not be edited by hand.
- After changes to `catalog.yml`, `requirements.yml`, `design.yml`, or `architecture.yml`, run regeneration before architecture-dependent planning/implementation.
- If regeneration fails, treat architecture context as untrusted until generation succeeds.
- For feature completion, run `/engmod.generate` and then `engflow gate`; do not consider implementation complete until both pass.

Required execution order for non-trivial feature delivery:
1. `/speckit.specify`
2. `/speckit.plan`
3. `/speckit.tasks`
4. Update canonical model files (`catalog.yml`, `requirements.yml`, `design.yml`, `architecture.yml`)
5. `/engmod.generate` (pre-implementation regeneration)
6. `/speckit.implement` (implementation stage)
7. `/engmod.generate` (final regeneration after implementation)
8. `engflow gate --config .engflow/config.yml --feature <feature-id>`

GitHub Actions gate is defined in:

- `.github/workflows/engflow-gate.yml`

## New Project Scaffold

Native command (recommended):

```bash
engflow init --project-dir /path/to/new-project --feature initial-feature
```

This creates:

- `README.md`
- `package.json`
- `.gitignore`
- `language-examples/{typescript,go,rust}/`
- `spec.md`
- `catalog.yml`
- `requirements.yml`
- `design.yml`
- `architecture.yml`
- `scripts/verify-requirements.js`
- `tests/*.test.js`
- `test-results/README.md`
- `infra/terraform/.gitkeep`
- `infra/flux/.gitkeep`
- `.engflow/config.yml`
- `AGENTS.md` (engflow default instructions section)
- `.specify/extensions.yml` (engflow-enforced hook wiring template)
- `.specify/templates/overrides/spec-template.md` (REQ-* single-source override)
- `.github/workflows/engflow-gate.yml`
- `.opencode/command/engflow.{status,verify,drift,gate,trace-query}.md`
- `.opencode/command/engmod.{generate,req-text,paths}.md`

By default, scaffold scripts include:

```bash
npm test
npm run arch:regen
npm run arch:pdf
npm run verify:all
```

`verify:all` is the canonical one-command local workflow:
1. run tests and emit `test-results/*.json`
2. regenerate `ARCHITECTURE.adoc` + `ARCHITECTURE.ai.json`
3. run `engflow verify` + `engflow drift` + `engflow status`
4. render `ARCHITECTURE.pdf`

Language examples:
- TypeScript is the active default scaffold.
- `language-examples/go` and `language-examples/rust` provide starter equivalents you can adopt.
- After selecting your stack, remove example folders you do not need.

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
