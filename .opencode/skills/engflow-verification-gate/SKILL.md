# engflow-verification-gate

Use this skill as a pre-merge quality gate for a feature.

## Inputs

- `feature-id`
- optional `regen-cmd`
- optional `test-cmd`
- optional `spec-path` and `requirements-path`

## Steps

1. Run verify:
   `go run ./cmd/engflow verify --feature <feature-id> [--regen-cmd "..."] [--test-cmd "..."]`
2. Run drift:
   `go run ./cmd/engflow drift --feature <feature-id> --spec <spec-path> --requirements <requirements-path>`
3. Run status:
   `go run ./cmd/engflow status --feature <feature-id>`
4. Return a single gate decision:
   - pass: no blocking issues and verify pass
   - fail: otherwise

## Expected output

- gate decision
- blocking issues (if any)
- exact next command

