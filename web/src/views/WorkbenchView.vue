<template>
  <section class="workbench">
    <teleport to="#topbar-extra">
      <div class="workbench-topbar">
        <span class="status-chip">运行进度 0.00%</span>
        <button class="btn btn-sm ghost" type="button" @click="autoLayout">自动布局</button>
        <button class="btn btn-sm" type="button" @click="handleAutoFix">校验</button>
        <button class="btn btn-sm" type="button" @click="handleRun">运行</button>
        <button class="btn btn-sm ghost">保存</button>
      </div>
    </teleport>

    <div class="workbench-body" :class="{ 'detail-open': !!selectedNode }">
      <aside class="library-pane">
        <NodeLibraryPanel />
      </aside>

      <main
        ref="canvasRef"
        class="canvas"
        @dragover.prevent
        @drop="handleDrop"
      >
        <div class="canvas-grid"></div>
        <svg class="edge-layer">
          <path
            v-for="edge in edgePaths"
            :key="edge.id"
            class="edge-path"
            :d="edge.path"
            @click.stop="removeEdge(edge.id)"
          />
          <path v-if="linkingPath" class="edge-path preview" :d="linkingPath" />
        </svg>
        <div class="canvas-hint">
          <span>拖拽左侧模板到画布</span>
          <span>选择节点后，按住 Shift 点击另一个节点建立连接</span>
        </div>
        <div
          v-for="node in nodes"
          :key="node.id"
          class="canvas-node"
          :class="{ dragging: dragging?.id === node.id }"
          :style="{ left: `${node.x}px`, top: `${node.y}px` }"
          @click="selectNode(node, $event)"
          @mousedown="startDrag(node, $event)"
        >
          <div class="node-handles">
            <button
              class="node-handle handle-in"
              type="button"
              @mouseenter="setLinkTarget(node.id)"
              @mouseleave="clearLinkTarget(node.id)"
              @mouseup.stop="finishLink(node.id)"
            ></button>
            <button
              class="node-handle handle-out"
              type="button"
              @mousedown.stop="startLink(node, $event)"
            ></button>
          </div>
          <div class="node-title">{{ node.name }}</div>
          <div class="node-action">{{ (node.data && node.data["action"]) || node.type }}</div>
          <button class="node-remove" type="button" @click.stop="removeNode(node.id)">×</button>
        </div>
      </main>

      <aside class="detail-pane" v-if="selectedNode">
        <div class="detail-head">
          <h3>节点详情</h3>
          <button class="btn ghost btn-sm" type="button" @click="selectedNodeId = null">关闭</button>
        </div>
        <div class="detail-body">
          <label class="field">
            <span>名称</span>
            <input v-model="selectedNode.name" type="text" />
          </label>
          <label class="field">
            <span>类型</span>
            <input v-model="selectedNode.type" type="text" disabled />
          </label>
          <label class="field">
            <span>配置 JSON</span>
            <textarea v-model="dataInput" rows="6" placeholder='{ "prompt": "Hello {{#start.query#}}" }'></textarea>
          </label>
          <div class="detail-actions">
            <button class="btn btn-sm" type="button" @click="applyDetail">应用</button>
            <button class="btn btn-sm ghost" type="button" @click="resetDetail">重置</button>
          </div>
        </div>
      </aside>
    </div>
    <ChatDrawer
      :selected-node="selectedNode"
      :status="chatStatus"
      :error="chatError"
      :busy="chatBusy"
      :run-status="runStatus"
      :run-summary="runSummary"
      :run-logs="runLogs"
      @generate="handleGenerate"
      @fix="handleFix"
      @regenerate="handleRegenerate"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import NodeLibraryPanel, { type TemplateSummary } from "../components/NodeLibraryPanel.vue";
import ChatDrawer from "../components/ChatDrawer.vue";
import { apiBase, request } from "../lib/api";

type RunSummary = {
  status: string;
  totalSteps: number;
  successSteps: number;
  failedSteps: number;
  durationMs: number;
  issues: string[];
  message?: string;
};

const canvasRef = ref<HTMLElement | null>(null);
const nodes = ref<
  Array<{
    id: string;
    type: string;
    name: string;
    data?: Record<string, unknown>;
    x: number;
    y: number;
  }>
