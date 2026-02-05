# Eino: Cookbook

本文档为 **eino-examples** 项目的示例索引，帮助开发者快速找到所需的示例代码与能力入口。

- Docs 文档：
  - https://www.cloudwego.io/zh/docs/eino/overview/eino_adk0_1
- GitHub 仓库：
  - https://github.com/cloudwego/eino-examples

---

## 📦 ADK (Agent Development Kit)

### Hello World
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| adk/helloworld | Hello World Agent | 最简单的 Agent 示例，展示如何创建一个基础的对话 Agent |

### 入门示例 (Intro)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| adk/intro/chatmodel | ChatModel Agent | 展示如何使用 ChatModelAgent 并配合 Interrupt 机制 |
| adk/intro/custom | 自定义 Agent | 展示如何实现符合 ADK 定义的自定义 Agent |
| adk/intro/workflow/loop | Loop Agent | 展示如何使用 LoopAgent 实现循环反思模式 |
| adk/intro/workflow/parallel | Parallel Agent | 展示如何使用 ParallelAgent 实现并行执行 |
| adk/intro/workflow/sequential | Sequential Agent | 展示如何使用 SequentialAgent 实现顺序执行 |
| adk/intro/session | Session 管理 | 展示如何通过 Session 在多个 Agent 之间传递数据和状态 |
| adk/intro/transfer | Agent 转移 | 展示 ChatModelAgent 的 Transfer 能力，实现 Agent 间的任务转移 |
| adk/intro/agent_with_summarization | 带摘要的 Agent | 展示如何为 Agent 添加对话摘要功能 |
| adk/intro/http-sse-service | HTTP SSE 服务 | 展示如何将 ADK Runner 暴露为支持 SSE 的 HTTP 服务 |

---

## 🧑‍🔧 Human-in-the-Loop (人机协作)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| adk/human-in-the-loop/1_approval | 审批模式 | 敏感操作前人工审批，Agent 执行前需用户确认 |
| adk/human-in-the-loop/2_review-and-edit | 审核编辑模式 | 工具调用参数的人工审核与编辑，支持修改/批准/拒绝 |
| adk/human-in-the-loop/3_feedback-loop | 反馈循环模式 | Writer 生成内容，Reviewer 收集人工反馈，支持迭代优化 |
| adk/human-in-the-loop/4_follow-up | 追问模式 | 识别信息缺失，多轮追问收集需求完成规划 |
| adk/human-in-the-loop/5_supervisor | Supervisor + 审批 | 多 Agent 结合审批，敏感操作需确认 |
| adk/human-in-the-loop/6_plan-execute-replan | 计划执行重规划 + 审核编辑 | Plan-Execute-Replan 结合参数审核编辑 |
| adk/human-in-the-loop/7_deep-agents | Deep Agents + 追问 | Deep Agents 结合追问，在分析前收集用户偏好 |
| adk/human-in-the-loop/8_supervisor-plan-execute | 嵌套多 Agent + 审批 | Supervisor 嵌套 Plan-Execute-Replan 子 Agent，支持深层嵌套中断 |

---

## 🤝 Multi-Agent (多 Agent 协作)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| adk/multiagent/supervisor | Supervisor Agent | 基础 Supervisor 协作模式，协调多个子 Agent |
| adk/multiagent/layered-supervisor | 分层 Supervisor | 多层 Supervisor 嵌套，一个 Supervisor 作为另一个的子 Agent |
| adk/multiagent/plan-execute-replan | Plan-Execute-Replan | 计划-执行-重规划，支持动态调整执行计划 |
| adk/multiagent/integration-project-manager | 项目管理器 | Coder / Researcher / Reviewer 协作示例 |
| adk/multiagent/deep | Deep Agents (Excel Agent) | 智能 Excel 助手，支持 Python 执行 |
| adk/multiagent/integration-excel-agent | Excel Agent (ADK 集成版) | Planner / Executor / Replanner / Reporter 组合 |

---

## 🕸️ GraphTool (图工具)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| adk/common/tool/graphtool | GraphTool 包 | 将 Graph/Chain/Workflow 封装为 Agent 工具 |
| adk/common/tool/graphtool/examples/1_chain_summarize | Chain 文档摘要 | 使用 compose.Chain 实现文档摘要工具 |
| adk/common/tool/graphtool/examples/2_graph_research | Graph 多源研究 | Graph 实现并行多源搜索与流式输出 |
| adk/common/tool/graphtool/examples/3_workflow_order | Workflow 订单处理 | Workflow 处理订单并结合审批 |
| adk/common/tool/graphtool/examples/4_nested_interrupt | 嵌套中断 | 外层审批与内层风控双层中断机制 |

---

## 🔗 Compose (编排)

