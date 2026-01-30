<template>
  <aside class="chat-drawer">
    <div class="drawer-head">
      <div class="title">节点助手</div>
      <div class="status">{{ status || '就绪' }}</div>
    </div>
    <div class="drawer-body">
      <label class="prompt">
        <span class="label">节点需求</span>
        <textarea v-model="prompt" rows="3" placeholder="描述需要生成或重生成的节点内容"></textarea>
      </label>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="selectedNode" class="node-info">
        <div class="label">当前节点</div>
        <div class="value">{{ selectedNode.name }} ({{ selectedNode.type }})</div>
      </div>
      <div v-if="runSummary" class="summary">
        <div class="label">运行结果</div>
        <div class="value">{{ runSummary.status }}</div>
      </div>
      <div v-if="runLogs.length" class="logs">
        <div class="label">日志</div>
        <pre>{{ runLogs.join('\n') }}</pre>
      </div>
    </div>
    <div class="drawer-actions">
      <button class="btn btn-sm" type="button" :disabled="busy" @click="$emit('generate', prompt)">生成</button>
      <button class="btn btn-sm ghost" type="button" :disabled="busy" @click="$emit('fix')">修复</button>
      <button class="btn btn-sm ghost" type="button" :disabled="busy" @click="$emit('regenerate', prompt)">重生成</button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from "vue";

type RunSummary = {
  status: string;
  totalSteps: number;
  successSteps: number;
  failedSteps: number;
  durationMs: number;
  issues: string[];
  message?: string;
};

type NodeInfo = {
  id: string;
  type: string;
  name: string;
  data?: Record<string, unknown>;
  x: number;
  y: number;
};

defineProps<{
  selectedNode: NodeInfo | null;
  status: string;
  error: string;
  busy: boolean;
  runStatus: string;
  runSummary: RunSummary | null;
  runLogs: string[];
}>();

defineEmits<{
  (event: "generate", prompt: string): void;
  (event: "fix"): void;
  (event: "regenerate", prompt: string): void;
}>();

const prompt = ref("");

</script>

<style scoped>
.chat-drawer {
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: rgba(255, 255, 255, 0.68);
  border-radius: 14px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.drawer-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title {
  font-weight: 600;
  font-size: 14px;
}

.status {
  font-size: 12px;
  color: #6f6f6f;
}

.drawer-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}

.prompt textarea {
  width: 100%;
  resize: vertical;
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 6px 8px;
  font-size: 12px;
}

.label {
  color: #6f6f6f;
  margin-bottom: 4px;
}

.value {
  color: #2b2b2b;
}

.logs pre {
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 11px;
  margin: 0;
}

.error {
  color: #c2352b;
  font-size: 12px;
}

.drawer-actions {
  display: flex;
  gap: 8px;
}
</style>
