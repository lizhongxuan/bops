<template>
  <aside class="node-library">
    <div class="panel-title">节点库</div>
    <div class="panel-body">暂未实现节点库面板</div>
  </aside>
</template>

<script setup lang="ts">
export type TemplateSummary = {
  id?: string;
  name?: string;
  description?: string;
  type?: string;
  schema?: Record<string, unknown>;
  node?: {
    type?: string;
    name?: string;
    data?: Record<string, unknown>;
  };
};
</script>

<style scoped>
.node-library {
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: rgba(255, 255, 255, 0.6);
  border-radius: 14px;
  padding: 12px;
}

.panel-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 8px;
}

.panel-body {
  font-size: 12px;
  color: #6f6f6f;
}
</style>
