<template>
  <div class="summary-card">
    <div class="summary-header">
      <div>
        <div class="summary-title">{{ workflowName || "未命名草稿" }}</div>
        <div class="summary-sub">{{ workflowDesc || "通过聊天不断完善细节" }}</div>
      </div>
      <div class="summary-tags">
        <span class="tag">{{ planMode }}</span>
        <span v-if="selectedValidationEnv" class="tag">验证: {{ selectedValidationEnv }}</span>
        <span v-if="envPackagesInput" class="tag">变量包: {{ envPackagesInput }}</span>
      </div>
    </div>
    <div class="summary-body">
      <div class="field">
        <label>工作流名称</label>
        <input
          :value="workflowName"
          @input="$emit('update:workflowName', readValue($event))"
          type="text"
          placeholder="例如 deploy-nginx"
        />
      </div>
      <div class="field">
        <label>描述</label>
        <textarea
          :value="workflowDesc"
          @input="$emit('update:workflowDesc', readValue($event))"
          rows="2"
          placeholder="一句话说明这个流程"
        />
      </div>
      <div class="field">
        <label>目标主机/分组</label>
        <input
          :value="targetsInput"
          @input="$emit('update:targetsInput', readValue($event))"
          type="text"
          placeholder="web, db"
        />
      </div>
      <div class="field">
        <label>执行策略</label>
        <select :value="planMode" @change="$emit('update:planMode', readValue($event))">
          <option value="manual-approve">manual-approve</option>
          <option value="auto">auto</option>
        </select>
      </div>
      <div class="field">
        <label>环境变量包</label>
        <input
          :value="envPackagesInput"
          @input="$emit('update:envPackagesInput', readValue($event))"
          type="text"
          placeholder="prod-env, staging"
        />
      </div>
      <div class="field">
        <label>验证环境</label>
        <select
          :value="selectedValidationEnv"
          @change="$emit('update:selectedValidationEnv', readValue($event))"
        >
          <option value="">请选择验证环境</option>
          <option v-for="env in validationEnvs" :key="env.name" :value="env.name">
            {{ env.name }}
          </option>
        </select>
        <div v-if="!selectedValidationEnv" class="field-hint">未选择验证环境将跳过执行</div>
      </div>
      <div class="field">
        <label>最大修复次数</label>
        <input
          :value="maxRetries"
          @input="$emit('update:maxRetries', Number(readValue($event)))"
          type="number"
          min="0"
          max="5"
        />
      </div>
      <div class="field toggle">
        <label>自动同步 YAML</label>
        <button class="toggle-btn" type="button" :class="autoSync ? 'on' : 'off'" @click="$emit('toggle-auto-sync')">
          {{ autoSync ? "启用" : "关闭" }}
        </button>
      </div>
    </div>
    <div class="meta-actions">
      <button class="btn" type="button" @click="$emit('apply-draft')">同步到 YAML</button>
      <button class="btn ghost" type="button" @click="$emit('apply-yaml')">从 YAML 解析</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { PropType } from "vue";

function readValue(event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null;
  return target?.value ?? "";
}

defineProps({
  workflowName: String,
  workflowDesc: String,
  targetsInput: String,
  planMode: {
    type: String,
    default: "manual-approve"
  },
  envPackagesInput: String,
  selectedValidationEnv: String,
  validationEnvs: {
    type: Array as PropType<{ name: string }[]>,
    default: () => []
  },
  maxRetries: {
    type: Number,
    default: 2
  },
  autoSync: {
    type: Boolean,
    default: true
  }
});

</script>

<style scoped>
.summary-card {
  background: var(--panel);
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-title {
  font-size: 20px;
  font-family: "Space Grotesk", sans-serif;
  margin-bottom: 4px;
}

.summary-sub {
  font-size: 12px;
  color: var(--muted);
}

.summary-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag {
  background: #f6f2ec;
  color: var(--muted);
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 11px;
}

.summary-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field input,
.field textarea,
.field select {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 12px;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
}

.field.toggle {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.toggle-btn {
  border-radius: 999px;
  padding: 4px 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #f7f2ec;
  font-size: 12px;
}

.toggle-btn.on {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}
</style>
