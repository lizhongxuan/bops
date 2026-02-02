<template>
  <div class="card yaml-patch-card">
    <div class="card-title">片段更新</div>
    <div class="meta">
      <span v-if="card.step_name || card.step_id" class="meta-item">
        {{ card.step_name || card.step_id }}
      </span>
      <span v-if="card.agent_name || card.agent_role" class="meta-item">
        {{ card.agent_name || "agent" }}<span v-if="card.agent_role"> · {{ card.agent_role }}</span>
      </span>
    </div>
    <pre v-if="card.yaml_fragment" class="yaml-content">{{ card.yaml_fragment }}</pre>
    <div v-else class="card-empty">暂无片段</div>
  </div>
</template>

<script setup lang="ts">
export type YamlPatchCardPayload = {
  card_type: "yaml_patch";
  step_id?: string;
  step_name?: string;
  yaml_fragment?: string;
  agent_name?: string;
  agent_role?: string;
  parent_step_id?: string;
};

defineProps<{ card: YamlPatchCardPayload }>();
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

.yaml-content {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  background: rgba(27, 27, 27, 0.03);
  padding: 8px;
  border-radius: 10px;
}

.card-empty {
  font-size: 12px;
  color: #8c8c8c;
}
</style>
