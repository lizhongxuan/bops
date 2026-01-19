<template>
  <section class="envs">
    <div class="envs-header">
      <div>
        <h1>环境变量包</h1>
        <p>集中管理可复用的环境变量配置。</p>
      </div>
      <div class="actions">
        <input v-model="query" type="text" placeholder="搜索名称或描述" />
        <button class="btn primary" type="button" @click="createPackage">新建包</button>
      </div>
    </div>

    <div class="envs-body">
      <aside class="panel list">
        <div class="panel-title">变量包列表</div>
        <div v-if="loading" class="empty">加载中...</div>
        <div v-else-if="error" class="empty">{{ error }}</div>
        <div v-else class="list-body">
          <button
            v-for="item in filteredPackages"
            :key="item.name"
            class="list-item"
            :class="{ active: item.name === selectedName }"
            type="button"
            @click="selectPackage(item.name)"
          >
            <div class="item-title">{{ item.name }}</div>
            <div class="item-desc">{{ item.description || "暂无描述" }}</div>
            <div class="item-meta">更新 {{ formatAge(item.updated_at) }}</div>
          </button>
          <div v-if="filteredPackages.length === 0" class="empty">暂无变量包</div>
        </div>
      </aside>

      <section class="panel editor">
        <div class="panel-title">变量包配置</div>
        <div v-if="!selectedName" class="empty">请选择或新建一个变量包</div>
        <div v-else class="editor-body">
          <label class="field">
            <span>名称</span>
            <input v-model="form.name" type="text" :disabled="saving" />
          </label>
          <label class="field">
            <span>描述</span>
            <input v-model="form.description" type="text" :disabled="saving" />
          </label>

          <div class="env-grid">
            <div class="env-header">
              <span>变量名</span>
              <span>变量值</span>
              <span></span>
            </div>
            <div v-for="(row, index) in envRows" :key="index" class="env-row">
              <input v-model="row.key" type="text" placeholder="TOKEN" />
              <input v-model="row.value" type="text" placeholder="value" />
              <button class="ghost" type="button" @click="removeEnv(index)">删除</button>
            </div>
            <button class="btn" type="button" @click="addEnv">添加变量</button>
          </div>

          <div class="editor-actions">
            <button class="btn primary" type="button" :disabled="saving" @click="savePackage">
              保存
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

type EnvPackageSummary = {
  name: string;
  description: string;
  updated_at: string;
};

type EnvPackage = {
  name: string;
  description: string;
  env: Record<string, string>;
};

const query = ref("");
const packages = ref<EnvPackageSummary[]>([]);
const selectedName = ref("");
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const statusMessage = ref("");

const form = ref<EnvPackage>({ name: "", description: "", env: {} });
const envRows = ref<{ key: string; value: string }[]>([]);

const filteredPackages = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  if (!keyword) return packages.value;
  return packages.value.filter((item) => {
    const haystack = `${item.name} ${item.description}`.toLowerCase();
    return haystack.includes(keyword);
  });
});

async function loadPackages() {
  loading.value = true;
  error.value = "";
  try {
    const data = await request<{ items: EnvPackageSummary[] }>("/envs");
    packages.value = data.items || [];
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

async function selectPackage(name: string) {
  selectedName.value = name;
  statusMessage.value = "";
  try {
    const data = await request<EnvPackage>(`/envs/${name}`);
    form.value = {
      name: data.name,
      description: data.description || "",
      env: data.env || {}
    };
    envRows.value = Object.entries(form.value.env).map(([key, value]) => ({
      key,
      value
    }));
  } catch (err) {
    statusMessage.value = "加载失败";
  }
}

function addEnv() {
  envRows.value.push({ key: "", value: "" });
}

function removeEnv(index: number) {
  envRows.value.splice(index, 1);
}

async function savePackage() {
  if (!form.value.name.trim()) {
    statusMessage.value = "名称不能为空";
    return;
  }
  saving.value = true;
  statusMessage.value = "保存中...";

  const env: Record<string, string> = {};
  for (const row of envRows.value) {
    if (!row.key.trim()) continue;
    env[row.key.trim()] = row.value;
  }

  try {
    const payload = {
      name: form.value.name.trim(),
      description: form.value.description.trim(),
      env
    };
    const data = await request<EnvPackage>(`/envs/${payload.name}`, {
      method: "PUT",
      body: payload
    });
    form.value = {
      name: data.name,
      description: data.description || "",
      env: data.env || {}
    };
    envRows.value = Object.entries(form.value.env).map(([key, value]) => ({
      key,
      value
    }));
    selectedName.value = data.name;
    statusMessage.value = "保存成功";
    await loadPackages();
  } catch (err) {
    const apiErr = err as ApiError;
    statusMessage.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败";
  } finally {
    saving.value = false;
  }
}

async function createPackage() {
  const raw = window.prompt("请输入环境变量包名称（字母/数字/短横线/下划线）");
  if (!raw) return;
  const name = raw.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }
  selectedName.value = name;
  form.value = { name, description: "", env: {} };
  envRows.value = [];
  await savePackage();
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
  loadPackages();
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

.envs-header h1 {
  font-family: "Space Grotesk", sans-serif;
  font-size: 28px;
  margin: 0 0 6px;
}

.envs-header p {
  margin: 0;
  color: var(--muted);
}

.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.actions input {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
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

.ghost {
  border: 1px solid var(--grid);
  background: transparent;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  cursor: pointer;
}

.envs-body {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 18px;
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

.list-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.list-item {
  text-align: left;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  background: #faf8f4;
  padding: 10px 12px;
}

.list-item.active {
  border-color: var(--brand);
  box-shadow: 0 12px 18px rgba(232, 93, 42, 0.12);
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
  display: grid;
  gap: 6px;
}

.field span {
  font-size: 12px;
  color: var(--muted);
}

.field input {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
}

.env-grid {
  border-top: 1px dashed var(--grid);
  padding-top: 12px;
  display: grid;
  gap: 8px;
}

.env-header {
  display: grid;
  grid-template-columns: 1fr 1fr 80px;
  gap: 8px;
  font-size: 12px;
  color: var(--muted);
}

.env-row {
  display: grid;
  grid-template-columns: 1fr 1fr 80px;
  gap: 8px;
}

.env-row input {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status {
  font-size: 12px;
  color: var(--muted);
}

.empty {
  font-size: 12px;
  color: var(--muted);
  padding: 8px 0;
}

@media (max-width: 1100px) {
  .envs-body {
    grid-template-columns: 1fr;
  }
}
</style>
