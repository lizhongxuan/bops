<template>
  <section class="scripts">
    <div class="scripts-header">
      <div>
        <h1>脚本库</h1>
        <p>沉淀可复用的 shell / python 脚本片段。</p>
      </div>
      <div class="actions">
        <input v-model="query" type="text" placeholder="搜索名称或描述" />
        <button class="btn primary" type="button" @click="createScript">新建脚本</button>
      </div>
    </div>

    <div class="scripts-body">
      <aside class="panel list">
        <div class="panel-title">脚本列表</div>
        <div v-if="loading" class="empty">加载中...</div>
        <div v-else-if="error" class="empty">{{ error }}</div>
        <div v-else class="list-body">
          <button
            v-for="item in filteredScripts"
            :key="item.name"
            class="list-item"
            :class="{ active: item.name === selectedName }"
            type="button"
            @click="selectScript(item.name)"
          >
            <div class="item-title">{{ item.name }}</div>
            <div class="item-desc">{{ item.description || "暂无描述" }}</div>
            <div v-if="item.tags && item.tags.length" class="item-tags">
              {{ item.tags.join(" · ") }}
            </div>
            <div class="item-meta">{{ item.language || "未知" }} · 更新 {{ formatAge(item.updated_at) }}</div>
          </button>
          <div v-if="filteredScripts.length === 0" class="empty">暂无脚本</div>
        </div>
      </aside>

      <section class="panel editor">
        <div class="panel-title">脚本内容</div>
        <div v-if="!selectedName" class="empty">请选择或新建一个脚本</div>
        <div v-else class="editor-body">
          <label class="field">
            <span>名称</span>
            <input v-model="form.name" type="text" :disabled="saving" />
          </label>
          <label class="field">
            <span>语言</span>
            <select v-model="form.language" :disabled="saving">
              <option value="shell">shell</option>
              <option value="python">python</option>
            </select>
          </label>
          <label class="field">
            <span>描述</span>
            <input v-model="form.description" type="text" :disabled="saving" />
          </label>
          <label class="field">
            <span>标签</span>
            <input v-model="tagsInput" type="text" placeholder="nginx, setup" :disabled="saving" />
          </label>

          <div class="field">
            <span>脚本内容</span>
            <textarea v-model="form.content" rows="12" spellcheck="false"></textarea>
          </div>

          <div class="editor-actions">
            <button class="btn primary" type="button" :disabled="saving" @click="saveScript">
              保存
            </button>
            <button class="btn ghost" type="button" :disabled="saving" @click="deleteScript">
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

type ScriptSummary = {
  name: string;
  language: string;
  description: string;
  tags: string[];
  updated_at: string;
};

type Script = {
  name: string;
  language: string;
  description: string;
  tags: string[];
  content: string;
};

const query = ref("");
const scripts = ref<ScriptSummary[]>([]);
const selectedName = ref("");
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const statusMessage = ref("");

const form = ref<Script>({
  name: "",
  language: "shell",
  description: "",
  tags: [],
  content: ""
});
const tagsInput = ref("");

const filteredScripts = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  if (!keyword) return scripts.value;
  return scripts.value.filter((item) => {
    const tags = (item.tags || []).join(" ");
    const haystack = `${item.name} ${item.description} ${tags}`.toLowerCase();
    return haystack.includes(keyword);
  });
});

async function loadScripts() {
  loading.value = true;
  error.value = "";
  try {
    const data = await request<{ items: ScriptSummary[] }>("/scripts");
    scripts.value = data.items || [];
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

async function selectScript(name: string) {
  selectedName.value = name;
  statusMessage.value = "";
  try {
    const data = await request<Script>(`/scripts/${name}`);
    form.value = {
      name: data.name,
      language: data.language || "shell",
      description: data.description || "",
      tags: data.tags || [],
      content: data.content || ""
    };
    tagsInput.value = form.value.tags.join(", ");
  } catch (err) {
    statusMessage.value = "加载失败";
  }
}

async function saveScript() {
  if (!form.value.name.trim()) {
    statusMessage.value = "名称不能为空";
    return;
  }
  if (!form.value.language.trim()) {
    statusMessage.value = "语言不能为空";
    return;
  }
  saving.value = true;
  statusMessage.value = "保存中...";

  const tags = tagsInput.value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  try {
    const payload = {
      name: form.value.name.trim(),
      language: form.value.language.trim(),
      description: form.value.description.trim(),
      tags,
      content: form.value.content
    };
    const data = await request<Script>(`/scripts/${payload.name}`, {
      method: "PUT",
      body: payload
    });
    form.value = {
      name: data.name,
      language: data.language || "shell",
      description: data.description || "",
      tags: data.tags || [],
      content: data.content || ""
    };
    tagsInput.value = form.value.tags.join(", ");
    selectedName.value = data.name;
    statusMessage.value = "保存成功";
    await loadScripts();
  } catch (err) {
    const apiErr = err as ApiError;
    statusMessage.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败";
  } finally {
    saving.value = false;
  }
}

async function deleteScript() {
  if (!selectedName.value) return;
  if (!window.confirm(`确定删除脚本 ${selectedName.value} 吗？`)) return;
  saving.value = true;
  statusMessage.value = "删除中...";
  try {
    await request(`/scripts/${selectedName.value}`, { method: "DELETE" });
    selectedName.value = "";
    form.value = {
      name: "",
      language: "shell",
      description: "",
      tags: [],
      content: ""
    };
    tagsInput.value = "";
    statusMessage.value = "已删除";
    await loadScripts();
  } catch (err) {
    statusMessage.value = "删除失败";
  } finally {
    saving.value = false;
  }
}

async function createScript() {
  const raw = window.prompt("请输入脚本名称（字母/数字/短横线/下划线）");
  if (!raw) return;
  const name = raw.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }
  selectedName.value = name;
  form.value = {
    name,
    language: "shell",
    description: "",
    tags: [],
    content: ""
  };
  tagsInput.value = "";
  await saveScript();
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
  loadScripts();
});
</script>

<style scoped>
.scripts {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.scripts-header {
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

.scripts-body {
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

.item-tags {
  font-size: 11px;
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
.field select,
.field textarea {
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #fff;
  font-size: 13px;
}

.field textarea {
  font-family: "Space Grotesk", sans-serif;
  min-height: 220px;
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
  .scripts-body {
    grid-template-columns: 1fr;
  }
}
</style>
