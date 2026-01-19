<template>
  <section class="home">
    <div class="chat">
      <header class="chat-header">
        <div>
          <h1>AI 工作流助手</h1>
          <p>描述你的运维目标，生成可执行的工作流模板。</p>
        </div>
        <div class="quick-actions">
          <button class="btn ghost" type="button" @click="createSession">新会话</button>
          <button class="btn primary" type="button" :disabled="busy" @click="generateYaml">
            生成模板
          </button>
        </div>
      </header>

      <div class="session-bar">
        <div class="session-label">会话</div>
        <select class="session-select" v-model="activeSessionId" @change="selectSession">
          <option v-for="session in sessions" :key="session.id" :value="session.id">
            {{ session.title || "新会话" }}
          </option>
        </select>
        <div class="session-meta">{{ activeSessionMeta }}</div>
      </div>

      <div class="chat-body">
        <div class="message" v-for="(msg, index) in messages" :key="index" :class="msg.role">
          <div class="avatar">{{ msg.role === 'user' ? '你' : 'AI' }}</div>
          <div class="bubble">
            <div class="role">{{ msg.role === 'user' ? '用户' : '助手' }}</div>
            <div class="content">{{ msg.content }}</div>
          </div>
        </div>
        <div v-if="messages.length === 0" class="empty">
          先描述目标，例如：在 web1/web2 上安装 nginx，渲染配置并启动服务。
        </div>
      </div>

      <div class="chat-input">
        <textarea
          v-model="input"
          placeholder="描述你的运维步骤、主机与变量…"
          rows="4"
          @keydown.enter.exact.prevent="sendMessage"
        ></textarea>
        <div class="input-actions">
          <div class="hint">Enter 发送 · Shift+Enter 换行</div>
          <button class="btn" type="button" :disabled="busy || !input.trim()" @click="sendMessage">
            发送
          </button>
        </div>
      </div>
    </div>

    <aside class="editor">
      <div class="editor-header">
        <div>
          <h2>工作流模板</h2>
          <p>AI 生成或你手动修改的 YAML</p>
        </div>
        <div class="editor-actions">
          <button class="btn" type="button" :disabled="busy" @click="validateDraft">校验</button>
          <button
            class="btn ghost"
            type="button"
            :disabled="validationBusy || !selectedValidationEnv || !yaml.trim()"
            @click="runValidation"
          >
            验证环境测试
          </button>
          <button class="btn primary" type="button" :disabled="busy || !yaml.trim()" @click="saveWorkflow">
            保存为工作流
          </button>
        </div>
      </div>

      <div class="editor-body">
        <div class="validation-bar">
          <div class="validation-label">验证环境</div>
          <select v-model="selectedValidationEnv">
            <option value="">请选择环境</option>
            <option v-for="env in validationEnvs" :key="env.name" :value="env.name">
              {{ env.name }}
            </option>
          </select>
          <div class="validation-hint">
            {{ selectedValidationEnv ? `当前: ${selectedValidationEnv}` : "未选择" }}
          </div>
        </div>

        <textarea v-model="yaml" spellcheck="false" class="code" rows="20"></textarea>
        <div class="validation" :class="validation.ok ? 'ok' : 'warn'">
          {{ validation.ok ? '校验通过' : '校验未通过' }}
        </div>
        <ul class="issues" v-if="validation.issues.length">
          <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
        </ul>
        <div class="preview">
          <div class="preview-title">步骤预览</div>
          <div v-if="steps.length === 0" class="empty">未解析到步骤</div>
          <div v-else class="step-card" v-for="step in steps" :key="step.name">
            <div class="step-name">{{ step.name }}</div>
            <div class="step-meta">{{ step.action || '未指定动作' }}</div>
            <div class="step-targets" v-if="step.targets">目标: {{ step.targets }}</div>
          </div>
        </div>

        <div v-if="validationResult" class="validation-result" :class="validationResult.status">
          <div class="result-title">
            验证结果: {{ validationResult.status === 'success' ? '通过' : '失败' }}
            <span v-if="validationResult.code"> (code {{ validationResult.code }})</span>
          </div>
          <div v-if="validationResult.error" class="result-error">{{ validationResult.error }}</div>
          <div class="result-io">
            <div v-if="validationResult.stdout" class="result-block">
              <div class="result-label">stdout</div>
              <pre>{{ validationResult.stdout }}</pre>
            </div>
            <div v-if="validationResult.stderr" class="result-block">
              <div class="result-label">stderr</div>
              <pre>{{ validationResult.stderr }}</pre>
            </div>
          </div>
        </div>
      </div>
    </aside>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, request } from "../lib/api";

