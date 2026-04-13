# OpenCode Tools (Native CLI)

Shell wrappers were removed. Use native `engflow` commands directly:

- `trace_query` -> `engflow trace-query --id <REQ-ID> [--repo-root .]`
- `drift_check` -> `engflow drift --feature <feature-id>`
- `workflow_gate` -> `engflow gate --feature <feature-id>`
