# Skill 包说明

## 目录结构

```text
skills/
  └── demo-ping/
      ├── skill.yaml
      ├── knowledge/
      │   └── guide.md
      └── scripts/
          └── ping.py
```

## skill.yaml 关键字段

- `name` / `version` / `description`: Skill 标识与描述。
- `profile`: 注入到 System Prompt 的人设与指令。
  - `role`: 角色名称。
  - `instruction`: 行为规范。
- `memory`: 记忆加载策略。
  - `strategy`: 当前支持 `context`。
  - `files`: 以 skill 目录为基准的相对路径。
- `executables`: 可执行工具列表。
  - `type`: `script` / `binary` / `mcp`。
  - `parameters`: JSON Schema，用于参数校验与工具说明。

## Executable 类型

### script
- 通过 `runner` + `path` 执行脚本。
- 请求参数以 JSON 形式写入 stdin，并注入环境变量：
  - `BOPS_ARGS_JSON`
  - `BOPS_ARG_<KEY>`

### binary
- 直接执行二进制，支持固定 `args`。

### mcp
- 启动 MCP Client，通过 `tools/list` 自动注册工具。

## 示例 (demo-ping)

```yaml
name: "demo-ping"
version: "0.1.0"
description: "Simple ping skill for validating the loader and tool adapter."

profile:
  role: "Ping Operator"
  instruction: |
    You are a lightweight network check assistant.
    Always ask for the host if it is missing.

memory:
  strategy: "context"
  files:
    - "knowledge/guide.md"

executables:
  - name: "ping_host"
    type: "script"
    runner: "python3"
    path: "scripts/ping.py"
    parameters:
      type: object
      properties:
        host: { type: string }
        count: { type: integer, default: 1 }
      required: ["host"]
```

## 配置

在 `bops.json` 中声明技能与 Agent:

```json
{
  "claude_skills": ["demo-ping"],
  "agents": [
    {
      "name": "ops-agent",
      "model": "gpt-4o-mini",
      "skills": ["demo-ping"]
    }
  ],
  "tool_conflict_policy": "error"
}
```

支持环境变量覆盖:
- `BOPS_CLAUDE_SKILLS` (逗号分隔)
- `BOPS_AGENTS` (JSON)
- `BOPS_TOOL_CONFLICT_POLICY` (`error` / `overwrite` / `keep` / `prefix`)

## 管理与查看

- API:
  - `GET /api/skills`
  - `POST /api/skills/reload`
  - `GET /api/agents`
- Web 设置页: 展示已加载技能、版本、来源与错误提示。

## 验证终端

在首页执行“沙箱验证”后，可点击“终端详情”进入 `验证终端` 页面查看 stdout/stderr。
终端入口路径: `/validation-console`。
