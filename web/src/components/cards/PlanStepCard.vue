<template>
  <div class="card plan-step-card">
    <div class="line">
      <span class="status" :class="statusClass">({{ statusLabel }})</span>
      <span class="title">{{ card.step_name || card.step_id || "步骤" }}</span>
      <span v-if="card.change_summary" class="summary"> · {{ card.change_summary }}</span>
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
  change_summary?: string;
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
  if (raw === "updated") return "已更新";
  return props.card.step_status || props.card.event_type || "";
});

const statusClass = computed(() => {
  const raw = (props.card.step_status || props.card.event_type || "").toLowerCase();
  if (raw === "start" || raw === "in_progress" || raw === "running") return "running";
  if (raw === "done" || raw === "success" || raw === "completed") return "done";
  if (raw === "failed" || raw === "error") return "error";
  if (raw === "pending") return "pending";
  if (raw === "updated") return "updated";
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

.line {
  font-size: 13px;
  color: #2b2b2b;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.title {
  font-weight: 600;
}

.summary {
  color: #6f6f6f;
  font-size: 12px;
}

.status {
  font-weight: 600;
}

.status.running {
  color: #2c73b8;
}

.status.done {
  color: #1f8e4f;
}

.status.error {
  color: #c0392b;
}

.status.pending {
  color: #6c7a7a;
}

.status.updated {
  color: #7f8c8d;
}

.status.neutral {
  color: #6f6f6f;
}
</style>
