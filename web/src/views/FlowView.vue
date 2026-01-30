<template>
  <section class="flow">
    <div class="flow-header">
      <div>
        <h1>流程视图</h1>
        <p>工作流：{{ workflowName }} · 可视化编辑节点配置并同步回 YAML。</p>
      </div>
      <div class="actions">
        <RouterLink class="btn" :to="`/workflows/${workflowName}`">返回编排</RouterLink>
        <button class="btn primary" type="button" :disabled="runBusy || nodes.length === 0" @click="startRun">
          {{ runBusy ? "执行中..." : "执行" }}
        </button>
        <div class="sync">
          <span class="dot" :class="isSynced ? 'ok' : 'warn'"></span>
          {{ isSynced ? "已同步" : "同步中..." }}
        </div>
        <div v-if="runStatus" class="run-status" :class="runStatus">
          {{ statusLabel(runStatus) }}
        </div>
      </div>
    </div>
    <div v-if="runMessage" class="run-message">{{ runMessage }}</div>
    <div v-if="streamStatus" class="run-message warn">{{ streamStatus }}</div>

    <div class="flow-body">
      <div class="panel canvas">
        <div class="panel-title">
          <div>流程画布</div>
          <button class="ghost" type="button" @click="centerFlow">居中</button>
        </div>
        <div class="canvas-area">
          <div
            class="flow-node"
            v-for="(node, index) in nodes"
            :key="node.id"
            :class="[
              { active: index === selectedIndex },
              nodeStatus(node)
            ]"
            @click="selectNode(index)"
          >
            <div class="node-title">{{ node.name }}</div>
            <div class="node-meta">{{ node.action || "未指定动作" }}</div>
            <div class="node-targets" v-if="node.targets.length">
              目标: {{ node.targets.join(", ") }}
            </div>
            <span v-if="nodeStatus(node)" class="node-status" :class="nodeStatus(node)">
              {{ statusLabel(nodeStatus(node)) }}
            </span>
          </div>
          <div v-if="nodes.length === 0" class="empty">
            暂无节点
          </div>
        </div>
      </div>

      <aside class="panel config">
        <div class="panel-title">节点配置</div>
        <div v-if="activeNode" class="config-body">
          <label class="field">
            <span>名称</span>
            <input v-model="activeNode.name" type="text" />
          </label>
          <label class="field">
            <span>动作</span>
            <select v-model="activeNode.action">
              <option value="cmd.run">cmd.run</option>
              <option value="env.set">env.set</option>
              <option value="pkg.install">pkg.install</option>
              <option value="script.shell">script.shell</option>
              <option value="script.python">script.python</option>
              <option value="template.render">template.render</option>
              <option value="service.ensure">service.ensure</option>
              <option value="service.restart">service.restart</option>
            </select>
          </label>
          <label class="field">
            <span>目标主机</span>
            <input
              v-model="targetsInput"
              type="text"
              placeholder="web1, web2"
              @blur="syncTargets"
            />
          </label>

          <div class="field-group">
            <div class="field-title">参数</div>
            <div v-for="(item, idx) in activeNode.params" :key="item.key" class="param-row">
              <input v-model="item.key" type="text" placeholder="key" />
              <input v-model="item.value" type="text" placeholder="value" />
              <button class="ghost" type="button" @click="removeParam(idx)">-</button>
            </div>
            <button class="btn" type="button" @click="addParam">
              添加参数
            </button>
          </div>

          <div class="field-group">
            <div class="field-title">变量绑定</div>
            <div class="bind-row">
              <select v-model="selectedVar">
                <option value="">选择变量</option>
                <option v-for="item in vars" :key="item" :value="item">
                  {{ item }}
                </option>
              </select>
              <button class="btn" type="button" @click="insertVar">
                插入
              </button>
            </div>
            <div class="hint">选择变量插入到最后一个参数值中。</div>
          </div>
        </div>
        <div v-else class="empty">请选择一个节点进行编辑</div>
      </aside>
    </div>

    <div v-if="showOutputModal" class="modal-backdrop" @click.self="closeOutputModal">
      <div class="output-modal">
        <div class="modal-head">
          <div>
            <div class="modal-title">{{ outputStepName || "步骤终端" }}</div>
            <div class="modal-sub">
              {{ outputStep ? statusLabel(outputStep.status) : "暂无执行状态" }}
            </div>
          </div>
          <button class="icon-btn" type="button" @click="closeOutputModal">×</button>
        </div>
        <div class="terminal-window">
          <div class="terminal-head">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
            <span class="title">{{ outputStepName || "未选择步骤" }}</span>
          </div>
          <div class="terminal-body">
            <div v-if="!outputStep" class="line muted">暂无步骤输出</div>
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
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { ApiError, apiBase, request } from "../lib/api";

