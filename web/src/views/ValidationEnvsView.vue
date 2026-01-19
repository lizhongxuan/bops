<template>
  <section class="envs">
    <div class="envs-header">
      <div>
        <h1>验证环境</h1>
        <p>管理容器/SSH/Agent 运行环境，用于验证工作流。</p>
      </div>
      <div class="actions">
        <input v-model="query" type="text" placeholder="搜索名称或描述" />
        <button class="btn primary" type="button" @click="createEnv">新建环境</button>
      </div>
    </div>

    <div class="envs-body">
      <aside class="panel list">
        <div class="panel-title">环境列表</div>
        <div v-if="loading" class="empty">加载中...</div>
        <div v-else-if="error" class="empty">{{ error }}</div>
        <div v-else class="list-body">
          <button
            v-for="item in filteredEnvs"
            :key="item.name"
            class="list-item"
            :class="{ active: item.name === selectedName }"
            type="button"
            @click="selectEnv(item.name)"
          >
            <div class="item-title">{{ item.name }}</div>
            <div class="item-desc">{{ item.description || "暂无描述" }}</div>
            <div class="item-meta">{{ typeLabel(item.type) }} · 更新 {{ formatAge(item.updated_at) }}</div>
          </button>
          <div v-if="filteredEnvs.length === 0" class="empty">暂无验证环境</div>
        </div>
      </aside>

      <section class="panel editor">
        <div class="panel-title">环境配置</div>
        <div v-if="!selectedName" class="empty">请选择或新建一个环境</div>
        <div v-else class="editor-body">
          <label class="field">
            <span>名称</span>
            <input v-model="form.name" type="text" :disabled="saving" />
          </label>
          <label class="field">
            <span>类型</span>
            <select v-model="form.type" :disabled="saving">
              <option value="container">容器</option>
              <option value="ssh">SSH</option>
              <option value="agent">Agent</option>
            </select>
          </label>
          <label class="field">
            <span>描述</span>
            <input v-model="form.description" type="text" :disabled="saving" />
          </label>

          <div v-if="form.type === 'container'" class="field">
            <span>容器镜像</span>
            <input v-model="form.image" type="text" placeholder="bops-agent:latest" :disabled="saving" />
          </div>

          <div v-if="form.type === 'ssh'" class="field">
            <span>主机地址</span>
            <input v-model="form.host" type="text" placeholder="10.0.0.10 或 10.0.0.10:22" :disabled="saving" />
          </div>
          <div v-if="form.type === 'ssh'" class="field">
            <span>用户名</span>
            <input v-model="form.user" type="text" placeholder="root" :disabled="saving" />
          </div>
          <div v-if="form.type === 'ssh'" class="field">
            <span>SSH 私钥</span>
            <input v-model="form.ssh_key" type="text" placeholder="~/.ssh/id_rsa" :disabled="saving" />
          </div>

          <div v-if="form.type === 'agent'" class="field">
            <span>Agent 地址</span>
            <input v-model="form.agent_address" type="text" placeholder="10.0.0.10:7071" :disabled="saving" />
          </div>
          <div v-if="form.type === 'agent'" class="field">
            <span>用户名</span>
            <input v-model="form.user" type="text" placeholder="root" :disabled="saving" />
          </div>
          <div v-if="form.type === 'agent'" class="field">
            <span>SSH 私钥</span>
            <input v-model="form.ssh_key" type="text" placeholder="~/.ssh/id_rsa" :disabled="saving" />
          </div>

          <div class="env-grid">
            <div class="env-header">
              <span>标签</span>
              <span>值</span>
              <span></span>
            </div>
            <div v-for="(row, index) in labelRows" :key="index" class="env-row">
              <input v-model="row.key" type="text" placeholder="team" />
              <input v-model="row.value" type="text" placeholder="ops" />
              <button class="ghost" type="button" @click="removeLabel(index)">删除</button>
            </div>
            <button class="btn" type="button" @click="addLabel">添加标签</button>
          </div>

          <div class="editor-actions">
            <button class="btn primary" type="button" :disabled="saving" @click="saveEnv">
              保存
            </button>
            <button class="btn ghost" type="button" :disabled="saving" @click="deleteEnv">
              删除
            </button>
            <span class="status">{{ statusMessage }}</span>
          </div>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, request } from "../lib/api";

type EnvSummary = {
  name: string;
  type: string;
  description: string;
  updated_at: string;
};

type ValidationEnv = {
  name: string;
  type: string;
  description: string;
  labels: Record<string, string>;
  image: string;
  host: string;
  user: string;
  ssh_key: string;
  agent_address: string;
};

const query = ref("");
const envs = ref<EnvSummary[]>([]);
const selectedName = ref("");
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const statusMessage = ref("");

const form = ref<ValidationEnv>({
  name: "",
  type: "container",
  description: "",
  labels: {},
  image: "",
  host: "",
  user: "",
  ssh_key: "",
  agent_address: ""
});

const labelRows = ref<{ key: string; value: string }[]>([]);

const filteredEnvs = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  if (!keyword) return envs.value;
  return envs.value.filter((item) => {
    const haystack = `${item.name} ${item.description}`.toLowerCase();
    return haystack.includes(keyword);
  });
});

