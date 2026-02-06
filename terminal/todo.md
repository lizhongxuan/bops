# Terminal 实现任务清单（基于 terminal/design.md）

> 目标：按任务逐步落地“页面连接 Agent，实时观测终端输入输出与 Agent 交互”能力。  
> 使用方式：完成一项勾选一项，严格按“完成标准”验收。

## Phase 0：方案冻结与边界确认

- [ ] T00-01 冻结 V1 范围与不做项  
  描述：确认只做单主机单会话、controller/viewer、实时输入输出、Agent 交互时间线、事件回放。  
  产出：范围确认记录（含 V1 不包含项）。  
  完成标准：评审确认后，不再新增 V1 范围外需求。

- [ ] T00-02 冻结事件协议（Envelope + type + payload）  
  描述：定义 `terminal_input`、`terminal_output`、`terminal_resize`、`agent_action_*`、`presence_*` 等事件结构。  
  产出：协议文档（字段、示例、错误码、版本号）。  
  完成标准：前后端按同一协议联调通过。

- [ ] T00-03 冻结接口契约（HTTP + WebSocket）  
  描述：明确会话创建、查询、关闭、事件拉取、WS 实时流入口与消息格式。  
  产出：接口文档 + 示例请求响应。  
  完成标准：联调时无接口字段歧义。

## Phase 1：服务端 Terminal Hub 最小闭环

- [ ] T01-01 建立 Terminal 模块骨架  
  描述：在服务端创建会话管理、流转发、事件写入、回放服务的基础目录与接口。  
  产出：可编译的模块骨架代码。  
  完成标准：模块可启动，空实现可通过基础编译/启动检查。

- [ ] T01-02 实现 SessionManager（创建/查询/关闭）  
  描述：支持 session 生命周期管理，字段至少包含 `session_id`、`agent_id`、`status`、`owner`、`controller`。  
  产出：会话管理实现 + 单元测试。  
  完成标准：创建、查询、关闭三类流程可通过测试。

- [ ] T01-03 实现 HTTP 会话接口  
  描述：落地 `POST /api/terminal/sessions`、`GET /api/terminal/sessions/{id}`、`POST /api/terminal/sessions/{id}/close`。  
  产出：HTTP handler + 请求参数校验。  
  完成标准：接口返回结构与协议一致，异常场景有明确错误码。

- [ ] T01-04 实现 StreamBroker（会话内实时广播）  
  描述：支持同一 session 下 controller/viewer 广播，保证事件按 `seq` 单调。  
  产出：Broker 实现 + 并发单测。  
  完成标准：多连接场景下无乱序、无明显消息丢失。

- [ ] T01-05 实现 WS 流入口与基础会话握手  
  描述：落地 `GET /api/terminal/sessions/{id}/stream`，支持 `auth.init`、`session.ready`、heartbeat。  
  产出：WS handler + 连接管理。  
  完成标准：浏览器可稳定建立连接并保持在线。

## Phase 2：Agent PTY Bridge 接入

- [ ] T02-01 实现 Agent 侧 PTYManager  
  描述：支持在 Agent 主机拉起 PTY（shell）、读写 stdin/stdout/stderr、处理 resize。  
  产出：PTY 管理实现 + 基础测试。  
  完成标准：本地可启动 shell 并正确收发输入输出。

- [ ] T02-02 实现 Hub -> Agent 会话打开链路  
  描述：会话创建后，Hub 调 Agent 执行 `open_session`，拿到 `pty_id`。  
  产出：会话打开调用链 + 错误处理。  
  完成标准：Agent 在线时可成功创建远程终端会话。

- [ ] T02-03 实现终端输入链路（Browser -> Hub -> Agent -> PTY）  
  描述：打通 `terminal.input` 全链路，支持 base64 字节流透传。  
  产出：输入转发实现。  
  完成标准：页面输入可在远端 shell 实际执行并回显。

