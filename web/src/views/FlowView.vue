<template>
  <section class="flow">
    <div class="flow-header">
      <div>
        <h1>流程视图</h1>
        <p>工作流：{{ workflowName }} · 可视化编辑节点配置并同步回 YAML。</p>
      </div>
      <div class="actions">
        <RouterLink class="btn" :to="`/workflows/${workflowName}`">返回 YAML</RouterLink>
        <button class="btn" type="button" :disabled="runBusy" @click="planRun">计划</button>
        <button class="btn primary" type="button" :disabled="runBusy" @click="applyRun">
          执行
        </button>
        <button class="btn ghost" type="button" :disabled="runBusy" @click="stopRun">
          停止
        </button>
        <div class="sync">
          <span class="dot" :class="isSynced ? 'ok' : 'warn'"></span>
          {{ isSynced ? "已同步" : "同步中..." }}
        </div>
        <div v-if="runMessage" class="run-message">{{ runMessage }}</div>
      </div>
    </div>

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
            :class="{ active: index === selectedIndex }"
            @click="selectNode(index)"
          >
            <div class="node-title">{{ node.name }}</div>
            <div class="node-meta">{{ node.action || "未指定动作" }}</div>
            <div class="node-targets" v-if="node.targets.length">
              目标: {{ node.targets.join(", ") }}
            </div>
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
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ApiError, request } from "../lib/api";

type Param = { key: string; value: string };

type FlowNode = {
  id: string;
  name: string;
  action: string;
  targets: string[];
  params: Param[];
};

const route = useRoute();
const router = useRouter();
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
const currentRunId = ref("");
let skipNextSync = false;

const activeNode = computed(() => nodes[selectedIndex.value]);
const targetsInput = ref(activeNode.value ? activeNode.value.targets.join(", ") : "");

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

async function planRun() {
  runBusy.value = true;
  runMessage.value = "生成计划中...";
  try {
    await request(`/workflows/${workflowName.value}/plan`, { method: "POST" });
    runMessage.value = "计划生成成功";
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `计划失败: ${apiErr.message}` : "计划失败，请检查服务是否启动";
  } finally {
    runBusy.value = false;
  }
}

async function applyRun() {
  runBusy.value = true;
  runMessage.value = "正在执行...";
  try {
    const data = await request<{ run_id: string }>(`/workflows/${workflowName.value}/apply`, {
      method: "POST"
    });
    currentRunId.value = data.run_id;
    runMessage.value = "执行已开始，正在跳转运行控制台";
    await router.push({
      name: "run-console",
      params: { id: data.run_id },
      query: { workflow: workflowName.value }
    });
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `执行失败: ${apiErr.message}` : "执行失败，请检查服务是否启动";
  } finally {
    runBusy.value = false;
  }
}

async function stopRun() {
  if (!currentRunId.value) {
    runMessage.value = "暂无运行中的任务";
    return;
  }
  runBusy.value = true;
  runMessage.value = "正在停止...";
  try {
    await request(`/runs/${currentRunId.value}/stop`, { method: "POST" });
    runMessage.value = "已发送停止指令";
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `停止失败: ${apiErr.message}` : "停止失败，请检查服务是否启动";
  } finally {
    runBusy.value = false;
  }
}

function parseSteps(content: string): FlowNode[] {
  const lines = content.split(/\r?\n/);
  const steps: FlowNode[] = [];
  let current: FlowNode | null = null;
  let inWith = false;
  let withIndent = 0;

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
      const indent = line.match(/^(\s*)/)[1].length;
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
      const indent = line.match(/^(\s*)/)[1].length;
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
  void loadYaml();
});

onMounted(() => {
  void loadYaml();
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

@media (max-width: 1100px) {
  .flow-body {
    grid-template-columns: 1fr;
  }
}
</style>
