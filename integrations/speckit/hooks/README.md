# Spec Kit Hook Mapping (No Shell Scripts)

Use native `engflow` CLI commands directly in Spec Kit hook configuration:

Pre-plan sequence:

- `engflow status --feature <feature-id>`

Post-implementation sequence:

- `engflow verify --feature <feature-id>`
- `engflow drift --feature <feature-id> --spec <spec-path> --requirements <requirements-path>`
- `engflow status --feature <feature-id>`
