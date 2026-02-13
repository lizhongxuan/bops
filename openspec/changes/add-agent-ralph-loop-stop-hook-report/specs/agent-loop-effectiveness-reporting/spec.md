## ADDED Requirements

### Requirement: Loop runtime SHALL emit structured effectiveness telemetry
The runtime MUST emit structured telemetry for each loop session sufficient to evaluate completion quality, cost, and termination behavior.

#### Scenario: Session completes with terminal status
- **WHEN** a loop session reaches any terminal status
- **THEN** telemetry MUST include session identifier, mode/profile, terminal reason, iteration count, and tool-call statistics

#### Scenario: Session emits completion outcome signals
- **WHEN** a loop session declares completion
- **THEN** telemetry MUST include objective check outcomes used to decide completion

### Requirement: System MUST generate before/after effectiveness reports
The reporting pipeline MUST produce reproducible before/after comparisons for Ralph mode rollout using a defined evaluation window and comparable task cohorts.

#### Scenario: Report generated for rollout window
- **WHEN** reporting is triggered for a configured baseline and treatment window
- **THEN** the report MUST include completion rate, false-complete rate, handoff rate, iteration distribution, and latency/cost metrics

#### Scenario: Missing baseline data
- **WHEN** required baseline data is unavailable for comparison
- **THEN** report generation MUST fail with an explicit data-completeness error

### Requirement: Reports SHALL provide task-class and termination breakdowns
The reporting output MUST include segmented analysis to support operational decisions and guardrail tuning.

#### Scenario: Task classes are present in report dataset
- **WHEN** report generation processes tasks from multiple task classes
- **THEN** output MUST include per-class metric breakdowns and sample counts

#### Scenario: Termination reason analysis
- **WHEN** loop sessions contain mixed terminal reasons
- **THEN** output MUST include termination reason distribution and associated quality metrics

### Requirement: Reporting contract MUST support release-gate decision criteria
The reporting output MUST include explicit pass/fail criteria fields for deciding whether Ralph mode can advance rollout stage.

#### Scenario: Metrics meet configured rollout threshold
- **WHEN** report metrics satisfy configured release-gate thresholds
- **THEN** report output MUST mark rollout recommendation as pass

#### Scenario: Metrics violate configured rollout threshold
- **WHEN** report metrics fail one or more release-gate thresholds
- **THEN** report output MUST mark rollout recommendation as fail and include failed criteria details
