# bops 多 Agent 改造任务列表（按序执行）

> 来源：design.md（多 Agent 支持方案）。
> 目标：不引入多模型，仅实现多 Agent 并行/协作能力；保持 Loop Agent 可用；UI 能展示多 Agent 运行与输出。

---

## 阶段 P0：基础能力（必须）

### 任务 01：多 Agent 事件/状态协议
- 修改 `internal/aiworkflow/types.go`
  - `Event` 增加 `AgentID/AgentName/AgentRole`
  - `State` 增加 `AgentID/AgentName/AgentRole`
  - `RunOptions` 增加 `AgentSpec/SessionKey`
- 修改 `internal/server/ai.go`
  - `streamMessageExt` 增加 `agent_id/agent_name/agent_role`
  - `buildStreamMessageFromEvent(...)` 透传 agent 字段
- 验收：SSE message 的 `extra_info` 带 agent 字段
- 测试：`go test ./internal/server`
 - 状态：✅ 已完成

### 任务 02：意图识别扩展（b 步骤）
- 修改 `internal/aiworkflow/intent.go`
  - 扩展 `IntentType`：explain/debug/optimize/simulate/migrate
  - `intentExtract(...)` 输出 `intent_type` + `missing`
- 修改 `internal/aiworkflow/types.go`
  - `Intent` 结构增加 `Type`
- 修改 `internal/server/ai.go`
  - `handleAIWorkflowStream(...)` result payload 回传 `intent_type`
- 验收：result 中可见 `intent_type`
- 测试：新增 `internal/aiworkflow/intent_test.go`
- 状态：✅ 已完成

### 任务 03：Agent 管理器
- 新增 `internal/aiworkflow/agent_manager.go`
  - `AgentSpec { Name, Role, Skills []string }`
  - `AgentManager.Register/Get/List`
  - 默认 `main` agent
- 修改 `internal/server/skills.go`
  - `handleAgents(...)` 输出 role/skills
- 验收：`/api/agents` 返回 role/skills
- 测试：新增 `internal/aiworkflow/agent_manager_test.go`
- 状态：✅ 已完成

### 任务 04：Plan 生成（c 步骤）
- 新增 `internal/aiworkflow/plan.go`
  - `buildPlanPrompt(prompt, intentType, contextText)` 输出 Plan JSON
  - `parsePlanJSON(...)` 解析 `PlanStep{step_name, desc, deps}`
- 修改 `internal/aiworkflow/types.go`
  - `State.Plan` 与 `PlanStep` 结构
- 修改 `internal/server/ai.go`
  - result payload 透传 `plan`（或新增 `plan` 事件）
- 验收：result 中包含结构化 plan
- 测试：新增 `internal/aiworkflow/plan_test.go`
- 状态：✅ 已完成

### 任务 05：Agent Runner（单 Agent 入口）
- 新增 `internal/aiworkflow/agent_runner.go`
  - `RunAgent(...)`：按 AgentSpec 构建 ToolExecutor + system prompt
  - `RunAgentLoop(...)`：在 loop 内使用指定 agent 技能集合
- 修改 `internal/server/ai.go`
  - `handleAIWorkflowStream(...)` 支持 `agent_name`
- 验收：指定 `agent_name` 可运行单 agent
- 测试：新增 `internal/aiworkflow/agent_runner_test.go`
- 状态：✅ 已完成

### 任务 06：多 Agent 并行执行入口（multi 模式）
- 新增 `internal/aiworkflow/multi_agent.go`
  - `RunMultiAgent(...)`：主 agent + 子 agent 并行
  - 子 agent 输出标识 `subagent`
  - 子 agent 结果聚合回主 agent
- 修改 `internal/server/ai.go`
  - 支持 `agent_mode=multi` + `agents[]` 参数
- 验收：SSE 流中出现多个 agent 的 event
- 测试：新增 `internal/aiworkflow/multi_agent_test.go::TestMultiAgentParallel`
- 状态：✅ 已完成

### 任务 07：多 Agent SSE 透传
- 修改 `internal/server/ai.go`
  - 确保所有 message/verbose/status 事件都带 agent 字段
- 验收：前端可区分不同 agent 的输出
- 测试：`internal/server/ai_test.go::TestBuildStreamMessageFromEvent`
- 状态：✅ 已完成

