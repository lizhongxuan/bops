<template>
  <div class="card subloop-card">
    <div class="card-title">
      子循环<span v-if="card.round"> · 第 {{ card.round }} 轮</span>
    </div>
    <div class="meta">
      <span class="status" :class="statusClass">{{ statusLabel }}</span>
      <span v-if="card.step_name || card.step_id" class="meta-item">
        {{ card.step_name || card.step_id }}
      </span>
      <span v-if="card.agent_name || card.agent_role" class="meta-item">
        {{ card.agent_name || "agent" }}<span v-if="card.agent_role"> · {{ card.agent_role }}</span>
      </span>
    </div>
    <div v-if="card.message" class="message">{{ card.message }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

export type SubLoopCardPayload = {
  card_type: "subloop";
  step_id?: string;
  step_name?: string;
  round?: number;
  status?: string;
  message?: string;
  agent_name?: string;
  agent_role?: string;
  parent_step_id?: string;
};

const props = defineProps<{ card: SubLoopCardPayload }>();

const statusLabel = computed(() => {
  const raw = (props.card.status || "").toLowerCase();
  if (raw === "start" || raw === "running" || raw === "in_progress") return "运行中";
  if (raw === "done" || raw === "success" || raw === "completed") return "完成";
  if (raw === "error" || raw === "failed") return "失败";
  if (raw === "pending") return "待开始";
  return props.card.status || "";
});

const statusClass = computed(() => {
  const raw = (props.card.status || "").toLowerCase();
  if (raw === "start" || raw === "running" || raw === "in_progress") return "running";
  if (raw === "done" || raw === "success" || raw === "completed") return "done";
  if (raw === "error" || raw === "failed") return "error";
  if (raw === "pending") return "pending";
  return "neutral";
});
</script>

<style scoped>
.card {
  border: 1px solid rgba(27, 27, 27, 0.08);
  border-radius: 14px;
  background: #fff;
  padding: 12px;
}

.card-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 8px;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: #4b4b4b;
  margin-bottom: 8px;
}

.meta-item {
  background: rgba(27, 27, 27, 0.04);
  border-radius: 999px;
  padding: 2px 8px;
}

.status {
  border-radius: 999px;
  padding: 2px 8px;
  font-weight: 600;
}

.status.running {
  background: rgba(52, 152, 219, 0.12);
  color: #2c73b8;
}

.status.done {
  background: rgba(46, 204, 113, 0.12);
  color: #1f8e4f;
}

.status.error {
  background: rgba(231, 76, 60, 0.12);
  color: #c0392b;
}

.status.pending {
  background: rgba(149, 165, 166, 0.18);
  color: #6c7a7a;
}

.status.neutral {
  background: rgba(127, 140, 141, 0.12);
  color: #6f6f6f;
}

.message {
  font-size: 12px;
  color: #2b2b2b;
  background: rgba(27, 27, 27, 0.03);
  padding: 6px 8px;
  border-radius: 8px;
}
</style>
