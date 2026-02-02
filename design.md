# bops 多 Agent 支持改造清单（函数级，详细版）

> 目标：不引入多模型，仅实现多 Agent 并行/协作能力；保持 Loop Agent 可用；UI 能展示多 Agent 运行与输出。
> 约束：不改现有模型路由；以单模型为基础，通过 Agent 角色/技能集合区分能力。

---

## AI 对话流程与角色职责

### 对话步骤
1) 用户通过对话框发送需求
2) 需求意图识别（分类）
   - 解释/审计 (Explanation/Audit)
   - 查错/调试 (Debug/Fix)
   - 优化 (Optimization)
   - 模拟/校验 (Simulation/Dry Run)
   - 转换 (Migration)
3) 生成任务 Plan（结构化 JSON）
4) 多 Agent 协作执行 Plan
5) Loop 执行：Plan → 子任务 → 产出 → Reviewer 校验 → 合并
6) 动态调整：用户变更需求 → Architect 更新 Plan → Coder 回滚/修订
7) 直至 Plan 完成

### 角色分工（Agent 体系）
- Architect Agent：负责 Plan 生成与依赖关系（DAG）
- YAML Coder Agent：负责 YAML 片段创建/更新/删除
- Reviewer/Linter Agent：语法/幂等性/安全检查（ansible-lint / yamllint）
- RAG/Doc Agent：查询模块/参数文档（内部知识库或官方文档）
- User Proxy Agent：向用户提问/确认/汇总结果

### 关键技术要求（落实到工程）
- Structured Output：Plan/步骤必须输出 JSON（含 step_name/desc/deps）
- State Management：YAML 不仅是文本，维护对象树（DOM/AST）
- Sandboxing：Reviewer 支持 syntax-check（如 ansible-playbook --syntax-check）

---

## 阶段 P0：基础多 Agent 运行框架（必须）

### P0-1 定义多 Agent 运行协议（数据结构 + 事件）
- `internal/aiworkflow/types.go`
  - `type Event`：补充 `AgentID`、`AgentRole`、`AgentName` 字段（用于 UI 展示与路由）
  - `type State`：增加 `AgentID/AgentName/AgentRole`（默认主 agent）
  - `type RunOptions`：新增 `AgentSpec`（如 name/skills/role）、`SessionKey` 字段
- `internal/server/ai.go`
  - `streamMessageExt`：增加 `agent_id/agent_name/agent_role`
  - `buildStreamMessageFromEvent(...)`：写入 agent 字段到 extra_info

### P0-2 需求意图识别（步骤 b）
- `internal/aiworkflow/intent.go`
  - 扩展 `IntentType`（explain/debug/optimize/simulate/migrate）
  - `intentExtract(...)` 输出 `intent_type` + `missing`
- `internal/server/ai.go`
  - `handleAIWorkflowStream(...)`：result payload 回传 `intent_type`

### P0-3 Agent 管理器（注册/实例化）
- 新增 `internal/aiworkflow/agent_manager.go`
  - `type AgentSpec { Name, Role, Skills []string }`
  - `type AgentManager`
    - `Register(spec AgentSpec)`
    - `Get(name string) (AgentSpec, bool)`
    - `List() []AgentSpec`
  - 提供默认 agent：`main`
- `internal/server/skills.go`
  - `handleAgents(...)` 输出 agent 列表时增加 role/skills

### P0-4 Agent Runner（多 Agent 运行入口）
- 新增 `internal/aiworkflow/agent_runner.go`
  - `RunAgent(ctx, prompt, opts)`：按 AgentSpec 构建 ToolExecutor + system prompt
  - `RunAgentLoop(ctx, prompt, opts)`：在 loop 中使用指定 agent 的工具集合
  - 每个 agent 输出事件时带 `AgentID/AgentName/AgentRole`

### P0-5 Plan 生成（步骤 c）
- 新增 `internal/aiworkflow/plan.go`
  - `buildPlanPrompt(prompt, intentType, contextText)`：输出 Plan JSON
  - `parsePlanJSON(...)`：解析成 `PlanStep{step_name, desc, deps}`
  - `State.Plan`：保存结构化 plan
- `internal/server/ai.go`
  - `handleAIWorkflowStream(...)`：把 plan 写入 result（或单独事件 `plan`）

### P0-6 多 Agent 并行执行入口（并发调度）
- 新增 `internal/aiworkflow/multi_agent.go`
  - `RunMultiAgent(ctx, prompt, specs []AgentSpec, opts)`
    - 主 agent 先运行
    - 子 agent 并行执行（goroutine + waitgroup）
    - 子 agent 输出以 `subagent` 标识
  - 结果聚合：收集子 agent summary，回写给主 agent（作为补充上下文）

