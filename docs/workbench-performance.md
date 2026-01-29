# 工作台性能测试建议

> 目标：验证节点数量、事件流速率与自动布局耗时满足目标（见 `docs/workbench-decisions.md`）。

## 1. 节点规模测试
- 目标：200 节点 / 400 边以内画布可操作。
- 方法：使用模板快速生成节点（或通过调试脚本批量插入节点）。
- 观察：拖拽/选择/编辑响应是否明显卡顿。

## 2. 事件吞吐测试
- 目标：事件流速率 <= 50 events/sec。
- 方法：运行包含多步的工作流，观察 SSE 日志面板刷新。
- 观察：UI 是否掉帧/日志堆积。

## 3. 自动布局耗时
- 目标：200 节点布局耗时 < 300ms。
- 方法：在浏览器控制台执行：
```js
performance.mark("layout-start");
// 点击“自动布局”按钮或触发 autoLayout()
performance.mark("layout-end");
performance.measure("layout", "layout-start", "layout-end");
performance.getEntriesByName("layout").pop().duration;
```

## 4. 结果记录
- 记录节点数、边数、日志量、耗时，并在问题单中附上截图/耗时数据。
