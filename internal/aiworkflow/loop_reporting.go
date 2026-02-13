package aiworkflow

import (
	"fmt"
	"sort"
	"strings"
)

type LoopSessionTelemetry struct {
	SessionID      string                `json:"session_id"`
	TaskClass      string                `json:"task_class,omitempty"`
	ModeProfile    string                `json:"mode_profile,omitempty"`
	TerminalReason LoopTerminationReason `json:"terminal_reason"`
	Completed      bool                  `json:"completed"`
	FalseComplete  bool                  `json:"false_complete"`
	Handoff        bool                  `json:"handoff"`
	Iterations     int                   `json:"iterations"`
	LatencyMs      int64                 `json:"latency_ms"`
	ToolCalls      int                   `json:"tool_calls"`
}

type LoopEffectivenessReportInput struct {
	BaselineWindow  string                 `json:"baseline_window"`
	TreatmentWindow string                 `json:"treatment_window"`
	Cohort          string                 `json:"cohort,omitempty"`
	Baseline        []LoopSessionTelemetry `json:"baseline"`
	Treatment       []LoopSessionTelemetry `json:"treatment"`
	ReleaseGate     LoopReleaseGate        `json:"release_gate"`
}

type LoopReleaseGate struct {
	MinCompletionRateDelta float64 `json:"min_completion_rate_delta"`
	MaxFalseCompleteRate   float64 `json:"max_false_complete_rate"`
	MaxToolCallIncrease    float64 `json:"max_tool_call_increase"`
}

type LoopEffectivenessReport struct {
	BaselineWindow  string                         `json:"baseline_window"`
	TreatmentWindow string                         `json:"treatment_window"`
	Cohort          string                         `json:"cohort,omitempty"`
	Summary         LoopMetricSummary              `json:"summary"`
	PerClass        map[string]LoopMetricSummary   `json:"per_class"`
	Termination     map[string]LoopReasonBreakdown `json:"termination"`
	ReleaseDecision LoopReleaseDecision            `json:"release_decision"`
}

type LoopMetricSummary struct {
	Samples           int     `json:"samples"`
	CompletionRate    float64 `json:"completion_rate"`
	FalseCompleteRate float64 `json:"false_complete_rate"`
	HandoffRate       float64 `json:"handoff_rate"`
	MedianIterations  float64 `json:"median_iterations"`
	P95Iterations     float64 `json:"p95_iterations"`
	AverageLatencyMs  float64 `json:"average_latency_ms"`
	AverageToolCalls  float64 `json:"average_tool_calls"`
}

type LoopReasonBreakdown struct {
	Samples        int     `json:"samples"`
	CompletionRate float64 `json:"completion_rate"`
}

type LoopReleaseDecision struct {
	Pass     bool                    `json:"pass"`
	Criteria []LoopReleaseGateResult `json:"criteria"`
}

type LoopReleaseGateResult struct {
	Name   string  `json:"name"`
	Pass   bool    `json:"pass"`
	Value  float64 `json:"value"`
	Target float64 `json:"target"`
}

func TelemetryFromLoopMetrics(metrics *LoopMetrics, taskClass string) LoopSessionTelemetry {
	if metrics == nil {
		return LoopSessionTelemetry{TaskClass: normalizeTaskClass(taskClass)}
	}
	return LoopSessionTelemetry{
		SessionID:      metrics.SessionID,
		TaskClass:      normalizeTaskClass(taskClass),
		ModeProfile:    strings.TrimSpace(metrics.ModeProfile),
		TerminalReason: metrics.Terminal,
		Completed:      metrics.Terminal == LoopTerminationCompleted,
		Iterations:     metrics.Iterations,
		LatencyMs:      metrics.DurationMs,
		ToolCalls:      metrics.ToolCalls,
	}
}

func GenerateLoopEffectivenessReport(input LoopEffectivenessReportInput) (LoopEffectivenessReport, error) {
	if len(input.Baseline) == 0 {
		return LoopEffectivenessReport{}, fmt.Errorf("baseline data is required")
	}
	if len(input.Treatment) == 0 {
		return LoopEffectivenessReport{}, fmt.Errorf("treatment data is required")
	}
	if strings.TrimSpace(input.BaselineWindow) == "" || strings.TrimSpace(input.TreatmentWindow) == "" {
		return LoopEffectivenessReport{}, fmt.Errorf("baseline/treatment windows are required")
	}

	baselineSummary := summarizeTelemetry(input.Baseline)
	treatmentSummary := summarizeTelemetry(input.Treatment)
	perClass := summarizeByClass(input.Baseline, input.Treatment)
	termination := summarizeByTermination(input.Treatment)
	decision := evaluateReleaseGate(input.ReleaseGate, baselineSummary, treatmentSummary)

	return LoopEffectivenessReport{
		BaselineWindow:  input.BaselineWindow,
		TreatmentWindow: input.TreatmentWindow,
		Cohort:          strings.TrimSpace(input.Cohort),
		Summary: LoopMetricSummary{
			Samples:           treatmentSummary.Samples,
			CompletionRate:    treatmentSummary.CompletionRate - baselineSummary.CompletionRate,
			FalseCompleteRate: treatmentSummary.FalseCompleteRate - baselineSummary.FalseCompleteRate,
			HandoffRate:       treatmentSummary.HandoffRate - baselineSummary.HandoffRate,
			MedianIterations:  treatmentSummary.MedianIterations,
			P95Iterations:     treatmentSummary.P95Iterations,
			AverageLatencyMs:  treatmentSummary.AverageLatencyMs,
			AverageToolCalls:  treatmentSummary.AverageToolCalls,
		},
		PerClass:        perClass,
		Termination:     termination,
		ReleaseDecision: decision,
	}, nil
}