### P0-7 SSE 输出支持多 Agent
- `internal/server/ai.go`
  - `handleAIWorkflowStream(...)`：
    - 接收 `agent_name` 或 `agents[]` 参数
    - `agent_mode=multi` 触发 RunMultiAgent
  - `writeSSE(...)`：保持 event 不变，extra_info 增加 agent 字段

### P0-8 状态管理（全局 Context）
- 新增 `internal/aiworkflow/state_store.go`
  - 维护 `YAML DOM/AST` + `vars` + `plan` + `history`
  - `UpdateYAMLFragment(...)`：局部更新（避免整段重写）
  - `Snapshot()`：给 Agent 读取上下文

---

## 阶段 P1：多 Agent 协作与结果汇总（推荐）

### P1-1 子 Agent 结果汇总给主 Agent
- `internal/aiworkflow/multi_agent.go`
  - 子 agent 输出 `summary` 与 `facts`（简化文本）
  - 主 agent 再跑一轮 “汇总/融合” prompt
- `internal/aiworkflow/format.go`
  - 新增 `buildMultiAgentSummaryPrompt(mainPrompt, subSummaries)`

### P1-2 Reviewer 自纠错闭环（步骤 d/e 关键）
- `internal/aiworkflow/multi_agent.go`
  - Reviewer 失败则回退给 Coder（最多 N 次）
  - 失败原因写入 `State.History` 与 `issues`
- `internal/aiworkflow/format.go`
  - `buildReviewPrompt(yamlFragment, issues)`：输出 JSON 修复指令

### P1-3 sessionKey 与并发 lane
- 新增 `internal/aiworkflow/lanes.go`
  - `SessionLane`：同一 sessionKey 串行（并发=1）
  - `GlobalLane`：全局并发限制（默认 4）
- `internal/aiworkflow/multi_agent.go`
  - 主 agent 走 session lane
  - 子 agent 走 global lane

### P1-4 Agent 级别工具隔离
- `internal/server/ai.go`
  - `buildLoopToolExecutor(...)` 支持传入 `AgentSpec`（只加载其 skills）
- `internal/skills/agent_factory.go`
  - 确认支持 `Build(spec AgentSpec)` 的单 agent 工具集合

### P1-5 模拟/校验 & 转换能力（意图 b-4/b-5）
- `internal/aiworkflow/simulate.go`
  - `RunSimulation(yaml, vars)`：输出推演结果（不执行）
- `internal/aiworkflow/migrate.go`
  - `ConvertScriptToYAML(scriptText)`：转换入口（migration）

---

## 阶段 P2：UI 展示多 Agent（体验优化）

### P2-1 多 Agent 输出分组展示
- `web/src/views/HomeView.vue`
  - `handleFunctionCallMessage(...)`：按 `agent_id/agent_name` 分组
  - `timelineEntries`：新增 `agent` 标签展示
- `web/src/components/FunctionCallPanel.vue`
  - 在卡片标题栏展示 Agent 名称/角色

### P2-2 多 Agent 结果汇总展示
- `web/src/views/HomeView.vue`
  - `applyResult(...)`：显示 `subagent_summaries` 区块

### P2-3 Agent 选择器（可选）
- `web/src/views/HomeView.vue`
  - 增加下拉框 `agent_selector`，默认 `main`
  - 支持 `agents[]` 多选（多 agent 并行）

### P2-4 Plan 展示（可选）
- `web/src/views/HomeView.vue`
  - 支持 `plan` 卡片或简洁列表（默认折叠）
  - 显示 plan step 的依赖关系（DAG 简化）

---

## 阶段 P3：配置与文档

### P3-1 配置支持多 Agent
- `internal/config/config.go`
  - 增加 `agents[]` 的文档说明
- `docs/usage.md`
  - 描述 `agent_mode=multi`、`agent_name`、`agents[]`

---

## 测试清单（必须）

### T1 多 Agent 执行
- 新增 `internal/aiworkflow/multi_agent_test.go`
  - `TestMultiAgentParallel`：两个子 agent 并发
  - `TestMultiAgentSummaryMerge`：子 agent summary 汇总进主 agent

### T2 SSE 输出字段
- `internal/server/ai_test.go`
  - `TestBuildStreamMessageFromEvent`：验证 agent 字段透传

### T3 前端 E2E
- `web/scripts/stream-e2e.mjs`
  - 模拟两个 agent 输出 + UI 分组展示
  - 模拟 plan 事件与 reviewer 自纠错流程

---

## 关键接口约定（便于前后端对齐）

