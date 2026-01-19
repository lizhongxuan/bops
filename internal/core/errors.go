package core

import "fmt"

type ErrorCode string

const (
	ErrUnknown          ErrorCode = "unknown"
	ErrInvalidConfig    ErrorCode = "invalid_config"
	ErrWorkflowParse    ErrorCode = "workflow_parse"
	ErrWorkflowValidate ErrorCode = "workflow_validate"
	ErrModuleNotFound   ErrorCode = "module_not_found"
	ErrAgentUnavailable ErrorCode = "agent_unavailable"
	ErrPlanFailed       ErrorCode = "plan_failed"
	ErrApplyFailed      ErrorCode = "apply_failed"
)

type Error struct {
	Code ErrorCode
	Op   string
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil && e.Msg != "" {
		return fmt.Sprintf("%s: %s: %s: %v", e.Code, e.Op, e.Msg, e.Err)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Op, e.Err)
	}
	if e.Msg != "" {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Op, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Op)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Wrap(code ErrorCode, op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Op: op, Err: err}
}

func New(code ErrorCode, op, msg string) error {
	return &Error{Code: code, Op: op, Msg: msg}
}