func summarizeByClass(baseline, treatment []LoopSessionTelemetry) map[string]LoopMetricSummary {
	result := map[string]LoopMetricSummary{}
	classes := map[string]struct{}{}
	for _, item := range baseline {
		key := normalizeTaskClass(item.TaskClass)
		classes[key] = struct{}{}
	}
	for _, item := range treatment {
		key := normalizeTaskClass(item.TaskClass)
		classes[key] = struct{}{}
	}
	for class := range classes {
		baseItems := filterByClass(baseline, class)
		trItems := filterByClass(treatment, class)
		if len(trItems) == 0 {
			continue
		}
		baseSummary := summarizeTelemetry(baseItems)
		trSummary := summarizeTelemetry(trItems)
		result[class] = LoopMetricSummary{
			Samples:           trSummary.Samples,
			CompletionRate:    trSummary.CompletionRate - baseSummary.CompletionRate,
			FalseCompleteRate: trSummary.FalseCompleteRate - baseSummary.FalseCompleteRate,
			HandoffRate:       trSummary.HandoffRate - baseSummary.HandoffRate,
			MedianIterations:  trSummary.MedianIterations,
			P95Iterations:     trSummary.P95Iterations,
			AverageLatencyMs:  trSummary.AverageLatencyMs,
			AverageToolCalls:  trSummary.AverageToolCalls,
		}
	}
	return result
}

func summarizeByTermination(items []LoopSessionTelemetry) map[string]LoopReasonBreakdown {
	result := map[string]LoopReasonBreakdown{}
	grouped := map[string][]LoopSessionTelemetry{}
	for _, item := range items {
		key := string(item.TerminalReason)
		if key == "" {
			key = string(LoopTerminationError)
		}
		grouped[key] = append(grouped[key], item)
	}
	for reason, part := range grouped {
		summary := summarizeTelemetry(part)
		result[reason] = LoopReasonBreakdown{
			Samples:        summary.Samples,
			CompletionRate: summary.CompletionRate,
		}
	}
	return result
}

func summarizeTelemetry(items []LoopSessionTelemetry) LoopMetricSummary {
	if len(items) == 0 {
		return LoopMetricSummary{}
	}
	completed := 0
	falseComplete := 0
	handoffs := 0
	latencySum := int64(0)
	toolCallsSum := 0
	iters := make([]int, 0, len(items))
	for _, item := range items {
		if item.Completed {
			completed++
		}
		if item.FalseComplete {
			falseComplete++
		}
		if item.Handoff {
			handoffs++
		}
		latencySum += item.LatencyMs
		toolCallsSum += item.ToolCalls
		iters = append(iters, item.Iterations)
	}
	return LoopMetricSummary{
		Samples:           len(items),
		CompletionRate:    ratio(completed, len(items)),
		FalseCompleteRate: ratio(falseComplete, len(items)),
		HandoffRate:       ratio(handoffs, len(items)),
		MedianIterations:  percentileInt(iters, 50),
		P95Iterations:     percentileInt(iters, 95),
		AverageLatencyMs:  float64(latencySum) / float64(len(items)),
		AverageToolCalls:  float64(toolCallsSum) / float64(len(items)),
	}
}

func evaluateReleaseGate(gate LoopReleaseGate, baseline, treatment LoopMetricSummary) LoopReleaseDecision {
	results := make([]LoopReleaseGateResult, 0, 3)

	completionDelta := treatment.CompletionRate - baseline.CompletionRate
	results = append(results, LoopReleaseGateResult{
		Name:   "completion_rate_delta",
		Pass:   completionDelta >= gate.MinCompletionRateDelta,
		Value:  completionDelta,
		Target: gate.MinCompletionRateDelta,
	})

	results = append(results, LoopReleaseGateResult{
		Name:   "false_complete_rate",
		Pass:   treatment.FalseCompleteRate <= gate.MaxFalseCompleteRate,
		Value:  treatment.FalseCompleteRate,
		Target: gate.MaxFalseCompleteRate,
	})

	toolCallIncrease := 0.0
	if baseline.AverageToolCalls > 0 {
		toolCallIncrease = (treatment.AverageToolCalls - baseline.AverageToolCalls) / baseline.AverageToolCalls
	}
	results = append(results, LoopReleaseGateResult{
		Name:   "tool_call_increase",
		Pass:   toolCallIncrease <= gate.MaxToolCallIncrease,
		Value:  toolCallIncrease,
		Target: gate.MaxToolCallIncrease,
	})

	pass := true
	for _, item := range results {
		if !item.Pass {
			pass = false
			break
		}
	}
	return LoopReleaseDecision{
		Pass:     pass,
		Criteria: results,
	}
}

func filterByClass(items []LoopSessionTelemetry, class string) []LoopSessionTelemetry {
	filtered := make([]LoopSessionTelemetry, 0, len(items))
	for _, item := range items {
		if normalizeTaskClass(item.TaskClass) == class {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func normalizeTaskClass(class string) string {
	trimmed := strings.ToLower(strings.TrimSpace(class))
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func percentileInt(values []int, p int) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		p = 0
	}
	if p >= 100 {
		p = 100
	}
	cp := append([]int{}, values...)
	sort.Ints(cp)
	if len(cp) == 1 {
		return float64(cp[0])
	}
	rank := int(float64(p) / 100 * float64(len(cp)-1))
	return float64(cp[rank])
}
