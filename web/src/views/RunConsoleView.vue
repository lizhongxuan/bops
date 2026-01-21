<template>
  <section class="run">
    <div class="run-header">
      <div>
        <h1>运行控制台</h1>
        <div class="run-meta">
          <span v-if="workflowName">工作流：{{ workflowName }}</span>
          <span v-if="runId" class="mono">{{ runId }}</span>
          <span class="badge" :class="runStatus">{{ statusLabel(runStatus) }}</span>
          <span v-if="runMessage" class="message">{{ runMessage }}</span>
          <span v-if="streamStatus" class="message warn">{{ streamStatus }}</span>
        </div>
      </div>
      <div class="actions">
        <div class="filter">
          <span class="filter-label">主机筛选</span>
          <select v-model="hostFilter" :disabled="hosts.length === 0">
            <option value="all">全部主机</option>
            <option v-for="host in hosts" :key="host" :value="host">{{ host }}</option>
          </select>
        </div>
        <RouterLink class="btn" :to="workflowLink">
          查看流程
        </RouterLink>
      </div>
    </div>

    <div v-if="loading" class="empty">加载中...</div>
    <div v-else-if="error" class="empty">{{ error }}</div>

    <div v-else class="run-body">
      <aside class="panel steps">
        <div class="panel-title">步骤列表</div>
        <div v-if="steps.length === 0" class="empty">
          暂无步骤
          <div v-if="runMessage" class="hint">失败原因: {{ runMessage }}</div>
        </div>
        <div
          v-for="(step, index) in steps"
          :key="step.name"
          class="step"
          :class="step.status"
          @click="selectStep(index)"
        >
          <div>
            <div class="step-name">{{ step.name }}</div>
            <div class="step-meta">
              {{ formatDuration(step.started_at, step.finished_at) }} ·
              {{ formatTime(step.started_at) }}
            </div>
          </div>
          <span class="badge" :class="step.status">{{ statusLabel(step.status) }}</span>
        </div>
        <div v-if="steps.length" class="progress">
          <div class="bar" :style="{ width: progress + '%' }"></div>
        </div>
      </aside>

      <div class="main">
        <section class="panel terminal">
          <div class="panel-title">终端输出</div>
          <div class="terminal-window">
            <div class="terminal-head">
              <span class="dot red"></span>
              <span class="dot yellow"></span>
              <span class="dot green"></span>
              <span class="title">
                {{ selectedStep ? selectedStep.name : "未选择步骤" }}
              </span>
            </div>
            <div class="terminal-body">
              <div v-if="!selectedStep" class="line muted">暂无步骤输出</div>
              <div v-if="!selectedStep && runMessage" class="line err">{{ runMessage }}</div>
              <template v-else>
                <div v-if="outputBlocks.length === 0" class="line muted">暂无输出</div>
                <div v-for="block in outputBlocks" :key="block.host" class="output-block">
                  <div class="line muted"># {{ block.host }}</div>
                  <div v-if="block.stdout" class="line">{{ block.stdout }}</div>
                  <div v-if="block.stderr" class="line err">{{ block.stderr }}</div>
                  <div v-if="block.extra" class="line">{{ block.extra }}</div>
                  <div v-if="block.message" class="line err">{{ block.message }}</div>
                </div>
              </template>
            </div>
          </div>
        </section>

        <section class="panel details">
          <div class="panel-title">步骤详情</div>
          <div v-if="!selectedStep" class="empty">暂无步骤</div>
          <div v-else class="detail-grid">
            <div>
              <div class="label">步骤信息</div>
              <div class="kv">
                <span>状态</span>
                <strong>{{ statusLabel(selectedStep.status) }}</strong>
              </div>
              <div class="kv">
                <span>开始</span>
                <strong>{{ formatTime(selectedStep.started_at) }}</strong>
              </div>
              <div class="kv">
                <span>结束</span>
                <strong>{{ formatTime(selectedStep.finished_at) }}</strong>
              </div>
            </div>
            <div>
              <div class="label">主机</div>
              <div
                v-for="host in filteredHosts"
                :key="host.host"
                class="host"
                :class="host.status"
              >
                <div>{{ host.host }}</div>
                <span :class="host.status">{{ statusLabel(host.status) }}</span>
              </div>
              <div v-if="filteredHosts.length === 0" class="empty">暂无主机</div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { apiBase, request } from "../lib/api";

