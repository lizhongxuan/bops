<template>
  <div class="card plan-step-card">
    <div class="card-title">{{ card.step_name || card.step_id || "步骤" }}</div>
    <div class="meta">
      <span class="status" :class="statusClass">{{ statusLabel }}</span>
      <span v-if="card.step_id" class="meta-item">ID: {{ card.step_id }}</span>
      <span v-if="card.agent_name || card.agent_role" class="meta-item">
        {{ card.agent_name || "agent" }}<span v-if="card.agent_role"> · {{ card.agent_role }}</span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

export type PlanStepCardPayload = {
  card_type: "plan_step";
  step_id?: string;
  step_name?: string;
  step_status?: string;
  event_type?: string;
  agent_name?: string;
  agent_role?: string;
  parent_step_id?: string;
};

const props = defineProps<{ card: PlanStepCardPayload }>();

const statusLabel = computed(() => {
  const raw = (props.card.step_status || props.card.event_type || "").toLowerCase();
  if (raw === "start" || raw === "in_progress" || raw === "running") return "进行中";
  if (raw === "done" || raw === "success" || raw === "completed") return "已完成";
  if (raw === "failed" || raw === "error") return "失败";
  if (raw === "pending") return "待开始";
  return props.card.step_status || props.card.event_type || "";
});

const statusClass = computed(() => {
  const raw = (props.card.step_status || props.card.event_type || "").toLowerCase();
  if (raw === "start" || raw === "in_progress" || raw === "running") return "running";
  if (raw === "done" || raw === "success" || raw === "completed") return "done";
  if (raw === "failed" || raw === "error") return "error";
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
</style>
