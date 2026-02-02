<template>
  <div class="function-panel">
    <div v-if="items.length === 0" class="function-empty">暂无执行步骤</div>
    <div v-for="item in items" :key="item.callId" class="function-row" :class="item.status">
      <span class="status">{{ statusLabel(item.status) }}</span>
      <span class="title">{{ item.title }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
export type FunctionCallUnit = {
  callId: string;
  title: string;
  status: "running" | "done" | "failed";
  content?: string;
  index?: number;
  streamUuid?: string;
  loopId?: string;
  iteration?: number;
  agentStatus?: string;
  agentId?: string;
  agentName?: string;
  agentRole?: string;
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
  gap: 6px;
}

.function-empty {
  font-size: 12px;
  color: #8c8c8c;
}

.function-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #2b2b2b;
}

.status {
  font-weight: 600;
}

.function-row.running .status {
  color: #1764d1;
}

.function-row.done .status {
  color: #1f8a48;
}

.function-row.failed .status {
  color: #c2352b;
}

.title {
  font-weight: 600;
  color: #2b2b2b;
}
</style>
