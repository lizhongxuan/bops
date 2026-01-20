# 智能运维引擎设计

## 核心设计
- 1.工作流: 运维任务是线性的,通过yaml构建运维步骤,server能分发给agent去执行.
- 2.有界面: 能编排,看到执行流程,能看到每个步骤的交互.
- 3.测试系统: 能起一个容器agent来测试工作流是否有问题,报错后能显示哪个步骤出错,报错信息是什么.
- 4.AI修复: 能拆解用户需求成一个工作流; 基于测试系统的报错信息来修复.
- 5.可移植: 工作流可以导出yaml,在其他地方运行.
- 6.模板市场: 在本地安装二进制后,可以下载模板工具来本地运行.
- 7.MCP: 对MCP提供运维原子操作.
- 8.适配: 用golang实现功能,代替常用的linux命令,解决对环境的依赖.
- 9.人机协同：AI 负责写（生成 YAML）和 诊（分析报错），人负责 审（确认执行）和 改（调整逻辑）。

## 解决痛点
- 1.运维流程可视化可编辑可移植.
- 2.运维工具功能统一管理.
- 3.运维操作自动化生成.
- 4.自动化测试运维操作,自动化适配不同操作系统.

## AI 工作流助手快速使用
- 打开 `http://localhost:5173/`，在首页输入需求并生成 YAML。
- 通过“校验 / 沙箱验证”确认结果，风险为 `high` 时需要人工确认并填写原因。
- 保存后自动跳转到工作流编排页；更多说明见 `docs/usage.md`。

示例 prompt:
```
在 web1/web2 上安装 nginx，渲染配置并启动服务
```

生成 YAML 样例:
```yaml
version: v0.1
name: deploy-nginx
steps:
  - name: install nginx
    targets: [web]
    action: pkg.install
    with:
      name: nginx
```

## AI 运维应用模块

使用 `cloudwego/eino` 的强类型 `Graph` (图) 编排能力可以完美控制“生成 -> 验证 -> 修复”的循环逻辑。

以下是基于 Eino 的架构设计和核心代码实现方案。

---

### 1. 架构设计：自我修正循环 (The Self-Correction Loop)

我们需要构建一个包含 **状态机 (State Graph)** 的应用。

**核心节点 (Nodes):**

1. **Generator (生成器)**: 接收用户自然语言需求，生成初始的运维工作流（Workflow）。
2. **Executor (执行/验证器)**: 调用你现有的执行引擎，在容器中运行工作流，捕获日志和退出码。
3. **Fixer (修复器)**: 如果执行失败，接收“失败的工作流 + 错误日志”，生成修正后的新工作流。

**状态流转 (Edges):**

* `Start` -> `Generator` -> `Executor`
* `Executor` -> **判断逻辑**:
* 如果 **成功**: -> `End` (输出最终结果)
* 如果 **失败**: -> `Fixer` -> `Executor` (进入重试循环)



---
### 2.默认大模型使用deepseek


---

### 3. Eino 实现代码详解

这里使用 `eino/compose` 来编排图，并使用 `Lambda` 组件来封装你的业务逻辑。

#### A. 初始化组件 (Model & Prompts)

你需要定义两个 Prompt，一个用于初次生成，一个用于根据错误修复。


```

#### B. 定义节点函数 (Node Functions)

这里是逻辑的核心。

```go
// 1. Generator Node: 将自然语言转为 Workflow
func generateNode(ctx context.Context, state *State) (*State, error) {
    // 构造 Prompt：要求输出 JSON 格式的 Workflow
    // 实际项目中建议使用 Eino 的 Prompt Template 和 Output Parser
    // 这里简化演示
    prompt := "你是一个运维专家。请将以下需求转化为 JSON 格式的步骤列表...\n需求: " + state.UserRequest
    
    // 调用 LLM (伪代码)
    // response := llm.Generate(prompt)
    // newWorkflow := parseJSON(response)
    
    // 模拟生成
    state.CurrentWorkflow = &Workflow{Steps: []Step{{Name: "Check disk", Type: "cmd", Content: "df -h"}}}
    state.RetryCount = 0
    return state, nil
}

// 2. Executor Node: 调用你的容器环境进行验证 (关键集成点)
func executeNode(ctx context.Context, state *State) (*State, error) {
    println("正在容器中执行工作流...")
    
    // TODO: 这里调用你 "已实现部分" 的逻辑
    // logic: ConnectContainer -> Run Steps -> Capture Logs
    
    // 模拟执行结果
    success := false // 假设第一次失败
    logs := "Error: command 'nginxx' not found" // 模拟错误日志
    
    if state.RetryCount > 0 { // 模拟第二次成功
        success = true
        logs = "Execution finished successfully."
    }

    state.IsSuccess = success
    state.ExecutionLogs = logs
    if !success {
        state.LastError = logs
    }
    return state, nil
}

