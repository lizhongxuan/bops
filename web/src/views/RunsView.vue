<template>
  <section class="runs">
    <div class="runs-header">
      <div>
        <h1>运行记录</h1>
        <p v-if="isScoped">工作流：{{ workflowName }} · 查看该模板的执行历史</p>
        <p v-else>全局视角查看所有工作流的执行记录与状态</p>
      </div>
      <div class="actions">
        <input v-model="query" type="text" placeholder="搜索工作流或运行 ID" />
        <select v-model="status">
          <option value="all">全部状态</option>
          <option value="running">执行中</option>
          <option value="success">成功</option>
          <option value="failed">失败</option>
          <option value="stopped">已停止</option>
        </select>
        <select v-model="timeRange">
          <option value="all">全部时间</option>
          <option value="24h">最近 24 小时</option>
          <option value="7d">最近 7 天</option>
          <option value="30d">最近 30 天</option>
        </select>
      </div>
    </div>

    <section class="panel">
      <div class="list-head">
        <span>状态</span>
        <span>运行 ID</span>
        <span>工作流</span>
        <span>开始时间</span>
        <span>耗时</span>
        <span>失败步骤</span>
        <span>备注</span>
        <span>详情</span>
      </div>
      <div v-if="loading" class="empty">加载中...</div>
      <div v-else-if="error" class="empty">{{ error }}</div>
      <div v-else v-for="run in filteredRuns" :key="run.run_id" class="list-row">
        <span class="badge" :class="run.status">{{ statusLabel(run.status) }}</span>
        <span class="mono">{{ run.run_id }}</span>
        <span>{{ run.workflow_name }}</span>
        <span>{{ formatTime(run.started_at) }}</span>
        <span>{{ formatDuration(run.started_at, run.finished_at) }}</span>
        <span>{{ run.failed_step || "-" }}</span>
        <span class="step">{{ run.failed_host || "-" }}</span>
        <RouterLink
          class="link"
          :to="{
            name: 'run-console',
            params: { id: run.run_id },
            query: { workflow: run.workflow_name }
          }"
        >
          查看
        </RouterLink>
      </div>
      <div v-if="!loading && !error && filteredRuns.length === 0" class="empty">
        暂无匹配的运行记录
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { request } from "../lib/api";

type RunItem = {
  run_id: string;
  workflow_name: string;
  status: "queued" | "running" | "success" | "failed" | "stopped";
  started_at: string;
  finished_at: string;
  failed_step?: string;
  failed_host?: string;
};

const route = useRoute();
const workflowName = computed(() =>
  typeof route.params.name === "string" ? route.params.name : ""
);
const isScoped = computed(() => workflowName.value.length > 0);

const query = ref("");
const status = ref("all");
const timeRange = ref("all");
const loading = ref(false);
const error = ref("");

const runs = ref<RunItem[]>([]);

const filteredRuns = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  return runs.value.filter((run) => {
    if (!keyword) return true;
    const haystack = `${run.run_id} ${run.workflow_name}`.toLowerCase();
    return haystack.includes(keyword);
  });
});

async function loadRuns() {
  loading.value = true;
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (status.value !== "all") {
      params.set("status", status.value);
    }
    if (timeRange.value !== "all") {
      const now = Date.now();
      const range =
        timeRange.value === "24h"
          ? 24 * 60 * 60 * 1000
          : timeRange.value === "7d"
            ? 7 * 24 * 60 * 60 * 1000
            : 30 * 24 * 60 * 60 * 1000;
      params.set("from", new Date(now - range).toISOString());
      params.set("to", new Date(now).toISOString());
    }
    const base = isScoped.value
      ? `/workflows/${workflowName.value}/runs`
      : "/runs";
    const url = params.toString() ? `${base}?${params.toString()}` : base;
    const data = await request<{ items: RunItem[] }>(url);
    runs.value = data.items || [];
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

function statusLabel(value: RunItem["status"]) {
  if (value === "running") return "执行中";
  if (value === "success") return "成功";
  if (value === "stopped") return "已停止";
  if (value === "queued") return "排队中";
  return "失败";
}

function formatTime(value: string) {
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return "-";
  return new Date(ts).toLocaleString();
}

function formatDuration(started: string, finished: string) {
  const start = Date.parse(started);
  const end = Date.parse(finished);
  if (Number.isNaN(start) || Number.isNaN(end) || end <= start) return "-";
  const diff = Math.floor((end - start) / 1000);
  const mins = Math.floor(diff / 60);
  const secs = diff % 60;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}

watch([status, timeRange, workflowName], () => {
  void loadRuns();
});

onMounted(() => {
  void loadRuns();
});
</script>

<style scoped>
.runs {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.runs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.runs-header h1 {
  font-family: "Space Grotesk", sans-serif;
  font-size: 28px;
  margin: 0 0 6px;
}

.runs-header p {
  margin: 0;
  color: var(--muted);
}

.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.actions input,
.actions select {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
}

.list-head,
.list-row {
  display: grid;
  grid-template-columns: 90px 160px 160px 130px 90px 90px 1fr 80px;
  gap: 12px;
  align-items: center;
}

.list-head {
  font-size: 12px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  margin-bottom: 12px;
}

.list-row {
  border: 1px solid var(--grid);
  border-radius: var(--radius-md);
  padding: 12px;
  margin-bottom: 10px;
  background: #faf8f4;
}

.badge {
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  width: fit-content;
}

.badge.success {
  color: var(--ok);
  border-color: rgba(42, 157, 75, 0.3);
}

.badge.failed {
  color: var(--err);
  border-color: rgba(208, 52, 44, 0.3);
}

.badge.running {
  color: var(--info);
  border-color: rgba(46, 111, 227, 0.3);
}

.mono {
  font-family: "JetBrains Mono", monospace;
  font-size: 12px;
}

.step {
  color: var(--muted);
  font-size: 13px;
}

.link {
  font-size: 12px;
  color: var(--info);
}

.empty {
  font-size: 12px;
  color: var(--muted);
  padding: 8px 4px;
}

@media (max-width: 1100px) {
  .runs-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .list-head {
    display: none;
  }

  .list-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }
}
</style>
