# bops 对话“平滑输出”改造详细流程

> 基于 `chat-design.md`，给出可落地的改造流程、阶段划分、关键改动点与验收标准。
> 范围：后端流式接口（`internal/`）、前端 UI（`web/`）。

---

## 0. 目标与范围

**目标**
- 流式输出可见：回答与思考均可持续增量刷新。
- 过程可视化：工具调用/计划更新/文件创建形成卡片与步骤流。
- UI 结构更稳：消息分组 + 状态机，减少跳动。

**范围**
- 后端 SSE 事件协议与消息结构。
- 前端对话时间线、卡片渲染、状态机。

---

## 1. 总体路线（分阶段）

**阶段 1：流式回答补齐（低风险、快速见效）**
- 后端输出 `delta.Content`；前端已支持 `answer` 通道增量。

**阶段 2：工具调用步骤卡片**
- 将 `aiworkflow` 的事件映射成 `function_call/tool_response` 消息。
- 前端新增 FunctionCallPanel 面板。

**阶段 3：文件/计划专用卡片**
- 定义卡片协议，前端新增 CardRenderer。

**阶段 4：消息分组 + 状态机**
- 引入 `reply_id` 分组、waiting/responding 体验。

---

## 2. 详细流程

### 阶段 1：补齐回答流式输出

**目标**：回答能“边生成边显示”。

**后端改造**
- 文件：`internal/server/ai.go`
- 现状：仅推送 `delta.Thought`。
- 修改：在读取 `streamCh` 时同时推送 `delta.Content`。

**伪代码示例**
```go
case delta, ok := <-streamCh:
  if ok {
    if delta.Thought != "" {
      writeSSE(w, "delta", map[string]any{"channel": "thought", "content": delta.Thought})
    }
    if delta.Content != "" {
      writeSSE(w, "delta", map[string]any{"channel": "answer", "content": delta.Content})
    }
    flusher.Flush()
    continue
  }
```

**前端验证**
- `web/src/views/HomeView.vue` 已支持：
  - `handleSSEChunk` → `delta` → `appendAnswerDelta`
- 验收：发送请求后，回答逐字出现，无需等到 `result`。

**验收标准**
- 任何支持 ChatStream 的模型，UI 会显示回答实时流式输出。

---

### 阶段 2：工具调用步骤卡片

**目标**：把 `aiworkflow` 的过程节点转为“步骤卡片”。

**后端改造**
1) 扩展事件结构（`internal/aiworkflow/types.go`）
- 增加字段：`CallID` / `DisplayName` / `Stage` 等。

2) 在 `emitEvent` 时携带信息（`internal/aiworkflow/nodes.go`）
- 节点开始 → function_call
- 节点结束 → tool_response

3) SSE 输出消息（`internal/server/ai.go`）
- 在写 `status` 的同时，新增 `message` 事件：
  - `type = function_call`（开始）
  - `type = tool_response`（结束）
  - `extra_info.execute_display_name` 统一传递

**前端改造**
1) 新增组件：`web/src/components/FunctionCallPanel.vue`
- 数据结构：
  - `call_id` 关联 start/end
  - status: running / done / failed
- 展示：折叠面板 + 状态图标 + 耗时 + 请求/响应详情

2) Timeline 渲染
- 在聊天流中插入 FunctionCallPanel 类型条目。

**验收标准**
- 工具调用阶段可见“步骤列表”且状态动态更新。
- 每个步骤可展开查看细节。

---

### 阶段 3：文件/计划专用卡片

**目标**：创建文件 / 更新计划有专门卡片。

**协议设计**
- SSE `card` 事件或 `result.ui_resource` 携带结构化 JSON。

**示例：FileCreateCard**
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

**示例：PlanUpdateCard**
```json
{
  "card_type": "plan_update",
  "title": "更新计划",
  "items": [
    {"step": "解析需求", "status": "done"},
    {"step": "生成 YAML", "status": "running"}
  ],
  "diff": "...markdown..."
}
```

**前端改造**
- 新增 `CardRenderer` 组件，路由到：
  - `FileCreateCard.vue`
  - `PlanUpdateCard.vue`
- 卡片内支持：代码块、计划列表、操作按钮。

**验收标准**
- 任务过程中出现结构化卡片，可查看文件/计划。

---

### 阶段 4：消息分组 + 状态机

**目标**：避免对话跳动，提升过程可读性。

**前端改造**
1) **消息分组（reply_id）**
- 将 timeline 由一维数组升级为“Group + Items”。
- 组内顺序：function_call → tool_response → reasoning → answer。

2) **状态机（sending/waiting/responding）**
- 新增 waiting 状态控制：
  - waiting: 显示占位符（thinking）
  - responding: 输出回答

3) **滚动策略**
- 用户未滚离底部 → 自动跟随
- 用户滚动查看历史 → 不强制滚动

**验收标准**
- 用户在长流程中依旧能保持上下文稳定。

---

## 3. 测试与验证

**后端**
- 单测：SSE 是否正确写出 `delta answer`。
- 手动：curl 连接 SSE 流检查顺序。

**前端**
- E2E：发送需求 → 观察流式回答、工具面板、卡片出现。
- 边界：断流、错误事件、重复事件去重。

---

## 4. 迁移与兼容

- 现有 `status` 事件保留，不破坏旧 UI。
- 新增 `message` / `card` 事件为可选。
- 新 UI 能兼容“只有 answer/result”场景。

---

## 5. 风险与控制

- **流式输出过快**：前端渲染压力 → 使用节流或合并更新。
- **消息乱序**：事件补上 `index` 或 `ts` 排序。
- **卡片扩展频繁**：统一 `card_type` 枚举管理。

---

## 6. 交付物清单

- `internal/server/ai.go`：SSE answer delta + message/card 事件输出
- `internal/aiworkflow/types.go`：事件扩展
- `web/src/views/HomeView.vue`：分组、状态机、接入卡片/工具面板
- `web/src/components/FunctionCallPanel.vue`
- `web/src/components/CardRenderer.vue`
- `web/src/components/cards/FileCreateCard.vue`
- `web/src/components/cards/PlanUpdateCard.vue`

---

## 7. 验收标准汇总

- [ ] 回答与思考均可流式输出
- [ ] 工具调用步骤卡片可见且状态可更新
- [ ] 文件/计划卡片可展示并交互
- [ ] 对话过程不跳动、上下文稳定

---

## 8. 当前进度（自动更新）

- [x] 流式回答补齐（SSE `delta` 支持 answer）
- [x] 工具调用步骤卡片（`message` 事件 + FunctionCallPanel）
- [x] 卡片协议与渲染（`card` 事件 + CardRenderer）
- [x] reply_id 分组与状态机（sending/waiting/responding）
- [x] 断流/错误去重处理
- [ ] 手动验收：UI 逐字输出
- [ ] 手动验收：步骤卡片状态更新
- [ ] 手动验收：文件/计划卡片交互
