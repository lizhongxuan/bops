package approval

import (
	"context"
	"fmt"

	"bops/internal/planner"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionDeny    Decision = "deny"
)

type Request struct {
	Plan planner.Plan
	Mode string
}

type Approver interface {
	Approve(ctx context.Context, req Request) (Decision, error)
}

type AutoApprover struct{}

func (a AutoApprover) Approve(ctx context.Context, req Request) (Decision, error) {
	return DecisionApprove, nil
}

type ManualApprover struct {
	Prompt func(Request) (bool, error)
}

func (m ManualApprover) Approve(ctx context.Context, req Request) (Decision, error) {
	if m.Prompt == nil {
		return DecisionDeny, fmt.Errorf("manual approver has no prompt function")
	}
	ok, err := m.Prompt(req)
	if err != nil {
		return DecisionDeny, err
	}
	if ok {
		return DecisionApprove, nil
	}
	return DecisionDeny, nil
}
