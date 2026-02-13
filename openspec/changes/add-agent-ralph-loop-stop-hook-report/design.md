## Context

BOPS already has an iterative loop runtime in `internal/aiworkflow/agent_loop.go` with tool calls and iteration caps. However, completion currently depends on the model emitting `final`, which can produce false completion when objective criteria are not met. At the same time, loop state is mostly ephemeral (in-memory checkpoints), which weakens continuity for long tasks and restart scenarios.

This design introduces Ralph-mode semantics on top of the existing loop/runtime model: a stop-hook gate that blocks premature exits, objective completion checks, persistent memory artifacts, and measurable effectiveness reporting.

Constraints:
- Must preserve existing non-Ralph behavior for simple requests.
- Must avoid runaway loops and uncontrolled cost growth.
- Must integrate with existing server entry (`internal/server/ai.go`) and existing tool executor plumbing.

Stakeholders:
- AI workflow/runtime maintainers (`internal/aiworkflow/*`)
- API/server maintainers (`internal/server/*`)
- Operators relying on long-running automation task reliability

## Goals / Non-Goals

**Goals:**
- Add a stop-hook gate that intercepts `final` and allows completion only when objective checks pass.
- Add configurable completion contract fields and loop guardrails.
- Add persistent loop memory artifacts (`prd.json`, `progress.txt`, checkpoint snapshots) so each round can reload state from disk.
- Add Ralph mode/profile routing so default/simple flow remains unchanged.
- Add objective before/after effectiveness reporting with reproducible metrics.

**Non-Goals:**
- Not replacing current non-loop generation/fix pipelines.
- Not introducing distributed coordination across multiple nodes in this change.
- Not mandating automatic git commits in every iteration (may be optional later).
- Not redefining workflow YAML domain model outside loop-control requirements.

## Decisions

### 1) Add explicit Stop Hook Gate before loop completion

Decision:
- Introduce completion evaluation before returning from `final` action in `runAgentLoop`.
- If checks fail, block exit and continue loop with failure reasons injected into next-round context.
- Keep `LoopMaxIters` as a hard upper bound and add explicit stop reason classification (`completed`, `max_iters`, `no_progress`, `budget_exceeded`, `context_canceled`).

Rationale:
- Makes completion objective and reproducible.
- Implements Ralph core behavior without re-architecting the whole loop.

Alternatives considered:
- Alternative A: External shell-only stop hook (outside Go runtime).
  - Rejected for now because BOPS loop is API-driven and needs first-class structured telemetry.
- Alternative B: Keep current `final` semantics and rely on stronger prompting.
  - Rejected because prompt-only control does not guarantee objective success.

### 2) Introduce completion contract model (token + objective checks)

Decision:
- Add loop completion contract fields in request/options:
  - `completion_token` (for explicit promise markers)
  - `completion_checks[]` (objective checks, e.g. `prd_all_pass`, `tests_green`, `no_high_risk`)
  - guardrails (`loop_max_iters`, `no_progress_limit`, `per_iter_timeout_ms`, `max_tool_calls`, optional cost budget)
- Completion requires configured checks to pass; token match alone is insufficient when objective checks are configured.

Rationale:
- Supports both lightweight and strict usage.
- Prevents false completion by enforcing objective evidence.

Alternatives considered:
- Alternative A: Token-only completion.
  - Rejected due to easy false positives.
- Alternative B: Objective checks only, no token contract.
  - Not chosen as default; token still provides human-readable explicit intent and debugging signal.

### 3) Add persistent loop memory artifacts and pluggable memory store

Decision:
- Introduce `LoopMemoryStore` abstraction with a file-backed default implementation.
- Persist/reload per-session artifacts under a deterministic path (by `session_key`/`draft_id`):
  - `prd.json` (structured checklist status)
  - `progress.txt` (append-only learnings/log)
  - checkpoint snapshot(s) for resumable loop state
- Keep in-memory implementation as fallback for compatibility/testing.

Rationale:
- Moves long-horizon state from model context to filesystem, reducing context rot.
- Enables restart continuity.

Alternatives considered:
- Alternative A: Keep memory only in prompt/tool history.
  - Rejected due to context growth and reliability decay.
