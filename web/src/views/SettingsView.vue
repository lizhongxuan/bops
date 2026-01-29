<template>
  <section class="settings">
    <div class="settings-header">
      <div>
        <h1>设置</h1>
        <p>配置 AI Provider 与 API Key，生效后可在工作区直接使用对话生成。</p>
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

    <section class="panel settings-panel">
      <div class="panel-title">Skill / Agent</div>
      <div class="editor-body">
        <div class="skill-actions">
          <button class="btn" type="button" :disabled="reloadBusy" @click="reloadSkills">
            {{ reloadBusy ? "重新加载中..." : "重新加载 Skills" }}
          </button>
          <span class="status">{{ reloadMessage }}</span>
        </div>

        <div class="list-block">
          <div class="list-title">Skills</div>
          <div v-if="skillsLoading" class="empty">加载中...</div>
          <div v-else-if="skillsError" class="empty">{{ skillsError }}</div>
          <div v-else-if="skills.length === 0" class="empty">未配置 Skill</div>
          <div v-else class="skill-list">
            <div class="skill-card" v-for="skill in skills" :key="`${skill.name}-${skill.version || ''}`">
              <div class="skill-head">
                <div>
                  <div class="skill-name">
                    {{ skill.name }}
                    <span v-if="skill.version" class="skill-version">@{{ skill.version }}</span>
                  </div>
                  <div v-if="skill.description" class="skill-desc">{{ skill.description }}</div>
                  <div class="skill-meta">
                    <span v-if="skill.source_dir">{{ skill.source_dir }}</span>
                    <span v-if="skill.loaded_at"> · {{ skill.loaded_at }}</span>
                    <span v-if="skill.tool_count"> · tools {{ skill.tool_count }}</span>
                  </div>
                </div>
                <span class="status-tag" :class="skill.status === 'loaded' ? 'ok' : 'warn'">
                  {{ skill.status === "loaded" ? "已加载" : "加载失败" }}
                </span>
              </div>
              <div v-if="skill.error" class="skill-error">
                <div>{{ skill.error }}</div>
                <div v-if="skill.error_hint" class="skill-hint">建议: {{ skill.error_hint }}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="list-block">
          <div class="list-title">Agents</div>
          <div v-if="agentsLoading" class="empty">加载中...</div>
          <div v-else-if="agentsError" class="empty">{{ agentsError }}</div>
          <div v-else-if="agents.length === 0" class="empty">未配置 Agent</div>
          <div v-else class="agent-list">
            <div class="agent-card" v-for="agent in agents" :key="agent.name">
              <div class="agent-title">
                {{ agent.name }}
                <span v-if="agent.model" class="agent-model">({{ agent.model }})</span>
              </div>
              <div class="agent-skills">
                <span class="chip" v-for="skill in agent.skills" :key="skill">{{ skill }}</span>
              </div>
            </div>
          </div>
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

type SkillInfo = {
  name: string;
  version?: string;
  description?: string;
  source_dir?: string;
  loaded_at?: string;
  status?: string;
  error?: string;
  error_hint?: string;
  tool_count?: number;
};

type SkillsResponse = {
  items?: SkillInfo[];
  total?: number;
};

type AgentInfo = {
  name: string;
  model?: string;
  skills: string[];
};

type AgentsResponse = {
  items?: AgentInfo[];
  total?: number;
};

const loading = ref(false);
const saving = ref(false);
const statusMessage = ref("");
const apiKeySet = ref(false);
const skillsLoading = ref(false);
const agentsLoading = ref(false);
const reloadBusy = ref(false);
const reloadMessage = ref("");
const skillsError = ref("");
const agentsError = ref("");
const form = ref<AISettingsForm>({
  provider: "",
  apiKey: "",
  baseUrl: "",
  model: ""
});
const skills = ref<SkillInfo[]>([]);
const agents = ref<AgentInfo[]>([]);

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

async function loadSkills() {
  skillsLoading.value = true;
  skillsError.value = "";
  try {
    const data = await request<SkillsResponse>("/skills");
    skills.value = data.items || [];
  } catch (err) {
    skillsError.value = "加载 Skills 失败，请检查服务是否启动";
    skills.value = [];
  } finally {
    skillsLoading.value = false;
  }
}

async function reloadSkills() {
  reloadBusy.value = true;
  reloadMessage.value = "重新加载中...";
  try {
    const data = await request<SkillsResponse>("/skills/reload", { method: "POST" });
    skills.value = data.items || [];
    reloadMessage.value = "已重新加载";
  } catch (err) {
    const apiErr = err as ApiError;
    reloadMessage.value = apiErr.message ? `重新加载失败: ${apiErr.message}` : "重新加载失败";
  } finally {
    reloadBusy.value = false;
  }
}

async function loadAgents() {
  agentsLoading.value = true;
  agentsError.value = "";
  try {
    const data = await request<AgentsResponse>("/agents");
    agents.value = data.items || [];
  } catch (err) {
    agentsError.value = "加载 Agents 失败，请检查服务是否启动";
    agents.value = [];
  } finally {
    agentsLoading.value = false;
  }
}

onMounted(() => {
  loadSettings();
  loadSkills();
  loadAgents();
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

.skill-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.list-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.list-title {
  font-size: 13px;
  font-weight: 600;
}

.skill-list,
.agent-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.skill-card,
.agent-card {
  border: 1px solid rgba(27, 27, 27, 0.12);
  border-radius: 12px;
  padding: 12px;
  background: #fff;
}

.skill-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.skill-name,
.agent-title {
  font-size: 13px;
  font-weight: 600;
}

.skill-version,
.agent-model {
  font-size: 12px;
  color: var(--muted);
  margin-left: 6px;
}

.skill-desc {
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}

.skill-meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--muted);
}

.skill-error {
  margin-top: 8px;
  padding: 8px;
  border-radius: 8px;
  background: rgba(208, 52, 44, 0.08);
  color: var(--err);
  font-size: 12px;
}

.skill-hint {
  margin-top: 4px;
  color: var(--muted);
}

.agent-skills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.chip {
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(27, 27, 27, 0.08);
  color: var(--ink);
  font-size: 11px;
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
