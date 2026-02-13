## ADDED Requirements

### Requirement: Ralph stop hook SHALL intercept premature loop completion
The loop runtime SHALL evaluate completion conditions before accepting a model `final` action, and SHALL block completion when required conditions are not satisfied.

#### Scenario: Final action is intercepted when completion checks fail
- **WHEN** the model emits `final` and at least one configured completion check fails
- **THEN** the runtime MUST reject completion for that iteration and continue to a new iteration

#### Scenario: Completion succeeds only after all required checks pass
- **WHEN** the model emits `final` and all configured completion checks pass
- **THEN** the runtime MUST terminate the loop with completed status

### Requirement: Completion contract MUST support token and objective checks
The runtime MUST support a completion token contract and objective check set in the same loop request, and MUST evaluate objective checks as authoritative completion evidence.

#### Scenario: Token present but objective checks fail
- **WHEN** the completion token is present but one or more objective checks fail
- **THEN** the runtime MUST continue looping and MUST report failed check reasons

#### Scenario: Objective checks pass without token requirement
- **WHEN** no completion token is configured and all objective checks pass
- **THEN** the runtime MUST allow loop completion

### Requirement: Loop guardrails MUST produce deterministic termination reasons
The runtime MUST enforce configured guardrails and MUST report explicit terminal reasons for non-completion exits.

#### Scenario: Loop stops at max iterations
- **WHEN** the loop reaches configured maximum iterations before passing completion checks
- **THEN** the runtime MUST terminate with reason `max_iters`

#### Scenario: Loop stops on no-progress condition
- **WHEN** no-progress detection threshold is reached based on iteration fingerprints
- **THEN** the runtime MUST terminate with reason `no_progress`

### Requirement: Ralph mode SHALL be explicitly routed and isolated from default flow
Ralph stop-hook semantics SHALL only apply when Ralph mode/profile is enabled, and default non-Ralph execution behavior SHALL remain unchanged.

#### Scenario: Request does not enable Ralph mode
- **WHEN** a request is handled in non-Ralph mode
- **THEN** the runtime MUST keep existing completion behavior without stop-hook interception

#### Scenario: Request enables Ralph mode
- **WHEN** a request enables Ralph mode/profile
- **THEN** the runtime MUST apply stop-hook completion interception and Ralph guardrails
