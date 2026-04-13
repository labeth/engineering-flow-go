# Product Spec: engflow

## Summary

`engflow` provides a native Go workflow that combines Spec Kit authoring with
engineering-model-go artifacts and verification. It standardizes a single-source
artifact set:

- `spec.md`
- `requirements.yml`
- `design.yml`
- `architecture.yml`

and pairs it with status, drift, verification, and trace-query tooling.

## Problem

Teams using AI-assisted development often have specs, model files, code, and tests
but no consistent operational flow that keeps them aligned. This creates stale model
outputs, broken requirement links, and unclear next actions.

## Target users

### Primary

- developers using Spec Kit who also want typed engineering artifacts
- teams adopting engineering-model-go with strict `REQ-*` traceability
- maintainers adding repeatable quality gates to AI-native repos

### Secondary

- compliance-sensitive teams requiring evidence of requirement coverage
- platform engineers standardizing project initialization across repos

## Goals

1. Initialize projects with Spec Kit and engflow defaults in one step.
2. Keep one canonical requirements source (`requirements.yml`) with stable `REQ-*` IDs.
3. Make verification and drift checks routine and machine-runnable.
4. Provide clear next-action status for day-to-day development.
5. Work with engineering-model-go generation commands without replacing them.

## Non-goals

- replacing Spec Kit feature spec authoring
- replacing engineering-model-go schema/generation internals
- introducing parallel or duplicate artifact trees
- acting as a full project management suite

## User stories

### Story 1 - initialize a project with the intended stack
As a developer starting a new repo,
I want `engflow init` to run Spec Kit setup and scaffold engflow canonical files,
so that the project starts in the intended combined workflow.

Acceptance criteria:
- `engflow init` runs `specify init` by default.
- If `specify` is unavailable and init requires it, the command fails with a clear error.
- The command writes `.engflow/config.yml` and canonical input files.

### Story 2 - keep a single requirements source of truth
As a maintainer,
I want one requirements artifact with stable IDs,
so that code/tests/model outputs reference the same IDs consistently.

Acceptance criteria:
- Requirements live in `requirements.yml` and use `REQ-*` IDs.
- The Spec Kit override template enforces `REQ-*` IDs.
- No legacy requirements sidecar files are required for normal operation.

### Story 3 - run deterministic quality checks
As a reviewer,
I want a repeatable verify command and a gate command,
so that merges can be evaluated consistently.

Acceptance criteria:
- `engflow verify` regenerates artifacts via configured command and runs tests.
- `engflow gate` executes verify, drift, and status and returns non-zero on blocking issues.
- Reports are written under `.engflow/reports` and `.engflow/status`.

### Story 4 - detect and explain drift
As an engineer,
I want drift checks that classify problems by severity and suggest next steps,
so that misalignment is addressed early.

Acceptance criteria:
- `engflow drift` reports blocking/warning/informational findings.
- Drift includes missing canonical inputs, stale canonical artifacts, and unknown test IDs.
- Suggested actions are included for each issue type.

### Story 5 - query trace evidence quickly
As an engineer,
I want to query where a requirement ID appears,
so that I can inspect implementation and test evidence quickly.

Acceptance criteria:
- `engflow trace-query --id REQ-...` returns repository locations for that ID.
- Output is usable in local debugging and CI logs.

## Main interaction model

1. Initialize with `engflow init` (Spec Kit + engflow scaffold).
2. Author or update feature intent in `spec.md` (and Spec Kit artifacts).
3. Update canonical engineering inputs (`requirements.yml`, `design.yml`, `architecture.yml`).
4. Regenerate engineering-model-go outputs.
5. Run `engflow verify`, `engflow drift`, and `engflow status` (or `engflow gate`).
6. Use `engflow trace-query` when reviewing requirement coverage.

## Primary commands

- `engflow init`
- `engflow status`
- `engflow verify`
- `engflow drift`
- `engflow trace-query`
- `engflow gate`

## Quality bar

- Single canonical source for requirements and design inputs.
- Stable IDs are preserved and visible in code/test evidence.
- Verification and drift outputs are deterministic and reviewable.

## Success metrics

- fewer stale model outputs in reviews
- fewer unknown or missing `REQ-*` references
- faster identification of next required action
- lower setup time for new projects using Spec Kit + engflow
