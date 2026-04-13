# /engflow.drift

Run drift checks and summarize blocking/warning findings.

## Usage

`/engflow.drift <feature-id> [spec-path] [requirements-path]`

## Behavior

1. Resolve `feature-id` from first argument.
2. Optional `spec-path` defaults to `spec.md`.
3. Optional `requirements-path` defaults to `requirements.yml`.
4. Run:
   `go run ./cmd/engflow drift --feature <feature-id> --spec <spec-path> --requirements <requirements-path>`
5. Surface `.engflow/reports/drift.md`.

