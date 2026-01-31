<template>
  <div class="card-renderer">
    <FileCreateCard v-if="card.card_type === 'file_create'" :card="card as FileCreateCardPayload" />
    <div v-else class="card fallback">
      <div class="card-title">未知卡片</div>
      <pre>{{ prettyCard }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import FileCreateCard, { type FileCreateCardPayload } from "./cards/FileCreateCard.vue";

export type CardPayload = {
  card_type?: string;
  card_id?: string;
  reply_id?: string;
  title?: string;
  summary?: string;
  steps?: number;
  risk_level?: string;
  issues?: unknown[];
  questions?: unknown[];
  [key: string]: unknown;
};

const props = defineProps<{ card: CardPayload }>();

const prettyCard = computed(() => JSON.stringify(props.card, null, 2));
</script>

<style scoped>
.card-renderer {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

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

.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 8px;
}

.summary-item .label {
  font-size: 11px;
  color: #6f6f6f;
}

.summary-item .value {
  font-size: 13px;
  color: #2b2b2b;
}

.summary-text {
  font-size: 12px;
  color: #2b2b2b;
}

.fallback pre {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
</style>
