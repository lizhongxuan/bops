## Why

当前 Runner 的 AI 生成链路缺少“执行反馈驱动修复”闭环：AI 虽能生成 YAML，但无法稳定根据真实执行错误自动修复并重跑，导致脚本正确率和交付效率不稳定。现在需要把“整 YAML 验证 + 失败回传 + AI 修复 + 全量重跑”固化为标准流程，并评估 OpenHands 作为容器化校验执行器是否可行。

## What Changes

- 新增“YAML 自修复循环”能力：AI 生成或修改完整 workflow YAML 后，Runner 从头执行；失败时将结构化错误上下文回传给 AI，AI 返回修复版 YAML，再次从头执行，直到成功或达到重试上限。
- 新增“Runner 规范注入”能力：在 AI 调用前固定注入 Runner 格式与模块能力（action/args/变量导出规则等），减少无效 YAML 和模块参数错误。
- 新增“全量 YAML 校验协议”：对每次执行记录 run_id、attempt、失败 step、stderr/stdout 摘要、校验结论，作为下一轮修复输入。
- 引入 OpenHands 作为可选校验后端：在 Docker 沙箱中执行校验任务，支持流式终端输出和隔离执行；若未启用 OpenHands，保持现有 Runner 执行路径。
- 新增“实时可观测输出”：前端展示当前 attempt、执行步骤、终端流、修复次数与最终状态，明确系统是否仍在自动修复中。
- 新增“修复停止条件”：最大重试次数、重复错误检测、超时中止，避免无限循环。

## Capabilities

### New Capabilities
- `runner-yaml-self-heal-loop`: 提供整 YAML 的执行-报错-修复-重跑闭环，包括重试上限、错误上下文回传和最终收敛策略。
- `runner-ai-spec-injection`: 提供 Runner 格式与功能的标准化上下文注入，使 AI 在生成/修复 YAML 时遵循可执行约束。
- `runner-validation-backend-adapter`: 提供校验执行后端抽象，并支持 OpenHands（Docker 沙箱 + 流式事件）与现有本地执行器切换。
- `runner-repair-observability`: 提供修复循环的事件与日志模型，支持前端实时展示执行与修复进度。

### Modified Capabilities
- 无（当前 `openspec/specs/` 下无既有 capability 需要变更）。

## Impact

- Affected code:
  - Runner 执行编排层（增加 attempt 循环、停止条件、错误归一化）。
  - AI workflow 调用层（增加规范注入与修复 prompt 输入结构）。
  - 调度/后端接口层（新增校验后端适配接口与 OpenHands 实现）。
  - 前端执行面板（新增 attempt 与流式修复状态展示）。
- APIs:
  - 可能新增/扩展验证请求与事件流接口（包含 run_id、attempt、error_context、repair_status）。
- Dependencies:
  - 可选引入 OpenHands 服务端能力（推荐 Docker 沙箱模式）；需配置模型与沙箱访问策略。
- Systems:
  - 执行链路将从“单次生成执行”升级为“收敛式自修复执行”，对日志、超时和资源治理提出更高要求。