type HostResult = {
  host: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  message?: string;
  output?: Record<string, unknown>;
};

type StepState = {
  name: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  hosts?: Record<string, HostResult>;
};

type RunState = {
  run_id: string;
  workflow_name: string;
  status: string;
  message?: string;
  started_at?: string;
  finished_at?: string;
  steps?: StepState[];
};

type RunResponse = {
  run: RunState;
  steps?: StepState[];
};

type EventPayload = {
  type: string;
  time: string;
  run_id: string;
  step?: string;
  host?: string;
  data?: Record<string, unknown>;
};

type OutputBlock = {
  host: string;
  stdout: string;
  stderr: string;
  extra: string;
  message: string;
};

const route = useRoute();
const runId = computed(() => String(route.params.id || ""));
const run = ref<RunState | null>(null);
const steps = ref<StepState[]>([]);
const selectedIndex = ref(0);
const hostFilter = ref("all");
const loading = ref(false);
const error = ref("");
const streamStatus = ref("");
let stream: EventSource | null = null;

const workflowName = computed(() => {
  if (run.value?.workflow_name) return run.value.workflow_name;
  if (typeof route.query.workflow === "string") return route.query.workflow;
  return "";
});

const failedStepName = computed(() => {
  const failed = steps.value.find((step) => step.status === "failed");
  return failed?.name || "";
});

const workflowLink = computed(() => {
  if (!workflowName.value) return "/workflows";
  const query = failedStepName.value
    ? `?step=${encodeURIComponent(failedStepName.value)}`
    : "";
  return `/workflows/${workflowName.value}${query}`;
});

const runStatus = computed(() => run.value?.status || "queued");
const runMessage = computed(() => run.value?.message || "");

const hosts = computed(() => {
  const set = new Set<string>();
  for (const step of steps.value) {
    const hostMap = step.hosts || {};
    Object.keys(hostMap).forEach((host) => set.add(host));
  }
  return Array.from(set);
});

const selectedStep = computed(() => steps.value[selectedIndex.value]);

const filteredHosts = computed(() => {
  const step = selectedStep.value;
  if (!step || !step.hosts) return [];
  const list = Object.values(step.hosts);
  if (hostFilter.value === "all") return list;
  return list.filter((host) => host.host === hostFilter.value);
});

const outputBlocks = computed<OutputBlock[]>(() => {
  return filteredHosts.value.map((host) => {
    const { stdout, stderr, extra } = formatOutput(host.output);
    return {
      host: host.host,
      stdout,
      stderr,
      extra,
      message: host.message || ""
    };
  });
});

const progress = computed(() => {
  if (!steps.value.length) return 0;
  const finished = steps.value.filter((step) =>
    ["success", "failed", "skipped", "stopped"].includes(step.status)
  ).length;
  return Math.round((finished / steps.value.length) * 100);
});

function selectStep(index: number) {
  selectedIndex.value = index;
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    success: "成功",
    running: "执行中",
    failed: "失败",
    queued: "排队中",
    pending: "待执行",
    stopped: "已停止",
    skipped: "已跳过"
  };
  return map[status] || status;
}

function formatTime(value?: string) {
  if (!value) return "-";
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return "-";
  return new Date(ts).toLocaleString();
}

function formatDuration(started?: string, finished?: string) {
  const start = started ? Date.parse(started) : NaN;
  const end = finished ? Date.parse(finished) : NaN;
  if (Number.isNaN(start) || Number.isNaN(end) || end <= start) return "-";
  const diff = Math.floor((end - start) / 1000);
  const mins = Math.floor(diff / 60);
  const secs = diff % 60;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}

function normalizeText(value: unknown) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value.trimEnd();
  return String(value);
}