- [ ] T02-04 实现终端输出链路（PTY -> Agent -> Hub -> Browser）  
  描述：PTY 输出切片、封包、分配 `seq` 后实时推送到浏览器。  
  产出：输出事件流实现。  
  完成标准：终端输出延迟满足内网 100-300ms 目标。

- [ ] T02-05 实现 resize 链路  
  描述：打通 `terminal.resize`，将页面窗口大小同步到 PTY。  
  产出：resize 转发实现。  
  完成标准：页面缩放后远端终端排版正确刷新。

## Phase 3：Agent 交互事件并流

- [ ] T03-01 定义 Agent 动作事件标准  
  描述：统一 `agent_action_start`、`agent_action_log`、`agent_action_end` 字段（action、status、duration、exit_code）。  
  产出：事件字段定义与示例。  
  完成标准：不同动作类型可用同一结构上报。

- [ ] T03-02 Agent Runtime 事件采集  
  描述：在 Agent 执行器中打点动作开始/过程/结束并上报 Hub。  
  产出：事件采集实现。  
  完成标准：动作时间线可完整还原一次执行过程。

- [ ] T03-03 Hub 侧并流与广播  
  描述：将 Agent 事件并入 session 事件流并广播到所有 viewer。  
  产出：并流处理逻辑。  
  完成标准：终端输出与动作事件可按时间关联展示。

## Phase 4：持久化、回放、断线重连

- [ ] T04-01 建表与数据访问层  
  描述：实现 `terminal_sessions`、`terminal_events`、`terminal_presence` 的存储与查询。  
  产出：DDL/Migration + DAO。  
  完成标准：核心查询（按 session、按 seq）性能可用。

- [ ] T04-02 事件持久化写入  
  描述：所有关键事件写库，保证 `session_id + seq` 可检索。  
  产出：EventWriter 实现。  
  完成标准：断线后可从库中恢复事件。

- [ ] T04-03 历史事件拉取接口  
  描述：实现 `GET /api/terminal/sessions/{id}/events?from_seq=`。  
  产出：回放接口实现。  
  完成标准：给定 `from_seq` 可补齐缺失事件。

- [ ] T04-04 WS 重连补偿  
  描述：客户端带 `last_seq` 重连，服务端补发缺口 + 续流。  
  产出：重连补偿逻辑。  
  完成标准：30 秒内断线重连后，不丢关键事件。

- [ ] T04-05 慢消费者背压策略  
  描述：慢连接超过阈值后降级为“快照 + 增量”，避免拖垮主广播。  
  产出：背压/降级机制。  
  完成标准：单个慢 viewer 不影响其他连接实时性。

## Phase 5：权限、安全、审计

- [ ] T05-01 会话级鉴权 Token  
  描述：实现短期 token（5-15 分钟）签发与校验。  
  产出：token 签发/验证逻辑。  
  完成标准：无 token 或过期 token 无法接入 WS。

- [ ] T05-02 RBAC 权限校验  
  描述：落地 `terminal:view`、`terminal:control`、`terminal:audit` 三类权限。  
  产出：接口与事件流入口统一鉴权中间件。  
  完成标准：viewer 不能发送输入，只有 controller 可控制终端。

- [ ] T05-03 控制权切换机制  
  描述：实现 `session.take_control`、`controller_changed` 与冲突处理策略。  
  产出：控制权管理逻辑。  
  完成标准：切换过程可观测、无双 controller 冲突。

- [ ] T05-04 敏感信息脱敏  
  描述：对输出和动作事件执行脱敏规则（密码、token、私钥片段）。  
  产出：可配置脱敏器。  
  完成标准：审计与回放中不出现明文敏感值。

- [ ] T05-05 审计查询能力  
  描述：可查询会话创建、加入离开、控制权变更、输入事件。  
  产出：审计查询接口或管理页。  
  完成标准：能按 `session_id` 完整追溯关键行为。

