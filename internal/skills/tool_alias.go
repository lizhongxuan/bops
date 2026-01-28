package skills

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type AliasTool struct {
	name  string
	inner tool.InvokableTool
}

func NewAliasTool(name string, inner tool.InvokableTool) *AliasTool {
	return &AliasTool{name: name, inner: inner}
}

func (t *AliasTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	info, err := t.inner.Info(ctx)
	if err != nil {
		return nil, err
	}
	clone := *info
	clone.Name = t.name
	return &clone, nil
}

func (t *AliasTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}
