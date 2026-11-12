---
schema_version: 1
open_count: 1
waived_count: 0
fixed_count: 0
total_count: 1
last_updated: 2026-11-12T11:59:43.588Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 03 | deviation | beamsync/server.go |  | progressWriter payload is '\|'-delimited; a filename containing '\|' makes the Go->Svelte payload ambiguous (recorded, not fixed) | open |  | 2026-11-12T11:59:43.588Z |  |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "03",
    "file": "beamsync/server.go",
    "line": null,
    "description": "progressWriter payload is '|'-delimited; a filename containing '|' makes the Go->Svelte payload ambiguous (recorded, not fixed)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-11-12T11:59:43.588Z",
    "resolved_at": null
  }
]
````