## Phase 6：前端页面实现

- [ ] T06-01 新增 Terminal 页面路由与骨架  
  描述：完成顶栏、终端主区、右侧时间线、底部状态条布局。  
  产出：页面框架与基础状态管理。  
  完成标准：页面可打开并显示会话基础信息。

- [ ] T06-02 集成 xterm 渲染终端  
  描述：接入 xterm，支持输出渲染、键盘输入、复制粘贴、光标行为。  
  产出：终端组件实现。  
  完成标准：页面内可完成基础 shell 交互。

- [ ] T06-03 接入 WS 实时协议  
  描述：实现 `auth.init`、`terminal.output`、`terminal.input`、`resize`、heartbeat。  
  产出：WS 客户端与重连策略。  
  完成标准：长连接稳定，断线自动恢复。

- [ ] T06-04 实现 Agent 交互时间线  
  描述：实时展示 `agent_action_*` 事件，含状态、耗时、摘要。  
  产出：时间线组件。  
  完成标准：点击时间线条目可定位相关输出片段。

- [ ] T06-05 实现 viewer/controller 交互  
  描述：默认 viewer，只读展示；支持申请控制并切换状态提示。  
  产出：角色 UI 与状态提示。  
  完成标准：权限变化实时反馈，交互无歧义。

- [ ] T06-06 实现“跟随模式”与历史查看  
  描述：支持自动滚动到最新输出，也可暂停滚动查历史。  
  产出：终端滚动控制逻辑。  
  完成标准：高频输出下仍可稳定浏览历史内容。

## Phase 7：测试与压测

- [ ] T07-01 单元测试补齐  
  描述：覆盖 SessionManager、StreamBroker、EventWriter、权限控制。  
  产出：单测用例。  
  完成标准：关键模块分支覆盖达标（按团队标准）。

- [ ] T07-02 集成测试（Hub + Agent + PTY）  
  描述：验证创建会话、输入输出、动作事件、关闭会话完整链路。  
  产出：集成测试脚本。  
  完成标准：主链路端到端自动化可重复通过。

- [ ] T07-03 多 viewer 并发测试  
  描述：验证 1 controller + 多 viewer 广播一致性。  
  产出：并发测试报告。  
  完成标准：广播稳定，无大面积乱序和掉线。

- [ ] T07-04 断线重连与补偿测试  
  描述：模拟网络抖动，验证 `last_seq` 补偿和续流正确性。  
  产出：故障场景测试报告。  
  完成标准：满足“30 秒内重连不丢关键事件”验收标准。

- [ ] T07-05 性能压测  
  描述：压测并发会话、输出吞吐、慢消费者场景。  
  产出：压测报告与瓶颈分析。  
  完成标准：达到设计目标或给出可执行优化项。

## Phase 8：上线准备

- [ ] T08-01 监控指标与告警  
  描述：增加连接数、消息延迟、丢包率、重连率、Agent 健康等指标。  
  产出：监控看板 + 告警规则。  
  完成标准：异常能被及时发现并定位。

- [ ] T08-02 运维 Runbook  
  描述：整理会话故障排查、Agent 离线处理、数据回放操作手册。  
  产出：运维文档。  
  完成标准：值班同学可按文档完成常见问题处置。

- [ ] T08-03 灰度发布与回滚方案  
  描述：先小流量灰度，确认稳定后全量；准备快速回滚策略。  
  产出：发布计划与回滚预案。  
  完成标准：灰度期无 P0/P1 事故后全量上线。

## 里程碑验收（必须全部满足）

- [ ] M1 页面可连接 Agent 终端并实时看到输入输出。  
- [ ] M2 可实时看到 Agent 与终端交互时间线。  
- [ ] M3 多 viewer 可同时旁观且体验一致。  
- [ ] M4 断线重连后可按 `seq` 恢复，不丢关键事件。  
- [ ] M5 关键审计信息可查询、可追溯。
