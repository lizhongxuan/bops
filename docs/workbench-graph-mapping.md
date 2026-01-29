# Graph 与 YAML 映射规则（实现版）

## 1. 节点到步骤（Graph -> YAML）
- 节点集合映射到 `steps[]`。
- 执行顺序由拓扑排序决定：
  - 优先满足边的依赖关系（source -> target）。
  - 当有多个可执行节点时，使用图中原始顺序作为稳定排序。
- 当图存在环或拓扑排序失败时，回退为“节点原始顺序”。

## 2. 步骤到节点（YAML -> Graph）
- `steps[]` 逐条映射为节点：
  - 节点 `id` 默认生成 `step-{index}`。
  - `name/action/with/targets` 对应节点字段。
- 连线默认按步骤顺序串联（线性流程）：
  - `step-1 -> step-2 -> step-3`。
- 节点布局默认从左到右（LR），按索引设置基础坐标。

## 3. 同步策略
- YAML 作为执行真值源；Graph 作为编辑与交互层视图。
- 图变更后调用 Graph -> YAML 更新步骤；AI 或后端修改 YAML 时可重新生成 Graph。

## 4. 兼容性
- 旧草稿缺失 Graph 时，从 YAML 生成 Graph 并回写草稿。
