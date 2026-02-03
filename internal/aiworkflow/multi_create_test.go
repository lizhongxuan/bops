package aiworkflow

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"bops/internal/ai"
)

type fakeChatClient struct {
	mu      sync.Mutex
	replies []string
	index   int
}

func (c *fakeChatClient) Chat(_ context.Context, _ []ai.Message) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.index >= len(c.replies) {
		return "", fmt.Errorf("no reply available")
	}
	reply := c.replies[c.index]
	c.index++
	return reply, nil
}

func TestRunMultiCreateEmitsStepPatchEvents(t *testing.T) {
	planReply := `{"plan":[{"step_name":"install nginx","description":"install packages","dependencies":[]},{"step_name":"start nginx","description":"ensure service","dependencies":["install nginx"]}],"missing":[]}`
	step1Reply := `{"step_name":"install nginx","action":"pkg.install","targets":["local"],"with":{"name":"nginx"},"summary":"install nginx"}`
	step2Reply := `{"step_name":"start nginx","action":"service.ensure","targets":["local"],"with":{"name":"nginx","state":"started"},"summary":"start nginx"}`

	client := &fakeChatClient{replies: []string{planReply, step1Reply, step2Reply}}
	pipeline, err := New(Config{Client: client})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	var (
		mu     sync.Mutex
		events []Event
	)
	sink := func(evt Event) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	}

	_, err = pipeline.RunMultiCreate(context.Background(), "install nginx", nil, RunOptions{
		EventSink:   sink,
		SkipExecute: true,
	})
	if err != nil {
		t.Fatalf("run multi create: %v", err)
	}

	planIndex := -1
	stepPatchCount := 0
	stepPatchIndexes := []int{}

	for i, evt := range events {
		switch evt.EventType {
		case "plan_ready":
			planIndex = i
		case "step_patch_created":
			stepPatchCount++
			stepPatchIndexes = append(stepPatchIndexes, i)
			if evt.Data == nil {
				t.Fatalf("step_patch_created event missing data")
			}
			if _, ok := evt.Data["step_patch"]; !ok {
				t.Fatalf("step_patch_created event missing step_patch payload")
			}
			if evt.ParentStepID == "" {
				t.Fatalf("step_patch_created event missing parent_step_id")
			}
		}
	}

	if planIndex == -1 {
		t.Fatalf("plan_ready event not found")
	}
	if stepPatchCount != 2 {
		t.Fatalf("expected 2 step_patch_created events, got %d", stepPatchCount)
	}
	for _, idx := range stepPatchIndexes {
		if idx < planIndex {
			t.Fatalf("step_patch_created emitted before plan_ready")
		}
	}
}

func TestRunMultiCreateQuestionsOnlyWhenAsked(t *testing.T) {
	planReply := `{"plan":[],"missing":["inventory"]}`
	stepReply := `{"step_name":"install nginx","action":"pkg.install","targets":["local"],"with":{"name":"nginx"},"summary":"install nginx"}`

	client := &fakeChatClient{replies: []string{planReply, stepReply}}
	pipeline, err := New(Config{Client: client})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	state, err := pipeline.RunMultiCreate(context.Background(), "在 web1/web2 上安装 nginx", nil, RunOptions{
		SkipExecute: true,
	})
	if err != nil {
		t.Fatalf("run multi create: %v", err)
	}
	if len(state.Questions) != 0 {
		t.Fatalf("expected no questions when not explicitly asked, got %d", len(state.Questions))
	}
}

func TestRunMultiCreateQuestionsWhenAsked(t *testing.T) {
	planReply := `{"plan":[],"missing":["inventory","config"]}`

	client := &fakeChatClient{replies: []string{planReply}}
	pipeline, err := New(Config{Client: client})
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}

	state, err := pipeline.RunMultiCreate(context.Background(), "不清楚需要什么信息，请问我需要补充哪些？", nil, RunOptions{
		SkipExecute: true,
	})
	if err != nil {
		t.Fatalf("run multi create: %v", err)
	}
	if len(state.Questions) == 0 {
		t.Fatalf("expected questions when explicitly asked")
	}
	if len(state.Plan) == 0 {
		t.Fatalf("expected fallback plan when questions requested")
	}
}