### 请求参数（推荐）
```json
{
  "mode": "generate",
  "agent_mode": "multi",
  "agent_name": "main",
  "agents": ["main", "reviewer", "coder"],
  "prompt": "..."
}
```

### SSE extra_info 追加字段
```json
{
  "agent_id": "main",
  "agent_name": "main",
  "agent_role": "primary"
}
```

---

如果你认可这个清单，我可以进一步拆成“按提交顺序”的执行列表（每个 commit 的改动范围与测试点）。

---

## 按提交顺序的执行列表（Commit Plan）

> 每个提交聚焦一个可验收的最小闭环；每步都列出受影响的函数与测试点。

### Commit 01：意图识别扩展（b 步骤基础）
- 目标：支持 5 类意图（explain/debug/optimize/simulate/migrate）并在结果中返回。
- 代码改动：
  - `internal/aiworkflow/intent.go`
    - `IntentType` 枚举 + `parseIntentResponse(...)` 扩展
    - `intentExtract(...)` 返回 `intent_type`
  - `internal/aiworkflow/types.go`
    - `Intent` 结构补充 `Type`
  - `internal/server/ai.go`
    - `handleAIWorkflowStream(...)`：result payload 透传 `intent_type`
- 测试：
  - `internal/aiworkflow/intent_test.go`（新增：5 类意图解析）
  - `internal/server/ai_test.go`（result 包含 intent_type）

### Commit 02：多 Agent 协议字段落地（Event/State/RunOptions）
- 目标：SSE 能透传 agent 字段。
- 代码改动：
  - `internal/aiworkflow/types.go`
    - `Event` 增加 `AgentID/AgentName/AgentRole`
    - `State` 增加 `AgentID/AgentName/AgentRole`
    - `RunOptions` 增加 `AgentSpec/SessionKey`
  - `internal/server/ai.go`
    - `streamMessageExt` 增加 agent 字段
    - `buildStreamMessageFromEvent(...)` 写入 agent 字段
- 测试：
  - `internal/server/ai_test.go::TestBuildStreamMessageFromEvent`

### Commit 03：Agent 管理器（注册/查询/列表）
- 目标：具备 AgentSpec 注册与查询能力。
- 代码改动：
  - 新增 `internal/aiworkflow/agent_manager.go`
    - `Register/Get/List` + 默认 main
  - `internal/server/skills.go`
    - `handleAgents(...)` 输出 role/skills
- 测试：
  - `internal/aiworkflow/agent_manager_test.go`

### Commit 04：Plan 生成（结构化 JSON）
- 目标：基于意图生成 plan JSON 并落到 State。
- 代码改动：
  - 新增 `internal/aiworkflow/plan.go`
    - `buildPlanPrompt(...)`、`parsePlanJSON(...)`
  - `internal/aiworkflow/types.go`
    - `State.Plan` / `PlanStep`
  - `internal/server/ai.go`
    - result payload 透传 `plan`
- 测试：
  - `internal/aiworkflow/plan_test.go`

### Commit 05：Agent Runner（单 Agent 运行入口）
- 目标：按 AgentSpec 运行 loop/pipeline（单 agent）。
- 代码改动：
  - 新增 `internal/aiworkflow/agent_runner.go`
    - `RunAgent(...)` / `RunAgentLoop(...)`
  - `internal/server/ai.go`
    - `handleAIWorkflowStream(...)` 支持 `agent_name`
- 测试：
  - `internal/aiworkflow/agent_runner_test.go`

### Commit 06：多 Agent 并行入口（multi 模式）
- 目标：主 agent + 子 agent 并行执行，结果聚合。
- 代码改动：
  - 新增 `internal/aiworkflow/multi_agent.go`
    - `RunMultiAgent(...)`
  - `internal/server/ai.go`
    - 支持 `agent_mode=multi` + `agents[]`
- 测试：
  - `internal/aiworkflow/multi_agent_test.go::TestMultiAgentParallel`

### Commit 07：Reviewer 自纠错闭环
- 目标：Reviewer 失败 → 回退 Coder 修改（最多 N 次）。
- 代码改动：
  - `internal/aiworkflow/multi_agent.go`
    - reviewer fail → coder retry
  - `internal/aiworkflow/format.go`
    - `buildReviewPrompt(...)`
- 测试：
  - `internal/aiworkflow/multi_agent_test.go::TestMultiAgentSummaryMerge`

### Commit 08：Session / Global 并发 Lanes
- 目标：同 session 串行，不同 session 并行受控。
- 代码改动：
  - 新增 `internal/aiworkflow/lanes.go`
  - `internal/aiworkflow/multi_agent.go`
    - 主 agent 走 session lane
    - 子 agent 走 global lane
