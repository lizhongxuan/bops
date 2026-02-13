package aiworkflow

import "testing"

func TestGenerateLoopEffectivenessReport(t *testing.T) {
	input := LoopEffectivenessReportInput{
		BaselineWindow:  "2026-01-01..2026-01-07",
		TreatmentWindow: "2026-01-08..2026-01-14",
		Cohort:          "pg-backup",
		Baseline: []LoopSessionTelemetry{
			{SessionID: "b1", TaskClass: "backup", Completed: true, FalseComplete: true, Iterations: 4, LatencyMs: 1200, ToolCalls: 3, TerminalReason: LoopTerminationCompleted},
			{SessionID: "b2", TaskClass: "backup", Completed: false, Handoff: true, Iterations: 5, LatencyMs: 2000, ToolCalls: 4, TerminalReason: LoopTerminationMaxIters},
		},
		Treatment: []LoopSessionTelemetry{
			{SessionID: "t1", TaskClass: "backup", Completed: true, Iterations: 3, LatencyMs: 1000, ToolCalls: 3, TerminalReason: LoopTerminationCompleted},
			{SessionID: "t2", TaskClass: "backup", Completed: true, Iterations: 2, LatencyMs: 900, ToolCalls: 2, TerminalReason: LoopTerminationCompleted},
		},
		ReleaseGate: LoopReleaseGate{
			MinCompletionRateDelta: 0.1,
			MaxFalseCompleteRate:   0.2,
			MaxToolCallIncrease:    0.5,
		},
	}
	report, err := GenerateLoopEffectivenessReport(input)
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}
	if report.Summary.Samples != 2 {
		t.Fatalf("expected summary samples 2, got %d", report.Summary.Samples)
	}
	if report.ReleaseDecision.Pass != true {
		t.Fatalf("expected release decision pass")
	}
	if len(report.PerClass) == 0 {
		t.Fatalf("expected per-class metrics")
	}
	if len(report.Termination) == 0 {
		t.Fatalf("expected termination breakdown")
	}
}

func TestGenerateLoopEffectivenessReportMissingBaseline(t *testing.T) {
	_, err := GenerateLoopEffectivenessReport(LoopEffectivenessReportInput{
		BaselineWindow:  "2026-01-01..2026-01-07",
		TreatmentWindow: "2026-01-08..2026-01-14",
		Treatment: []LoopSessionTelemetry{
			{SessionID: "t1", Completed: true, TerminalReason: LoopTerminationCompleted},
		},
	})
	if err == nil {
		t.Fatalf("expected baseline missing error")
	}
}