function formatOutput(output?: Record<string, unknown>) {
  if (!output) return { stdout: "", stderr: "", extra: "" };
  const stdout = normalizeText(output.stdout);
  const stderr = normalizeText(output.stderr);
  const extraEntries = Object.entries(output).filter(
    ([key]) => key !== "stdout" && key !== "stderr"
  );
  const extra = extraEntries.length
    ? JSON.stringify(Object.fromEntries(extraEntries), null, 2)
    : "";
  return { stdout, stderr, extra };
}

function normalizeSteps(rawSteps: StepState[]) {
  return rawSteps.map((step) => ({
    ...step,
    hosts: step.hosts || {}
  }));
}

async function loadRun() {
  if (!runId.value) {
    error.value = "缺少运行 ID";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const data = await request<RunResponse>(`/runs/${runId.value}`);
    run.value = data.run;
    const nextSteps = data.run.steps && data.run.steps.length ? data.run.steps : data.steps || [];
    const runSteps = normalizeSteps(nextSteps);
    const workflowSteps = await loadWorkflowSteps(run.value?.workflow_name || workflowName.value);
    const merged = mergeSteps(workflowSteps, runSteps);
    steps.value = normalizeStepStatuses(merged, run.value?.status || "");
    selectDefaultStep(true);
  } catch (err) {
    error.value = "加载失败，请检查服务是否启动";
  } finally {
    loading.value = false;
  }
}

async function loadWorkflowSteps(name: string) {
  if (!name) return [] as StepState[];
  try {
    const data = await request<{ yaml: string }>(`/workflows/${name}`);
    const names = parseWorkflowSteps(data.yaml || "");
    return names.map((stepName) => ({
      name: stepName,
      status: "queued",
      hosts: {}
    }));
  } catch (err) {
    return [] as StepState[];
  }
}

function parseWorkflowSteps(content: string) {
  const lines = content.split(/\r?\n/);
  const steps: string[] = [];
  let inSteps = false;
  let stepsIndent = 0;

  for (const line of lines) {
    const stepsMatch = line.match(/^(\s*)steps\s*:\s*$/);
    if (stepsMatch) {
      inSteps = true;
      stepsIndent = stepsMatch[1].length;
      continue;
    }
    if (inSteps) {
      const indent = line.match(/^(\s*)/)?.[1].length ?? 0;
      if (indent <= stepsIndent && line.trim() !== "") {
        inSteps = false;
        continue;
      }
      const nameMatch = line.match(/^\s*-\s*name\s*:\s*(.+)$/);
      if (nameMatch) {
        steps.push(nameMatch[1].trim());
      }
    }
  }

  return steps;
}

function mergeSteps(workflowSteps: StepState[], runSteps: StepState[]) {
  if (!workflowSteps.length) return runSteps;

  const runMap = new Map(runSteps.map((step) => [step.name, step]));
  const workflowNames = new Set(workflowSteps.map((step) => step.name));
  const merged = workflowSteps.map((step) => {
    const runStep = runMap.get(step.name);
    if (!runStep) {
      return step;
    }
    return {
      ...step,
      ...runStep,
      hosts: runStep.hosts || {}
    };
  });

  for (const step of runSteps) {
    if (!workflowNames.has(step.name)) {
      merged.push(step);
    }
  }

  return merged;
}

function normalizeStepStatuses(list: StepState[], runStatus: string) {
  const normalized = list.map((step) => {
    if (step.status && step.status !== "queued" && step.status !== "pending") {
      return step;
    }
    const hostStates = step.hosts ? Object.values(step.hosts).map((host) => host.status) : [];
    const hasFailed = hostStates.includes("failed");
    const hasRunning = hostStates.includes("running");
    const hasSuccess = hostStates.includes("success");
    if (hasFailed) {
      return { ...step, status: "failed" };
    }
    if (hasRunning) {
      return { ...step, status: "running" };
    }
    if (hasSuccess) {
      return { ...step, status: "success" };
    }
    return step;
  });

  if (runStatus === "failed" && !normalized.some((step) => step.status === "failed")) {
    const candidate = normalized.find((step) =>
      ["running", "queued", "pending"].includes(step.status)
    );
    if (candidate) {
      candidate.status = "failed";
    }
  }

  return normalized;
}

