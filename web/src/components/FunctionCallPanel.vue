<template>
  <div class="function-panel">
    <div v-if="items.length === 0" class="function-empty">暂无执行步骤</div>
    <details v-for="item in items" :key="item.callId" class="function-item" :open="item.status === 'running'">
      <summary class="function-summary">
        <span class="status" :class="item.status">{{ statusLabel(item.status) }}</span>
        <span class="title">{{ item.title }}</span>
      </summary>
      <div v-if="item.content" class="function-body">
        <pre>{{ item.content }}</pre>
      </div>
    </details>
  </div>
</template>

<script setup lang="ts">
export type FunctionCallUnit = {
  callId: string;
  title: string;
  status: "running" | "done" | "failed";
  content?: string;
  index?: number;
};

defineProps<{ items: FunctionCallUnit[] }>();

function statusLabel(status: FunctionCallUnit["status"]) {
  if (status === "running") return "执行中";
  if (status === "failed") return "失败";
  return "完成";
}
</script>

<style scoped>
.function-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}

.function-empty {
  font-size: 12px;
  color: #8c8c8c;
}

.function-item {
  border: 1px solid #e6e3dc;
  border-radius: 12px;
  background: #fff;
  padding: 8px 10px;
}

.function-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  cursor: pointer;
}

.function-summary::-webkit-details-marker {
  display: none;
}

.status {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid transparent;
}

.status.running {
  color: #1764d1;
  border-color: rgba(23, 100, 209, 0.25);
  background: rgba(23, 100, 209, 0.08);
}

.status.done {
  color: #1f8a48;
  border-color: rgba(31, 138, 72, 0.25);
  background: rgba(31, 138, 72, 0.08);
}

.status.failed {
  color: #c2352b;
  border-color: rgba(194, 53, 43, 0.25);
  background: rgba(194, 53, 43, 0.08);
}

.title {
  font-weight: 600;
  color: #2b2b2b;
}

.function-body {
  margin-top: 8px;
  border-top: 1px dashed #e6e3dc;
  padding-top: 8px;
}

.function-body pre {
  margin: 0;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  white-space: pre-wrap;
  color: #3a3a3a;
}
</style>
