# Agent Skills 扩展架构设计

* **版本**: 2.0
* **核心理念**: Spec-Driven Development (规范驱动开发)
* **技术栈**: Eino (CloudWeGo), Go, MCP (Model Context Protocol), YAML

## 1. 核心架构理念

在 Eino 框架中，支持 Claude Skills 的本质是将磁盘上的**“静态文件”**（配置、文档、脚本）动态转化为运行时的 **Eino Graph 节点配置**。

一个完整的 Skill 定义为三个核心要素的集合：

* **Profile (人设/说明书)**: 定义 Agent 的 System Prompt，告诉 LLM 这个技能的边界和用途。
* **Memory (第一层记忆)**: 必要的领域知识文件（如错误码表、API文档、操作手册），用于 RAG 检索或直接注入 Context。
* **Executable (执行体)**: 实际的业务逻辑落地，表现为本地脚本（Python/Shell）、二进制工具或 MCP Server 连接。

## 2. Skill 包标准规范 (The Spec)

Skill 包不再仅仅是工具的集合，而是微型 Agent 的完整定义。所有定义收敛于 `skill.yaml`。

### 2.1 目录结构

```text
skills/
  └── ops-deploy/              # Skill ID
      ├── skill.yaml           # 核心定义文件 (OpenSpec 风格)
      ├── knowledge/           # [Memory] 静态知识库
      │   ├── deploy_guide.md
      │   └── error_codes.txt
      └── scripts/             # [Executable] 执行脚本
          ├── deploy.py
          └── rollback.sh

```

### 2.2 skill.yaml 定义示例

此 YAML 充当 Eino Loader 的蓝图。

```yaml
name: "ops-deploy"
version: "1.0.0"
description: "负责应用部署、回滚及状态检查的运维专家"

# --- 要素 1: Profile (人设) ---
profile:
  role: "Deployment Specialist"
  instruction: |
    你是一个负责 Kubernetes 应用部署的专家。
    在执行部署前，必须先使用 'check_status' 确认环境状态。
    遇到错误时，请参考知识库中的错误码进行解释。

# --- 要素 2: Memory (记忆) ---
memory:
  # 加载策略: 'context' (直接注入 System Prompt) 或 'rag' (向量检索)
  strategy: "context" 
  files:
    - "knowledge/error_codes.txt"
    - "knowledge/deploy_guide.md"

# --- 要素 3: Executable (执行体/工具) ---
executables:
  # 类型 A: 本地脚本 (通过 Eino Generic Executor 调用)
  - name: "deploy_service"
    description: "Execute deployment script for a specific service."
    type: "script"
    runner: "python3"
    path: "scripts/deploy.py"
    parameters: # OpenSpec/JSON Schema 定义
      type: object
      properties:
        service_name: { type: string }
        image_tag: { type: string }
      required: ["service_name"]

  # 类型 B: 现有二进制工具
  - name: "check_cluster_health"
    description: "Ping the cluster to check connectivity."
    type: "binary"
    path: "/usr/local/bin/kubectl"
    args: ["cluster-info"]

  # 类型 C: MCP Server (高级扩展)
  - name: "git_ops"
    type: "mcp"
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-github"]

```

## 3. 核心组件设计 (Eino Integration)

### 3.1 EinoSkillLoader (动态加载器)

这是系统的核心引擎，负责读取 YAML 并实例化 Eino 组件。

**工作流程:**

1. **Parse**: 读取 `skill.yaml`，校验结构合法性。
2. **Build Memory**:
* 读取 `memory.files` 中的内容。
* 如果是 `strategy: context`，将文本拼接准备注入 System Prompt。
* 如果是 `strategy: rag`，初始化 Eino Retriever 并建立临时索引。


3. **Build Tools**:
* 遍历 `executables` 列表。
* 对于 `script/binary` 类型：封装为 Eino Tool（使用 `os/exec` 的通用桥接器）。
* 对于 `mcp` 类型：启动 MCP Client，通过握手协议动态获取 Tool 列表并转换。


4. **Construct**: 返回一个 `LoadedSkill` 对象。

```go
type LoadedSkill struct {
    SystemMessage *schema.Message // 包含 Profile + Memory
    Tools         []tool.Tool     // 包含封装好的 Executable
}

```

### 3.2 动态激活 (Runtime Activation)

在 `bops` 主程序中，Agent 不再是硬编码的，而是根据配置动态组装。

```go
// 伪代码示例
func BuildDynamicAgent(skillName string) (*eino.Runnable, error) {
    // 1. Loader 工作
    loader := NewSkillLoader("./skills")
    skillData, _ := loader.Load(skillName)

    // 2. Eino Agent 组装
    // 将 Skill 的要素注入到 ReAct 或其他 Graph 模式中
    agent, _ := react.NewAgent(ctx, &react.AgentConfig{
        Model:        chatModel,
        SystemPrompt: skillData.SystemMessage.Content, // 注入 Profile & Memory
        Tools:        skillData.Tools,                 // 注入 Executable
    })
    
    return agent, nil
}

```

## 4. MCP-UI 与终端交互扩展

为了实现“交互式智能终端”的目标，`Executable` 的返回结果需要被特殊处理。

### 4.1 UI 协议扩展

Skill 中的脚本 (`scripts/*.py`) 执行后，除了返回文本，还可以返回结构化的 **UI Resource**。

* **约定**: 脚本输出标准 JSON。
* **格式**:

```json
{
  "type": "ui_resource",
  "component": "deployment-progress",
  "data": { "service": "api-gateway", "progress": 85, "status": "pulling_image" }
}

```

### 4.2 验证与回显 (Bops Terminal)

* **Serverless 容器运行**: 当 `Executable` 被调用时，若配置了隔离要求，bops 启动临时容器运行脚本。
* **流式回显**:
* 容器的 `Stdout/Stderr` -> SSE/WebSocket -> 前端 Terminal 组件。
* 前端识别到 JSON 中的 `ui_resource` 标记，自动渲染为 React 组件（如进度条、状态卡片），而非纯文本。



## 5. 里程碑任务分解 (Roadmap)

**Phase 1: 基础框架 (The Skeleton)**

* [ ] 定义 `skill.yaml` 的 JSON Schema 规范。
* [ ] 实现 `EinoSkillLoader`：
* [ ] 支持 YAML 解析。
* [ ] 支持 `context` 模式的 Memory 加载。
* [ ] 实现基础的 Script -> Eino Tool 通用适配器。


* [ ] 编写一个示例 Skill (`skills/demo-ping`) 进行验证。

**Phase 2: 运行时增强 (The Brain)**

* [ ] 支持 `mcp` 类型的 Executable，实现 Eino MCP Client。
* [ ] 实现 Agent 的热重载机制 (Config Watcher)。
* [ ] 实现 `Profile` 的模板渲染 (支持动态变量注入)。

**Phase 3: 交互升级 (The Face)**

* [ ] 扩展 Tool 返回协议，支持 `UIResource`。
* [ ] 前端集成 MCP-UI SDK，实现组件动态渲染。
* [ ] 实现脚本执行日志到前端 Terminal 的流式传输。

## 6. 总结

此方案将 Agent 的开发模式从 **“写 Go 代码”** 转变为 **“写 Spec 文档 + 脚本”**。

* **Profile** 赋予了 Agent 灵魂。
* **Memory** 赋予了 Agent 知识。
* **Executable** 赋予了 Agent 手脚。

Eino Loader 则是将这三者粘合起来的魔法胶水。