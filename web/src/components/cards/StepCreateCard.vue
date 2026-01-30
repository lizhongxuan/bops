<template>
  <div class="card step-card">
    <div class="card-title">{{ card.title || "创建步骤" }}</div>
    <div class="step-name">{{ card.step?.name || "(未命名步骤)" }}</div>
    <div class="step-meta">
      <span v-if="card.step?.action" class="pill">{{ card.step.action }}</span>
      <span v-if="card.step?.targets?.length" class="pill">targets: {{ card.step.targets.join(', ') }}</span>
    </div>
    <pre v-if="card.step?.with" class="step-with">{{ formatWith(card.step.with) }}</pre>
  </div>
</template>

<script setup lang="ts">
export type StepCreateCardPayload = {
  card_type: "create_step";
  title?: string;
  step_index?: number;
  step?: {
    name?: string;
    action?: string;
    targets?: string[];
    with?: Record<string, unknown>;
  };
};

defineProps<{ card: StepCreateCardPayload }>();

function formatWith(input?: Record<string, unknown>) {
  if (!input) return "";
  return JSON.stringify(input, null, 2);
}
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
  margin-bottom: 6px;
}

.step-name {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
}

.step-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}

.pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: rgba(27, 27, 27, 0.04);
}

.step-with {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
</style>
