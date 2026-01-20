# 首页 - AI 工作流助手方案 (Eino)

## 目标与定位
- 让用户在首页用一句话生成可执行的 YAML 工作流，并可视化预览。
- 采用 “生成 -> 验证 -> 修复” 闭环，默认人工确认后执行。
- 对齐现有执行引擎、容器验证、模板市场与 MCP 原子能力。

## 设计原则
- 人机协同：AI 负责写与诊，人负责审与改。
- 可验证：生成结果必须可校验、可在沙箱复现。
- 可解释：每次修复给出 Diff 与原因。
- 可扩展：用 Eino Graph 编排节点，便于替换模型与能力。

## 核心流程 (Self-Correction Loop)
1. 用户输入需求 + 目标环境 + 约束。
2. Generator 生成初版 YAML。
3. Validator 语法/规则校验 + 风险检测。
4. Executor 在沙箱中验证执行，采集日志。
5. 失败 -> Fixer 根据错误日志修复，进入下一轮。
6. 达到最大重试或通过 -> Human Gate 审批。
7. 保存为工作流，进入编排页继续编辑。

## Eino Graph 设计
### 节点
- InputNormalizer: 标准化用户需求与上下文。
- Generator: 生成 YAML。
- Validator: 语法 + 规则校验 (缺字段/格式/不支持动作)。
- SafetyCheck: 高危命令识别与降级建议。
- Executor: 沙箱执行、收集日志与失败步骤。
- Fixer: 根据日志修复 YAML，记录修复 History。
- Summarizer: 生成可读摘要与提示。
- HumanGate: 进入人工确认流程。

### 逻辑流转
- Start -> InputNormalizer -> Generator -> Validator -> SafetyCheck -> Executor
- Executor 成功 -> Summarizer -> HumanGate -> End
- Executor 失败 -> Fixer -> Validator -> SafetyCheck -> Executor
- Fixer 超过上限或风险过高 -> HumanGate (提示人工处理)

### 状态结构 (简化)
```go
type AIState struct {
  Prompt        string
  Context       map[string]any
  YAML          string
  Issues        []string
  RiskLevel     string
  RetryCount    int
  MaxRetries    int
  Logs          string
  FailedStep    string
  History       []string
  Summary       string
  IsSuccess     bool
}
```

## Prompt 与结构化输出
- 生成阶段输出 YAML，约束字段完整 (version/name/plan/steps/targets/action/with)。
- 修复阶段输入：旧 YAML + 错误日志 + Issues。
- 输出需结构化解析，必要时先生成 JSON 再渲染 YAML。
- 默认模型：DeepSeek (可配置多模型切换)。

## 首页信息架构
1. Hero 区：价值主张 + 信任提示 (可视化/可测试/可移植)。
2. 需求输入区：主输入框 + 示例模板 + 高级约束。
3. 结果区 (双栏)：
   - 左：步骤卡片 (targets/action/with 摘要)。
   - 右：YAML 预览 + Diff 历史。
4. 验证与修复区：状态、日志、失败步骤、修复次数。
5. 行动区：保存为工作流 / 继续编辑 / 下载 / 复制。

## 交互细节
- 结果区支持步骤与 YAML 双向定位。
- 修复历史以时间轴展示，每次可回滚。
- 高风险提示需二次确认或输入原因说明。
- 运行状态通过 SSE 推送：生成中/验证中/修复中/完成。

## 接口建议 (与现有端点对齐)
现有：
- POST /api/ai/workflow/generate
- POST /api/ai/workflow/fix

建议补充：
- POST /api/ai/workflow/validate
- POST /api/ai/workflow/execute
- POST /api/ai/workflow/summary

## 安全与风险控制
- 沙箱执行：隔离容器，执行完成即销毁。
- 高危命令检测 (rm/shutdown/mkfs/iptables)。
- 黑名单 + 白名单组合策略，保留人工确认出口。

## 里程碑
- M1: 需求输入 + 生成 YAML + 步骤预览 + 保存。
- M2: 校验 + Diff + 修复历史展示。
- M3: 沙箱执行 + 自动修复回路 + SSE 状态流。

## 待确认问题
- 最大修复次数上限与超时策略。
- 高危规则清单与白名单机制。
- 多模型切换策略与成本控制。
