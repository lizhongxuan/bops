## ADDED Requirements

### Requirement: Runner SHALL assign a unique run_id for each execution
Runner MUST create one unique `run_id` per workflow execution and SHALL attach it to all run-level state records for the full lifecycle.

#### Scenario: New run gets run_id
- **WHEN** a workflow execution starts
- **THEN** Runner MUST generate or accept exactly one `run_id` for that execution

#### Scenario: run_id remains stable through lifecycle
- **WHEN** step and host status events are persisted during the same execution
- **THEN** every state update MUST reference the same `run_id`

### Requirement: Runner MUST enforce a valid run status lifecycle
Runner SHALL implement run status transitions that follow `queued -> running -> (success | failed | canceled | interrupted)` and MUST reject invalid backward transitions.

#### Scenario: Successful completion
- **WHEN** all workflow steps finish without unhandled errors
- **THEN** run status MUST transition from `running` to `success`

#### Scenario: Execution failure
- **WHEN** workflow execution exits with an unhandled step error
- **THEN** run status MUST transition from `running` to `failed`

#### Scenario: Invalid transition is rejected
- **WHEN** an update attempts to move a terminal run status back to `running`
- **THEN** Runner MUST reject the transition and keep the previous terminal status

### Requirement: Runner SHALL preserve run traceability across process restart
Runner MUST keep historical run records queryable after restart, and any run that was previously persisted as `running` but not finalized MUST be marked as `interrupted` with a reason.

#### Scenario: Historical run remains queryable
- **WHEN** Runner process restarts
- **THEN** previously persisted completed runs MUST still be retrievable by `run_id`

#### Scenario: In-progress run is marked interrupted on restart
- **WHEN** Runner detects a persisted run in `running` state during startup reconciliation
- **THEN** Runner MUST mark that run as `interrupted` and record an interruption reason
