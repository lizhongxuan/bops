package aiworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bops/runner/logging"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

func (p *Pipeline) RunMultiCreate(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	return p.runMultiCreateADK(ctx, prompt, context, opts)
}

func (p *Pipeline) runMultiCreateADK(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	if p == nil {
		return nil, errors.New("pipeline is nil")
	}
	if p.cfg.Client == nil {
		return nil, errors.New("ai client is not configured")
	}
	systemPrompt := pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)
	state := &State{
		Mode:          ModeGenerate,
		Prompt:        prompt,
		Context:       context,
		ContextText:   opts.ContextText,
		SystemPrompt:  systemPrompt,
		BaseYAML:      opts.BaseYAML,
		MaxRetries:    pickMaxRetries(opts.MaxRetries, p.cfg.MaxRetries),
		ValidationEnv: opts.ValidationEnv,
		SkipExecute:   opts.SkipExecute,
		EventSink:     opts.EventSink,
		StreamSink:    opts.StreamSink,
	}
	draftID := strings.TrimSpace(opts.DraftID)
	if draftID == "" {
		draftID = strings.TrimSpace(opts.SessionKey)
	}
	if draftID == "" {
		draftID = fmt.Sprintf("draft-%d", time.Now().UnixNano())
	}
	draftStore := p.draftStore
	draftStore.GetOrCreate(draftID, opts.BaseYAML)

	logging.L().Info("multi-create adk start",
		zap.String("draft_id", draftID),
		zap.Int("prompt_len", len(prompt)),
	)

	planner := &bopsPlannerAgent{
		pipeline:     p,
		state:        state,
		systemPrompt: systemPrompt,
		store:        draftStore,
		draftID:      draftID,
	}

	modelAdapter := newADKModelAdapter(p.cfg.Client)
	stepTool := &stepPatchTool{
		pipeline: p,
		state:    state,
		store:    draftStore,
		draftID:  draftID,
		opts:     opts,
	}

	executor, err := planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: modelAdapter,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               []tool.BaseTool{stepTool},
				ExecuteSequentially: true,
			},
			ReturnDirectly:     map[string]bool{"step_patch": true},
			EmitInternalEvents: true,
		},
		GenInputFn: buildExecutorInputFn(state, systemPrompt, opts.ContextText),
	})
	if err != nil {
		return state, err
	}

	replanner := &bopsReplannerAgent{
		state:   state,
		store:   draftStore,
		draftID: draftID,
	}

	agent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: pickLoopMaxIters(opts.LoopMaxIters),
	})
	if err != nil {
		return state, err
	}

	checkpointID := strings.TrimSpace(opts.ResumeCheckpointID)
	if checkpointID == "" {
		checkpointID = fmt.Sprintf("checkpoint-%s", draftID)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: opts.StreamSink != nil,
		CheckPointStore: p.checkpoints,
	})

	runOpts := []adk.AgentRunOption{}
	runOpts = append(runOpts, adk.WithCheckPointID(checkpointID))
	if opts.StreamSink != nil {
		runOpts = append(runOpts, adk.WithChatModelOptions([]model.Option{withADKStreamSink(opts.StreamSink)}))
	}

	var iter *adk.AsyncIterator[*adk.AgentEvent]
	if strings.TrimSpace(opts.ResumeCheckpointID) != "" {
		iter, err = runner.Resume(ctx, checkpointID, runOpts...)
		if err != nil {
			return state, err
		}
	} else {
		iter = runner.Run(ctx, []adk.Message{aiMessageToSchema(prompt)}, runOpts...)
	}

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return state, event.Err
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			emitCustomEvent(state, "checkpoint", "paused", "workflow paused", map[string]any{
				"checkpoint_id": checkpointID,
			})
			return state, nil
		}
	}

	if snapshot := draftStore.Snapshot(draftID); snapshot.DraftID != "" {
		if yamlText, err := buildFinalYAML(snapshot); err == nil {
			state.YAML = yamlText
		}
	}
	return state, nil
}

func aiMessageToSchema(prompt string) adk.Message {
	return &schema.Message{Role: schema.User, Content: strings.TrimSpace(prompt)}
}

func pickLoopMaxIters(value int) int {
	if value > 0 {
		return value
	}
	return 10
}
