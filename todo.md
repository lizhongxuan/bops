# 多 Agent 并行可行方案改造任务列表（分步骤）

> 目标：将 “需求解析 / 规划 / 风险评估 / 仓库扫描”等拆成多个 Agent 并行执行，提升首屏响应与整体延迟。

## 阶段 0：并行协议与配置
- [ ] `internal/aiworkflow/types.go`: 扩展 `Event`（新增 `AgentID/AgentRole/AgentLabel` 或统一放入 `Data`），用于前端区分多个 Agent 的步骤来源。
- [ ] `internal/config/*` 或 `bops.json`: 增加多 Agent 配置项（启用开关、并行 Agent 列表、每个 Agent 的模型/温度/超时）。
- [ ] `internal/ai/client.go`: 提供可选 `CompositeClient` / `Router` 接口，为不同 Agent 绑定不同模型配置。

## 阶段 1：并行 Orchestrator 基础
- [ ] 新增 `internal/aiworkflow/orchestrator.go`：  
  - `RunParallel(ctx, state, opts)` 使用 `errgroup` 并行执行多个 Agent  
  - 为每个 Agent 创建独立 `State` 副本（隔离上下文/提示词）  
  - 统一 `EventSink` 透传 `AgentID` 字段到 SSE
- [ ] 新增 `internal/aiworkflow/agent_results.go`：  
  - `AgentResult` 结构（`Name/Output/Artifacts/Issues/Questions`）  
  - `mergeAgentResults(base, results)` 合并结果（冲突处理策略）
- [ ] `internal/server/ai.go` → `handleAIWorkflowStream`:  
  - 调用 `RunParallel` 并处理并行结果  
  - SSE `message` 里加入 `agent_id/agent_label`（可放 `extra_info` 或 `message_title`）

## 阶段 2：拆分 Agent 角色（最小实用集）
- [ ] `internal/aiworkflow/intent.go`: 抽出 `RunIntentAgent(ctx, state)` 用于并行执行意图解析。
- [ ] `internal/aiworkflow/safety.go`（或现有 `safetyCheck`）：抽出 `RunRiskAgent(ctx, state)`。
- [ ] `internal/aiworkflow/questions.go`: 抽出 `RunQuestionAgent(ctx, state)`。
- [ ] 新增 `internal/aiworkflow/repo_scan.go`:  
  - 以工具（本地文件/搜索）扫描现有结构  
  - 产出 `RepoSummary` 写入 `AgentResult.Data`

## 阶段 3：主流程编排（并行 → 汇总 → 生成）
- [ ] `internal/aiworkflow/pipeline.go`（或现有 `RunGenerate`/`RunFix` 调用处）：  
  - 并行完成后，统一进入 `generator/fixer`  
  - 优先使用并行 Agent 的输出作为上下文输入（减少长思考）
- [ ] `internal/ai/prompt.go`:  
  - 新增 `composeParallelContext(results)` 将多 Agent 输出整合为系统提示/上下文

## 阶段 4：前端展示（区分 Agent）
- [ ] `web/src/views/HomeView.vue`:  
  - `handleFunctionCallMessage` 读取 `agent_id`/`message_title`  
  - 按 Agent 分组展示步骤（或为每步加 “来源标签”）
- [ ] `web/src/components/FunctionCallPanel.vue`:  
  - 增加 `agent` badge（如 “规划Agent / 风险Agent”）  
  - 保持现有流式状态显示

## 阶段 5：测试与观测
- [ ] `internal/aiworkflow/orchestrator_test.go`: 并行执行 + 合并策略单测  
- [ ] `internal/server/ai_test.go`: SSE 输出包含 `agent_id` / 多 Agent 步骤顺序  
- [ ] `web/scripts/stream-e2e.mjs`: 模拟多 Agent 并行 SSE 流  
- [ ] 可选：新增 `metrics`（每个 Agent 的时延/耗时）

## 阶段 6：成本与稳定性策略（可选）
- [ ] 增加 `AgentTimeout` / `MaxParallelAgents` 限制  
- [ ] Fallback：某个 Agent 失败不影响主流程  
- [ ] 允许按任务类型路由 Agent（简单任务只启用 1-2 个）
