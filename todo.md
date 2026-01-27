# 任务清单 (基于 design.md)

## Phase 0: 规范与基础准备
- [ ] 定义 `skill.yaml` 的 JSON Schema (字段: name/version/description/profile/memory/executables; 校验规则: 必填/类型/路径相对/文件存在; 版本兼容策略与错误提示模板)
- [ ] 确定技能包目录约定与路径解析规则 (默认 `./skills`; 支持配置路径; 允许相对/绝对路径; 冲突处理策略)
- [ ] 设计 Skill 加载错误的返回结构 (包含 skill 名称、文件路径、出错字段、修复建议)

## Phase 1: Skill Loader (基础框架)
- [ ] 实现 `EinoSkillLoader` 读取 `skill.yaml` 并做结构校验 (使用 JSON Schema; 返回结构化错误)
- [ ] 实现 `LoadedSkill` 数据结构 (SystemMessage + Tools; 记录来源 skill 信息与版本)
- [ ] 实现 `memory.strategy=context` 的加载 (读取 `memory.files`; 支持多文件拼接; 支持字符集与空文件处理)
- [ ] 实现 `executables.type=script` 的适配 (命令构造: runner + path + args; 参数校验与注入; stdout/stderr 捕获)
- [ ] 实现 `executables.type=binary` 的适配 (支持固定 args; 提供环境变量与工作目录配置)
- [ ] 提供示例 Skill `skills/demo-ping` (包含 profile、memory、script; 用于端到端验证)

## Phase 2: 运行时装配 (Agent 动态化)
- [ ] 实现 `SkillRegistry` (按 name/version 去重; 支持缓存与刷新; 记录加载来源)
- [ ] 实现 `AgentFactory` (按配置组装: profile + memory + tools; 支持多 skill 组合与排序)
- [ ] 增加配置项 `claude_skills` 与 `agents` (读取 `bops.json`; 支持环境变量覆盖; 校验配置合法性)
- [ ] 支持 Agent 热重载 (文件监听 skills 目录与配置变更; reload 时保证不中断现有请求)
- [ ] 支持 Profile 模板渲染 (注入运行时变量: env 列表/脚本库/验证环境/用户上下文)

## Phase 3: MCP 执行体与工具扩展
- [ ] 实现 `executables.type=mcp` (启动 MCP Client; 握手获取工具列表; 映射为 Eino Tool)
- [ ] 设计工具冲突策略 (同名工具覆盖/拒绝/前缀化; 配置化策略开关)
- [ ] 增加权限模型 (skill permissions -> API 白名单; 拒绝未授权调用并记录审计)

## Phase 4: 验证与终端回显 (Serverless 容器)
- [ ] 设计 serverless 容器执行流程 (创建容器->注入工作流与脚本->执行->回收; 失败重试与超时策略)
- [ ] 实现验证执行器与适配层 (将 workflow step 调度到容器; 捕获 stdout/stderr)
- [ ] 增加终端日志流式通道 (SSE 或 WebSocket; 消息格式包含时间/来源/step/level)
- [ ] 前端新增终端页面 (验证过程中实时展示 AI 与容器交互; 支持过滤/暂停/复制)
- [ ] 日志回传与审计落盘 (保存原始流; 关联 workflow/run_id; 支持检索)

## Phase 5: MCP-UI 与交互组件
- [ ] 扩展工具返回协议 (支持 `ui_resource` JSON; 标准字段: component/data)
- [ ] 前端集成 UI 渲染 (识别 ui_resource; 使用 MCP-UI SDK 渲染组件或降级为文本)
- [ ] 设计并实现示例 UI 组件 (如部署进度、风险提示、步骤对齐)

## Phase 6: API 与管理界面
- [ ] 新增技能/Agent 管理 API (`GET /api/skills`, `POST /api/skills/reload`, `GET /api/agents`)
- [ ] 在设置页增加 Skill/Agent 配置入口 (展示已加载技能、版本、来源与错误提示)
- [ ] 增加验证终端入口 (从工作流/验证结果页跳转到终端详情)

## Phase 7: 测试与文档
- [ ] 单元测试: skill.yaml 校验、Memory 加载、Tool 适配器参数注入
- [ ] 集成测试: demo skill 端到端执行 (profile + memory + script)
- [ ] 验证流程测试: 容器执行 + 日志流式回显
- [ ] 文档完善: Skill 包规范、示例、配置说明、终端页面说明