type Param = { key: string; value: string };

type FlowNode = {
  id: string;
  name: string;
  action: string;
  targets: string[];
  params: Param[];
};

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
const workflowName = computed(() => String(route.params.name || "workflow"));

const yaml = ref(defaultYaml(workflowName.value));
const vars = ref(parseVars(yaml.value));
const nodes = reactive<FlowNode[]>(parseSteps(yaml.value));
const selectedIndex = ref(0);
const selectedVar = ref("");
const isSynced = ref(true);
const loading = ref(false);
const saving = ref(false);
const runMessage = ref("");
const runBusy = ref(false);
const runId = ref("");
const runStatus = ref("");
const streamStatus = ref("");
const runSteps = ref<StepState[]>([]);
const outputStepName = ref("");
const showOutputModal = ref(false);
let skipNextSync = false;
let stream: EventSource | null = null;

const activeNode = computed(() => nodes[selectedIndex.value]);
const targetsInput = ref(activeNode.value ? activeNode.value.targets.join(", ") : "");
const outputStep = computed(() => runSteps.value.find((step) => step.name === outputStepName.value));
const outputBlocks = computed<OutputBlock[]>(() => {
  if (!outputStep.value || !outputStep.value.hosts) return [];
  return Object.values(outputStep.value.hosts).map((host) => {
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

watch(
  () => nodes,
  () => {
    if (skipNextSync) {
      skipNextSync = false;
      return;
    }
    void syncYaml();
  },
  { deep: true }
);

watch(
  () => activeNode.value,
  (node) => {
    if (node) {
      targetsInput.value = node.targets.join(", ");
    }
  }
);

function selectNode(index: number) {
  selectedIndex.value = index;
  const node = nodes[index];
  if (node) {
    targetsInput.value = node.targets.join(", ");
    if (runId.value) {
      openOutputModal(node.name);
    }
  }
}

function addParam() {
  if (!activeNode.value) return;
  activeNode.value.params.push({ key: "", value: "" });
}

function removeParam(index: number) {
  if (!activeNode.value) return;
  activeNode.value.params.splice(index, 1);
}

function insertVar() {
  if (!activeNode.value || !selectedVar.value) return;
  if (!activeNode.value.params.length) {
    activeNode.value.params.push({ key: "", value: "" });
  }
  const last = activeNode.value.params[activeNode.value.params.length - 1];
  const token = "${" + selectedVar.value + "}";
  last.value = `${last.value || ""}${last.value ? " " : ""}${token}`;
}

function syncTargets() {
  if (!activeNode.value) return;
  const items = targetsInput.value
    .split(/,\s*/)
    .map((item) => item.trim())
    .filter(Boolean);
  activeNode.value.targets = items;
}

async function syncYaml() {
  isSynced.value = false;
  const serialized = replaceStepsBlock(yaml.value || defaultYaml(workflowName.value), nodes);
  yaml.value = serialized;
  vars.value = parseVars(serialized);
  await saveYaml(serialized);
  isSynced.value = true;
}

function centerFlow() {
  // Placeholder for future zoom/pan control.
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

function nodeStatus(node: FlowNode) {
  if (!runId.value) return "";
  const step = runSteps.value.find((item) => item.name === node.name);
  return step?.status || "queued";
}

function openOutputModal(stepName: string) {
  outputStepName.value = stepName;
  showOutputModal.value = true;
}

function closeOutputModal() {
  showOutputModal.value = false;
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

function ensureStep(name: string) {
  let step = runSteps.value.find((item) => item.name === name);
  if (!step) {
    step = { name, status: "queued", hosts: {} };
    runSteps.value.push(step);
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

  switch (payload.type) {
    case "workflow_start":
      runStatus.value = String(payload.data?.status || "running");
      break;
    case "workflow_end":
      runStatus.value = String(payload.data?.status || "success");
      runMessage.value = String(payload.data?.message || "");
      stopStream();
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
    } catch {
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

async function loadRun() {
  if (!runId.value) return;
  try {
    const data = await request<RunResponse>(`/runs/${runId.value}`);
    runStatus.value = data.run.status || runStatus.value;
    runMessage.value = data.run.message || runMessage.value;
    const nextSteps =
      data.run.steps && data.run.steps.length ? data.run.steps : data.steps || [];
    runSteps.value = normalizeSteps(nextSteps);
  } catch (err) {
    runMessage.value = "运行加载失败，请检查服务是否启动";
  }
}

async function startRun() {
  if (runBusy.value) return;
  runBusy.value = true;
  runMessage.value = "执行中...";
  try {
    const data = await request<{ run_id: string }>(`/workflows/${workflowName.value}/apply`, {
      method: "POST"
    });
    if (data.run_id) {
      runId.value = data.run_id;
      runStatus.value = "running";
      runMessage.value = "执行已启动";
      await loadRun();
      startStream();
    }
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `执行失败: ${apiErr.message}` : "执行失败";
  } finally {
    runBusy.value = false;
  }
}

async function loadYaml() {
  loading.value = true;
  try {
    const data = await request<{ yaml: string }>(`/workflows/${workflowName.value}`);
    applyYaml(data.yaml || defaultYaml(workflowName.value));
  } catch (err) {
    if ((err as ApiError).status === 404) {
      const fallback = defaultYaml(workflowName.value);
      applyYaml(fallback);
      try {
        await saveYaml(fallback);
        runMessage.value = "已创建默认工作流";
      } catch (saveErr) {
        runMessage.value = "创建默认工作流失败，请检查服务是否启动";
      }
    } else {
      runMessage.value = "加载失败，请检查服务是否启动";
    }
  } finally {
    loading.value = false;
  }
}

async function saveYaml(content: string) {
  if (saving.value) return;
  saving.value = true;
  try {
    await request(`/workflows/${workflowName.value}`, {
      method: "PUT",
      body: { yaml: content }
    });
  } catch (err) {
    runMessage.value = "保存失败，请检查服务是否启动";
  } finally {
    saving.value = false;
  }
}

function applyYaml(content: string) {
  skipNextSync = true;
  yaml.value = content;
  vars.value = parseVars(content);
  const next = parseSteps(content);
  nodes.splice(0, nodes.length, ...next);
  selectedIndex.value = 0;
  selectedVar.value = "";
  targetsInput.value = next[0] ? next[0].targets.join(", ") : "";
}

function parseSteps(content: string): FlowNode[] {
  const lines = content.split(/\r?\n/);
  const steps: FlowNode[] = [];
  let current: FlowNode | null = null;
  let inWith = false;
  let withIndent = 0;
  const getIndent = (line: string) => {
    const match = line.match(/^(\s*)/);
    return match ? match[1].length : 0;
  };

  for (const line of lines) {
    const nameMatch = line.match(/^\s*-\s*name\s*:\s*(.+)$/);
    if (nameMatch) {
      current = {
        id: `node-${steps.length}`,
        name: nameMatch[1].trim(),
        action: "",
        targets: [],
        params: []
      };
      steps.push(current);
      inWith = false;
      continue;
    }

    if (!current) {
      continue;
    }

    const actionMatch = line.match(/^\s*action\s*:\s*(.+)$/);
    if (actionMatch) {
      current.action = actionMatch[1].trim();
      continue;
    }

    const targetsMatch = line.match(/^\s*targets\s*:\s*(.+)$/);
    if (targetsMatch) {
      current.targets = parseTargets(targetsMatch[1].trim());
      continue;
    }

    const withMatch = line.match(/^(\s*)with\s*:\s*$/);
    if (withMatch) {
      inWith = true;
      withIndent = withMatch[1].length;
      continue;
    }

    if (inWith) {
      const indent = getIndent(line);
      if (indent <= withIndent) {
        inWith = false;
        continue;
      }
      const paramMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:\s*(.+)$/);
      if (paramMatch) {
        current.params.push({
          key: paramMatch[1],
          value: paramMatch[2].trim()
        });
      }
    }
  }

  return steps;
}

function parseTargets(raw: string) {
  const cleaned = raw.replace(/[\[\]]/g, "");
  return cleaned
    .split(/,\s*/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseVars(content: string) {
  const lines = content.split(/\r?\n/);
  const vars: string[] = [];
  let inVars = false;
  let varsIndent = 0;

  for (const line of lines) {
    const varsMatch = line.match(/^(\s*)vars\s*:\s*$/);
    if (varsMatch) {
      inVars = true;
      varsIndent = varsMatch[1].length;
      continue;
    }
    if (inVars) {
      const indent = line.match(/^(\s*)/)?.[1].length ?? 0;
      if (indent <= varsIndent) {
        inVars = false;
        continue;
      }
      const varMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:/);
      if (varMatch) {
        vars.push(varMatch[1]);
      }
    }
  }

  return Array.from(new Set(vars));
}

function serializeSteps(steps: FlowNode[]) {
  const lines: string[] = ["steps:"];
  for (const step of steps) {
    lines.push(`  - name: ${step.name || "step"}`);
    if (step.targets.length) {
      lines.push(`    targets: [${step.targets.join(", ")}]`);
    }
    if (step.action) {
      lines.push(`    action: ${step.action}`);
    }
    if (step.params.length) {
      lines.push("    with:");
      for (const param of step.params) {
        if (!param.key) continue;
        lines.push(`      ${param.key}: ${param.value || ""}`);
      }
    }
    lines.push("");
  }
  return lines.join("\n").trimEnd();
}

function replaceStepsBlock(content: string, steps: FlowNode[]) {
  const lines = content.split(/\r?\n/);
  const stepsIndex = lines.findIndex((line) => /^steps\s*:\s*$/.test(line));
  const stepsBlock = serializeSteps(steps).split("\n");

  if (stepsIndex === -1) {
    return `${content.trim()}\n\n${stepsBlock.join("\n")}`;
  }

  let endIndex = stepsIndex + 1;
  while (endIndex < lines.length) {
    const line = lines[endIndex];
    if (line.trim() === "") {
      endIndex += 1;
      continue;
    }
    if (/^[a-zA-Z0-9_-]+\s*:/i.test(line) && !/^\s/.test(line)) {
      break;
    }
    endIndex += 1;
  }

  const next = [...lines.slice(0, stepsIndex), ...stepsBlock, ...lines.slice(endIndex)];
  return next.join("\n");
}

watch(workflowName, () => {
  stopStream();
  runId.value = "";
  runStatus.value = "";
  runSteps.value = [];
  runMessage.value = "";
  streamStatus.value = "";
  void loadYaml();
});

onMounted(() => {
  void loadYaml();
});

watch(runId, () => {
  if (runId.value) {
    void loadRun();
    startStream();
  }
});

onBeforeUnmount(() => {
  stopStream();
});

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
</script>

<style scoped>
.flow {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-bottom: 24px;
}

.flow-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.flow-header h1 {
  font-family: "Space Grotesk", sans-serif;
  margin: 0 0 6px;
  font-size: 26px;
}

.flow-header p {
  margin: 0;
  color: var(--muted);
}

.actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 14px;
  cursor: pointer;
  font-size: 12px;
  border-radius: var(--radius-sm);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.ghost {
  border: 1px solid var(--grid);
  background: #fff;
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.sync {
  font-size: 12px;
  color: var(--muted);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.run-message {
  font-size: 12px;
  color: var(--muted);
}

.run-message.warn {
  color: var(--warn);
}

.run-status {
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 12px;
  border: 1px solid var(--grid);
  color: var(--muted);
}

.run-status.running {
  border-color: rgba(232, 93, 42, 0.4);
  color: var(--brand);
}

.run-status.success {
  border-color: rgba(42, 157, 75, 0.4);
  color: var(--ok);
}

.run-status.failed {
  border-color: rgba(226, 85, 85, 0.45);
  color: #e25555;
}

.sync .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--warn);
}

.sync .dot.ok {
  background: var(--ok);
}

.flow-body {
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  gap: 18px;
}

.canvas {
  min-height: 520px;
  display: flex;
  flex-direction: column;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  margin-bottom: 12px;
}

.canvas-area {
  position: relative;
  padding: 24px 24px 24px 64px;
  border-radius: var(--radius-md);
  border: 1px dashed var(--grid);
  background: linear-gradient(180deg, #fbfaf7 0%, #f5f1eb 100%);
  flex: 1;
  overflow: auto;
}

.canvas-area::before {
  content: "";
  position: absolute;
  left: 36px;
  top: 20px;
  bottom: 20px;
  width: 2px;
  background: linear-gradient(180deg, rgba(225, 221, 214, 0.4), rgba(225, 221, 214, 1));
}

.flow-node {
  position: relative;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  background: #ffffff;
  padding: 12px 16px 12px 18px;
  margin-bottom: 18px;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}

.flow-node::before {
  content: "";
  position: absolute;
  left: -34px;
  top: 18px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 0 0 6px rgba(232, 93, 42, 0.16);
}

.flow-node::after {
  content: "";
  position: absolute;
  right: 12px;
  top: 18px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1px solid var(--grid);
  background: #fff;
}

.flow-node.active {
  border-color: rgba(232, 93, 42, 0.5);
  box-shadow: 0 0 0 2px rgba(232, 93, 42, 0.18);
  transform: translateX(4px);
}

.flow-node.running {
  border-color: rgba(232, 93, 42, 0.5);
  box-shadow: 0 0 0 2px rgba(232, 93, 42, 0.2);
}

.flow-node.running::before {
  background: var(--brand);
  box-shadow: 0 0 0 6px rgba(232, 93, 42, 0.18);
}

.flow-node.success {
  border-color: rgba(42, 157, 75, 0.45);
  box-shadow: 0 0 0 2px rgba(42, 157, 75, 0.16);
}

.flow-node.success::before {
  background: var(--ok);
  box-shadow: 0 0 0 6px rgba(42, 157, 75, 0.18);
}

.flow-node.failed {
  border-color: rgba(226, 85, 85, 0.45);
  box-shadow: 0 0 0 2px rgba(226, 85, 85, 0.16);
}

.flow-node.failed::before {
  background: #e25555;
  box-shadow: 0 0 0 6px rgba(226, 85, 85, 0.18);
}

.flow-node.queued {
  border-style: dashed;
}

.node-status {
  position: absolute;
  right: 12px;
  bottom: 10px;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  color: var(--muted);
  background: #fff;
}

.node-status.running {
  border-color: rgba(232, 93, 42, 0.4);
  color: var(--brand);
}

.node-status.success {
  border-color: rgba(42, 157, 75, 0.4);
  color: var(--ok);
}

.node-status.failed {
  border-color: rgba(226, 85, 85, 0.4);
  color: #e25555;
}

.node-title {
  font-weight: 600;
}

.node-meta,
.node-targets {
  font-size: 12px;
  color: var(--muted);
}

.config {
  min-height: 520px;
  display: flex;
  flex-direction: column;
}

.config-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field input,
.field select {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 13px;
  color: var(--ink);
}

.field-group {
  border-top: 1px solid var(--grid);
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field-title {
  font-size: 12px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.param-row {
  display: grid;
  grid-template-columns: 1fr 1.2fr 32px;
  gap: 8px;
}

.param-row input {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  font-size: 12px;
}

.bind-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
}

.bind-row select {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 8px 10px;
}

.hint {
  font-size: 11px;
  color: var(--muted);
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(14, 10, 6, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 40;
  padding: 24px;
}

.output-modal {
  background: #fff;
  border-radius: 18px;
  width: min(780px, 92vw);
  max-height: 86vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 24px 60px rgba(15, 8, 4, 0.25);
  border: 1px solid rgba(27, 27, 27, 0.1);
}

.modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid var(--grid);
}

.modal-title {
  font-weight: 600;
}

.modal-sub {
  font-size: 12px;
  color: var(--muted);
  margin-top: 4px;
}

.icon-btn {
  border: none;
  background: transparent;
  font-size: 18px;
  cursor: pointer;
  color: var(--muted);
}

.terminal-window {
  display: flex;
  flex-direction: column;
  background: #101010;
  color: #f0f0f0;
  margin: 16px;
  border-radius: 12px;
  overflow: hidden;
}

.terminal-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: #1a1a1a;
  font-size: 12px;
}

.terminal-head .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.terminal-head .dot.red {
  background: #ff5f57;
}

.terminal-head .dot.yellow {
  background: #febc2e;
}

.terminal-head .dot.green {
  background: #28c840;
}

.terminal-head .title {
  margin-left: 8px;
  color: #c9c9c9;
}

.terminal-body {
  padding: 12px 14px;
  font-family: "JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
    "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  overflow: auto;
  max-height: 60vh;
  white-space: pre-wrap;
}

.terminal-body .line {
  margin-bottom: 6px;
}

.terminal-body .line.muted {
  color: rgba(240, 240, 240, 0.5);
}

.terminal-body .line.err {
  color: #ff9087;
}

.output-block {
  margin-bottom: 12px;
}

@media (max-width: 1100px) {
  .flow-body {
    grid-template-columns: 1fr;
  }
}
</style>
