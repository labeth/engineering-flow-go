# /engflow.verify

Run verification checks and produce a verify report.

## Usage

`/engflow.verify <feature-id> [regen-cmd] [test-cmd]`

## Behavior

1. Resolve `feature-id` from first argument.
2. Optionally use second argument as `regen-cmd` and third as `test-cmd`.
3. Run `go run ./cmd/engflow verify --feature <feature-id> ...`.
4. Surface `.engflow/reports/verify.md` and return pass/fail.