- 测试：
  - `internal/aiworkflow/lanes_test.go`

### Commit 09：状态管理（YAML DOM/AST）
- 目标：局部更新 YAML，避免全量重写。
- 代码改动：
  - 新增 `internal/aiworkflow/state_store.go`
    - `UpdateYAMLFragment(...)` / `Snapshot()`
  - `internal/aiworkflow/agent_runner.go`
    - 读写 StateStore
- 测试：
  - `internal/aiworkflow/state_store_test.go`

### Commit 10：模拟/转换能力（Intent 扩展落地）
- 目标：simulate/migrate 意图可执行。
- 代码改动：
  - 新增 `internal/aiworkflow/simulate.go`
  - 新增 `internal/aiworkflow/migrate.go`
  - `internal/server/ai.go`：新增入口或 mode 分支
- 测试：
  - `internal/aiworkflow/simulate_test.go`
  - `internal/aiworkflow/migrate_test.go`

### Commit 11：前端多 Agent 展示
- 目标：不同 agent 输出分组显示。
- 代码改动：
  - `web/src/views/HomeView.vue`
    - `handleFunctionCallMessage(...)` 按 agent 分组
  - `web/src/components/FunctionCallPanel.vue`
    - 标题栏显示 agent 名称/角色
- 测试：
  - `npm --prefix web run typecheck`

### Commit 12：前端展示 Plan + 子 Agent 汇总
- 目标：Plan 卡片 + subagent_summaries 区块。
- 代码改动：
  - `web/src/views/HomeView.vue`
    - `applyResult(...)` 展示 plan 和 subagent_summaries
- 测试：
  - `web/scripts/stream-e2e.mjs`（扩展）

### Commit 13：文档与配置说明
- 目标：补充使用说明。
- 代码改动：
  - `docs/usage.md`：`agent_mode=multi`、`agent_name`、`agents[]`
  - `internal/config/config.go`：说明 agents 配置结构
- 测试：无

---


## 用户侧体验变化（效果说明）

### 全局体验提升（完成 P0-P2 后）
- 对话不再是“一次性输出”，而是多 Agent 分工、分阶段实时输出。
- 用户能看到“是谁在做什么”（Architect/Coder/Reviewer/RAG/User Proxy）。
- Plan 先可视化，执行中可中断、可改动，流程更透明。
- YAML 不会被整段重写，变更更稳定、更可控。

### 关键节点的用户感知（按步骤）
- 需求发送后：系统会先返回“意图类型”（解释/调试/优化/模拟/转换）。
- Plan 生成后：用户会看到结构化 Plan（步骤 + 依赖），可提前确认方向。
- Loop 执行中：每个子任务都有独立卡片与状态（运行中/完成/失败）。
- Reviewer 介入：若发现问题，会自动打回 Coder 并显示原因（自纠错可见）。
- User Proxy 提示：关键节点会弹出“请确认/补充”的问题列表。
- 动态变更：用户中途改需求会触发 Plan 更新，同时同步修订已生成的 YAML。

### 按提交顺序的用户体验变化（对应 Commit Plan）

- Commit 01：意图识别可见
  - 用户会看到系统判断的意图类型（例如“调试/优化/转换”）。

- Commit 02：多 Agent 标识可见
  - 对话流里每条步骤卡片会标明来自哪个 Agent。

- Commit 03：Agent 列表可查询
  - UI/接口可查看当前可用 Agent（角色/技能）。

- Commit 04：Plan 可见
  - 用户先看到结构化 Plan，再进入细节生成。

- Commit 05：单 Agent 执行更稳定
  - 仍保持当前体验，但事件更清晰、角色更明确。

- Commit 06：多 Agent 并行
  - Coder/Reviewer/RAG 可同时推进，用户感知速度更快。

- Commit 07：Reviewer 自纠错
  - 用户能看到“自动修复循环”，减少手工干预。

- Commit 08：并发顺序一致
  - 同一会话不会乱序，多会话并行更流畅。

- Commit 09：局部更新 YAML
  - 修改更稳定，不会出现“整段被重写”的跳跃感。

- Commit 10：模拟/转换能力
  - 用户可请求“推演结果”或“脚本转换”，无需实际执行。

- Commit 11：UI 多 Agent 分组
  - 聊天中按 Agent 分组展示，易于理解分工。

- Commit 12：Plan + 汇总展示
  - Plan 与各子 Agent 的总结都会显示在结果区域。

- Commit 13：文档与配置完善
  - 用户可按文档快速配置多 Agent 并体验上述流程。
