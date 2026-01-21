<template>
  <section class="settings">
    <div class="settings-header">
      <div>
        <h1>设置</h1>
        <p>配置 AI Provider 与 API Key，生效后可在首页直接使用对话生成。</p>
      </div>
    </div>

    <section class="panel settings-panel">
      <div class="panel-title">AI 配置</div>
      <div v-if="loading" class="empty">加载中...</div>
      <div v-else class="editor-body">
        <div class="status-row">
          <span>当前状态</span>
          <span class="status-tag" :class="configured ? 'ok' : 'warn'">
            {{ configured ? "已配置" : "未配置" }}
          </span>
        </div>

        <label class="field">
          <span>Provider</span>
          <select v-model="form.provider" :disabled="saving">
            <option value="">未配置</option>
            <option value="openai">OpenAI</option>
            <option value="deepseek">Deepseek</option>
            <option value="gemini">Gemini</option>
          </select>
        </label>

        <label class="field">
          <span>模型</span>
          <input v-model="form.model" type="text" placeholder="例如 gpt-4o-mini" :disabled="saving" />
        </label>

        <label class="field">
          <span>Base URL</span>
          <input v-model="form.baseUrl" type="text" placeholder="https://api.openai.com/v1" :disabled="saving" />
        </label>

        <label class="field">
          <span>API Key</span>
          <input v-model="form.apiKey" type="password" placeholder="sk-***" :disabled="saving" />
          <span class="hint" v-if="apiKeySet">已配置 API Key，留空则保持不变。</span>
        </label>

        <div class="editor-actions">
          <button class="btn primary" type="button" :disabled="saving" @click="saveSettings">
            {{ saving ? "保存中..." : "保存" }}
          </button>
          <span class="status">{{ statusMessage }}</span>
        </div>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, request } from "../lib/api";

type AISettingsResponse = {
  ai_provider?: string;
  ai_api_key_set?: boolean;
  ai_base_url?: string;
  ai_model?: string;
  configured?: boolean;
};

type AISettingsForm = {
  provider: string;
  apiKey: string;
  baseUrl: string;
  model: string;
};

const loading = ref(false);
const saving = ref(false);
const statusMessage = ref("");
const apiKeySet = ref(false);
const form = ref<AISettingsForm>({
  provider: "",
  apiKey: "",
  baseUrl: "",
  model: ""
});

const configured = computed(() => form.value.provider.trim() !== "" && apiKeySet.value);

async function loadSettings() {
  loading.value = true;
  statusMessage.value = "";
  try {
    const data = await request<AISettingsResponse>("/settings/ai");
    form.value = {
      provider: data.ai_provider || "",
      apiKey: "",
      baseUrl: data.ai_base_url || "",
      model: data.ai_model || ""
    };
    apiKeySet.value = Boolean(data.ai_api_key_set);
  } catch (err) {
    statusMessage.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

async function saveSettings() {
  saving.value = true;
  statusMessage.value = "保存中...";
  const payload: Record<string, unknown> = {
    ai_provider: form.value.provider.trim(),
    ai_base_url: form.value.baseUrl.trim(),
    ai_model: form.value.model.trim()
  };
  if (form.value.apiKey.trim()) {
    payload.ai_api_key = form.value.apiKey.trim();
  }
  try {
    const data = await request<AISettingsResponse>("/settings/ai", {
      method: "PUT",
      body: payload
    });
    apiKeySet.value = Boolean(data.ai_api_key_set);
    form.value.apiKey = "";
    statusMessage.value = "保存成功";
  } catch (err) {
    const apiErr = err as ApiError;
    statusMessage.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败";
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  loadSettings();
});
</script>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 12px;
  color: var(--ink);
}

.settings-header h1 {
  margin: 0 0 6px;
  font-size: 22px;
}

.settings-header p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
}

.panel-title {
  font-weight: 600;
  margin-bottom: 12px;
}

.editor-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}

.field span {
  color: var(--muted);
}

input,
select {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 10px 12px;
  font-size: 13px;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  background: #fff;
}

.hint {
  font-size: 12px;
  color: var(--muted);
}

.status-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.status-tag {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 11px;
  background: rgba(27, 27, 27, 0.08);
  color: var(--muted);
}

.status-tag.ok {
  color: var(--ok);
  background: rgba(42, 157, 75, 0.12);
}

.status-tag.warn {
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
}

.editor-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.btn {
  border: 1px solid rgba(27, 27, 27, 0.16);
  background: #fff;
  border-radius: 10px;
  padding: 8px 14px;
  font-size: 12px;
  cursor: pointer;
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.status {
  font-size: 12px;
  color: var(--muted);
}

.empty {
  font-size: 12px;
  color: var(--muted);
}
</style>