type Message = {
  role: "user" | "assistant";
  content: string;
};

type SessionSummary = {
  id: string;
  title: string;
  updated_at: string;
  message_count: number;
};

type Session = {
  id: string;
  title: string;
  messages: Message[];
};

type ValidationEnvSummary = {
  name: string;
};

type ValidationResult = {
  status: string;
  stdout?: string;
  stderr?: string;
  code?: number;
  error?: string;
};

type ValidationState = {
  ok: boolean;
  issues: string[];
};

const messages = ref<Message[]>([]);
const input = ref("");
const yaml = ref("");
const busy = ref(false);
const validation = ref<ValidationState>({ ok: true, issues: [] });
const sessions = ref<SessionSummary[]>([]);
const activeSessionId = ref("");
const validationEnvs = ref<ValidationEnvSummary[]>([]);
const selectedValidationEnv = ref("");
const validationBusy = ref(false);
const validationResult = ref<ValidationResult | null>(null);

const steps = computed(() => parseSteps(yaml.value));
const activeSessionMeta = computed(() => {
  const current = sessions.value.find((session) => session.id === activeSessionId.value);
  if (!current) return "暂无会话";
  const count = current.message_count || 0;
  return `消息 ${count} 条`;
});

onMounted(() => {
  loadSessions();
  loadValidationEnvs();
});

async function loadSessions() {
  try {
    const data = await request<{ items: SessionSummary[] }>("/ai/chat/sessions");
    sessions.value = data.items || [];
    if (sessions.value.length > 0) {
      await loadSession(sessions.value[0].id);
    } else {
      await createSession();
    }
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value = [
      {
        role: "assistant",
        content: apiErr.message ? `加载会话失败: ${apiErr.message}` : "加载会话失败，请检查服务是否启动"
      }
    ];
  }
}

async function loadValidationEnvs() {
  try {
    const data = await request<{ items: ValidationEnvSummary[] }>("/validation-envs");
    validationEnvs.value = data.items || [];
    if (!selectedValidationEnv.value && validationEnvs.value.length > 0) {
      selectedValidationEnv.value = validationEnvs.value[0].name;
    }
  } catch (err) {
    validationEnvs.value = [];
  }
}

async function loadSession(id: string) {
  if (!id) return;
  try {
    const data = await request<{ session: Session }>(`/ai/chat/sessions/${id}`);
    activeSessionId.value = data.session.id;
    messages.value = data.session.messages || [];
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value = [
      {
        role: "assistant",
        content: apiErr.message ? `加载会话失败: ${apiErr.message}` : "加载会话失败，请检查服务是否启动"
      }
    ];
  }
}

async function createSession() {
  try {
    const data = await request<{ session: Session }>("/ai/chat/sessions", { method: "POST", body: {} });
    activeSessionId.value = data.session.id;
    messages.value = data.session.messages || [];
    syncSessionSummary(data.session);
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value = [
      {
        role: "assistant",
        content: apiErr.message ? `创建会话失败: ${apiErr.message}` : "创建会话失败，请检查服务是否启动"
      }
    ];
  }
}

async function selectSession() {
  if (!activeSessionId.value) return;
  await loadSession(activeSessionId.value);
}

async function sendMessage() {
  if (!input.value.trim()) return;
  const content = input.value.trim();
  input.value = "";
  await ensureSession();
  await sendChatMessage(content);
}

async function sendChatMessage(content: string) {
  if (!activeSessionId.value) return;
  busy.value = true;
  try {
    const data = await request<{ reply: Message; session: Session }>(
      `/ai/chat/sessions/${activeSessionId.value}/messages`,
      {
        method: "POST",
        body: { content }
      }
    );
    messages.value = data.session.messages || [];
    syncSessionSummary(data.session);
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value.push({
      role: "assistant",
      content: apiErr.message ? `发送失败: ${apiErr.message}` : "发送失败，请检查服务是否启动"
    });
  } finally {
    busy.value = false;
  }
}

