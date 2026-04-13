# engineering-model-go Integration

Yes, `engflow` can be extended against `engineering-model-go` similarly to Spec Kit integration.

## Integration model

`engflow` should not replace canonical engineering-model-go schema/artifacts.  
It should call engineering-model-go commands and interpret their outputs.

## Recommended adapter responsibilities

1. Locate canonical engineering inputs (requirements/design/model files).
2. Run engineering-model-go generation command(s).
3. Read `ARCHITECTURE.ai.json` for impact derivation.
4. Compare regenerated outputs for deterministic verification.
5. Feed missing-link and drift signals back into `engflow` reports.

## Suggested command contract

Use env/config values so the adapter stays tool-version agnostic:

- `ENGMODEL_GENERATE_CMD`
- `ENGMODEL_ARCH_AI_PATH` (default `ARCHITECTURE.ai.json`)
- `ENGMODEL_REQUIREMENTS_PATH` (default `requirements.yml`)
- `ENGMODEL_DESIGN_PATH` (default `design.yml`)

## Incremental rollout

1. Start with verify-time generation checks.
2. Add impact extraction from `ARCHITECTURE.ai.json`.
3. Add stable-ID chain checks (`REQ/FU/RT/CODE/VER`) in drift reports.
4. Add richer graph queries via a dedicated adapter package.
