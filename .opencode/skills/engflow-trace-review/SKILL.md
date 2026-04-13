# engflow-trace-review

Use this skill to audit traceability coverage for a feature.

## Inputs

- `feature-id`
- optional `spec-path` (default `spec.md`)
- optional `requirements-path` (default `requirements.yml`)

## Steps

1. Run drift check:
   `go run ./cmd/engflow drift --feature <feature-id> --spec <spec-path> --requirements <requirements-path>`
2. Read `.engflow/reports/drift.md`.
3. Group findings by severity.
4. For each blocking/warning issue, provide an explicit remediation action.

## Expected output

- blocking issues first
- warnings next
- concrete next command to run

