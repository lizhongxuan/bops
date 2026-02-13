## Why

The current agent loop can terminate as soon as the model emits a `final` action, even when objective completion criteria are not met. For long-running automation tasks, this leads to premature success claims, repeated regressions, and weak reproducibility when sessions restart.

## What Changes

- Add a Ralph-style stop-hook gate in the loop runtime that intercepts model exit attempts and only allows completion when objective criteria pass.
- Add configurable completion contract fields (for example: completion token, objective checks, max loop iterations, no-progress cutoff, timeout/cost guards).
- Add persistent loop memory for file-backed progress and checklist state (for example `progress.txt`, `prd.json`, and checkpoint snapshots) so context is reloaded from environment state rather than long prompt history.
- Add a dedicated Ralph loop mode/profile in AI API request handling so simple tasks can still use the current single-pass/standard loop path.
- Add loop observability and a post-change impact report pipeline with before/after metrics (completion rate, false-complete rate, handoff rate, iteration cost, and latency).

## Capabilities

### New Capabilities
- `agent-ralph-stop-hook-gate`: Defines stop-hook interception behavior, completion contract validation, and loop continuation semantics when completion criteria fail.
- `agent-loop-persistent-memory`: Defines persistent memory artifacts and reload rules for multi-iteration/multi-session progress continuity.
- `agent-loop-effectiveness-reporting`: Defines required evaluation metrics, experiment protocol, and reporting output for measuring improvement after Ralph mode rollout.

### Modified Capabilities
- None.

## Impact

- Affected code:
  - `internal/aiworkflow/agent_loop.go`
  - `internal/aiworkflow/types.go`
  - `internal/aiworkflow/format.go`
  - `internal/aiworkflow/checkpoint_store.go`
  - `internal/server/ai.go`
  - related tests under `internal/aiworkflow/*_test.go` and `internal/server/*`
- APIs:
  - Extend AI stream request/options with stop-hook and completion-check configuration fields.
  - Add mode/profile routing to explicitly enable Ralph loop behavior.
- Dependencies/systems:
  - File-backed state storage for loop memory/checkpoints.
  - Test runner and git/file inspection tools become first-class objective signal sources.
- Risk/operational notes:
  - Increased token/tool usage on complex tasks; must be bounded by strict guardrails.
  - Need anti-loop controls to avoid over-iteration on simple tasks.