### 任务 08：状态管理（全局 Context）
- 新增 `internal/aiworkflow/state_store.go`
  - 维护 YAML DOM/AST + vars + plan + history
  - `UpdateYAMLFragment(...)` / `Snapshot()`
- 修改 `internal/aiworkflow/agent_runner.go`
  - 读写 StateStore
- 验收：局部更新 YAML，不整段重写
- 测试：新增 `internal/aiworkflow/state_store_test.go`
- 状态：✅ 已完成

---

## 阶段 P1：协作优化（推荐）

### 任务 09：Reviewer 自纠错闭环
- 修改 `internal/aiworkflow/multi_agent.go`
  - Reviewer 失败 → Coder 回退修复（最多 N 次）
  - 失败原因写入 `State.History` 与 `issues`
- 修改 `internal/aiworkflow/format.go`
  - `buildReviewPrompt(yamlFragment, issues)` 输出 JSON 修复指令
- 验收：Review 失败自动触发修复循环
- 测试：`internal/aiworkflow/multi_agent_test.go::TestMultiAgentSummaryMerge`
- 状态：✅ 已完成

### 任务 10：Session / Global 并发 Lanes
- 新增 `internal/aiworkflow/lanes.go`
  - `SessionLane`：同 session 串行
  - `GlobalLane`：全局并发限制
- 修改 `internal/aiworkflow/multi_agent.go`
  - 主 agent 走 session lane
  - 子 agent 走 global lane
- 验收：同 session 不乱序，多 session 并行
- 测试：新增 `internal/aiworkflow/lanes_test.go`
- 状态：✅ 已完成

### 任务 11：Agent 级别工具隔离
- 修改 `internal/server/ai.go`
  - `buildLoopToolExecutor(...)` 支持传入 AgentSpec（只加载其 skills）
- 修改 `internal/skills/agent_factory.go`
  - 校验单 agent tool bundle 是否隔离
- 验收：不同 agent 只可调用其 skills
- 测试：新增 `internal/skills/agent_factory_test.go`（隔离场景）
- 状态：✅ 已完成

### 任务 12：模拟/转换能力（b-4/b-5）
- 新增 `internal/aiworkflow/simulate.go`
  - `RunSimulation(yaml, vars)` 输出推演结果
- 新增 `internal/aiworkflow/migrate.go`
  - `ConvertScriptToYAML(scriptText)`
- 修改 `internal/server/ai.go`
  - 新增 simulate/migrate 入口或 mode 分支
- 验收：simulate/migrate 有独立输出
- 测试：新增 `simulate_test.go` / `migrate_test.go`
- 状态：✅ 已完成

---

## 阶段 P2：UI 体验（必须）

### 任务 13：多 Agent 输出分组展示
- 修改 `web/src/views/HomeView.vue`
  - `handleFunctionCallMessage(...)` 按 `agent_id/agent_name` 分组
  - timeline 显示 agent 标签
- 修改 `web/src/components/FunctionCallPanel.vue`
  - 标题栏显示 Agent 名称/角色
- 验收：UI 能区分不同 agent 输出
- 测试：`npm --prefix web run typecheck`
- 状态：✅ 已完成

### 任务 14：Plan + 子 Agent 汇总展示
- 修改 `web/src/views/HomeView.vue`
  - `applyResult(...)` 展示 `plan` 与 `subagent_summaries`
- 验收：结果面板看到 plan & 汇总
- 测试：扩展 `web/scripts/stream-e2e.mjs`
- 状态：✅ 已完成

### 任务 15：Agent 选择器（可选）
- 修改 `web/src/views/HomeView.vue`
  - 增加 agent selector（单选/多选）
  - payload 支持 `agent_name` 或 `agents[]`
- 验收：用户可选择 agent / 多 agent
- 状态：✅ 已完成

---

## 阶段 P3：配置与文档

### 任务 16：配置与文档完善
- 修改 `internal/config/config.go`
  - 明确 `agents[]` 配置结构与默认值
- 修改 `docs/usage.md`
  - 描述 `agent_mode=multi`、`agent_name`、`agents[]`
- 验收：配置文档可直接使用
- 状态：✅ 已完成

---

## 统一验收（完成后）
- `go test ./internal/...`
- `npm --prefix web run typecheck`
- `node web/scripts/stream-e2e.mjs`
