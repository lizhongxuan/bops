# design2.md

本设计文档覆盖三项改造：
1) 支持多模型配置与按 Agent 指定模型；
2) 构建上下文工程，保证每次询问都有“任务全貌”；
3) plan step 进行中状态增加动画效果。

---

## 1. 目标与原则

### 目标
- 配置页支持多个模型配置（多 Provider / 多 Model），并可为不同 Agent 绑定模型。
- 每次 Agent 询问都能获得当前“上下文全貌”，减少缺失信息造成的重复追问。
- plan step 进行中卡片有明显动态反馈，提升“逐步推进”的体感。

### 原则
- 兼容现有单模型配置；升级不破坏现有 API。
- 上下文最小充分原则（只提供必要信息、可压缩）。
- UI 动效轻量化，性能优先。

---

## 2. 多模型配置（配置页）

### 2.1 数据模型（建议）
新增模型配置结构与 Agent 绑定关系：

```json
{
  "model_profiles": [
    {
      "id": "deepseek-default",
      "name": "DeepSeek-R1",
      "provider": "deepseek",
      "base_url": "https://...",
      "api_key": "***",
      "model": "deepseek-r1",
      "temperature": 0.3,
      "max_tokens": 4096,
      "timeout_ms": 60000
    }
  ],
  "agent_model_bindings": {
    "Coordinator": "deepseek-default",
    "Coder": "deepseek-default",
    "Reviewer": "deepseek-default"
  },
  "default_model_profile": "deepseek-default"
}
```

### 2.2 后端配置与加载
- 新增配置存储：`ai.model_profiles` + `ai.agent_model_bindings` + `ai.default_model_profile`。
- 加载策略：
  - 若 Agent 未配置绑定 -> 使用 `default_model_profile`。
  - 若 profiles 为空 -> 回退当前单模型配置。
- 需要支持更新/保存：`/api/settings` 或 `/api/ai/config` 扩展字段。

### 2.3 AI Client 路由策略
- 新增 “ProfileRouterClient”：根据 AgentName / AgentRole 选择模型配置。
- 生成实际 Client 时复用现有 `ai.NewClient(cfg)`，但参数来自选中的 profile。
- 兼容现有 Planner/Executor 模型（可在 profile 内扩展字段）。

### 2.4 配置页 UI
- 新增 “模型配置”管理界面：
  - 增加/编辑/删除模型 profile。
  - 显示 provider / model / base_url / key 等。
- 新增 “Agent-模型绑定”区域：
  - 下拉选择模型 profile。
- 显示使用优先级：绑定 > default profile > legacy config。

---

## 3. 上下文工程（Context Engineering）

### 3.1 目标
让每次 Agent 询问都能了解“当前任务与步骤进度、关键上下文、最新结果”，减少重复追问和“断层对话”。

### 3.2 上下文结构（建议）
为每次请求构建统一 Context Envelope：

```
Context Envelope:
- Workflow Snapshot
  - name, description
  - steps summary (step_name + action + targets + status)
- Plan Snapshot
  - plan steps
  - current step / progress
- Open Questions
  - outstanding questions
- Env & Constraints
  - validation_env / env_packages / user constraints
- Recent Timeline
  - last 6~10 user messages summary
  - last 6~10 agent outputs summary
```

### 3.3 上下文生成策略
- 只保留必要字段，避免把完整 YAML 注入每次 Prompt。
- 对“历史对话”使用摘要：
  - 在 session 内缓存 `context_summary`。
  - 新对话时触发增量总结（Summarizer Agent 或简单规则摘要）。
- 每次调用前构造 “任务快照 + 进度 + 关键变更”。

### 3.4 具体实现建议
- 新增 context builder：`buildAgentContext(agentName, sessionId, draftId)`。
- 在 `RunMultiCreate` / `runCoordinator` / `runCoderStep` / `runReviewerFix` 前调用。
- 允许按 Agent 定制上下文：
  - Coordinator：重点 plan + missing + 全局摘要
  - Coder：重点 steps + 当前 step + inputs
  - Reviewer：重点 step + issues + validation logs

---

## 4. plan step “进行中”动画

### 4.1 交互目标
- 当某个 plan step 为 in_progress 时，卡片显示动态状态（如转圈/闪烁）。
- 完成/失败时停止动画。

### 4.2 UI 建议
- 在 `PlanStepCard.vue` 增加状态动画：
  - `statusClass === running` 时显示小圆点脉冲。
  - 文案："(进行中)" 保持。

示例：
```css
.status.running::after {
  content: "";
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-left: 6px;
  border-radius: 50%;
  background: #2c73b8;
  animation: pulse 1.2s infinite;
}
@keyframes pulse {
  0% { transform: scale(1); opacity: .4; }
  50% { transform: scale(1.6); opacity: 1; }
  100% { transform: scale(1); opacity: .4; }
}
```

---

## 5. 风险与兼容性

- 多模型配置若缺省：必须回落到原有单模型配置，否则无法调用。
- 上下文工程若过长：需限制 token 数 / 压缩摘要。
- 计划卡动画：需避免大量 DOM 动画导致性能抖动。

---

## 6. 验证与测试

- 配置页：
  - 新增/删除模型 profile
  - Agent 绑定生效
- 调用链路：
  - Coordinator / Coder / Reviewer 调用是否走正确 model profile
- 上下文：
  - 每次 prompt 是否包含 plan + steps summary
  - 缺少信息时是否只在用户明确要求时提问
- UI：
  - plan step 进行中动画可见，完成即停止

---

## 7. 迭代建议

- V1：仅支持 Profile + 绑定；context builder 使用规则拼接。
- V2：加入 Summarizer Agent，自动压缩上下文。
- V3：支持按任务动态路由（如 planner/exec 在不同模型）。

