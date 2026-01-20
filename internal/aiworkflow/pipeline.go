package aiworkflow

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/compose"
)

type Pipeline struct {
	cfg            Config
	generateRunner compose.Runnable[*State, *State]
	fixRunner      compose.Runnable[*State, *State]
}

func New(cfg Config) (*Pipeline, error) {
	if len(cfg.RiskRules) == 0 {
		cfg.RiskRules = DefaultRiskRules()
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	p := &Pipeline{cfg: cfg}
	gen, err := p.buildGenerateGraph()
	if err != nil {
		return nil, err
	}
	fix, err := p.buildFixGraph()
	if err != nil {
		return nil, err
	}
	p.generateRunner = gen
	p.fixRunner = fix
	return p, nil
}

func (p *Pipeline) RunGenerate(ctx context.Context, prompt string, context map[string]any, opts RunOptions) (*State, error) {
	if p.generateRunner == nil {
		return nil, errors.New("generate pipeline is not initialized")
	}
	state := &State{
		Mode:          ModeGenerate,
		Prompt:        prompt,
		Context:       context,
		ContextText:   opts.ContextText,
		SystemPrompt:  pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt),
		MaxRetries:    pickMaxRetries(opts.MaxRetries, p.cfg.MaxRetries),
		ValidationEnv: opts.ValidationEnv,
		SkipExecute:   opts.SkipExecute,
		EventSink:     opts.EventSink,
	}
	return p.generateRunner.Invoke(ctx, state)
}

func (p *Pipeline) RunFix(ctx context.Context, yaml string, issues []string, opts RunOptions) (*State, error) {
	if p.fixRunner == nil {
		return nil, errors.New("fix pipeline is not initialized")
	}
	state := &State{
		Mode:          ModeFix,
		YAML:          yaml,
		Issues:        issues,
		ContextText:   opts.ContextText,
		SystemPrompt:  pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt),
		MaxRetries:    pickMaxRetries(opts.MaxRetries, p.cfg.MaxRetries),
		ValidationEnv: opts.ValidationEnv,
		SkipExecute:   opts.SkipExecute,
		EventSink:     opts.EventSink,
	}
	return p.fixRunner.Invoke(ctx, state)
}

func pickSystemPrompt(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func pickMaxRetries(primary, fallback int) int {
	if primary > 0 {
		return primary
	}
	return fallback
}

func (p *Pipeline) buildGenerateGraph() (compose.Runnable[*State, *State], error) {
	graph := compose.NewGraph[*State, *State]()
	if err := graph.AddLambdaNode("normalize", compose.InvokableLambda(p.inputNormalize)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("generator", compose.InvokableLambda(p.generate)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("validator", compose.InvokableLambda(p.validate)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("safety", compose.InvokableLambda(p.safetyCheck)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("executor", compose.InvokableLambda(p.execute)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("fixer", compose.InvokableLambda(p.fix)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("summarizer", compose.InvokableLambda(p.summarize)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("human_gate", compose.InvokableLambda(p.humanGate)); err != nil {
		return nil, err
	}

	if err := graph.AddEdge(compose.START, "normalize"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("normalize", "generator"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("generator", "validator"); err != nil {
		return nil, err
	}

	validatorBranch := compose.NewGraphBranch(func(ctx context.Context, state *State) (string, error) {
		if len(state.Issues) > 0 {
			if state.RetryCount >= state.MaxRetries {
				return "summarizer", nil
			}
			return "fixer", nil
		}
		return "safety", nil
	}, map[string]bool{
		"fixer":      true,
		"safety":     true,
		"summarizer": true,
	})
	if err := graph.AddBranch("validator", validatorBranch); err != nil {
		return nil, err
	}

	if err := graph.AddEdge("fixer", "validator"); err != nil {
		return nil, err
	}

	safetyBranch := compose.NewGraphBranch(func(ctx context.Context, state *State) (string, error) {
		if state.SkipExecute || state.ValidationEnv == nil {
			state.ExecutionSkipped = true
			return "summarizer", nil
		}
		return "executor", nil
	}, map[string]bool{
		"executor":   true,
		"summarizer": true,
	})
	if err := graph.AddBranch("safety", safetyBranch); err != nil {
		return nil, err
	}

	executorBranch := compose.NewGraphBranch(func(ctx context.Context, state *State) (string, error) {
		if state.ExecutionSkipped || state.IsSuccess {
			return "summarizer", nil
		}
		if state.RetryCount >= state.MaxRetries {
			return "summarizer", nil
		}
		return "fixer", nil
	}, map[string]bool{
		"fixer":      true,
		"summarizer": true,
	})
	if err := graph.AddBranch("executor", executorBranch); err != nil {
		return nil, err
	}

	if err := graph.AddEdge("summarizer", "human_gate"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("human_gate", compose.END); err != nil {
		return nil, err
	}

	return graph.Compile(context.Background())
}

func (p *Pipeline) buildFixGraph() (compose.Runnable[*State, *State], error) {
	graph := compose.NewGraph[*State, *State]()
	if err := graph.AddLambdaNode("normalize", compose.InvokableLambda(p.inputNormalize)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("fixer", compose.InvokableLambda(p.fix)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("validator", compose.InvokableLambda(p.validate)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("safety", compose.InvokableLambda(p.safetyCheck)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("executor", compose.InvokableLambda(p.execute)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("summarizer", compose.InvokableLambda(p.summarize)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("human_gate", compose.InvokableLambda(p.humanGate)); err != nil {
		return nil, err
	}

	if err := graph.AddEdge(compose.START, "normalize"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("normalize", "fixer"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("fixer", "validator"); err != nil {
		return nil, err
	}

	validatorBranch := compose.NewGraphBranch(func(ctx context.Context, state *State) (string, error) {
		if len(state.Issues) > 0 {
			if state.RetryCount >= state.MaxRetries {
				return "summarizer", nil
			}
			return "fixer", nil
		}
		return "safety", nil
	}, map[string]bool{
		"fixer":      true,
		"safety":     true,
		"summarizer": true,
	})
	if err := graph.AddBranch("validator", validatorBranch); err != nil {
		return nil, err
	}

	safetyBranch := compose.NewGraphBranch(func(ctx context.Context, state *State) (string, error) {
		if state.SkipExecute || state.ValidationEnv == nil {
			state.ExecutionSkipped = true
			return "summarizer", nil
		}
		return "executor", nil
	}, map[string]bool{
		"executor":   true,
		"summarizer": true,
	})
	if err := graph.AddBranch("safety", safetyBranch); err != nil {
		return nil, err
	}

	if err := graph.AddEdge("executor", "summarizer"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("summarizer", "human_gate"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("human_gate", compose.END); err != nil {
		return nil, err
	}

	return graph.Compile(context.Background())
}