### Chain (链式编排)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| compose/chain | Chain 基础示例 | compose.Chain 顺序编排，包含 Prompt + ChatModel |

### Graph (图编排)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| compose/graph/simple | 简单 Graph | Graph 基础用法示例 |
| compose/graph/state | State Graph | 带状态的 Graph 示例 |
| compose/graph/tool_call_agent | Tool Call Agent | 使用 Graph 构建工具调用 Agent |
| compose/graph/tool_call_once | 单次工具调用 | 展示单次工具调用 Graph 实现 |
| compose/graph/two_model_chat | 双模型对话 | 两个模型相互对话示例 |
| compose/graph/async_node | 异步节点 | 异步 Lambda 节点，报告生成与实时转录 |
| compose/graph/react_with_interrupt | ReAct + 中断 | 票务预订场景，中断 + checkpoint 实践 |

### Workflow (工作流编排)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| compose/workflow/1_simple | 简单 Workflow | 最简单的 Workflow 示例（等价 Graph） |
| compose/workflow/2_field_mapping | 字段映射 | Workflow 字段映射功能 |
| compose/workflow/3_data_only | 纯数据流 | 仅数据流的 Workflow 示例 |
| compose/workflow/4_control_only_branch | 控制流分支 | 控制流分支示例 |
| compose/workflow/5_static_values | 静态值 | Workflow 中使用静态值 |
| compose/workflow/6_stream_field_map | 流式字段映射 | 流式场景的字段映射 |

---

## 📦 Batch (批处理)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| compose/batch | BatchNode | 批量处理组件，支持并发控制/中断恢复 |

---

## 🌊 Flow (流程模块)

### ReAct Agent
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| flow/agent/react | ReAct Agent | ReAct 基础示例（餐厅推荐） |
| flow/agent/react/memory_example | 短期记忆 | ReAct 短期记忆，支持内存与 Redis |
| flow/agent/react/dynamic_option_example | 动态选项 | 运行时动态修改 Model Option |
| flow/agent/react/unknown_tool_handler_example | 未知工具处理 | 处理模型幻觉工具调用，提升鲁棒性 |

### Multi-Agent
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| flow/agent/multiagent/host/journal | 日记助手 | Host Multi-Agent，写/读日记与问答 |
| flow/agent/multiagent/plan_execute | Plan-Execute | 计划执行模式的 Multi-Agent 示例 |

### 完整应用示例
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| flow/agent/manus | Manus Agent | 基于 Eino 实现的 Manus Agent（参考 OpenManus） |
| flow/agent/deer-go | Deer-Go | 参考 deer-flow，研究团队协作的状态图流转 |

---

## 🧩 Components (组件)

### Model (模型)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/model/abtest | A/B 测试路由 | 动态路由 ChatModel，支持 A/B 测试 |
| components/model/httptransport | HTTP 传输日志 | cURL 风格日志，支持流式响应与脱敏 |

### Retriever (检索器)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/retriever/multiquery | 多查询检索 | LLM 生成多个查询变体，提高召回 |
| components/retriever/router | 路由检索 | 根据查询内容动态路由检索器 |

### Tool (工具)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/tool/jsonschema | JSON Schema 工具 | 使用 JSON Schema 定义工具参数 |
| components/tool/mcptool/callresulthandler | MCP 工具结果处理 | MCP 工具调用结果自定义处理 |
| components/tool/middlewares/errorremover | 错误移除中间件 | 将错误转换为友好提示 |
| components/tool/middlewares/jsonfix | JSON 修复中间件 | 修复 LLM 生成的错误 JSON 参数 |

### Document (文档)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/document/parser/customparser | 自定义解析器 | 自定义文档解析器示例 |
| components/document/parser/extparser | 扩展解析器 | HTML 等格式扩展解析 |
| components/document/parser/textparser | 文本解析器 | 基本文本文档解析 |

### Prompt (提示词)
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/prompt/chat_prompt | Chat Prompt | Chat Prompt 模板示例 |

### Lambda
| 目录 | 名称 | 说明 |
| --- | --- | --- |
| components/lambda | Lambda 组件 | Lambda 函数组件示例 |

---

## 🚀 QuickStart (快速开始)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| quickstart/chat | Chat 快速开始 | 基础对话示例（模板/生成/流式） |
| quickstart/eino_assistant | Eino 助手 | 完整 RAG 示例（索引/Agent/服务/Web） |
| quickstart/todoagent | Todo Agent | 简单 Todo 管理 Agent |

---

## 🛠️ DevOps (开发运维)

| 目录 | 名称 | 说明 |
| --- | --- | --- |
| devops/debug | 调试工具 | 支持 Chain / Graph 调试 |
| devops/visualize | 可视化工具 | Graph/Chain/Workflow 渲染为 Mermaid |