>([]);
const edges = ref<Array<{ id: string; source: string; target: string }>>([]);
const linking = ref<{
  sourceId: string;
  start: { x: number; y: number };
  current: { x: number; y: number };
} | null>(null);
const dragging = ref<{
  id: string;
  offsetX: number;
  offsetY: number;
} | null>(null);
const linkTargetId = ref<string | null>(null);
let nodeIndex = 0;
const selectedNodeId = ref<string | null>(null);
const dataInput = ref("");
const chatStatus = ref("");
const chatError = ref("");
const chatBusy = ref(false);
const draftId = ref("");
const yamlText = ref("");
const autoFixRunning = ref(false);
const runId = ref("");
const runLogs = ref<string[]>([]);
const runStatus = ref("");
const runSummary = ref<RunSummary | null>(null);
let draftSaveTimer: number | null = null;
let lastSavedYaml = "";
let lastSavedGraph = "";

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null;
  return nodes.value.find((node) => node.id === selectedNodeId.value) || null;
});

function handleDrop(event: DragEvent) {
  if (!event.dataTransfer || !canvasRef.value) return;
  const raw = event.dataTransfer.getData("application/json");
  if (!raw) return;
  let template: TemplateSummary | null = null;
  try {
    template = JSON.parse(raw) as TemplateSummary;
  } catch {
    template = null;
  }
  if (!template) return;

  const rect = canvasRef.value.getBoundingClientRect();
  const x = event.clientX - rect.left;
  const y = event.clientY - rect.top;

  const id = `node-${Date.now()}-${nodeIndex++}`;
  const nodeSpec = template.node || {};
  const newNode = {
    id,
    type: nodeSpec.type || "custom",
    name: nodeSpec.name || template.name || "未命名节点",
    data: nodeSpec.data ? { ...nodeSpec.data } : {},
    x: Math.max(20, x - 60),
    y: Math.max(20, y - 20)
  };
  const previous = nodes.value.length ? nodes.value[nodes.value.length - 1] : null;
  nodes.value.push(newNode);
  if (previous) {
    addEdge(previous.id, newNode.id);
  }
  syncYamlFromNodes();
  scheduleDraftSave("drop");
}

function selectNode(node: { id: string }, event?: MouseEvent) {
  if (event?.shiftKey && selectedNodeId.value && selectedNodeId.value !== node.id) {
    addEdge(selectedNodeId.value, node.id);
  }
  selectedNodeId.value = node.id;
}

function removeNode(id: string) {
  nodes.value = nodes.value.filter((node) => node.id !== id);
  edges.value = edges.value.filter((edge) => edge.source !== id && edge.target !== id);
  if (selectedNodeId.value === id) {
    selectedNodeId.value = null;
  }
  syncYamlFromNodes();
  scheduleDraftSave("remove");
}

function addEdge(source: string, target: string) {
  if (source === target) return;
  const exists = edges.value.some((edge) => edge.source === source && edge.target === target);
  if (exists) return;
  edges.value.push({
    id: `edge-${Date.now()}-${Math.floor(Math.random() * 10000)}`,
    source,
    target
  });
  scheduleDraftSave("edge");
}

function removeEdge(id: string) {
  edges.value = edges.value.filter((edge) => edge.id !== id);
  scheduleDraftSave("edge-remove");
}

function startLink(node: { id: string }, event: MouseEvent) {
  if (!canvasRef.value) return;
  const handleRect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const canvasRect = canvasRef.value.getBoundingClientRect();
  const start = {
    x: handleRect.left + handleRect.width / 2 - canvasRect.left,
    y: handleRect.top + handleRect.height / 2 - canvasRect.top
  };
  linking.value = {
    sourceId: node.id,
    start,
    current: start
  };
  linkTargetId.value = null;
}

function finishLink(targetId: string) {
  if (!linking.value) return;
  if (linking.value.sourceId !== targetId) {
    addEdge(linking.value.sourceId, targetId);
  }
  linking.value = null;
  linkTargetId.value = null;
}

function setLinkTarget(id: string) {
  if (!linking.value) return;
  linkTargetId.value = id;
}

function clearLinkTarget(id: string) {
  if (linkTargetId.value === id) {
    linkTargetId.value = null;
  }
}

function updateLinkingPosition(event: MouseEvent) {
  if (!linking.value || !canvasRef.value) return;
  const canvasRect = canvasRef.value.getBoundingClientRect();
  linking.value.current = {
    x: event.clientX - canvasRect.left,
    y: event.clientY - canvasRect.top
  };
}

