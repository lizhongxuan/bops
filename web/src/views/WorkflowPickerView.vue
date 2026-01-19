<template>
  <section class="picker">
    <div class="picker-header">
      <div>
        <h1>选择工作流</h1>
        <p>从工作流库中选择一个开始编排、执行与追踪。</p>
      </div>
      <div class="actions">
        <button class="btn primary" type="button" @click="createWorkflow">新建工作流</button>
      </div>
    </div>

    <div class="picker-body">
      <aside class="panel side">
        <div class="panel-title">最近使用</div>
        <button
          v-if="recentWorkflow"
          class="recent-card"
          type="button"
          @click="selectWorkflow(recentWorkflow)"
        >
          <div class="recent-name">{{ recentWorkflow }}</div>
          <div class="recent-meta">上次打开 5 分钟前</div>
        </button>
        <div v-else class="empty">暂无最近记录</div>

        <div class="panel-title small">筛选</div>
        <input
          class="search"
          v-model="query"
          type="text"
          placeholder="搜索名称或描述"
        />
        <div class="tag-row">
          <button class="tag" type="button">发布中</button>
          <button class="tag" type="button">生产环境</button>
          <button class="tag" type="button">变更计划</button>
        </div>
      </aside>

      <section class="panel list">
        <div class="panel-title">
          <div>工作流库</div>
          <div class="count">{{ filteredWorkflows.length }} 个</div>
        </div>
        <div class="grid">
          <div v-if="loading" class="empty">加载中...</div>
          <div v-else-if="error" class="empty">{{ error }}</div>
          <template v-else>
            <button
              v-for="item in filteredWorkflows"
              :key="item.name"
              class="workflow-card"
              type="button"
              @click="selectWorkflow(item.name)"
            >
              <div class="card-top">
                <div class="workflow-name">{{ item.name }}</div>
                <span class="status" :class="item.status">{{ statusLabel(item.status) }}</span>
              </div>
              <div class="workflow-desc">{{ item.desc }}</div>
              <div class="workflow-meta">
                <span>更新 {{ formatAge(item.updatedAt) }}</span>
              </div>
              <div v-if="item.tags.length" class="chip-row">
                <span class="chip" v-for="tag in item.tags" :key="tag">{{ tag }}</span>
              </div>
            </button>
            <div v-if="filteredWorkflows.length === 0" class="empty">
              未找到匹配的工作流
            </div>
          </template>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError, request } from "../lib/api";

type WorkflowCard = {
  name: string;
  desc: string;
  status: "draft" | "active" | "locked";
  updatedAt: string;
  tags: string[];
};

type WorkflowListResponse = {
  items: { name: string; description: string; updated_at: string }[];
};

const router = useRouter();
const query = ref("");
const recentWorkflow = ref(localStorage.getItem("bops-last-workflow") || "");
const loading = ref(false);
const error = ref("");

const workflows = ref<WorkflowCard[]>([]);

const filteredWorkflows = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  if (!keyword) return workflows.value;
  return workflows.value.filter((item) => {
    const haystack = [item.name, item.desc, item.tags.join(" ")].join(" ").toLowerCase();
    return haystack.includes(keyword);
  });
});

function selectWorkflow(name: string) {
  localStorage.setItem("bops-last-workflow", name);
  recentWorkflow.value = name;
  router.push(`/workflows/${name}`);
}

async function createWorkflow() {
  const raw = window.prompt("请输入工作流名称（字母/数字/短横线/下划线）");
  if (!raw) return;
  const name = raw.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }

  const yaml = defaultYaml(name);
  try {
    await request(`/workflows/${name}`, {
      method: "PUT",
      body: { yaml }
    });
    await loadWorkflows();
    selectWorkflow(name);
  } catch (err) {
    const message = (err as ApiError).message || "创建失败";
    window.alert(`创建失败: ${message}`);
  }
}

async function loadWorkflows() {
  loading.value = true;
  error.value = "";
  try {
    const data = await request<WorkflowListResponse>("/workflows");
    workflows.value = data.items.map((item) => ({
      name: item.name,
      desc: item.description || "暂无描述",
      status: "draft",
      updatedAt: item.updated_at,
      tags: []
    }));
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

function statusLabel(status: WorkflowCard["status"]) {
  if (status === "active") return "运行中";
  if (status === "locked") return "锁定";
  return "草稿";
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

function defaultYaml(name: string) {
  return `version: v0.1
name: ${name}
description: new workflow

inventory:
  hosts:
    local:
      address: 127.0.0.1

plan:
  mode: manual-approve
  strategy: sequential

steps:
  - name: run command
    targets: [local]
    action: cmd.run
    with:
      cmd: \"echo hello\"
`;
}

onMounted(() => {
  loadWorkflows();
});
</script>

<style scoped>
.picker {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.picker-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.picker-header h1 {
  font-family: "Space Grotesk", sans-serif;
  font-size: 30px;
  margin: 0 0 6px;
}

.picker-header p {
  margin: 0;
  color: var(--muted);
}

.actions {
  display: flex;
  gap: 10px;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 16px;
  cursor: pointer;
  font-size: 13px;
  border-radius: var(--radius-sm);
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  box-shadow: 0 14px 24px rgba(232, 93, 42, 0.24);
}

.picker-body {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 18px;
  min-height: 520px;
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
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-title.small {
  margin-top: 16px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--muted);
}

.side {
  display: flex;
  flex-direction: column;
}

.recent-card {
  text-align: left;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 12px 14px;
  background: #faf8f4;
  cursor: pointer;
}

.recent-card:hover {
  background: #fff;
}

.recent-name {
  font-weight: 600;
}

.recent-meta {
  font-size: 12px;
  color: var(--muted);
  margin-top: 4px;
}

.search {
  width: 100%;
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.tag {
  border: 1px solid var(--grid);
  border-radius: 999px;
  padding: 6px 10px;
  background: #fff;
  font-size: 12px;
  cursor: pointer;
}

.list .grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}

.workflow-card {
  text-align: left;
  border-radius: 18px;
  border: 1px solid var(--grid);
  padding: 16px;
  background: #ffffff;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 180px;
}

.workflow-card:hover {
  border-color: rgba(232, 93, 42, 0.4);
  box-shadow: 0 16px 26px rgba(232, 93, 42, 0.12);
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.workflow-name {
  font-weight: 600;
}

.workflow-desc {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.5;
}

.workflow-meta {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip {
  font-size: 11px;
  border: 1px solid var(--grid);
  padding: 4px 8px;
  border-radius: 999px;
  background: #faf8f4;
}

.status {
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--grid);
}

.status.active {
  color: var(--ok);
  border-color: rgba(42, 157, 75, 0.3);
}

.status.draft {
  color: var(--warn);
  border-color: rgba(224, 121, 53, 0.3);
}

.status.locked {
  color: var(--err);
  border-color: rgba(208, 52, 44, 0.3);
}

.count {
  font-size: 12px;
  color: var(--muted);
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 1100px) {
  .picker-body {
    grid-template-columns: 1fr;
  }
}
</style>