- Alternative B: Force external DB-only persistence.
  - Deferred; file-backed local persistence is simpler and enough for first rollout.

### 4) Mode routing: Ralph profile opt-in, default path preserved

Decision:
- Add explicit Ralph mode/profile in API request handling (`internal/server/ai.go`).
- Default behavior remains existing flow; Ralph mode opt-in for tasks that need objective iterative closure.
- Add configurable policy to auto-route only when request includes strict completion checks.

Rationale:
- Avoids penalizing simple tasks with unnecessary looping.
- Reduces risk of regressions in existing usage.

Alternatives considered:
- Alternative A: Make Ralph mode default globally.
  - Rejected; high risk for latency/cost and simple-task UX.
- Alternative B: Separate endpoint/service for Ralph only.
  - Deferred; profile-based routing gives faster adoption with lower operational friction.

### 5) Anti-loop protections and no-progress detection

Decision:
- Add no-progress detection using iteration fingerprints (e.g., normalized YAML digest + key tool output digest + check result tuple).
- If unchanged for N consecutive iterations, terminate with `no_progress` and expose clear diagnostics.
- Enforce bounded retries for failing checks to avoid flapping.

Rationale:
- Directly addresses the risk of looping on simple or blocked tasks.

Alternatives considered:
- Alternative A: rely on max iterations only.
  - Rejected; wastes cycles and gives poor diagnostics.
- Alternative B: semantic similarity only (LLM judged progress).
  - Rejected for determinism concerns; prefer structural fingerprints first.

### 6) Effectiveness reporting as first-class output

Decision:
- Define reporting pipeline and required metrics, generated after rollout experiment windows.
- Report includes before/after and per-task-class breakdown:
  - completion rate
  - false-complete rate
  - handoff/escalation rate
  - median/95p iterations
  - latency and tool-call cost
  - termination reason distribution

Rationale:
- Ensures this change is evaluated by outcomes, not only architecture.

Alternatives considered:
- Alternative A: ad-hoc manual comparison.
  - Rejected due to weak reproducibility.

## Risks / Trade-offs

- [Risk] Increased latency and token/tool cost for complex loops.
  - Mitigation: opt-in mode, strict guardrails, default caps, and per-profile budgets.

- [Risk] False negatives from strict checks (loop never completes although work is acceptable).
  - Mitigation: profile-specific check sets, fallback threshold, and explicit operator override path.

- [Risk] Flaky tests causing oscillation.
  - Mitigation: flaky classification, bounded retries, and failure reason taxonomy.

- [Risk] File-based memory corruption or partial writes.
  - Mitigation: atomic write strategy (temp file + rename), schema validation on load, fallback snapshot.

- [Risk] Behavioral drift between Ralph and non-Ralph mode.
  - Mitigation: integration tests covering both modes, explicit mode in telemetry, and gradual rollout.

## Migration Plan

1. Add completion contract types and stop-hook evaluation interfaces in `aiworkflow`.
2. Wire stop-hook gate into `runAgentLoop` finalization path with structured termination reasons.
3. Add `LoopMemoryStore` abstraction and file-backed implementation; integrate load/save in each iteration boundary.
4. Extend server request/options for Ralph profile and completion contract fields.
5. Add no-progress detector and budget/time guardrails.
6. Add tests:
   - completion intercepted when checks fail
   - completion succeeds when checks pass
   - no-progress termination
   - restart/resume with persistent memory
   - compatibility for non-Ralph mode
7. Add observability fields and emit reporting dataset.
8. Run staged rollout (shadow/opt-in), generate impact report, then decide default policy.

Rollback strategy:
- Disable Ralph profile routing via config flag.
- Keep non-Ralph path intact; no data migration required for core workflow data.
- Retain memory files for debugging but ignore them when Ralph mode is off.

## Open Questions

- Which completion checks are required by default in Ralph mode (`tests_green` only vs. broader set)?
- Should automatic git commit be in scope now or postponed to a later change?
- What is the default memory directory strategy in production (workspace-local vs configurable root)?
- Should cost budget be mandatory for Ralph mode in production environments?
- What rollout threshold should trigger default-on decision (for example, minimum false-complete reduction with bounded cost increase)?
