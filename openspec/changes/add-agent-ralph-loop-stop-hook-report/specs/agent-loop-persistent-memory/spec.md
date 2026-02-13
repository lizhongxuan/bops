## ADDED Requirements

### Requirement: Loop memory SHALL persist across iterations and process restarts
The loop runtime MUST persist session memory artifacts to durable storage and MUST reload them for subsequent iterations and resume operations.

#### Scenario: Next iteration reloads persisted state
- **WHEN** iteration N+1 starts for an existing session
- **THEN** the runtime MUST load persisted memory artifacts created in prior iterations

#### Scenario: Resume after process restart
- **WHEN** a loop session is resumed after process restart with a valid resume identifier
- **THEN** the runtime MUST reconstruct loop state from persisted memory artifacts

### Requirement: Session memory schema MUST include checklist and progress artifacts
The persistent memory contract MUST include structured checklist state and append-only progress logs for each loop session.

#### Scenario: Required artifacts are initialized for a new session
- **WHEN** a Ralph-mode loop session starts without existing memory files
- **THEN** the runtime MUST initialize session memory including `prd.json` and `progress.txt`

#### Scenario: Checklist and progress updates are persisted
- **WHEN** a loop iteration updates completion checklist or learnings
- **THEN** the runtime MUST persist updates to the corresponding session artifacts

### Requirement: Persistent memory writes MUST be crash-safe
Memory persistence operations MUST use atomic write semantics to avoid partially written artifacts becoming active state.

#### Scenario: Write interrupted during artifact update
- **WHEN** a persistence operation is interrupted before completion
- **THEN** the runtime MUST keep the previous valid artifact version readable on next load

#### Scenario: Corrupted artifact is detected on load
- **WHEN** an artifact fails schema or format validation during load
- **THEN** the runtime MUST report a load error with artifact context and MUST NOT treat corrupted content as valid state

### Requirement: Runtime SHALL provide fallback memory behavior with explicit warning
When durable memory backend is unavailable, the runtime SHALL allow fallback to in-memory mode and SHALL surface non-durable warning signals.

#### Scenario: Durable memory backend not configured
- **WHEN** Ralph mode starts without durable memory backend configuration
- **THEN** the runtime MUST run with fallback memory and MUST emit a non-durable warning

#### Scenario: Durable backend recovers in later runs
- **WHEN** durable memory backend is configured in a subsequent run
- **THEN** the runtime MUST use durable persistence for new session memory operations
