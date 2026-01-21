# 新功能方案设计 (AI 首页 / 验证环境 / 脚本库 / Script Actions)

目标: 在现有工作流平台基础上新增 AI 首页、验证环境与脚本库能力，并通过 `script.shell` / `script.python` 动作扩展可执行内容。

## 1. 信息架构与导航
- 菜单调整:
  - 首页 (替代原“总览”)
  - 工作流
  - 环境变量包
  - 验证环境
  - 脚本库
  - 运行记录
  - 设置

## 2. 首页: AI 对话 + 工作流编辑器
### 2.1 页面布局
- 左侧: AI 对话区
  - 消息列表 + 输入框
  - 预置按钮: 生成模板 / 解释 / 修复 / 校验 / 在验证环境测试
- 右侧: 工作流编辑器
  - YAML 编辑器 + 结构预览 + 校验结果
  - 生成结果可直接落盘

### 2.2 交互流程
1) 用户在对话框描述需求
2) 服务端调用 LLM, 返回 YAML
3) YAML 填入编辑器, 用户可二次编辑
4) 点击校验/测试, 生成结果与错误定位
5) 一键保存为工作流模板

### 2.3 数据模型 (建议)
- ChatSession:
  - `id`, `title`, `created_at`, `messages[]`
- ChatMessage:
  - `role: system|user|assistant`, `content`, `created_at`
- WorkflowDraft:
  - `session_id`, `yaml`, `validation`, `last_saved_at`

### 2.4 API 设计 (建议)
- `POST /api/ai/chat/sessions` 新建会话
- `GET /api/ai/chat/sessions/{id}` 获取会话
- `POST /api/ai/chat/sessions/{id}/messages` 追加消息并返回 AI 回复
- `POST /api/ai/workflow/generate` 只生成 YAML (无需聊天)
- `POST /api/ai/workflow/fix` 根据报错修复 YAML
- `POST /api/ai/workflow/stream` SSE 流式生成/修复 (推荐用于首页 AI 助手)

示例请求 (generate):
```json
{
  "prompt": "生成一个 nginx 部署工作流",
  "context": {
    "env_packages": ["prod-env"],
    "scripts": ["install-nginx"]
  }
}
```

示例响应:
```json
{
  "yaml": "version: v0.1\\nname: deploy-nginx\\n..."
}
```

流式 result payload (event: result):
```json
{
  "yaml": "version: v0.1\\nname: deploy-nginx\\n...",
  "summary": "steps=3 risk=low issues=0",
  "issues": [],
  "risk_level": "low",
  "needs_review": false,
  "questions": ["Which hosts should this run on?"],
  "history": [],
  "draft_id": "draft-123"
}
```

### 2.5 LLM 接入策略
- 目标: 支持多供应商 (Deepseek / OpenAI / Gemini)。
- 方案: provider 抽象 + 配置切换。
  - `provider`: `deepseek` | `openai` | `gemini`
  - `api_key`, `base_url` (可选)
  - `model` (例如 `gpt-4o-mini` / `deepseek-chat` / `gemini-1.5-pro`)
- 运行时: 基于配置选择 provider 实现, 提供统一的 `Chat()` 接口。

配置示例:
```json
{
  "ai_provider": "openai",
  "ai_api_key": "sk-***",
  "ai_base_url": "https://api.openai.com/v1",
  "ai_model": "gpt-4o-mini"
}
```

### 2.6 Prompt 策略
- 使用 `docs/prompt-workflow.md` 作为基础提示词
- 返回必须是 YAML, 失败时返回错误原因
- 将环境变量包/脚本库/验证环境列表作为上下文注入

## 3. 验证环境 (容器/虚拟机)
### 3.1 目标
- AI 可在指定环境中执行 `test`/`apply` 验证工作流
- 多环境并行验证兼容性

### 3.2 环境类型
- Container (Docker/Podman) - 优先支持
- VM/SSH (远程虚拟机)
- Agent (在主机部署 bops-agent 执行命令)

### 3.3 数据模型 (建议)
- ValidationEnv:
  - `name`, `type: container|vm`
  - `image` / `host`, `user`, `ssh_key`
  - `labels`, `description`, `created_at`, `updated_at`
  - `default_inventory` (可选)

### 3.4 存储与 API
- 存储: `data/validation_envs/*.yaml`
- API:
  - `GET /api/validation-envs`
  - `GET /api/validation-envs/{name}`
  - `PUT /api/validation-envs/{name}`
  - `DELETE /api/validation-envs/{name}`

示例响应:
```json
{
  "name": "staging-docker",
  "type": "container",
  "description": "staging env",
  "labels": {"team": "ops"},
  "image": "bops-agent:latest"
}
```

### 3.5 执行方式
- 容器模式:
  - 复用 `internal/tester` (DockerRunner)
  - 拉起 `bops-agent` 容器执行测试
- VM 模式:
  - 通过 SSH 远程启动 agent
- Agent 模式:
  - 通过 `agent_listen` 注册已有主机
  - Server 分发执行任务到 agent

### 3.6 AI 验证流程
- 用户在首页选择验证环境
- AI 触发 `POST /api/validation-runs`
- 返回验证结果 + 日志 + 失败步骤

## 4. 脚本库
### 4.1 目标
- 保存可复用脚本片段 (shell/python)
- 供 `script.shell` / `script.python` 调用

### 4.2 数据模型 (建议)
- Script:
  - `name`, `language: shell|python`
  - `content`, `description`, `tags`, `updated_at`

### 4.3 存储与 API
- 存储: `data/scripts/*.yaml`
- API:
  - `GET /api/scripts`
  - `GET /api/scripts/{name}`
  - `PUT /api/scripts/{name}`
  - `DELETE /api/scripts/{name}`

示例响应:
```json
{
  "name": "install-nginx",
  "language": "shell",
  "description": "install nginx",
  "tags": ["nginx", "setup"],
  "content": "apt-get update && apt-get install -y nginx"
}
```

### 4.4 前端页面
- 脚本列表 + 编辑区
- 语言切换、标签、搜索
- 可插入模板引用脚本库

## 5. 新动作: script.shell / script.python
### 5.1 with 参数
- `script`: 直接脚本文本 (可选)
- `script_ref`: 脚本库名称 (可选)
- `args`: 参数数组 (可选)
- `env`: 环境变量 (可选)
- `dir`: 工作目录 (可选)

规则:
- `script` 与 `script_ref` 二选一
- `script_ref` 从脚本库读取内容

### 5.2 执行行为
- `script.shell`: /bin/sh -c 执行
- `script.python`: python3 执行
- 输出写入 stdout/stderr

## 6. 工作流关联字段
- `env_packages`: 关联环境变量包 (已实现)
- 新增 `validation_env`: 默认验证环境 (可选)
- 步骤内可用 `env.set` 动态注入环境变量

## 7. 安全与权限
- 脚本执行需 UI 二次确认
- 验证环境只允许在受控容器/VM 运行
- API 增加 RBAC 与访问审计 (后续)

## 8. 迁移与兼容
- 旧 workflow 不受影响
- 新字段为可选
