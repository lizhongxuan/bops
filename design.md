# Claude Skills 扩展方案设计

目标: 让项目支持 Claude Skills, 添加新 Agent 只需导入一个新的 Skill 包, 无需修改核心代码。

## 背景与痛点
- 目前 Agent 能力主要依赖内置逻辑, 扩展新能力需要改动代码。
- Claude Skills 已形成可复用的技能包模式, 适合按需装配。
- 需要一套标准化的 Skill 包规范 + 运行时装配机制。

## 目标
- 通过配置导入 Skill 包, 自动生成可用 Agent。
- 技能包即插件, 支持版本化与可追溯。
- Skill 与工作流/工具体系可组合, 运行时动态加载。

## 非目标
- 不在此阶段提供在线技能市场。
- 不引入强依赖的外部运行时, 保持本地优先。

## 核心设计
### 1) Skill 包规范
Skill 包采用目录形式, 包含 manifest 与实现文件。

推荐结构:
```
skills/
  ops-deploy/
    skill.json
    prompts/
      system.md
      tools.md
    tools/
      deploy.yaml
    assets/
      icon.svg
```

skill.json 示例:
```json
{
  "name": "ops-deploy",
  "version": "0.1.0",
  "description": "部署类运维技能",
  "provider": "claude",
  "entry": "prompts/system.md",
  "tools": ["tools/deploy.yaml"],
  "permissions": ["workflow.read", "workflow.write", "env.read"]
}
```

### 2) Agent 配置即装配
Agent 由多个 Skill 包组装而成, 启动时自动生成。

配置示例 (bops.json):
```json
{
  "claude_skills": [
    "./skills/ops-deploy",
    "./skills/monitoring"
  ],
  "agents": [
    {
      "name": "ops",
      "model": "claude-3.5-sonnet",
      "skills": ["ops-deploy", "monitoring"]
    }
  ]
}
```

新增 Agent 流程:
1) 导入 Skill 包到 `skills/` 或指定路径
2) 在配置中声明 skill
3) 重启服务, 自动生效

### 3) 运行时组件
- SkillLoader: 解析配置并加载技能包 (本地/压缩包/git)。
- SkillRegistry: 去重、版本管理、缓存。
- AgentFactory: 按配置组装 Agent, 生成工具集合与提示词。
- ToolAdapter: 将 skill 工具描述映射到 Eino 节点/工具协议。

## Claude Skills 兼容策略
Skill 包内容映射到 Claude Tools 规范:
- name/description/input_schema -> 工具声明
- prompts/system.md -> 系统提示词注入
- tools/*.yaml -> tool 定义与执行适配

处理规则:
- 同名工具冲突: 后加载覆盖或拒绝 (通过策略配置)。
- 权限控制: 每个 skill 声明 permissions, 决定可调用的接口集合。

## API 与管理能力
建议新增:
- `GET /api/skills` 列出已加载技能
- `POST /api/skills/reload` 重新加载技能包
- `GET /api/agents` 列出可用 Agent

## 安全与隔离
- Skill 包只允许访问白名单 API。
- 可设置 `read_only` 与 `restricted` 两种运行模式。
- 记录 skill 的调用审计, 便于回溯。

## 数据结构建议
```go
type SkillManifest struct {
  Name        string
  Version     string
  Description string
  Provider    string
  Entry       string
  Tools       []string
  Permissions []string
}
```

## 新增需求: 验证与终端回显
### 1) 验证执行 (Serverless 容器)
- 验证功能启动短生命周期的 serverless 容器, 用于运行工作流。
- 容器内执行工作流步骤, 终端输出与状态回传到 bops, 用于展示与审计。

### 2) 终端页面与实时交互
- 验证过程中提供终端页面, 实时展示 AI 与容器的交互内容。
- UI 建议标注来源 (AI/容器/系统) 与步骤, 便于定位问题。
- 日志以流式方式推送到前端 (SSE/WebSocket 均可), 保证低延迟可视化。

## 里程碑任务
- [ ] 定义 skill.json 与工具描述 schema
- [ ] 实现 SkillLoader + SkillRegistry
- [ ] AgentFactory 装配与热加载
- [ ] Claude Tools 适配层
- [ ] API 与可视化管理页