function startDrag(node: { id: string; x: number; y: number }, event: MouseEvent) {
  if (!canvasRef.value) return;
  if (event.button !== 0) return;
  const target = event.target as HTMLElement | null;
  if (target?.closest(".node-handle") || target?.closest(".node-remove")) {
    return;
  }
  const canvasRect = canvasRef.value.getBoundingClientRect();
  const offsetX = event.clientX - canvasRect.left - node.x;
  const offsetY = event.clientY - canvasRect.top - node.y;
  dragging.value = { id: node.id, offsetX, offsetY };
}

function updateDraggingPosition(event: MouseEvent) {
  if (!dragging.value || !canvasRef.value) return;
  const canvasRect = canvasRef.value.getBoundingClientRect();
  const nextX = event.clientX - canvasRect.left - dragging.value.offsetX;
  const nextY = event.clientY - canvasRect.top - dragging.value.offsetY;
  const node = nodes.value.find((item) => item.id === dragging.value?.id);
  if (!node) return;
  node.x = Math.max(12, nextX);
  node.y = Math.max(12, nextY);
}

function stopDragging() {
  if (!dragging.value) return;
  dragging.value = null;
  scheduleDraftSave("move");
}

function stopLinking() {
  if (!linking.value) return;
  if (linkTargetId.value) {
    addEdge(linking.value.sourceId, linkTargetId.value);
  }
  linking.value = null;
  linkTargetId.value = null;
}

const edgePaths = computed(() => {
  const map = new Map(nodes.value.map((node) => [node.id, node]));
  return edges.value
    .map((edge) => {
      const source = map.get(edge.source);
      const target = map.get(edge.target);
      if (!source || !target) return null;
      const sx = source.x + 90;
      const sy = source.y + 45;
      const tx = target.x + 90;
      const ty = target.y + 45;
      const dx = Math.max(40, Math.abs(tx - sx) * 0.5);
      const path = `M ${sx} ${sy} C ${sx + dx} ${sy}, ${tx - dx} ${ty}, ${tx} ${ty}`;
      return { id: edge.id, path };
    })
    .filter(Boolean) as Array<{ id: string; path: string }>;
});

const linkingPath = computed(() => {
  if (!linking.value) return "";
  const { start, current } = linking.value;
  const dx = Math.max(40, Math.abs(current.x - start.x) * 0.5);
  return `M ${start.x} ${start.y} C ${start.x + dx} ${start.y}, ${current.x - dx} ${current.y}, ${current.x} ${current.y}`;
});

function onWindowMouseMove(event: MouseEvent) {
  updateLinkingPosition(event);
  updateDraggingPosition(event);
}

function onWindowMouseUp() {
  stopLinking();
  stopDragging();
}

onMounted(() => {
  window.addEventListener("mousemove", onWindowMouseMove);
  window.addEventListener("mouseup", onWindowMouseUp);
});

onBeforeUnmount(() => {
  window.removeEventListener("mousemove", onWindowMouseMove);
  window.removeEventListener("mouseup", onWindowMouseUp);
});

function applyDetail() {
  if (!selectedNode.value) return;
  try {
    const parsed = dataInput.value.trim() ? JSON.parse(dataInput.value) : {};
    selectedNode.value.data = parsed;
  } catch {
    // ignore invalid json
  }
  syncYamlFromNodes();
  scheduleDraftSave("detail");
}

function resetDetail() {
  if (!selectedNode.value) return;
  dataInput.value = JSON.stringify(selectedNode.value.data || {}, null, 2);
}

watch(selectedNode, (node) => {
  if (!node) return;
  dataInput.value = JSON.stringify(node.data || {}, null, 2);
});

function autoLayout() {
  if (!canvasRef.value || nodes.value.length === 0) return;
  const rect = canvasRef.value.getBoundingClientRect();
  const paddingX = 32;
  const paddingY = 32;
  const nodeWidth = 180;
  const nodeHeight = 90;
  const gapX = 40;
  const gapY = 30;
  const availableWidth = Math.max(rect.width - paddingX * 2, nodeWidth);
  const columns = Math.max(1, Math.floor(availableWidth / (nodeWidth + gapX)));

  nodes.value = nodes.value.map((node, index) => {
    const col = index % columns;
    const row = Math.floor(index / columns);
    return {
      ...node,
      x: paddingX + col * (nodeWidth + gapX),
      y: paddingY + row * (nodeHeight + gapY)
    };
  });
  scheduleDraftSave("layout");
}