// 3. Fixer Node: 根据错误日志修复 Workflow
func fixerNode(ctx context.Context, state *State) (*State, error) {
    state.RetryCount++
    println("检测到执行失败，正在尝试 AI 修复... 第", state.RetryCount, "次")

    // 构造修复 Prompt
    // prompt := fmt.Sprintf("原来的脚本:\n%v\n执行报错:\n%s\n请修正脚本...", state.CurrentWorkflow, state.LastError)
    
    // 调用 LLM 获取新 Plan
    // newWorkflow := llm.Generate(prompt)
    
    // 模拟修复
    state.CurrentWorkflow = &Workflow{Steps: []Step{{Name: "Check disk fixed", Type: "cmd", Content: "df -h && echo 'fixed'"}}}
    
    return state, nil
}

```

#### C. 编排图 (Graph Orchestration)

这是 Eino 最强大的地方，将逻辑串联起来。

```go
func buildGraph() (eino.Runnable, error) {
    // 创建一个 StateGraph
    graph := compose.NewGraph[State, State]()

    // 1. 添加节点
    // 使用 compose.InvokableLambda 将函数转换为图节点
    graph.AddNode("generator", compose.InvokableLambda(generateNode))
    graph.AddNode("executor", compose.InvokableLambda(executeNode))
    graph.AddNode("fixer", compose.InvokableLambda(fixerNode))

    // 2. 定义基本流程
    // Start -> Generator -> Executor
    graph.AddEdge(compose.START, "generator")
    graph.AddEdge("generator", "executor")

    // 3. 定义分支 (Condition)
    // 从 Executor 出来后，决定去哪
    graph.AddBranch("executor", func(ctx context.Context, s *State) (string, error) {
        if s.IsSuccess {
            return compose.END, nil // 成功则结束
        }
        if s.RetryCount >= 3 {
            // 防止死循环，超过3次重试强制结束
            return compose.END, nil 
        }
        return "fixer", nil // 失败则去修复
    })

    // 4. 闭环
    // Fixer -> Executor (修复后再次去验证)
    graph.AddEdge("fixer", "executor")

    // 5. 编译图
    return graph.Compile(context.Background())
}

```

#### D. 运行应用

```go
func main() {
    runnable, err := buildGraph()
    if err != nil {
        panic(err)
    }

    // 初始输入
    initialState := &State{
        UserRequest: "帮我部署一个 Nginx 并在无法访问时重启 Docker",
    }

    // 运行
    finalState, err := runnable.Invoke(context.Background(), initialState)
    if err != nil {
        panic(err)
    }

    if finalState.IsSuccess {
        println("✅ 最终生成并验证通过的工作流：")
        // print finalState.CurrentWorkflow
    } else {
        println("❌ AI 尝试修复多次后仍然失败。最后报错：", finalState.LastError)
    }
}

```

---

### 4. 关键技术细节与建议

#### 1. 结构化输出 (Structured Output)

在 `Generator` 和 `Fixer` 节点中，让 LLM 输出稳定的 JSON 是关键。

* **Eino 方案**: 使用 Eino 的 `schema` 定义能力，或者在 Prompt 中强制要求 JSON Mode。
* **技巧**: 在 Prompt 中给出一两个具体的 JSON 示例（Few-Shot Prompting），能大幅提高准确率。

#### 2. 安全性 (Safety)

既然是运维需求，AI 可能会生成 `rm -rf /` 或停止核心服务的命令。

* **沙箱 (Sandbox)**: 你的“配置好的容器”必须是隔离环境（Ephemeral Container），验证完即销毁。
* **规则过滤**: 在 `Generator` 之后增加一个简单的 `SafetyCheck` 节点（正则匹配），拦截高危命令。

#### 3. 上下文管理 (Context)

* `Fixer` 节点不仅需要当前的 Error，最好还需要知道“我之前尝试修了什么”。
* 可以在 `State` 中增加一个 `History []string` 字段，记录每一次的修改思路，防止 AI 在两个错误的方案之间反复横跳（死循环）。

#### 4. 流式输出 (Streaming)

如果用户在前端等待，整个过程可能耗时较长。Eino 支持 `Stream` 模式。你可以通过 SSE (Server-Sent Events) 将以下状态推送到前端：

* "正在生成初步方案..."
* "正在容器中执行..."
* "执行失败，正在思考修复方案..."
* "修复完成，重试中..."