function selectDefaultStep(force = false) {
  if (!steps.value.length) {
    selectedIndex.value = 0;
    return;
  }
  if (!force && steps.value[selectedIndex.value]) {
    return;
  }
  const runningIndex = steps.value.findIndex((step) => step.status === "running");
  if (runningIndex >= 0) {
    selectedIndex.value = runningIndex;
    return;
  }
  selectedIndex.value = Math.min(selectedIndex.value, steps.value.length - 1);
}

function ensureStep(name: string) {
  let step = steps.value.find((item) => item.name === name);
  if (!step) {
    step = { name, status: "queued", hosts: {} };
    steps.value.push(step);
  }
  if (!step.hosts) step.hosts = {};
  return step;
}

function updateHost(step: StepState, hostName: string, patch: Partial<HostResult>) {
  if (!step.hosts) step.hosts = {};
  const prev = step.hosts[hostName] || { host: hostName, status: "queued" };
  step.hosts[hostName] = { ...prev, ...patch, host: hostName };
}

function applyEvent(payload: EventPayload) {
  if (!payload || payload.run_id !== runId.value) return;

  if (!run.value) {
    run.value = {
      run_id: payload.run_id,
      workflow_name: workflowName.value,
      status: "running"
    } as RunState;
  }

  switch (payload.type) {
    case "workflow_start":
      run.value.status = String(payload.data?.status || "running");
      run.value.started_at = payload.time;
      break;
    case "workflow_end":
      run.value.status = String(payload.data?.status || "success");
      run.value.message = String(payload.data?.message || "");
      run.value.finished_at = payload.time;
      if (stream) {
        stream.close();
        stream = null;
      }
      break;
    case "step_start": {
      if (!payload.step) break;
      const step = ensureStep(payload.step);
      step.status = "running";
      step.started_at = payload.time;
      const targets = Array.isArray(payload.data?.targets) ? payload.data?.targets : [];
      for (const target of targets as Array<{ name?: string }>) {
        if (target && target.name) {
          updateHost(step, target.name, { status: "running", started_at: payload.time });
        }
      }
      break;
    }
    case "step_end":
    case "step_failed": {
      if (!payload.step) break;
      const step = ensureStep(payload.step);
      step.status = String(payload.data?.status || (payload.type === "step_failed" ? "failed" : "success"));
      step.finished_at = payload.time;
      break;
    }
    case "agent_output": {
      if (!payload.step || !payload.host) break;
      const step = ensureStep(payload.step);
      updateHost(step, payload.host, {
        status: String(payload.data?.status || "success"),
        output: payload.data?.output as Record<string, unknown>,
        message: payload.data?.error ? String(payload.data?.error) : "",
        finished_at: payload.time
      });
      break;
    }
    default:
      break;
  }
}

function startStream() {
  stopStream();
  if (!runId.value) return;
  streamStatus.value = "";
  const url = `${apiBase()}/runs/${runId.value}/stream`;
  stream = new EventSource(url);
  stream.onopen = () => {
    streamStatus.value = "";
  };
  stream.onerror = () => {
    streamStatus.value = "日志流连接中断，正在重试...";
  };

  const handler = (event: MessageEvent) => {
    try {
      const payload = JSON.parse(event.data) as EventPayload;
      applyEvent(payload);
    } catch (err) {
      // ignore malformed data
    }
  };

  [
    "workflow_start",
    "workflow_end",
    "step_start",
    "step_end",
    "step_failed",
    "agent_output"
  ].forEach((type) => {
    if (!stream) return;
    stream.addEventListener(type, handler as EventListener);
  });
}

function stopStream() {
  if (stream) {
    stream.close();
    stream = null;
  }
}

watch(
  () => steps.value.length,
  () => {
    if (!steps.value.length) {
      selectedIndex.value = 0;
      return;
    }
    if (selectedIndex.value >= steps.value.length) {
      selectedIndex.value = steps.value.length - 1;
    }
  }
);