async function loadEnvs() {
  loading.value = true;
  error.value = "";
  try {
    const data = await request<{ items: EnvSummary[] }>("/validation-envs");
    envs.value = data.items || [];
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

async function selectEnv(name: string) {
  selectedName.value = name;
  statusMessage.value = "";
  try {
    const data = await request<ValidationEnv>(`/validation-envs/${name}`);
    form.value = {
      name: data.name,
      type: data.type || "container",
      description: data.description || "",
      labels: data.labels || {},
      image: data.image || "",
      host: data.host || "",
      user: data.user || "",
      ssh_key: data.ssh_key || "",
      agent_address: data.agent_address || ""
    };
    labelRows.value = Object.entries(form.value.labels).map(([key, value]) => ({ key, value }));
  } catch (err) {
    statusMessage.value = "加载失败";
  }
}

function addLabel() {
  labelRows.value.push({ key: "", value: "" });
}

function removeLabel(index: number) {
  labelRows.value.splice(index, 1);
}

async function saveEnv() {
  if (!form.value.name.trim()) {
    statusMessage.value = "名称不能为空";
    return;
  }
  if (!form.value.type.trim()) {
    statusMessage.value = "类型不能为空";
    return;
  }
  saving.value = true;
  statusMessage.value = "保存中...";

  const labels: Record<string, string> = {};
  for (const row of labelRows.value) {
    if (!row.key.trim()) continue;
    labels[row.key.trim()] = row.value.trim();
  }

  try {
    const payload = {
      name: form.value.name.trim(),
      type: form.value.type,
      description: form.value.description.trim(),
      labels,
      image: form.value.image.trim(),
      host: form.value.host.trim(),
      user: form.value.user.trim(),
      ssh_key: form.value.ssh_key.trim(),
      agent_address: form.value.agent_address.trim()
    };
    const data = await request<ValidationEnv>(`/validation-envs/${payload.name}`, {
      method: "PUT",
      body: payload
    });
    form.value = {
      name: data.name,
      type: data.type || "container",
      description: data.description || "",
      labels: data.labels || {},
      image: data.image || "",
      host: data.host || "",
      user: data.user || "",
      ssh_key: data.ssh_key || "",
      agent_address: data.agent_address || ""
    };
    labelRows.value = Object.entries(form.value.labels).map(([key, value]) => ({ key, value }));
    selectedName.value = data.name;
    statusMessage.value = "保存成功";
    await loadEnvs();
  } catch (err) {
    const apiErr = err as ApiError;
    statusMessage.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败";
  } finally {
    saving.value = false;
  }
}

async function deleteEnv() {
  if (!selectedName.value) return;
  if (!window.confirm(`确定删除环境 ${selectedName.value} 吗？`)) return;
  saving.value = true;
  statusMessage.value = "删除中...";
  try {
    await request(`/validation-envs/${selectedName.value}`, { method: "DELETE" });
    selectedName.value = "";
    form.value = {
      name: "",
      type: "container",
      description: "",
      labels: {},
      image: "",
      host: "",
      user: "",
      ssh_key: "",
      agent_address: ""
    };
    labelRows.value = [];
    statusMessage.value = "已删除";
    await loadEnvs();
  } catch (err) {
    statusMessage.value = "删除失败";
  } finally {
    saving.value = false;
  }
}

async function createEnv() {
  const raw = window.prompt("请输入验证环境名称（字母/数字/短横线/下划线）");
  if (!raw) return;
  const name = raw.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }
  selectedName.value = name;
  form.value = {
    name,
    type: "container",
    description: "",
    labels: {},
    image: "",
    host: "",
    user: "",
    ssh_key: "",
    agent_address: ""
  };
  labelRows.value = [];
  await saveEnv();
}

function typeLabel(value: string) {
  switch (value) {
    case "container":
      return "容器";
    case "ssh":
      return "SSH";
    case "agent":
      return "Agent";
    default:
      return value || "未知";
  }
}

function formatAge(value: string) {
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return value || "未知";
  const diff = Math.max(0, Date.now() - ts);
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  return `${days} 天前`;
}

onMounted(() => {
  loadEnvs();
});
</script>

<style scoped>
.envs {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.envs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.actions input {
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: #fff;
}

.envs-body {
  display: grid;
  grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
  gap: 18px;
  min-height: 520px;
}

.panel {
  background: var(--panel);
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 16px;
  box-shadow: var(--shadow);
}

.panel-title {
  font-weight: 600;
  margin-bottom: 12px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.list-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: auto;
  max-height: 520px;
}

.list-item {
  text-align: left;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid transparent;
  background: #fff;
  cursor: pointer;
}

.list-item.active {
  border-color: var(--brand);
  box-shadow: 0 10px 18px rgba(232, 93, 42, 0.12);
}

.item-title {
  font-weight: 600;
}

.item-desc {
  font-size: 12px;
  color: var(--muted);
  margin-top: 4px;
}

.item-meta {
  font-size: 11px;
  color: var(--muted);
  margin-top: 6px;
}

.editor-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field input,
.field select {
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #fff;
}

.env-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.env-header,
.env-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

.env-row input {
  padding: 6px 8px;
  border-radius: 8px;
  border: 1px solid rgba(27, 27, 27, 0.12);
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status {
  font-size: 12px;
  color: var(--muted);
}

.empty {
  color: var(--muted);
  font-size: 13px;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 14px;
  border-radius: 999px;
  font-size: 12px;
  cursor: pointer;
}

.btn.primary {
  background: var(--brand);
  color: #fff;
  border-color: var(--brand);
}

.btn.ghost {
  border-color: rgba(27, 27, 27, 0.2);
}

@media (max-width: 980px) {
  .envs-body {
    grid-template-columns: 1fr;
  }
}
</style>