async function generateYaml(promptOverride?: string) {
  const prompt = promptOverride || input.value.trim() || buildPromptFromMessages();
  if (!prompt) return;
  busy.value = true;
  try {
    const data = await request<{ yaml?: string }>("/ai/workflow/generate", {
      method: "POST",
      body: { prompt }
    });
    if (data.yaml) {
      yaml.value = data.yaml;
      await validateDraft();
      messages.value.push({ role: "assistant", content: "模板已生成，请查看右侧编辑器。" });
      syncSessionSummaryFromMessages();
    }
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value.push({
      role: "assistant",
      content: apiErr.message ? `生成失败: ${apiErr.message}` : "生成失败，请检查服务是否启动"
    });
  } finally {
    busy.value = false;
  }
}

async function runValidation() {
  if (!yaml.value.trim() || !selectedValidationEnv.value) return;
  validationBusy.value = true;
  validationResult.value = null;
  try {
    const data = await request<ValidationResult>("/validation-runs", {
      method: "POST",
      body: { env: selectedValidationEnv.value, yaml: yaml.value }
    });
    validationResult.value = data;
  } catch (err) {
    const apiErr = err as ApiError;
    validationResult.value = {
      status: "failed",
      error: apiErr.message ? `验证失败: ${apiErr.message}` : "验证失败，请检查服务是否启动"
    };
  } finally {
    validationBusy.value = false;
  }
}

async function validateDraft() {
  if (!yaml.value.trim()) return;
  try {
    const data = await request<{ ok: boolean; issues?: string[] }>(
      "/workflows/draft/validate",
      { method: "POST", body: { yaml: yaml.value } }
    );
    validation.value = { ok: data.ok, issues: data.issues || [] };
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [apiErr.message ? `校验失败: ${apiErr.message}` : "校验失败，请检查服务是否启动"]
    };
  }
}

async function saveWorkflow() {
  const name = window.prompt("请输入工作流名称（字母/数字/短横线/下划线）");
  if (!name) return;
  const trimmed = name.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(trimmed)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }
  busy.value = true;
  try {
    await request(`/workflows/${trimmed}`, {
      method: "PUT",
      body: { yaml: yaml.value }
    });
    messages.value.push({ role: "assistant", content: `已保存为工作流: ${trimmed}` });
  } catch (err) {
    const apiErr = err as ApiError;
    messages.value.push({
      role: "assistant",
      content: apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败，请检查服务是否启动"
    });
  } finally {
    busy.value = false;
  }
}

function parseSteps(content: string) {
  const lines = content.split(/\r?\n/);
  const parsed: { name: string; action: string; targets: string }[] = [];
  let current: { name: string; action: string; targets: string } | null = null;

  for (const line of lines) {
    const nameMatch = line.match(/^\s*-\s*name\s*:\s*(.+)$/);
    if (nameMatch) {
      current = { name: nameMatch[1].trim(), action: "", targets: "" };
      parsed.push(current);
      continue;
    }
    if (!current) continue;
    const actionMatch = line.match(/^\s*action\s*:\s*(.+)$/);
    if (actionMatch) {
      current.action = actionMatch[1].trim();
    }
    const targetsMatch = line.match(/^\s*targets\s*:\s*(.+)$/);
    if (targetsMatch) {
      current.targets = targetsMatch[1].trim();
    }
  }

  return parsed;
}

async function ensureSession() {
  if (activeSessionId.value) return;
  await createSession();
}

function syncSessionSummary(session: Session) {
  const updated = {
    id: session.id,
    title: session.title,
    updated_at: new Date().toISOString(),
    message_count: session.messages.length
  };
  const index = sessions.value.findIndex((item) => item.id === session.id);
  if (index >= 0) {
    sessions.value[index] = updated;
  } else {
    sessions.value.unshift(updated);
  }
}

function syncSessionSummaryFromMessages() {
  if (!activeSessionId.value) return;
  const index = sessions.value.findIndex((item) => item.id === activeSessionId.value);
  if (index >= 0) {
    sessions.value[index] = {
      ...sessions.value[index],
      message_count: messages.value.length,
      updated_at: new Date().toISOString()
    };
  }
}

function buildPromptFromMessages() {
  const userMessages = messages.value
    .filter((msg) => msg.role === "user")
    .map((msg) => msg.content.trim())
    .filter(Boolean);
  if (userMessages.length === 0) return "";
  return userMessages.map((msg, idx) => `${idx + 1}. ${msg}`).join("\n");
}
</script>

<style scoped>
.home {
  display: grid;
  grid-template-columns: minmax(360px, 1.2fr) minmax(360px, 0.8fr);
  gap: 18px;
  min-height: calc(100vh - 140px);
}

