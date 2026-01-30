# bops Chat 平滑输出分析与设计（参考 Coze Studio）

> 目标：结合 `coze-studio/design.md` 与相关源码，分析 Coze 的“平滑输出”实现，并给出 bops 侧实现同等体验的修改方案。
> 范围：前端 UI（`web/`）+ 后端流式接口（`internal/server/ai.go` 等）。

---

## 1. 现象与目标

**现象**（来自用户体验反馈）：
- 提需求后需要长时间等待，直到最终结果才出现；过程感弱。
- 工具执行/创建文件/更新计划等“过程型输出”不够可视化。
- 缺少可交互的卡片（代码块、计划更新、文件操作）。

**目标**：
- 输出过程“不断刷新”，让用户感知模型正在思考与执行。
- 工具调用形成“步骤卡片”，状态清晰（执行中/完成/失败）。
- 文件创建/计划更新有“专用卡片”，可查看详情并执行交互。

---

## 2. Coze 的“平滑输出”实现机制（基于源码 + design.md）

> 本节结论来自 `coze-studio/design.md` 和源码：
> `frontend/packages/common/chat-area/chat-core/...`、`chat-area/...`、`chat-workflow-render/...`、`chat-area-plugin-reasoning/...`。

### 2.1 流式消息拼接（content + reasoning）
- **ChunkProcessor** 在流式过程中将同一 `message_id` 的增量内容不断拼接。
- `streamMessageBuffer` 维护“同一消息的增量合并”，保证 UI 持续刷新。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-core/src/message/chunk-processor.ts`

**效果**：长思考/长回答不会等到最后一次返回，而是持续输出。

### 2.2 Waiting/Responding 状态机
- `WaitingStore` 维护 sending / waiting / responding / finish。
- UI 在 waiting 状态展示 `ThinkingPlaceholder`，避免“空白等待”。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/store/waiting.ts`
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/components/wait-generating/index.tsx`

**效果**：模型思考很久时依旧有“正在生成”的反馈。

### 2.3 Function Call 过程面板（步骤卡片）
- `FunctionCallMessageBox` 统一展示 function_call / tool_response / verbose。
- `getMessageUnitsByFunctionCallMessageList` 通过 `call_id` 或 index 关联步骤。
- `execute_display_name` 决定卡片标题（正在…/已完成…/失败…）。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/utils/fucntion-call/function-message-unit.ts`
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/components/fuction-call-message/function-call-content/function-call-message.tsx`

**效果**：工具执行步骤被“卡片化”，并随流式更新状态。

### 2.4 Reasoning 插槽
- `reasoning_content` 单独渲染（Markdown 引用块 + 流式指示）。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-area-plugin-reasoning/src/custom-components/message-inner-addon-bottom/index.tsx`

**效果**：思考过程可视化、且不阻塞最终回答。