function buildYamlFromNodes() {
  if (!nodes.value.length) return "";
  const actionNodes = nodes.value.filter((node) => node.type === "action");
  if (!actionNodes.length) return "";
  const lines: string[] = ["version: v0.1", "name: draft-workbench", "steps:"];
  actionNodes.forEach((node) => {
    const data = node.data || {};
    const actionRaw = data["action"];
    const targetsRaw = data["targets"];
    const withRaw = data["with"];
    const action = typeof actionRaw === "string" ? actionRaw : "";
    const targets = Array.isArray(targetsRaw)
      ? targetsRaw.map((item) => String(item))
      : [];
    const withValue =
      withRaw && typeof withRaw === "object" && !Array.isArray(withRaw)
        ? (withRaw as Record<string, unknown>)
        : null;
    lines.push(`  - name: ${yamlScalar(node.name)}`);
    if (action) {
      lines.push(`    action: ${yamlScalar(action)}`);
    }
    if (targets.length) {
      lines.push(`    targets: ${JSON.stringify(targets)}`);
    }
    if (withValue) {
      lines.push(`    with: ${JSON.stringify(withValue)}`);
    }
  });
  return lines.join("\n");
}

function syncYamlFromNodes() {
  yamlText.value = buildYamlFromNodes();
}

function yamlScalar(value: string) {
  if (!value) return "\"\"";
  if (/[:#\n]/.test(value)) {
    return JSON.stringify(value);
  }
  return value;
}

function normalizeYamlText(value: string) {
  return value
    .split("\n")
    .map((line) => line.trimEnd())
    .join("\n")
    .trim();
}

function stableStringify(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableStringify(item)).join(",")}]`;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    const keys = Object.keys(record).sort();
    return `{${keys.map((key) => `${key}:${stableStringify(record[key])}`).join(",")}}`;
  }
  return String(value);
}

function stepsSignatureFromNodes() {
  const steps = nodes.value.map((node) => ({
    name: node.name || "",
    type: node.type || "",
    data: stableStringify(node.data || {})
  }));
  const links = edges.value.map((edge) => ({
    source: edge.source,
    target: edge.target
  }));
  return JSON.stringify({ steps, links });
}

function stepsSignatureFromGraph(graph: any) {
  const graphNodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
  const graphEdges = Array.isArray(graph?.edges) ? graph.edges : [];
  const steps = graphNodes.map((node: any) => ({
    name: String(node?.name || ""),
    type: String(node?.type || ""),
    data: stableStringify(
      node?.data ??
        (node?.action || node?.with || node?.targets
          ? { action: node.action, with: node.with, targets: node.targets }
          : {})
    )
  }));
  const links = graphEdges.map((edge: any) => ({
    source: String(edge?.source || ""),
    target: String(edge?.target || "")
  }));
  return JSON.stringify({ steps, links });
}

function buildGraphFromNodes() {
  const graphNodes = nodes.value.map((node, idx) => ({
    id: node.id,
    type: node.type || "custom",
    name: node.name || `node-${idx + 1}`,
    data: node.data || {},
    ui: { x: node.x, y: node.y }
  }));
  const validIds = new Set(graphNodes.map((node) => node.id));
  const graphEdges = edges.value.filter(
    (edge) => validIds.has(edge.source) && validIds.has(edge.target)
  );
  return {
    version: "v1",
    layout: { direction: "LR" },
    nodes: graphNodes,
    edges: graphEdges
  };
}

function ensureDraftId() {
  if (draftId.value) return draftId.value;
  const fallback = `draft-${Date.now()}-${Math.floor(Math.random() * 10000)}`;
  const id =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? `draft-${crypto.randomUUID()}`
      : fallback;
  draftId.value = id;
  return id;
}

function scheduleDraftSave(reason?: string) {
  if (draftSaveTimer) {
    window.clearTimeout(draftSaveTimer);
  }
  draftSaveTimer = window.setTimeout(() => {
    void saveDraft(reason);
  }, 600);
}

async function saveDraft(_reason?: string) {
  const yaml = yamlText.value || buildYamlFromNodes();
  const graph = buildGraphFromNodes();
  const graphText = JSON.stringify(graph);
  if (!yaml.trim() && (!graph.nodes || graph.nodes.length === 0)) return;
  if (
    normalizeYamlText(yaml) === normalizeYamlText(lastSavedYaml) &&
    graphText === lastSavedGraph
  ) {
    return;
  }
  const id = ensureDraftId();
  try {
    const resp = await request<{ draft?: { id?: string } }>(`/ai/workflow/draft/${id}`, {
      method: "PUT",
      body: { id, yaml, graph }
    });
    if (resp?.draft?.id) {
      draftId.value = resp.draft.id;
    }
    lastSavedYaml = yaml;
    lastSavedGraph = graphText;
  } catch {
    // ignore auto-save errors
  }
}

async function fetchGraphFromYaml(yaml: string) {
  if (!yaml.trim()) return null;
  try {
    const resp = await request<{ graph: any }>("/ai/workflow/graph-from-yaml", {
      method: "POST",
      body: { yaml }
    });
    return resp.graph || null;
  } catch {
    return null;
  }
}

async function resolveGraphYamlConflict(draftYaml: string, graphYaml: string) {
  const useYaml = window.confirm(
    "检测到草稿 YAML 与图不一致，是否以 YAML 覆盖图？点击取消将以图覆盖 YAML。"
  );
  if (useYaml) {
    yamlText.value = draftYaml;
    const graph = await fetchGraphFromYaml(draftYaml);
    if (graph) {
      applyGraph(graph);
      await saveDraft("yaml-override");
    } else {
      chatError.value = "无法根据 YAML 生成图，请稍后重试。";
    }
  } else {
    yamlText.value = graphYaml;
    await saveDraft("graph-override");
  }
}

async function handleGenerate(prompt: string) {
  chatError.value = "";
  chatStatus.value = "";
  if (!prompt.trim()) {
    chatError.value = "请输入需求后再生成。";
    return;
  }
  chatBusy.value = true;
  chatStatus.value = "正在生成流程...";
  try {
    const data = await request<{ yaml: string; draft_id?: string; message?: string }>(
      "/ai/workflow/generate",
      {
        method: "POST",
        body: { prompt }
      }
    );
    yamlText.value = data.yaml || "";
    draftId.value = data.draft_id || "";
    await syncGraphFromDraft();
    scheduleDraftSave("generate");
    chatStatus.value = data.message || "生成完成";
  } catch (err) {
    chatError.value = "生成失败，请稍后重试。";
  } finally {
    chatBusy.value = false;
  }
}

async function handleFix() {
  chatError.value = "";
  chatStatus.value = "";
  const yaml = yamlText.value || buildYamlFromNodes();
  if (!yaml.trim()) {
    chatError.value = "当前没有可修复的流程。";
    return;
  }
  chatBusy.value = true;
  chatStatus.value = "正在检查问题...";
  try {
    const summary = await request<{
      issues: string[];
      summary: string;
    }>("/ai/workflow/summary", { method: "POST", body: { yaml } });
    if (!summary.issues || summary.issues.length === 0) {
      chatStatus.value = "未发现问题，无需修复。";
      return;
    }
    chatStatus.value = "正在修复流程...";
    const fixed = await request<{ yaml: string; draft_id?: string; message?: string }>(
      "/ai/workflow/fix",
      {
        method: "POST",
        body: { yaml, issues: summary.issues }
      }
    );
    yamlText.value = fixed.yaml || yamlText.value;
    draftId.value = fixed.draft_id || draftId.value;
    await syncGraphFromDraft();
    scheduleDraftSave("fix");
    chatStatus.value = fixed.message || "修复完成";
  } catch (err) {
    chatError.value = "修复失败，请稍后重试。";
  } finally {
    chatBusy.value = false;
  }
}

async function handleRegenerate(prompt: string) {
  chatError.value = "";
  chatStatus.value = "";
  if (!selectedNode.value) {
    chatError.value = "请先选择节点。";
    return;
  }
  if (selectedNode.value.type !== "action") {
    chatError.value = "该节点类型暂不支持重生成。";
    return;
  }
  const yaml = yamlText.value || buildYamlFromNodes();
  if (!yaml.trim()) {
    chatError.value = "当前没有可重生成的流程。";
    return;
  }
  chatBusy.value = true;
  chatStatus.value = "正在重生成节点...";
  try {
    const index = nodes.value.findIndex((node) => node.id === selectedNode.value?.id);
    const prev = index > 0 ? [nodes.value[index - 1]] : [];
    const next = index >= 0 && index < nodes.value.length - 1 ? [nodes.value[index + 1]] : [];
    const nodeData = selectedNode.value.data || {};
    const actionRaw = nodeData["action"];
    const withRaw = nodeData["with"];
    const targetsRaw = nodeData["targets"];
    const resp = await request<{ yaml: string; graph?: any; message?: string }>(
      "/ai/workflow/node-regenerate",
      {
        method: "POST",
        body: {
          node: {
            id: selectedNode.value.id,
            index,
            name: selectedNode.value.name,
            action: String(actionRaw || ""),
            with: (withRaw as Record<string, unknown>) || {},
            targets: Array.isArray(targetsRaw) ? targetsRaw : []
          },
          neighbors: {
            prev: prev.map((item) => ({
              name: item.name,
              action: String(item.data ? item.data["action"] : "")
            })),
            next: next.map((item) => ({
              name: item.name,
              action: String(item.data ? item.data["action"] : "")
            }))
          },
          workflow: { yaml },
          intent: prompt
        }
      }
    );
    if (resp.yaml) {
      yamlText.value = resp.yaml;
    }
    if (resp.graph) {
      applyGraph(resp.graph);
    } else {
      await syncGraphFromDraft();
    }
    scheduleDraftSave("regenerate");
    chatStatus.value = resp.message || "节点已更新";
  } catch (err) {
    chatError.value = "重生成失败，请稍后重试。";
  } finally {
    chatBusy.value = false;
  }
}

async function handleAutoFix() {
  chatError.value = "";
  chatStatus.value = "";
  const yaml = yamlText.value || buildYamlFromNodes();
  if (!yaml.trim()) {
    chatError.value = "当前没有可校验的流程。";
    return;
  }
  if (autoFixRunning.value) return;
  autoFixRunning.value = true;
  chatBusy.value = true;
  chatStatus.value = "正在校验与修复...";

  try {
    const response = await fetch(`${apiBase()}/ai/workflow/auto-fix-run`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ yaml, max_retries: 2 })
    });
    if (!response.ok || !response.body) {
      chatError.value = "校验启动失败。";
      return;
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder("utf-8");
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split("\n\n");
      buffer = parts.pop() || "";
      for (const part of parts) {
        const lines = part.split("\n");
        let eventName = "";
        let data = "";
        for (const line of lines) {
          if (line.startsWith("event:")) {
            eventName = line.replace("event:", "").trim();
          } else if (line.startsWith("data:")) {
            data += line.replace("data:", "").trim();
          }
        }
        if (!data) continue;
        try {
          const payload = JSON.parse(data);
          if (eventName === "status") {
            chatStatus.value = payload.message || "运行中...";
          } else if (eventName === "result") {
            if (typeof payload.yaml === "string") {
              yamlText.value = payload.yaml;
            }
            if (payload.draft_id) {
              draftId.value = String(payload.draft_id);
            }
            await syncGraphFromDraft();
            if (Array.isArray(payload.diffs) && payload.diffs.length) {
              chatStatus.value = `${payload.summary || "校验完成"}; 修复差异: ${payload.diffs.join(", ")}`;
            } else {
              chatStatus.value = payload.summary || "校验完成";
            }
            if (Array.isArray(payload.issues) && payload.issues.length) {
              runLogs.value.push(`[issues] ${payload.issues.join("; ")}`);
            }
            scheduleDraftSave("auto-fix");
          } else if (eventName === "error") {
            chatError.value = payload.error || "校验失败";
          }
        } catch {
          // ignore parse errors
        }
      }
    }
  } catch (err) {
    chatError.value = "校验失败，请稍后重试。";
  } finally {
    autoFixRunning.value = false;
    chatBusy.value = false;
  }
}

async function handleRun() {
  chatError.value = "";
  chatStatus.value = "";
  const graph = buildGraphFromNodes();
  if (!graph.nodes.length) {
    chatError.value = "当前没有可运行的流程。";
    return;
  }
  chatBusy.value = true;
  runLogs.value = [];
  runSummary.value = null;
  runStatus.value = "running";
  try {
    const resp = await request<{ run_id: string; status: string }>("/runs/graph", {
      method: "POST",
      body: { graph, inputs: {} }
    });
    runId.value = resp.run_id;
    runStatus.value = resp.status;
    await subscribeRunStream(resp.run_id);
  } catch (err) {
    chatError.value = "运行启动失败。";
    runStatus.value = "failed";
  } finally {
    chatBusy.value = false;
  }
}

async function subscribeRunStream(id: string) {
  const response = await fetch(`${apiBase()}/runs/${id}/stream`);
  if (!response.body) return;
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() || "";
    for (const part of parts) {
      const lines = part.split("\n");
      let eventName = "";
      let data = "";
      for (const line of lines) {
        if (line.startsWith("event:")) {
          eventName = line.replace("event:", "").trim();
        } else if (line.startsWith("data:")) {
          data += line.replace("data:", "").trim();
        }
      }
      if (!data) continue;
      try {
        const payload = JSON.parse(data);
        const eventData =
          payload && typeof payload === "object" && payload.data && typeof payload.data === "object"
            ? payload.data
            : {};
        if (eventName === "workflow_start") {
          const status = String(eventData.status || payload.status || "running");
          runStatus.value = status;
          runLogs.value.push(`[workflow_start] ${status}`);
          continue;
        }
        if (eventName === "workflow_end") {
          const summary: RunSummary = {
            status: String(eventData.status || payload.status || "finished"),
            totalSteps: Number(eventData.total_steps || 0),
            successSteps: Number(eventData.success_steps || 0),
            failedSteps: Number(eventData.failed_steps || 0),
            durationMs: Number(eventData.duration_ms || 0),
            issues: Array.isArray(eventData.issues)
              ? eventData.issues.map((item: unknown) => String(item))
              : [],
            message: String(eventData.message || payload.message || "")
          };
          runSummary.value = summary;
          runStatus.value = summary.status || "finished";
          runLogs.value.push(`[summary] ${formatSummaryLine(summary)}`);
          if (summary.issues.length) {
            runLogs.value.push(`[issues] ${summary.issues.join("; ")}`);
          }
        } else {
          const line = formatRunEventLine(eventName, payload, eventData);
          if (line) {
            runLogs.value.push(line);
          }
        }
      } catch {
        // ignore
      }
    }
  }
}

async function syncGraphFromDraft() {
  if (!draftId.value) {
    await applyGraphFromYamlFallback();
    return;
  }
  try {
    const draft = await request<{ draft: { graph?: any; yaml?: string } }>(
      `/ai/workflow/draft/${draftId.value}`
    );
    const draftYaml = draft?.draft?.yaml || "";
    const draftGraph = draft?.draft?.graph;
    if (draftGraph) {
      applyGraph(draftGraph);
      if (draftYaml) {
        const graphYaml = buildYamlFromNodes();
        const graphSignature = stepsSignatureFromNodes();
        const yamlGraph = await fetchGraphFromYaml(draftYaml);
        if (yamlGraph) {
          const yamlSignature = stepsSignatureFromGraph(yamlGraph);
          if (graphSignature !== yamlSignature) {
            await resolveGraphYamlConflict(draftYaml, graphYaml);
            return;
          }
        }
        yamlText.value = draftYaml;
      } else {
        syncYamlFromNodes();
      }
    } else if (draftYaml) {
      yamlText.value = draftYaml;
      const graph = await fetchGraphFromYaml(draftYaml);
      if (graph) {
        applyGraph(graph);
      }
    }
  } catch {
    await applyGraphFromYamlFallback();
  }
}

async function applyGraphFromYamlFallback() {
  const yaml = yamlText.value.trim();
  if (!yaml) return;
  const graph = await fetchGraphFromYaml(yaml);
  if (graph) {
    applyGraph(graph);
  }
}

function applyGraph(raw: any) {
  const graph = raw?.graph ? raw.graph : raw;
  if (!graph || !Array.isArray(graph.nodes)) return;
  nodes.value = graph.nodes.map((node: any, idx: number) => ({
    id: String(node.id ?? `node-${idx}`),
    type: String(
      node.type || (node.action || node.with || node.targets ? "action" : "custom")
    ),
    name: node.name || `node-${idx + 1}`,
    data:
      node.data ??
      (node.action || node.with || node.targets
        ? {
            action: node.action || "",
            with: node.with || {},
            targets: node.targets || []
          }
        : {}),
    x: Number(node.ui?.x ?? 40 + idx * 200),
    y: Number(node.ui?.y ?? 80)
  }));
  edges.value = Array.isArray(graph.edges)
    ? graph.edges.map((edge: any, idx: number) => ({
        id: String(edge.id ?? `edge-${idx}`),
        source: String(edge.source || ""),
        target: String(edge.target || "")
      }))
    : [];
}

function formatDuration(ms: number) {
  if (!ms || ms <= 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds - minutes * 60);
  return `${minutes}m ${rest}s`;
}

function formatSummaryLine(summary: RunSummary) {
  const total = summary.totalSteps || 0;
  const success = summary.successSteps || 0;
  const failed = summary.failedSteps || 0;
  const duration = formatDuration(summary.durationMs);
  return `status=${summary.status || "finished"} success=${success}/${total} failed=${failed} duration=${duration}`;
}

function formatRunEventLine(eventName: string, payload: any, eventData: Record<string, any>) {
  const parts: string[] = [`[${eventName}]`];
  if (payload?.step) {
    parts.push(String(payload.step));
  }
  if (payload?.host) {
    parts.push(`@${payload.host}`);
  }
  const status = eventData?.status ?? payload?.status;
  if (status) {
    parts.push(String(status));
  }
  const message = eventData?.message ?? payload?.message;
  if (message) {
    parts.push(String(message));
  }
  const err = eventData?.error;
  if (err) {
    parts.push(String(err));
  }
  return parts.join(" ").trim();
}
</script>

<style scoped>
.workbench {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 0;
  position: relative;
  flex: 1;
  min-height: 0;
  height: 100%;
  width: 100%;
}

.workbench-topbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.workbench-topbar .btn {
  border-radius: 999px;
  border-color: transparent;
  background: #f2ede7;
  color: var(--ink);
  box-shadow: 0 8px 16px rgba(27, 27, 27, 0.08);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.workbench-topbar .btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 18px rgba(27, 27, 27, 0.12);
}

.workbench-topbar .btn.ghost {
  background: #ffffff;
  border: 1px solid #e3ded7;
  color: var(--muted);
  box-shadow: none;
}

.workbench-topbar .btn.ghost:hover {
  box-shadow: 0 8px 16px rgba(27, 27, 27, 0.08);
  color: var(--ink);
}

.workbench-topbar .btn.btn-sm {
  padding: 6px 14px;
}

.status-chip {
  font-size: 12px;
  color: var(--muted);
  border: 1px solid var(--grid);
  border-radius: 999px;
  padding: 6px 12px;
  background: #f6f3ef;
}

.workbench-body {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 16px;
  flex: 1;
  min-height: 0;
  height: 100%;
}

.workbench-body.detail-open {
  grid-template-columns: 280px minmax(0, 1fr) 280px;
}

.library-pane {
  height: 100%;
  min-height: 0;
}

.canvas {
  position: relative;
  background: #f1ede7;
  border-radius: var(--radius-lg);
  border: 1px solid #ddd6ce;
  overflow: hidden;
  min-height: 0;
  height: 100%;
}

.edge-layer {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
}

.edge-path {
  fill: none;
  stroke: rgba(90, 90, 90, 0.5);
  stroke-width: 2;
  pointer-events: stroke;
  cursor: pointer;
}

.edge-path:hover {
  stroke: rgba(232, 93, 42, 0.7);
}

.edge-path.preview {
  stroke-dasharray: 4 6;
  stroke: rgba(232, 93, 42, 0.5);
  pointer-events: none;
}

.canvas-grid {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(#d7d0c7 1px, transparent 1px);
  background-size: 22px 22px;
  opacity: 0.6;
}

.canvas-node {
  position: absolute;
  z-index: 2;
  min-width: 140px;
  padding: 10px 12px;
  background: #ffffff;
  border: 1px solid #e3ded7;
  border-radius: var(--radius-md);
  box-shadow: 0 10px 20px rgba(27, 27, 27, 0.08);
  cursor: grab;
}

.canvas-node.dragging {
  cursor: grabbing;
}

.node-handles {
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  display: flex;
  justify-content: space-between;
  transform: translateY(-50%);
  pointer-events: none;
}

.node-handle {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid #e3ded7;
  background: #fff;
  pointer-events: auto;
  cursor: crosshair;
}

.node-handle.handle-in {
  margin-left: -8px;
}

.node-handle.handle-out {
  margin-right: -8px;
}

.node-remove {
  position: absolute;
  top: 6px;
  right: 6px;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.canvas-hint {
  position: absolute;
  top: 14px;
  left: 16px;
  font-size: 12px;
  color: var(--muted);
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-pane {
  background: var(--panel);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.detail-head h3 {
  margin: 0;
  font-size: 14px;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field input,
.field textarea {
  border: 1px solid var(--grid);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 12px;
  font-family: inherit;
}

.detail-actions {
  display: flex;
  gap: 8px;
}

.node-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}

.node-action {
  font-size: 11px;
  color: var(--muted);
  margin-top: 4px;
}

@media (max-width: 980px) {
  .workbench-body {
    grid-template-columns: 1fr;
  }
  .canvas {
    min-height: 0;
    height: 60vh;
  }
}
</style>
