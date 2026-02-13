## ADDED Requirements

### Requirement: Runner SHALL expose a pluggable run state store interface
Runner MUST define a stable interface for run state persistence that can be implemented by integrating business applications, and Runner core SHALL depend on the interface instead of concrete database drivers.

#### Scenario: Business application injects a custom store
- **WHEN** Runner starts with a custom `RunStateStore` implementation
- **THEN** Runner MUST use the injected implementation for run state create/update/query operations

#### Scenario: No concrete storage dependency in Runner core
- **WHEN** Runner is built without application-side database adapters
- **THEN** Runner MUST still compile and run because persistence integration is bound only through the interface

### Requirement: Store operations MUST be keyed by run_id
The run state store interface SHALL support at least create, update, and get operations using `run_id` as the primary lookup key for one workflow execution.

#### Scenario: Create and fetch by run_id
- **WHEN** a new workflow run is created with `run_id`
- **THEN** a subsequent get by the same `run_id` MUST return the created run snapshot

#### Scenario: Update by run_id
- **WHEN** Runner emits step or host progress for an existing `run_id`
- **THEN** store update MUST apply to the same run record without creating a second run entry

### Requirement: Runner SHALL provide a default non-production store fallback
Runner MUST provide a default fallback store implementation for local development and testing, and SHALL log a clear warning when no external durable store is configured.

#### Scenario: Fallback store is used
- **WHEN** Runner starts without an injected external store
- **THEN** Runner MUST initialize the default fallback store and continue processing workflow runs

#### Scenario: Warning is emitted for non-durable mode
- **WHEN** Runner is operating on fallback-only storage
- **THEN** Runner MUST emit a warning that persistence durability across process lifecycle is not guaranteed
