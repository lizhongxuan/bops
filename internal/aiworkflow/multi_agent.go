package aiworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"bops/internal/ai"
)

func (p *Pipeline) RunMultiAgent(ctx context.Context, prompt string, context map[string]any, specs []AgentSpec, opts RunOptions) (*State, error) {
	if len(specs) == 0 {
		return p.RunAgent(ctx, prompt, context, opts)
	}
	mainSpec := normalizeAgentSpec(specs[0])
	mainOpts := opts
	mainOpts.AgentSpec = mainSpec
	var (
		state *State
		err   error
	)
	runMain := func() error {
		state, err = p.RunAgent(ctx, prompt, context, mainOpts)
		return err
	}
	if lane := p.sessionLane; lane != nil && strings.TrimSpace(opts.SessionKey) != "" {
		err = lane.Do(ctx, strings.TrimSpace(opts.SessionKey), runMain)
	} else {
		err = runMain()
	}
	if err != nil {
		return state, err
	}
	if len(specs) == 1 {
		return state, nil
	}

	var reviewerSpec *AgentSpec
	var coderSpec *AgentSpec
	otherSpecs := make([]AgentSpec, 0)
	for _, spec := range specs[1:] {
		role := strings.ToLower(strings.TrimSpace(spec.Role))
		switch role {
		case "reviewer", "linter", "qa":
			next := spec
			reviewerSpec = &next
		case "coder", "writer", "yaml":
			next := spec
			coderSpec = &next
		default:
			otherSpecs = append(otherSpecs, spec)
		}
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		summaries []AgentSummary
	)

	for _, spec := range otherSpecs {
		subSpec := normalizeAgentSpec(spec)
		wg.Add(1)
		go func() {
			defer wg.Done()
			subOpts := opts
			subOpts.AgentSpec = subSpec
			var subState *State
			var subErr error
			runSub := func() error {
				subState, subErr = p.RunAgent(ctx, prompt, context, subOpts)
				return subErr
			}
			if lane := p.globalLane; lane != nil {
				_ = lane.Do(ctx, runSub)
			} else {
				_ = runSub()
			}
			if subErr != nil || subState == nil {
				return
			}
			mu.Lock()
			summaries = append(summaries, AgentSummary{
				AgentName: subSpec.Name,
				Summary:   subState.Summary,
			})
			mu.Unlock()
		}()
	}

	wg.Wait()

	if reviewerSpec != nil {
		var (
			issues    []string
			reviewErr error
		)
		runReview := func() error {
			issues, reviewErr = p.reviewYAML(ctx, state.YAML, *reviewerSpec, opts)
			return reviewErr
		}
		if lane := p.globalLane; lane != nil {
			_ = lane.Do(ctx, runReview)
		} else {
			_ = runReview()
		}
		if reviewErr != nil {
			state.History = append(state.History, fmt.Sprintf("reviewer error: %v", reviewErr))
		} else {
			state.Issues = issues
			if len(issues) > 0 {
				state.History = append(state.History, fmt.Sprintf("reviewer: %s", strings.Join(issues, "; ")))
			}
		}

		finalIssues := issues
		if reviewErr == nil && coderSpec != nil && len(issues) > 0 {
			for attempt := 0; attempt < 2; attempt++ {
				fixOpts := opts
				fixOpts.AgentSpec = *coderSpec
				var fixState *State
				var fixErr error
				runFix := func() error {
					fixState, fixErr = p.RunAgentFix(ctx, state.YAML, issues, fixOpts)
					return fixErr
				}
				if lane := p.globalLane; lane != nil {
					_ = lane.Do(ctx, runFix)
				} else {
					_ = runFix()
				}
				if fixErr != nil || fixState == nil || fixState.YAML == "" {
					state.History = append(state.History, "coder: fix failed")
					continue
				}
				state.YAML = fixState.YAML
				state.History = append(state.History, "coder: applied fixes")
				var reviewIssues []string
				var nextErr error
				runReviewAgain := func() error {
					reviewIssues, nextErr = p.reviewYAML(ctx, state.YAML, *reviewerSpec, opts)
					return nextErr
				}
				if lane := p.globalLane; lane != nil {
					_ = lane.Do(ctx, runReviewAgain)
				} else {
					_ = runReviewAgain()
				}
				if nextErr != nil {
					state.History = append(state.History, fmt.Sprintf("reviewer error: %v", nextErr))
					continue
				}
				finalIssues = reviewIssues
				state.Issues = reviewIssues
				if len(reviewIssues) == 0 {
					break
				}
				state.History = append(state.History, fmt.Sprintf("reviewer: %s", strings.Join(reviewIssues, "; ")))
				issues = reviewIssues
			}
		}

		summary := "review_failed"
		if reviewErr == nil {
			summary = fmt.Sprintf("issues=%d", len(finalIssues))
		}
		summaries = append(summaries, AgentSummary{
			AgentName: reviewerSpec.Name,
			Summary:   summary,
		})
	}

	if len(summaries) > 0 {
		state.SubagentSummaries = summaries
	}
	return state, nil
}

func (p *Pipeline) reviewYAML(ctx context.Context, yamlText string, spec AgentSpec, opts RunOptions) ([]string, error) {
	if p == nil || p.cfg.Client == nil {
		return nil, fmt.Errorf("ai client is not configured")
	}
	sink := wrapEventSinkWithAgent(opts.EventSink, normalizeAgentSpec(spec))
	if sink != nil {
		sink(Event{Node: "reviewer", Status: "start", Message: ""})
	}
	prompt := buildReviewPrompt(yamlText, nil)
	messages := []ai.Message{
		{Role: "system", Content: pickSystemPrompt(opts.SystemPrompt, p.cfg.SystemPrompt)},
		{Role: "user", Content: prompt},
	}
	reply, _, err := p.chatWithThought(ctx, messages, nil)
	if err != nil {
		if sink != nil {
			sink(Event{Node: "reviewer", Status: "error", Message: err.Error()})
		}
		return nil, err
	}
	issues, err := parseReviewJSON(reply)
	if sink != nil {
		sink(Event{Node: "reviewer", Status: "done", Message: fmt.Sprintf("issues=%d", len(issues))})
	}
	return issues, err
}

func parseReviewJSON(reply string) ([]string, error) {
	jsonText := extractJSONBlock(strings.TrimSpace(reply))
	if jsonText == "" {
		return nil, fmt.Errorf("review response is not json")
	}
	var payload struct {
		Issues []string `json:"issues"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, err
	}
	return normalizeQuestions(payload.Issues), nil
}
