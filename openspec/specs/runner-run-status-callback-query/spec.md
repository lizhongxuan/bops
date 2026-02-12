# runner-run-status-callback-query Specification

## Purpose
TBD - created by archiving change add-runner-run-state-persistence-interface. Update Purpose after archive.
## Requirements
### Requirement: Runner SHALL provide run status query by run_id
Runner MUST expose a query path that returns the latest run snapshot by `run_id`, including run status, timestamps, and step/host execution progress.

#### Scenario: Query existing run
- **WHEN** a client queries with an existing `run_id`
- **THEN** Runner MUST return the corresponding run snapshot with current lifecycle status

#### Scenario: Query unknown run
- **WHEN** a client queries with a non-existent `run_id`
- **THEN** Runner MUST return a not-found result instead of fabricating a run state

### Requirement: Runner SHALL emit standardized status callback payloads
Runner MUST publish a standardized callback payload for run state changes that includes `run_id`, `workflow_name`, `status`, `timestamp`, and error details when present.

#### Scenario: Callback on status transition
- **WHEN** run status changes to a new lifecycle state
- **THEN** Runner MUST emit one callback event containing the standardized fields

#### Scenario: Callback includes error context
- **WHEN** run status changes to `failed`
- **THEN** callback payload MUST include error information sufficient for downstream diagnosis

### Requirement: Callback delivery failures MUST NOT change run execution result
Runner SHALL treat callback delivery as side-effect behavior and MUST NOT flip a successful run to failed due to callback transport issues.

#### Scenario: Callback fails after successful run
- **WHEN** run execution reaches `success` but callback delivery fails
- **THEN** persisted run status MUST remain `success` and callback failure MUST be recorded separately

#### Scenario: Callback retry policy is applied
- **WHEN** callback transport fails with retryable error
- **THEN** Runner MUST apply configured retry policy without blocking state persistence updates

