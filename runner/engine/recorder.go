package engine

import (
	"context"

	"bops/runner/scheduler"
	"bops/runner/workflow"
)

type Recorder interface {
	StepStart(step workflow.Step, targets []workflow.HostSpec)
	StepFinish(step workflow.Step, status string)
	HostResult(step workflow.Step, host workflow.HostSpec, result scheduler.Result)
}

type recorderKey struct{}

func WithRecorder(ctx context.Context, recorder Recorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, recorderKey{}, recorder)
}

func recorderFromContext(ctx context.Context) Recorder {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(recorderKey{}).(Recorder)
	return recorder
}
