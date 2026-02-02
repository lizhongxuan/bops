# 多 Agent 混合方案任务清单（详细版）

> 目标：全局 A（Coordinator Loop）+ 局部 C（Nested Loop），可流式展示进度与卡片。

---

## 阶段 1：数据结构与配置

1) **扩展 Plan/Step 结构**
- 文件：`internal/aiworkflow/types.go`
- 修改：
  - `PlanStep` 增加 `ID`, `Status`, `ParentID`（可选）
  - `State` 增加 `CurrentStepID`, `StepStatuses`
- 说明：主 Plan 可追踪执行状态
- 状态：✅ 已完成

2) **配置角色映射**
- 文件：`internal/config/config.go`
- 修改：
  - `AgentConfig` 增加 `role`（已添加，需补充默认值 & 校验）
  - 增加 `architect/coder/reviewer` 默认角色配置
- 状态：✅ 已完成

---

## 阶段 2：主循环（Coordinator Loop）

3) **新增 Coordinator Loop 入口**
- 文件：`internal/aiworkflow/agent_loop.go`
- 新增：`RunCoordinatorLoop(...)`
  - 生成主 Plan
  - 逐步执行（依次 pick step）
  - 每步触发 Nested Loop
- 状态：✅ 已完成

4) **Step 状态更新**
- 文件：`internal/aiworkflow/pipeline.go` / `internal/aiworkflow/plan.go`
- 修改：
  - `plan` 阶段输出包含 `step_id`
  - `State` 中记录 `current_step_id`
- 状态：✅ 已完成

---

## 阶段 3：Nested Loop（Coder + Reviewer）

5) **新增子循环 Runner**
- 文件：`internal/aiworkflow/subloop.go`（新增）
- 功能：
  - 传入 `parent_step_id` 与 step 描述
  - Coder + Reviewer 迭代（最多 N 轮）
  - 输出 YAML fragment + review 状态
- 状态：✅ 已完成

6) **Coder / Reviewer prompts**
- 文件：`internal/aiworkflow/format.go`
- 修改：
  - `buildSubloopPrompt(...)`
  - `buildReviewPrompt(...)` 复用或扩展
- 状态：✅ 已完成

7) **子 Plan 约束校验**
- 文件：`internal/aiworkflow/contract.go`
- 新增：`validateSubPlan(...)`
  - 只允许修改当前 step
- 状态：✅ 已完成

---

## 阶段 4：合并与状态更新

8) **YAML 片段合并**
- 文件：`internal/aiworkflow/state_store.go`
- 修改：
  - `UpdateYAMLFragment(...)` 支持基于 `parent_step_id` 合并
- 状态：✅ 已完成

9) **Plan 状态更新**
- 文件：`internal/aiworkflow/agent_runner.go`
- 修改：
  - 每次子 loop 完成后更新 step 状态
- 状态：✅ 已完成

---

## 阶段 5：事件流与卡片

10) **新增事件类型**
- 文件：`internal/aiworkflow/types.go`
- 新增：事件 `plan_step_start`, `subloop_round`, `merge_patch`, `plan_step_done`
- 状态：✅ 已完成

11) **SSE 透传**
- 文件：`internal/server/ai.go`
- 修改：
  - SSE message 支持 `event_type` + `parent_step_id`
  - card 增加 `plan_step` / `subloop` / `yaml_patch`
- 状态：✅ 已完成

---

## 阶段 6：前端展示

12) **Timeline 分组**
- 文件：`web/src/views/HomeView.vue`
- 修改：
  - 按 `parent_step_id` 分组展示子 loop
  - CardRenderer 支持新卡片类型
- 状态：✅ 已完成

13) **新增卡片组件**
- 文件：`web/src/components/cards/`
- 新增组件：
  - `PlanStepCard.vue`
  - `SubLoopCard.vue`
  - `YamlPatchCard.vue`
- 状态：✅ 已完成

---

## 阶段 7：测试

14) **后端单测**
- 文件：`internal/aiworkflow/subloop_test.go`
- 覆盖：
  - 子 loop 迭代次数
  - review 不通过回退
- 状态：✅ 已完成

15) **前端 E2E 流测试**
- 文件：`web/scripts/stream-e2e.mjs`
- 扩展：
  - 检查 `plan_step` / `subloop` / `yaml_patch` 卡片
- 状态：✅ 已完成

---

## 阶段 8：验收

- `go test ./internal/...`
- `npm --prefix web run typecheck`
- `node web/scripts/stream-e2e.mjs`

---

## 交付效果

- 主 Plan 逐步推进
- 子 Loop 可见、逐步输出
- YAML patch 卡片实时出现
- UI 体验接近 Coze
