# bops 对话平滑输出改造任务列表（TODO）

## 阶段 1：流式回答补齐
- [x] 后端：在 `internal/server/ai.go` 读取 `streamCh` 时推送 `delta.Content`，事件 `delta`，`channel: "answer"`
- [x] 前端：确认 `web/src/views/HomeView.vue` 中 `handleSSEChunk` 对 `answer` 通道正常拼接
- [x] 验收：页面发起对话后能看到回答逐字出现（无需等到 `result`）

## 阶段 2：工具调用步骤卡片
- [x] 后端：扩展 `internal/aiworkflow/types.go` 的 `Event`（增加 `CallID`/`DisplayName`/`Stage`）
- [x] 后端：在 `internal/aiworkflow/nodes.go` 中为每个节点 emit start/end 对应 `function_call/tool_response`
- [x] 后端：在 `internal/server/ai.go` 将事件映射为 SSE `message`（带 `extra_info.execute_display_name`）
- [x] 前端：新增 `web/src/components/FunctionCallPanel.vue`
- [x] 前端：在 `web/src/views/HomeView.vue` 将 function_call/tool_response 组装为步骤列表并渲染
- [x] 验收：执行过程有清晰的步骤卡片，状态可更新

## 阶段 3：文件/计划卡片
- [x] 协议：定义 `card_type` 枚举（`file_create` / `plan_update` / `workflow_summary`）
- [x] 后端：在 `internal/server/ai.go` 输出 `card` 事件或 `result.ui_resource` 中附加卡片数据
- [x] 前端：新增 `web/src/components/CardRenderer.vue`
- [x] 前端：新增 `web/src/components/cards/FileCreateCard.vue`
- [x] 前端：新增 `web/src/components/cards/PlanUpdateCard.vue`
- [x] 验收：文件创建/计划更新以卡片形式出现，支持代码块/列表展示

## 阶段 4：消息分组 + 状态机
- [x] 前端：在 `web/src/views/HomeView.vue` 引入 `reply_id` 分组结构
- [x] 前端：加入 sending/waiting/responding 状态机
- [x] 前端：优化滚动策略（用户未滚离底部自动跟随）
- [x] 验收：长对话过程稳定、不跳动

## 测试与验证
- [x] 后端：SSE 流输出顺序测试（delta/status/result）
- [x] 前端：E2E 测试流式输出 + 步骤卡片 + 计划/文件卡片
- [x] 边界：断流/错误事件去重处理

## 文档与验收
- [x] 更新 `chat-design.md` 记录最终协议
- [x] 在 `chat-upgrade.md` 标记完成情况
- [x] 形成演示用录屏或截图
