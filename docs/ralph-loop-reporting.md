# Ralph Loop 效果报告

本页说明如何使用 `internal/aiworkflow.GenerateLoopEffectivenessReport` 对比 Ralph 模式上线前后效果。

## 输入数据要求

- 必须提供 `baseline` 与 `treatment` 两个窗口数据集（为空会报错）
- 每条记录建议包含:
  - `session_id`
  - `task_class`
  - `terminal_reason`
  - `completed`
  - `false_complete`
  - `handoff`
  - `iterations`
  - `latency_ms`
  - `tool_calls`

## 核心指标

- completion rate
- false-complete rate
- handoff rate
- median/p95 iterations
- average latency / tool calls
- termination reason breakdown
- per task class breakdown

## Release Gate

报告内置 `release_decision`，默认按以下阈值判定 pass/fail:

- completion rate 增量是否达到阈值
- false-complete rate 是否低于阈值
- tool call 增幅是否在预算内

## 示例

```go
report, err := aiworkflow.GenerateLoopEffectivenessReport(aiworkflow.LoopEffectivenessReportInput{
  BaselineWindow:  "2026-01-01..2026-01-07",
  TreatmentWindow: "2026-01-08..2026-01-14",
  Cohort:          "pg-backup",
  Baseline:        baselineItems,
  Treatment:       treatmentItems,
  ReleaseGate: aiworkflow.LoopReleaseGate{
    MinCompletionRateDelta: 0.10,
    MaxFalseCompleteRate:   0.05,
    MaxToolCallIncrease:    0.30,
  },
})
```

若 `err != nil`，优先检查 baseline/treatment 窗口是否缺失或数据不完整。
