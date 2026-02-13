package aiworkflow

import "strings"

type LoopCompletionEvaluation struct {
	Passed       bool
	CheckResults []CompletionCheckResult
	Failed       []string
}

func evaluateLoopCompletion(state *State, action loopAction, reply string, opts RunOptions, snapshot LoopMemorySnapshot) LoopCompletionEvaluation {
	results := make([]CompletionCheckResult, 0, len(opts.CompletionChecks)+1)
	failures := make([]string, 0, len(opts.CompletionChecks)+1)

	token := strings.TrimSpace(opts.CompletionToken)
	if token != "" {
		passed := loopTokenPresent(token, action, reply, state)
		result := CompletionCheckResult{
			Name:   "completion_token",
			Passed: passed,
		}
		if !passed {
			result.Reason = "completion token not found"
			failures = append(failures, result.Reason)
		}
		results = append(results, result)
	}

	for _, rawName := range opts.CompletionChecks {
		name := strings.ToLower(strings.TrimSpace(rawName))
		if name == "" {
			continue
		}
		result := CompletionCheckResult{Name: name}
		switch name {
		case "yaml_non_empty":
			result.Passed = strings.TrimSpace(state.YAML) != ""
			if !result.Passed {
				result.Reason = "yaml is empty"
			}
		case "has_steps":
			steps := strings.TrimSpace(stepsOnlyYAML(state.YAML))
			result.Passed = steps != "" && strings.Contains(steps, "- name:")
			if !result.Passed {
				result.Reason = "workflow steps are missing"
			}
		case "no_high_risk":
			result.Passed = strings.ToLower(strings.TrimSpace(string(state.RiskLevel))) != string(RiskLevelHigh)
			if !result.Passed {
				result.Reason = "risk level is high"
			}
		case "prd_all_pass":
			if len(snapshot.PRD.UserStories) == 0 {
				result.Passed = false
				result.Reason = "no user stories in prd"
				break
			}
			allPass := true
			for _, story := range snapshot.PRD.UserStories {
				if !story.Passes {
					allPass = false
					break
				}
			}
			result.Passed = allPass
			if !result.Passed {
				result.Reason = "not all PRD stories pass"
			}
		case "tests_green":
			result.Passed = hasPassingTestSignal(snapshot.Checkpoint.ToolHistory)
			if !result.Passed {
				result.Reason = "no green test signal found"
			}
		default:
			result.Passed = false
			result.Reason = "unknown completion check"
		}
		if !result.Passed {
			failures = append(failures, name+": "+result.Reason)
		}
		results = append(results, result)
	}

	if len(opts.CompletionChecks) > 0 {
		return LoopCompletionEvaluation{
			Passed:       len(failures) == 0,
			CheckResults: results,
			Failed:       failures,
		}
	}
	if token != "" {
		return LoopCompletionEvaluation{
			Passed:       len(failures) == 0,
			CheckResults: results,
			Failed:       failures,
		}
	}
	return LoopCompletionEvaluation{
		Passed:       true,
		CheckResults: results,
		Failed:       failures,
	}
}

func loopTokenPresent(token string, action loopAction, reply string, state *State) bool {
	target := strings.TrimSpace(token)
	if target == "" {
		return true
	}
	candidates := []string{
		action.Message,
		action.Result,
		reply,
	}
	if state != nil {
		candidates = append(candidates, state.YAML)
	}
	for _, c := range candidates {
		if strings.Contains(c, target) {
			return true
		}
	}
	return false
}

func hasPassingTestSignal(toolHistory []string) bool {
	if len(toolHistory) == 0 {
		return false
	}
	candidates := tailStrings(toolHistory, 4)
	joined := strings.ToLower(strings.Join(candidates, "\n"))
	if strings.Contains(joined, "fail") || strings.Contains(joined, "error") {
		return false
	}
	return strings.Contains(joined, "pass") || strings.Contains(joined, "ok")
}
