# 目标：把 Coze 的“过程实时可见”迁到 bops
核心迁移点：

function_call / tool_response 作为“步骤卡片”
execute_display_name / call_id / plugin_status 作为状态驱动
reasoning_content 流式显示“思考过程”
chunk 拼接与 UI 状态机保持连续性
一、协议对齐（后端输出结构）
参考 coze 源码
message.thrift
关键字段：type / reasoning_content / extra_info.execute_display_name / extra_info.call_id / extra_info.plugin_status / stream_plugin_running
迁移到 bops
建议在 bops SSE message 中补齐：

{
  "message_id": "...",
  "reply_id": "...",
  "type": "function_call | tool_response | answer | verbose",
  "content": "...",
  "reasoning_content": "...", 
  "extra_info": {
    "call_id": "...",
    "execute_display_name": "{\"name_executing\":\"正在...\",\"name_executed\":\"已完成...\",\"name_execute_failed\":\"失败...\"}",
    "plugin_status": "0|1",
    "stream_plugin_running": "uuid"
  }
}
bops 修改位置
types.go（Event 增强字段）
ai.go（buildStreamMessageFromEvent）
二、后端流式逻辑（对齐 Coze 的事件节奏）
Coze 的做法
function_call 来了 → UI 立刻插步骤行
tool_response 来了 → 更新状态/内容
reasoning_content 在 chunk 过程中持续追加
bops 需要改的点
节点开始 → 立刻发送 message: function_call
节点结束 → 发送 message: tool_response
思考过程直接挂到 reasoning_content（而不是独立卡片）
bops 修改位置
nodes.go
节点 start/done 继续 emit，但要保证：
call_id 稳定
execute_display_name 完整
ai.go
stream 消费时：
先发 function_call
后发 tool_response
reasoning 走 reasoning_content
三、前端步骤卡片逻辑（与 Coze 同结构）
Coze 逻辑核心
function_message_unit.ts 用 call_id 匹配
collapse-panel-header.tsx 用 execute_display_name 控制标题
bops 迁移方案
继续保留 FunctionCallPanel
由 call_id 做匹配更新（已经做过，但可加强）
execute_display_name 直接驱动标题变化
tool_response 改写为“状态更新”而不是新增条目
bops 修改位置
HomeView.vue
handleFunctionCallMessage → 与 Coze 逻辑对齐
FunctionCallPanel.vue
标题/状态通过 execute_display_name 控制
四、思考过程显示（reasoning_content）
Coze 参考
coze-studio/frontend/packages/common/chat-area/chat-area-plugin-reasoning/...
reasoning 是同一条 message 的一个字段，不是单独卡片
bops 迁移方案
不再独立创建 thinking entry
把 delta.Thought 合并到回答消息的 reasoning_content
前端渲染为「引用 Markdown 块 + 流式指示」
修改位置
ai.go（delta.Thought → reasoning_content）
HomeView.vue（消息渲染时显示 reasoning）
五、卡片体系定位（跟 Coze 一致）
Coze 的“创建文件/更新计划”更像是 function_call / tool_response 的 UI效果，不是独立 card content。
所以 bops 的改动应是：

减少 card 类型
“步骤/文件创建”更多走 function_call / tool_response
六、实施顺序（推荐落地节奏）
协议对齐（call_id / execute_display_name / plugin_status）
后端 event → SSE message（function_call / tool_response）
前端步骤更新与状态
reasoning_content 迁移
移除或压缩卡片体系

# 结合 coze-studio 源码对“对话平滑输出/工作流过程展示”的实现分析（聚焦你关心的“步骤卡片 + 思考过程 + 流式输出”）


1) 协议层：Coze 后端给前端什么“信号”
message.thrift 定义了关键字段：

type：function_call / tool_response / verbose / answer
reasoning_content：思考过程内容
extra_info.execute_display_name：显示“正在/已完成/失败”的标题文案
extra_info.call_id：把 function_call 和 tool_response 绑定
extra_info.plugin_status：标记成功/失败
extra_info.stream_plugin_running：支持工具流式结果
结论：Coze 的“步骤/卡片”不是等最终结果一次性拼，而是靠中间事件 + 统一协议字段持续刷新。

2) 流式拼接：每个 chunk 立即更新 UI
chunk-processor.ts

concatContentAndUpdateMessage 会把流式 chunk 累加到同一条 message_id
content + reasoning_content 都是增量追加
前端无需等待最后一个 chunk 就会更新
结论：这就是 Coze “思考过程实时显示”的基础。

3) 步骤卡片：function_call / tool_response 的匹配逻辑
function-message-unit.ts

把消息列表转换成 FunctionCallMessageUnit
优先用 call_id 匹配，没有就用 index 匹配
tool_response 到来后更新同一条步骤单元
结论：Coze 的“步骤列表”不是一次性生成，而是“function_call 到来就插入，tool_response 到来就更新”。

4) 步骤标题与状态：execute_display_name
function-call-message.tsx
collapse-panel-header.tsx

execute_display_name 解析后得到：
name_executing → “正在……”
name_executed → “已完成……”
name_execute_failed → “失败……”
plugin_status 决定状态颜色（失败/成功）
结论：Coze 是靠 execute_display_name + plugin_status 来驱动 UI 的“步骤体验”。

5) “思考过程”卡片来自 reasoning_content
index.tsx

reasoning_content 会在每次 chunk 追加
UI 会以 Markdown 方式渲染，并有流式指示器
结论：Coze 的“思考过程”不是工具卡，而是独立 reasoning 区块。

6) 卡片渲染（ContentType.Card）
index.tsx
index.tsx

ContentType.Card 只负责card 内容渲染
当前仓库里只支持 option / form_schema
并没有你截图里的“创建文件/更新计划”专用卡
结论：你看到的“创建文件/更新计划”更像是 function_call/tool_response 生成的步骤面板，而不是 workflow 卡片体系。

总结：Coze 的“平滑输出”靠什么？
流式 chunk 拼接（ChunkProcessor）
明确的 waiting/responding 状态机（waiting.ts）
function_call / tool_response 实时插入 + 状态更新
reasoning_content 流式渲染思考过程
execute_display_name 统一标题规范
如果你要对齐 Coze 的体验，你后端要做到的是：

节点开始就发 function_call（带 call_id + execute_display_name）
节点结束就发 tool_response（带 plugin_status）
思考过程使用 reasoning_content 流式推送
不要等最终 YAML 才生成“步骤卡片”