watch(hosts, (next) => {
  if (hostFilter.value !== "all" && !next.includes(hostFilter.value)) {
    hostFilter.value = "all";
  }
});

async function reloadRun() {
  await loadRun();
  startStream();
}

watch(runId, () => {
  void reloadRun();
});

onMounted(() => {
  void reloadRun();
});

onBeforeUnmount(() => {
  stopStream();
});
</script>

<style scoped>
.run {
  display: flex;
  flex-direction: column;
  gap: 18px;
  flex: 1;
  min-height: 0;
}

.run-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.run-header h1 {
  font-family: "Space Grotesk", sans-serif;
  margin: 0 0 6px;
  font-size: 26px;
}

.run-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 12px;
  color: var(--muted);
  align-items: center;
}

.message {
  color: var(--muted);
}

.message.warn {
  color: var(--warn);
}

.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
}

.filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 12px;
  color: var(--muted);
}

.actions select {
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

.run-body {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 18px;
  flex: 1;
  min-height: 0;
  align-items: stretch;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step {
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 10px 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  background: #faf8f4;
}

.step-name {
  font-weight: 600;
}

.step-meta {
  font-size: 12px;
  color: var(--muted);
}

.badge {
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  text-transform: uppercase;
}

.badge.success {
  color: var(--ok);
  border-color: rgba(42, 157, 75, 0.3);
}

.badge.running {
  color: var(--info);
  border-color: rgba(46, 111, 227, 0.3);
}

.badge.failed {
  color: var(--err);
  border-color: rgba(208, 52, 44, 0.3);
}

.badge.stopped {
  color: var(--warn);
  border-color: rgba(230, 167, 0, 0.3);
}

.badge.queued,
.badge.pending {
  color: var(--muted);
  border-color: rgba(111, 111, 111, 0.3);
}

.progress {
  height: 6px;
  border-radius: 999px;
  background: #efeae2;
  overflow: hidden;
  margin-top: auto;
}

.progress .bar {
  height: 100%;
  background: linear-gradient(90deg, var(--brand), #f2a353);
}

.main {
  display: grid;
  grid-template-rows: minmax(0, 2fr) minmax(0, 1fr);
  gap: 18px;
  min-height: 0;
}

.terminal {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.terminal-window {
  background: #121212;
  border-radius: var(--radius-md);
  color: #f5f2ec;
  overflow: hidden;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  flex: 1;
}

.terminal-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: #1d1d1d;
}

.terminal-head .title {
  margin-left: 8px;
  font-size: 12px;
  color: #cfcac2;
}

.terminal-head .dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.terminal-head .dot.red {
  background: #e0504d;
}

.terminal-head .dot.yellow {
  background: #f3b74f;
}

.terminal-head .dot.green {
  background: #60c16d;
}

.terminal-body {
  font-family: "JetBrains Mono", monospace;
  font-size: 12px;
  padding: 16px;
  line-height: 1.5;
  flex: 1;
  overflow: auto;
}

.output-block {
  margin-bottom: 12px;
}

.line {
  white-space: pre-wrap;
}

.line.err {
  color: #f26a5d;
}

.line.muted {
  color: #c8c2b8;
}

.details .detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.label {
  font-size: 12px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.12em;
  margin-bottom: 8px;
}

.kv {
  display: flex;
  justify-content: space-between;
  border-bottom: 1px dashed var(--grid);
  padding: 6px 0;
  font-size: 13px;
}

.host {
  display: flex;
  justify-content: space-between;
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 6px 10px;
  margin-bottom: 8px;
  font-size: 13px;
}

.host.success {
  color: var(--ok);
}

.host.failed {
  color: var(--err);
}

.host.running {
  color: var(--info);
}

.mono {
  font-family: "JetBrains Mono", monospace;
}

.empty {
  font-size: 12px;
  color: var(--muted);
  padding: 8px 4px;
}

.hint {
  margin-top: 6px;
  color: var(--err);
  font-size: 12px;
}

@media (max-width: 1100px) {
  .run-body {
    grid-template-columns: 1fr;
  }

  .main {
    grid-template-rows: auto;
  }
}
</style>
