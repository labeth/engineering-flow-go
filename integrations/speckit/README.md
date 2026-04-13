# Spec Kit Integration

`engflow` is currently a standalone CLI, but this folder provides a practical integration pattern for Spec Kit.

## What this provides

- an `engflow` preset contract for planning/tasks requirements
- a pre-plan check that runs `status` on canonical artifacts
- a post-implementation hook that runs `verify`, `drift`, and `status`

## Files

- `preset/engflow.preset.yml`
- `hooks/README.md`

## Hook usage

Run native `engflow` commands from the repository root:

Pre-plan:

```bash
engflow status --feature <feature-id>
```

Post-implementation:

```bash
engflow verify --feature <feature-id>
engflow drift --feature <feature-id> --spec <spec-path> --requirements <requirements-path>
engflow status --feature <feature-id>
```

See `hooks/README.md` for the direct mapping.

## Intended flow

1. Spec Kit authoring produces/updates `spec.md`.
2. Canonical requirements/design/architecture are updated directly.
3. Implementation proceeds.
4. Post-implementation hook runs verification and drift gates.