### 2.5 消息分组 + Reverse Scroll
- `MessageGroupList` 以“分组 + reverse scroll”保持上下文稳定。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/components/message-group-list/index.tsx`
  - `coze-studio/frontend/packages/common/chat-area/chat-area/src/utils/message-group/message-group.ts`

**效果**：多步骤输出不会“跳动”，上下文稳定。

### 2.6 Card 渲染机制
- `ContentType.Card` 通过 **EnhancedContentConfig** 扩展渲染。
- `WorkflowRenderEntry` 目前仅支持 workflow 卡片，但机制可扩展。
- 关键文件：
  - `coze-studio/frontend/packages/common/chat-area/chat-workflow-render/src/components/workflow-render/index.tsx`
  - `coze-studio/frontend/packages/common/chat-area/chat-workflow-render/src/components/workflow-render/components/index.tsx`

**效果**：卡片能力可插拔，易于扩展。

### 2.7 中间状态标题
- `message.extra_info.message_title` 可用于插入“阶段提示”。
- 关键文件：
  - `coze-studio/frontend/packages/agent-ide/chat-answer-action-adapter/src/components/message-box-action-bar/index.tsx`

---

## 3. bops 当前实现与差距（基于本仓库源码）

### 3.1 已具备能力
- SSE 流式通道已存在：`POST /api/ai/workflow/stream`。
- 前端已支持增量显示（`appendAnswerDelta` / `appendThoughtDelta`）。
- 进度事件已有（`status` event）并显示为“进度列表”。
- 具备基础 UI 卡片机制：`ui_resource`（iframe / mcp-ui renderer）。

相关文件：
- `internal/server/ai.go`（SSE 输出）
- `web/src/views/HomeView.vue`（流式解析 / timeline 渲染）
- `internal/ai/openai.go`（ChatStream delta）

### 3.2 关键缺口
1) **回答内容未流式输出**
   - 后端当前只推送 `delta.Thought`，忽略 `delta.Content`。
   - 前端已支持 `channel === "answer"` 的增量更新，但服务端未发送。

2) **缺少 function_call / tool_response 语义**
   - 现有 `status` 事件更偏“流程节点”，未映射成“步骤卡片”。

3) **卡片类型不足**
   - 当前 `ui_resource` 仅工作流概览；没有文件创建/计划更新卡片。

4) **消息分组与状态机缺失**
   - timeline 是扁平数组，缺少 `reply_id` 维度的分组管理。

---

## 4. 设计：在 bops 中实现 Coze 级“平滑输出”

### 4.1 流式消息协议（建议）

**事件类型（SSE）**：
- `message` / `chunk`：统一承载 `answer` / `reasoning` / `function_call` / `tool_response`。
- `status`：流程节点进度（可保留）。
- `card`：专用卡片（文件/计划等）。
- `error` / `done`：错误或结束。

**统一消息结构**（与 Coze 对齐，便于组件化）：
```json
{
  "message_id": "m-123",
  "reply_id": "r-456",
  "role": "assistant",
  "type": "answer | function_call | tool_response | verbose",
  "content": "...",
  "reasoning_content": "...",
  "is_finish": false,
  "index": 3,
  "extra_info": {
    "call_id": "call-1",
    "execute_display_name": "{\"name_executing\":\"创建文件\",...}",
    "message_title": "更新计划...",
    "plugin_status": "0",
    "time_cost": "1.2",
    "stream_plugin_running": "uuid"
  }
}
```

**核心点**：前端只需要“拼接与渲染”，不需要理解业务细节。

### 4.2 后端修改建议（bops）

1) **补齐回答流式输出**
- 文件：`internal/server/ai.go`
- 逻辑：将 `delta.Content` 也推送为 SSE（`channel: "answer"`）。

2) **补齐 function_call / tool_response 语义**
- 将 aiworkflow 的节点开始/结束映射为 `function_call` / `tool_response`。
- 建议在 `aiworkflow.Event` 中增加 `call_id` / `display_name` 等字段。
- 让前端可以渲染“步骤卡片”。

3) **输出卡片 payload**
- 在 `result` 内或独立 `card` event 发送卡片 JSON：
  - `content_type: card`
  - `card_type: file_create | plan_update | workflow_summary`

4) **补齐 message_title**
- 当后端处于“中间阶段”时发送 `message_title`，前端渲染为轻量提示。

### 4.3 前端修改建议（bops）

1) **引入消息分组与状态机**
- 在 `web/src/views/HomeView.vue` 建立 MessageStore（按 `reply_id` 分组）。
- 维护 sending / waiting / responding 状态，渲染占位符。

2) **实现 FunctionCall 面板**
- 新建组件 `FunctionCallPanel`：
  - 每个 `call_id` 显示一条卡片
  - 标题来自 `execute_display_name`
  - 状态来自 `plugin_status`
  - 内容区渲染 Markdown / JSON

3) **Reasoning 插槽**
- 参考 Coze：思考内容独立区块，并显示“流式指示”。

4) **卡片渲染体系**
- 建议创建 `CardRenderer`：
  - `workflow_summary` 复用现有 `ui_resource`
  - `file_create`：展示文件路径 + 代码块 + 操作按钮
  - `plan_update`：展示计划列表 + diff + 状态

5) **滚动体验优化**
- 参考 Coze reverse scroll：当用户未滚离底部时自动跟随；否则不强制滚动。

### 4.4 文件/计划专用卡片设计

**FileCreateCard**（创建文件）
```json
{
  "card_type": "file_create",
  "title": "创建文件",
  "files": [
    {"path": "internal/ai/client.go", "language": "go", "content": "..."}
  ],
  "actions": ["open", "apply"]
}
```

**PlanUpdateCard**（计划更新）
```json
{
  "card_type": "plan_update",
  "title": "更新计划",
  "items": [
    {"step": "解析需求", "status": "done"},
    {"step": "生成 YAML", "status": "running"},
    {"step": "校验结果", "status": "pending"}
  ],
  "diff": "...markdown..."
}
```

---

## 5. 分阶段落地建议（可快速迭代）

**阶段 1：修复流式回答**
- 后端推送 `delta.Content`。
- 前端已有 `appendAnswerDelta`，可立即生效。

**阶段 2：工具调用卡片**
- 将 `status` event 映射为 function_call / tool_response。
- UI 以 collapsible panel 方式展示步骤。

**阶段 3：文件/计划卡片**
- 定义 `card_type` 协议。
- 在 UI 增加卡片渲染组件。

**阶段 4：消息分组 + 状态机**
- 引入 `reply_id` 分组，减少跳动。
- 建立 waiting/responding 占位反馈。

---

## 6. 需要确认的问题

1) 后端是否能输出 `function_call / tool_response` 语义？
2) “文件创建 / 计划更新”是 LLM 直接生成，还是由工具执行产生？
3) UI 卡片是倾向 **HTML 资源**（`ui_resource`）还是 **JSON 结构化**？

---

## 7. 附录：bops 关键修改点（文件清单）

- `internal/server/ai.go`：SSE delta answer + card event 输出
- `internal/ai/openai.go`：已具备 Content delta，无需大改
- `internal/aiworkflow/*`：事件结构拓展（call_id, display_name）
- `web/src/views/HomeView.vue`：消息分组、FunctionCall 面板、CardRenderer
- `web/src/components/*`：新增 `FunctionCallPanel.vue`、`FileCreateCard.vue`、`PlanUpdateCard.vue`

---

## 8. 简要对照（Coze → bops）

- ChunkProcessor → bops SSE chunk + message buffer
- WaitingStore → bops chatPending + waiting state
- FunctionCallMessageBox → bops FunctionCallPanel
- reasoning_content → bops thought channel + reasoning slot
- ContentType.Card → bops CardRenderer / ui_resource 扩展

---

### 总结
Coze 的“平滑输出”核心在于：
1) **流式拼接** + 2) **明确状态机** + 3) **工具调用过程卡片** + 4) **卡片扩展能力**。

bops 已具备部分基础（SSE/Thought/UI 卡片），只需补齐回答流式、工具语义与卡片体系，即可达到同等体验。

---

## 9. 当前落地的协议补充（已实现）

### 9.1 SSE 事件
- `status`：原有流程节点事件（保留）。
- `message`：工具调用步骤（`function_call` / `tool_response`）。
- `delta`：流式输出（`channel=thought|answer`）。
- `card`：结构化卡片（已输出 `workflow_summary`）。
- `result` / `error`：最终结果与错误。

### 9.2 `message` 事件结构（后端已输出）
```json
{
  "message_id": "generator-start-1",
  "reply_id": "reply-1700000000000-0",
  "role": "assistant",
  "type": "function_call | tool_response",
  "content": "...",
  "is_finish": false,
  "index": 1,
  "extra_info": {
    "call_id": "generator",
    "execute_display_name": "{\"name_executing\":\"正在生成工作流\",\"name_executed\":\"已完成生成工作流\",\"name_execute_failed\":\"生成工作流失败\"}",
    "plugin_status": "0"
  }
}
```

### 9.3 `card` 事件（已输出 workflow_summary）
```json
{
  "card_type": "workflow_summary",
  "title": "工作流概览",
  "summary": "steps=3 risk=low issues=0",
  "steps": 3,
  "risk_level": "low",
  "issues": [],
  "questions": []
}
```

### 9.4 前端渲染
- `FunctionCallPanel`：渲染 `message` 事件聚合后的步骤卡片。
- `CardRenderer`：渲染 `card` 事件（含 `workflow_summary` / `file_create` / `plan_update`）。
