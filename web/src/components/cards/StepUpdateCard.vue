<template>
  <div class="card step-card">
    <div class="card-title">{{ card.title || "修改步骤" }}</div>
    <div class="step-name">{{ card.after?.name || card.before?.name || "(未命名步骤)" }}</div>
    <div class="step-grid">
      <div class="block">
        <div class="block-title">修改前</div>
        <pre>{{ formatStep(card.before) }}</pre>
      </div>
      <div class="block">
        <div class="block-title">修改后</div>
        <pre>{{ formatStep(card.after) }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
export type StepUpdateCardPayload = {
  card_type: "update_step";
  title?: string;
  step_index?: number;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
};

defineProps<{ card: StepUpdateCardPayload }>();

function formatStep(input?: Record<string, unknown>) {
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
  margin-bottom: 10px;
}

.step-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 8px;
}

.block-title {
  font-size: 11px;
  color: #6f6f6f;
  margin-bottom: 4px;
}

.block pre {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
</style>