.chat,
.editor {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 18px;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.chat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.session-bar {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: #fff7ef;
  margin-bottom: 14px;
}

.session-label {
  font-size: 12px;
  color: var(--muted);
}

.session-select {
  background: #fff;
  border: 1px solid rgba(27, 27, 27, 0.2);
  border-radius: 10px;
  padding: 6px 10px;
  font-size: 13px;
}

.session-meta {
  font-size: 12px;
  color: var(--muted);
}

.validation-bar {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: #fff7ef;
}

.validation-bar select {
  background: #fff;
  border: 1px solid rgba(27, 27, 27, 0.2);
  border-radius: 10px;
  padding: 6px 10px;
  font-size: 13px;
}

.validation-label,
.validation-hint {
  font-size: 12px;
  color: var(--muted);
}

.validation-result {
  margin-top: 16px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: #f7f7f2;
}

.validation-result.success {
  border-color: rgba(34, 139, 77, 0.4);
}

.validation-result.failed {
  border-color: rgba(222, 57, 24, 0.4);
}

.result-title {
  font-weight: 600;
  margin-bottom: 8px;
}

.result-error {
  color: #c0392b;
  font-size: 12px;
  margin-bottom: 8px;
}

.result-io {
  display: grid;
  gap: 10px;
}

.result-block {
  background: #11100f;
  color: #f2f2e9;
  border-radius: 10px;
  padding: 8px 10px;
  font-size: 12px;
}

.result-label {
  font-size: 11px;
  opacity: 0.7;
  margin-bottom: 6px;
}

.result-block pre {
  margin: 0;
  white-space: pre-wrap;
}

.chat-header h1 {
  font-family: "Space Grotesk", sans-serif;
  font-size: 26px;
  margin: 0 0 6px;
}

.chat-header p {
  margin: 0;
  color: var(--muted);
}

.quick-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 14px;
  cursor: pointer;
  font-size: 12px;
  border-radius: var(--radius-sm);
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.btn.ghost {
  border-color: var(--grid);
  color: var(--muted);
}

.chat-body {
  flex: 1;
  overflow: auto;
  margin: 16px 0;
  padding-right: 4px;
}

.message {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.message .avatar {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  font-size: 12px;
  font-weight: 600;
  background: #f4efe6;
  color: var(--ink);
}

.message.assistant .avatar {
  background: #fff1e6;
  color: var(--brand);
}

.message .bubble {
  flex: 1;
  border-radius: 14px;
  border: 1px solid var(--grid);
  background: #faf8f4;
  padding: 10px 12px;
}

.message.assistant .bubble {
  background: #fff;
}

.message .role {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--muted);
  margin-bottom: 6px;
}

.message .content {
  font-size: 14px;
  line-height: 1.5;
}

.chat-input textarea {
  width: 100%;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 12px;
  font-size: 13px;
  background: #fff;
  resize: vertical;
}

.input-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 10px;
}

.input-actions .hint {
  font-size: 11px;
  color: var(--muted);
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.editor-header h2 {
  margin: 0 0 6px;
  font-size: 20px;
}

.editor-header p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.editor-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.editor-body {
  display: grid;
  gap: 12px;
  margin-top: 12px;
  overflow: auto;
}

.code {
  width: 100%;
  min-height: 260px;
  border-radius: var(--radius-md);
  border: 1px solid #111111;
  background: #111111;
  color: #f4f1ec;
  font-family: "JetBrains Mono", monospace;
  font-size: 13px;
  padding: 12px;
  line-height: 1.5;
  resize: vertical;
}

.validation {
  font-size: 12px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  width: fit-content;
}

.validation.ok {
  color: var(--ok);
}

.validation.warn {
  color: var(--err);
}

.issues {
  margin: 0;
  padding-left: 18px;
  color: var(--err);
  font-size: 12px;
}

.preview {
  border-top: 1px dashed var(--grid);
  padding-top: 12px;
}

.preview-title {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--muted);
  margin-bottom: 8px;
}

.step-card {
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  margin-bottom: 8px;
  background: #faf8f4;
}

.step-name {
  font-weight: 600;
}

.step-meta,
.step-targets {
  font-size: 12px;
  color: var(--muted);
  margin-top: 4px;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 1100px) {
  .home {
    grid-template-columns: 1fr;
  }
}
</style>
