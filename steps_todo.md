# Steps 轻量分层实现清单（基于 steps_design.md）

> 目标：按任务顺序逐步落地，可从任一任务继续。

## 1. 数据模型与存储拆分
- [x] 新增 steps 与 inventory 的独立存储结构（DB 字段或文件路径）。
- [x] 实现 workflow 拆分/合并的内部模型结构（steps-only + inventory-only）。
- [x] 明确 workflow name 来源（由 UI/后端维护，不由 AI 生成）。

## 2. API 读写与权限隔离
- [x] 新增 `GET /workflows/:id/steps` 与 `PUT /workflows/:id/steps`。
- [x] 新增 `GET /workflows/:id/inventory` 与 `PUT /workflows/:id/inventory`。
- [x] 强制 AI 侧只能访问 steps 接口；inventory 接口仅人工编辑可见。
- [x] 运行接口读取 steps + inventory 并合并执行。

## 3. AI 交互隔离规则
- [x] Prompt 规则更新：仅允许操作 steps，不得询问/生成 inventory 信息。
- [x] 对 AI 输出做 guard：丢弃任何 inventory 字段尝试写入。
- [x] 若缺 targets 信息，仅提示“去 Inventory 页面补充”。

## 4. 变量收集与优先级
- [x] 实现变量池收集顺序：
      运行时 vars > inventory host vars > inventory group vars > inventory vars > system defaults
- [x] 当前 step 的 `args` 仅对本 step 生效（不写入全局）。
- [x] `export_vars` 写回全局 vars（供后续 steps 使用）。

## 5. must_vars / expect_vars 校验
- [x] 支持 `must_vars`（list）解析与执行前校验。
- [x] `must_vars` 仅来源：inventory + 已有 `expect_vars`。
- [x] step 执行后校验 `expect_vars`，缺失即终止执行并报错。
- [x] 明确错误提示格式（缺失变量列表）。

## 6. 执行合并逻辑
- [x] 运行入口合并 steps + inventory 为完整 workflow。
- [x] 确认 steps-only 执行不依赖 inventory 时的降级策略（例如错误提示）。

## 7. UI 分区
- [x] 工作流编辑页新增 Inventory 配置区（仅手动编辑）。
- [x] Steps 编辑区支持 AI 更新/手动编辑。
- [x] 运行前提示缺失必填 inventory/vars。

## 8. 迁移策略
- [x] 全量 YAML 上传后自动拆分为 steps + inventory。
- [x] 不保留原始 YAML（仅保留拆分后的两部分）。
- [x] 对旧数据做一次性迁移脚本/任务。

## 9. 测试与验证
- [x] steps-only 执行流程测试。
- [x] must_vars 缺失报错测试。
- [x] export_vars 对下一步影响测试。
- [x] AI 只读 steps 的权限测试。
