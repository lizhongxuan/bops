<template>
  <section class="home-ai">
    <div class="hero">
      <div class="hero-copy">
        <div class="eyebrow">AI Workflow Assistant</div>
        <h1>从一句需求到可执行 YAML，几分钟内完成验证与修复。</h1>
        <p>
          生成 → 校验 → 修复 → 审核，一条链路把运维流程变成可追溯的工作流。
        </p>
        <div class="hero-actions">
          <button class="btn primary" type="button" :disabled="busy" @click="startStream">
            生成方案
          </button>
          <button class="btn ghost" type="button" @click="applyExample(examples[0])">
            插入示例
          </button>
        </div>
      </div>
      <div class="hero-card">
        <div class="hero-stat">
          <span class="label">当前草稿</span>
          <span class="value">{{ draftId || "未保存" }}</span>
        </div>
        <div class="hero-stat">
          <span class="label">风险等级</span>
          <span class="value" :class="`risk-${summary.riskLevel}`">
            {{ summary.riskLevel || "-" }}
          </span>
        </div>
        <div class="hero-stat">
          <span class="label">步骤数量</span>
          <span class="value">{{ summary.steps || 0 }}</span>
        </div>
        <div class="hero-stat">
          <span class="label">待审核</span>
          <span class="value">{{ summary.needsReview ? "是" : "否" }}</span>
        </div>
      </div>
    </div>

    <div class="composer-grid">
      <div class="panel composer">
        <div class="panel-head">
          <div>
            <h2>需求输入</h2>
            <p>描述目标、主机与步骤，AI 会生成 YAML 与步骤预览。</p>
          </div>
          <div class="status-tag" :class="busy ? 'busy' : 'idle'">
            {{ busy ? "生成中" : "就绪" }}
          </div>
        </div>

        <textarea
          v-model="prompt"
          placeholder="例如：在 web1/web2 上安装 nginx，渲染配置并启动服务"
          rows="6"
        ></textarea>

        <div class="example-row">
          <button
            v-for="item in examples"
            :key="item"
            class="chip"
            type="button"
            @click="applyExample(item)"
          >
            {{ item }}
          </button>
        </div>

        <div class="constraints">
          <div class="field">
            <label>目标环境</label>
            <input v-model="environmentNote" type="text" placeholder="例如 Ubuntu 22.04 / macOS M1" />
          </div>
          <div class="field">
            <label>目标主机/分组</label>
            <input v-model="targetHint" type="text" placeholder="例如 web, db" />
          </div>
          <div class="field">
            <label>执行策略</label>
            <select v-model="planMode">
              <option value="manual-approve">manual-approve</option>
              <option value="auto">auto</option>
            </select>
          </div>
          <div class="field">
            <label>环境变量包</label>
            <input v-model="envPackages" type="text" placeholder="prod-env, staging" />
          </div>
          <div class="field">
            <label>最大修复次数</label>
            <input v-model.number="maxRetries" type="number" min="0" max="5" />
          </div>
          <div class="field">
            <label>验证环境</label>
            <select v-model="selectedValidationEnv">
              <option value="">默认</option>
              <option v-for="env in validationEnvs" :key="env.name" :value="env.name">
                {{ env.name }}
              </option>
            </select>
          </div>
          <div class="field toggle">
            <label>自动执行验证</label>
            <button
              class="toggle-btn"
              type="button"
              :class="executeEnabled ? 'on' : 'off'"
              @click="executeEnabled = !executeEnabled"
            >
              {{ executeEnabled ? "启用" : "关闭" }}
            </button>
          </div>
        </div>

        <div class="composer-actions">
          <button class="btn primary" type="button" :disabled="busy" @click="startStream">
            生成方案
          </button>
          <button class="btn" type="button" :disabled="busy || !yaml.trim()" @click="refreshSummary">
            刷新概览
          </button>
          <button class="btn ghost" type="button" :disabled="busy || !yaml.trim()" @click="validateDraft">
            校验
          </button>
        </div>
      </div>

      <div class="panel progress">
        <div class="panel-head">
          <div>
            <h2>运行进度</h2>
            <p>流式展示节点状态与修复过程。</p>
          </div>
          <div class="status-tag" :class="streamError ? 'error' : 'idle'">
            {{ streamError ? "异常" : "监控中" }}
          </div>
        </div>

        <div v-if="streamError" class="alert error">{{ streamError }}</div>

        <div class="progress-list">
          <div v-if="progressEvents.length === 0" class="empty">等待生成…</div>
          <div v-else class="progress-item" v-for="(evt, index) in progressEvents" :key="index">
            <div class="node">{{ formatNode(evt.node) }}</div>
            <div class="status" :class="evt.status">{{ evt.status }}</div>
            <div class="message" v-if="evt.message">{{ evt.message }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="result-grid">
      <div class="panel steps">
        <div class="panel-head">
          <div>
            <h2>步骤预览</h2>
            <p>可视化查看目标与动作。</p>
          </div>
          <div class="status-tag" :class="summary.needsReview ? 'warn' : 'ok'">
            {{ summary.needsReview ? "待审核" : "已就绪" }}
          </div>
        </div>

        <div v-if="steps.length === 0" class="empty">未解析到步骤</div>
        <div
          class="step-card"
          v-for="(step, index) in steps"
          :key="step.name"
          :class="{ active: selectedStep === step.name, error: stepIssueIndexes.includes(index) }"
          role="button"
          tabindex="0"
          @click="focusStep(step)"
          @keydown.enter.prevent="focusStep(step)"
          @keydown.space.prevent="focusStep(step)"
        >
          <div class="step-name">{{ step.name }}</div>
          <div class="step-meta">{{ step.action || "未指定动作" }}</div>
          <div class="step-targets" v-if="step.targets">目标: {{ step.targets }}</div>
        </div>

        <div class="history" v-if="historyTimeline.length">
          <h3>修复时间轴</h3>
          <div class="history-list">
            <button
              class="history-item"
              v-for="item in historyTimeline"
              :key="item.index"
              type="button"
              @click="restoreHistory(item.index)"
            >
              <div>
                <div class="history-title">{{ item.label }}</div>
                <div class="history-diff">{{ item.diff }}</div>
              </div>
              <span class="history-restore">恢复</span>
            </button>
          </div>
        </div>
      </div>

      <div class="panel yaml">
        <div class="panel-head">
          <div>
            <h2>YAML 工作流</h2>
            <p>生成结果可直接保存或继续手动编辑。</p>
          </div>
          <div class="status-tag" :class="validation.ok ? 'ok' : 'warn'">
            {{ validation.ok ? "校验通过" : "待修复" }}
          </div>
        </div>

        <textarea ref="yamlRef" v-model="yaml" spellcheck="false" class="code" rows="18"></textarea>

        <div class="yaml-actions">
          <button class="btn" type="button" :disabled="validationBusy || !yaml.trim()" @click="validateDraft">
            校验
          </button>
          <button
            class="btn"
            type="button"
            :disabled="executeBusy || !yaml.trim()"
            @click="runExecution"
          >
            沙箱验证
          </button>
          <button
            class="btn primary"
            type="button"
            :disabled="busy || !yaml.trim() || requiresConfirm"
            @click="saveWorkflow"
          >
            保存为工作流
          </button>
        </div>

        <div class="alert" :class="validation.ok ? 'ok' : 'warn'">
          {{ validation.ok ? "校验通过" : "校验未通过" }}
        </div>
        <ul class="issues" v-if="validation.issues.length">
          <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
        </ul>

        <div class="summary">
          <div class="summary-item">
            <span>概览</span>
            <strong>{{ summary.summary || "-" }}</strong>
          </div>
          <div class="summary-item">
            <span>风险</span>
            <strong :class="`risk-${summary.riskLevel}`">{{ summary.riskLevel || "-" }}</strong>
          </div>
          <div class="summary-item" v-if="summary.riskNotes.length">
            <span>提示</span>
            <strong>{{ summary.riskNotes.join(" · ") }}</strong>
          </div>
        </div>

        <div v-if="summary.needsReview" class="human-gate">
          <div class="gate-copy">检测到风险或校验失败，需要人工确认后才能保存。</div>
          <div v-if="requiresReason" class="gate-reason">
            <label>确认原因</label>
            <input v-model="confirmReason" type="text" placeholder="填写原因" />
          </div>
          <div class="gate-actions">
            <button
              class="btn ghost"
              type="button"
              :disabled="requiresReason && !confirmReason.trim() && !humanConfirmed"
              @click="humanConfirmed = !humanConfirmed"
            >
              {{ humanConfirmed ? "已确认" : "人工确认" }}
            </button>
          </div>
        </div>

        <div v-if="executeResult" class="execution-result" :class="executeResult.status">
          <div class="result-title">
            执行结果: {{ executeResult.status }}
            <span v-if="executeResult.code">(code {{ executeResult.code }})</span>
          </div>
          <div v-if="executeResult.error" class="result-error">{{ executeResult.error }}</div>
          <div class="result-io">
            <div v-if="executeResult.stdout" class="result-block">
              <div class="result-label">stdout</div>
              <pre>{{ executeResult.stdout }}</pre>
            </div>
            <div v-if="executeResult.stderr" class="result-block">
              <div class="result-label">stderr</div>
              <pre>{{ executeResult.stderr }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { ApiError, apiBase, request } from "../lib/api";
import { parseSteps, type StepSummary } from "../lib/workflowSteps";

type ValidationEnvSummary = {
  name: string;
};

type ValidationState = {
  ok: boolean;
  issues: string[];
};

type ExecutionResult = {
  status: string;
  stdout?: string;
  stderr?: string;
  code?: number;
  error?: string;
};

type ProgressEvent = {
  node: string;
  status: string;
  message?: string;
};

type SummaryState = {
  summary: string;
  steps: number;
  riskLevel: string;
  riskNotes: string[];
  issues: string[];
  needsReview: boolean;
};

type SummaryResponse = {
  summary?: string;
  steps?: number;
  risk_level?: string;
  riskLevel?: string;
  risk_notes?: string[];
  riskNotes?: string[];
  issues?: string[];
  needs_review?: boolean;
  needsReview?: boolean;
};

type HistoryEntry = {
  index: number;
  label: string;
  diff: string;
};

const prompt = ref("");
const yaml = ref("");
const yamlRef = ref<HTMLTextAreaElement | null>(null);
const busy = ref(false);
const streamError = ref("");
const progressEvents = ref<ProgressEvent[]>([]);
const selectedStep = ref("");
const stepIssueIndexes = ref<number[]>([]);
const draftId = ref("");
const history = ref<string[]>([]);
const validation = ref<ValidationState>({ ok: true, issues: [] });
const validationBusy = ref(false);
const executeBusy = ref(false);
const executeResult = ref<ExecutionResult | null>(null);
const summary = ref<SummaryState>({
  summary: "",
  steps: 0,
  riskLevel: "",
  riskNotes: [],
  issues: [],
  needsReview: false
});
const humanConfirmed = ref(false);
const confirmReason = ref("");

const validationEnvs = ref<ValidationEnvSummary[]>([]);
const selectedValidationEnv = ref("");
const executeEnabled = ref(false);
const maxRetries = ref(2);
const planMode = ref("manual-approve");
const envPackages = ref("");
const environmentNote = ref("");
const targetHint = ref("");
const router = useRouter();

const examples = [
  "在 web1/web2 上安装 nginx，渲染配置并启动服务",
  "检查磁盘空间，超过 80% 则告警",
  "拉取脚本库中的备份脚本并执行"
];

const steps = computed<StepSummary[]>(() => parseSteps(yaml.value));
const requiresReason = computed(() => summary.value.riskLevel === "high");
const requiresConfirm = computed(() => {
  if (!summary.value.needsReview) return false;
  if (!humanConfirmed.value) return true;
  if (requiresReason.value && !confirmReason.value.trim()) return true;
  return false;
});
const historyTimeline = computed<HistoryEntry[]>(() => buildHistoryTimeline());

let summaryTimer: number | null = null;
watch(yaml, () => {
  if (summaryTimer) {
    window.clearTimeout(summaryTimer);
  }
  selectedStep.value = "";
  stepIssueIndexes.value = [];
  humanConfirmed.value = false;
  confirmReason.value = "";
  summaryTimer = window.setTimeout(() => {
    void refreshSummary();
  }, 600);
});

onMounted(() => {
  loadValidationEnvs();
});

async function loadValidationEnvs() {
  try {
    const data = await request<{ items: ValidationEnvSummary[] }>("/validation-envs");
    validationEnvs.value = data.items || [];
  } catch (err) {
    validationEnvs.value = [];
  }
}

function applyExample(text: string) {
  prompt.value = text;
}

function formatNode(node: string) {
  return node.replace(/_/g, " ");
}

function focusStep(step: StepSummary) {
  selectedStep.value = step.name;
  const textarea = yamlRef.value;
  if (!textarea) return;
  const lines = yaml.value.split(/\r?\n/);
  let lineIndex = typeof step.line === "number" ? step.line : -1;
  if (lineIndex < 0) {
    lineIndex = lines.findIndex((line) => line.trim() === `- name: ${step.name}`);
  }
  if (lineIndex < 0 || lineIndex >= lines.length) return;
  let start = 0;
  for (let i = 0; i < lineIndex; i++) {
    start += lines[i].length + 1;
  }
  const end = start + lines[lineIndex].length;
  textarea.focus();
  textarea.setSelectionRange(start, end);
  const style = window.getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight || "") || 18;
  textarea.scrollTop = Math.max(0, lineIndex * lineHeight - lineHeight);
}

function buildContext() {
  const packages = envPackages.value
    .split(/,\s*/)
    .map((item) => item.trim())
    .filter(Boolean);
  const payload: Record<string, unknown> = {
    plan_mode: planMode.value,
    max_retries: maxRetries.value
  };
  if (environmentNote.value.trim()) {
    payload.environment = environmentNote.value.trim();
  }
  if (targetHint.value.trim()) {
    payload.targets = targetHint.value.trim();
  }
  if (packages.length) {
    payload.env_packages = packages;
  }
  if (selectedValidationEnv.value) {
    payload.validation_env = selectedValidationEnv.value;
  }
  return payload;
}

async function startStream() {
  if (!prompt.value.trim()) return;
  busy.value = true;
  streamError.value = "";
  progressEvents.value = [];
  executeResult.value = null;
  const payload = {
    mode: "generate",
    prompt: prompt.value.trim(),
    context: buildContext(),
    env: selectedValidationEnv.value || undefined,
    execute: executeEnabled.value,
    max_retries: maxRetries.value,
    draft_id: draftId.value || undefined
  };
  try {
    await streamWorkflow(payload);
  } finally {
    busy.value = false;
  }
}

async function streamWorkflow(payload: Record<string, unknown>) {
  const url = `${apiBase()}/ai/workflow/stream`;
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  if (!response.ok || !response.body) {
    streamError.value = "流式连接失败";
    return;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const chunk = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      handleSSEChunk(chunk);
      boundary = buffer.indexOf("\n\n");
    }
  }
}

function handleSSEChunk(chunk: string) {
  const lines = chunk.split("\n");
  let eventName = "message";
  let data = "";
  for (const line of lines) {
    if (line.startsWith("event:")) {
      eventName = line.replace("event:", "").trim();
    } else if (line.startsWith("data:")) {
      data += line.replace("data:", "").trim();
    }
  }
  if (!data) return;
  try {
    const payload = JSON.parse(data);
    if (eventName === "status") {
      progressEvents.value = [...progressEvents.value, payload].slice(-40);
    } else if (eventName === "result") {
      applyResult(payload);
    } else if (eventName === "error") {
      streamError.value = payload.error || "生成失败";
    }
  } catch (err) {
    streamError.value = "解析流式数据失败";
  }
}

function applyResult(payload: Record<string, unknown>) {
  const nextYaml = typeof payload.yaml === "string" ? payload.yaml : "";
  if (nextYaml) {
    yaml.value = nextYaml;
  }
  if (typeof payload.summary === "string") {
    summary.value.summary = payload.summary;
  }
  summary.value.riskLevel = String(payload.risk_level || "");
  summary.value.needsReview = Boolean(payload.needs_review);
  summary.value.issues = Array.isArray(payload.issues) ? payload.issues : [];
  if (Array.isArray(payload.history)) {
    history.value = payload.history.filter((item) => typeof item === "string");
  }
  if (typeof payload.draft_id === "string") {
    draftId.value = payload.draft_id;
  }
  humanConfirmed.value = false;
  confirmReason.value = "";
  selectedStep.value = "";
  void refreshSummary();
}

async function refreshSummary() {
  if (!yaml.value.trim()) return;
  try {
    const data = await request<SummaryResponse>("/ai/workflow/summary", {
      method: "POST",
      body: { yaml: yaml.value }
    });
    summary.value = {
      summary: data.summary || "",
      steps: data.steps || 0,
      riskLevel: data.risk_level || data.riskLevel || "",
      riskNotes: data.risk_notes || data.riskNotes || [],
      issues: data.issues || [],
      needsReview: Boolean(data.needs_review ?? data.needsReview)
    };
    stepIssueIndexes.value = summary.value.issues.length ? deriveStepIssues(summary.value.issues) : [];
    if (!summary.value.needsReview) {
      humanConfirmed.value = false;
      confirmReason.value = "";
    }
  } catch (err) {
    summary.value.summary = "概览获取失败";
  }
}

async function validateDraft() {
  if (!yaml.value.trim()) return;
  validationBusy.value = true;
  try {
    const data = await request<{ ok: boolean; issues?: string[] }>("/workflows/_draft/validate", {
      method: "POST",
      body: { yaml: yaml.value }
    });
    const issues = data.issues || [];
    validation.value = { ok: data.ok, issues };
    stepIssueIndexes.value = data.ok ? [] : deriveStepIssues(issues);
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [apiErr.message ? `校验失败: ${apiErr.message}` : "校验失败，请检查服务是否启动"]
    };
    stepIssueIndexes.value = [];
  } finally {
    validationBusy.value = false;
  }
}

async function runExecution() {
  if (!yaml.value.trim()) return;
  executeBusy.value = true;
  executeResult.value = null;
  try {
    const data = await request<ExecutionResult>("/ai/workflow/execute", {
      method: "POST",
      body: { yaml: yaml.value, env: selectedValidationEnv.value || undefined }
    });
    executeResult.value = data;
  } catch (err) {
    const apiErr = err as ApiError;
    executeResult.value = {
      status: "failed",
      error: apiErr.message ? `验证失败: ${apiErr.message}` : "验证失败，请检查服务是否启动"
    };
  } finally {
    executeBusy.value = false;
  }
}

async function saveWorkflow() {
  if (requiresConfirm.value) {
    window.alert(requiresReason.value ? "需要人工确认并填写原因后才能保存" : "需要人工确认后才能保存");
    return;
  }
  const name = window.prompt("请输入工作流名称（字母/数字/短横线/下划线）");
  if (!name) return;
  const trimmed = name.trim();
  if (!/^[a-zA-Z0-9_-]+$/.test(trimmed)) {
    window.alert("名称格式不正确，仅支持字母、数字、短横线、下划线");
    return;
  }
  const reason = confirmReason.value.trim();
  busy.value = true;
  try {
    await request(`/workflows/${trimmed}`, {
      method: "PUT",
      body: { yaml: yaml.value, confirm_reason: reason || undefined }
    });
    draftId.value = "";
    confirmReason.value = "";
    await router.push({ name: "workflow", params: { name: trimmed } });
  } catch (err) {
    const apiErr = err as ApiError;
    streamError.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败，请检查服务是否启动";
  } finally {
    busy.value = false;
  }
}

function restoreHistory(index: number) {
  const snapshot = history.value[index];
  if (snapshot) {
    yaml.value = snapshot;
    humanConfirmed.value = false;
    confirmReason.value = "";
    selectedStep.value = "";
  }
}

function deriveStepIssues(issues: string[]) {
  const indexes = new Set<number>();
  for (const issue of issues) {
    const indexMatch = issue.match(/steps\[(\d+)\]/i);
    if (indexMatch) {
      const idx = Number(indexMatch[1]);
      if (!Number.isNaN(idx)) {
        indexes.add(idx);
      }
      continue;
    }
    const nameMatch = issue.match(/step name \"([^\"]+)\"/i);
    if (nameMatch) {
      const name = nameMatch[1];
      const idx = steps.value.findIndex((step) => step.name === name);
      if (idx >= 0) {
        indexes.add(idx);
      }
    }
  }
  return Array.from(indexes).sort((a, b) => a - b);
}

function buildHistoryTimeline(): HistoryEntry[] {
  if (!history.value.length) return [];
  const snapshots = [...history.value, yaml.value];
  const items = history.value.map((entry, index) => {
    const next = snapshots[index + 1] || "";
    const diff = diffSummary(entry, next);
    const label = index === 0 ? "初版" : `修复 ${index}`;
    return { index, label, diff };
  });
  return items.reverse();
}

function diffSummary(prev: string, next: string) {
  if (!prev.trim()) return "initial";
  const prevLines = prev.split(/\r?\n/);
  const nextLines = next.split(/\r?\n/);
  let added = 0;
  let removed = 0;
  const max = Math.max(prevLines.length, nextLines.length);
  for (let i = 0; i < max; i += 1) {
    if (i >= prevLines.length) {
      added += 1;
    } else if (i >= nextLines.length) {
      removed += 1;
    } else if (prevLines[i] !== nextLines[i]) {
      added += 1;
      removed += 1;
    }
  }
  return `+${added}/-${removed}`;
}

</script>

<style scoped>
.home-ai {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding-bottom: 32px;
  color: var(--ink);
}

.hero {
  position: relative;
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(240px, 0.6fr);
  gap: 20px;
  padding: 28px;
  border-radius: 24px;
  background: radial-gradient(circle at top left, #fff0e5, #f4f1ec 55%, #f9f8f6 100%);
  border: 1px solid rgba(27, 27, 27, 0.08);
  overflow: hidden;
}

.hero::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(120deg, rgba(232, 93, 42, 0.08), rgba(46, 111, 227, 0.08));
  pointer-events: none;
}

.hero-copy {
  position: relative;
  z-index: 1;
}

.eyebrow {
  display: inline-flex;
  font-size: 12px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 10px;
}

.hero h1 {
  font-family: "Space Grotesk", "Manrope", sans-serif;
  font-size: 32px;
  margin: 0 0 8px;
}

.hero p {
  margin: 0 0 18px;
  color: var(--muted);
  max-width: 520px;
}

.hero-actions {
  display: flex;
  gap: 12px;
}

.hero-card {
  position: relative;
  z-index: 1;
  background: #ffffff;
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 16px;
  display: grid;
  gap: 12px;
  box-shadow: var(--shadow);
}

.hero-stat {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: var(--muted);
}

.hero-stat .value {
  color: var(--ink);
  font-weight: 600;
}

.risk-high {
  color: var(--err);
}

.risk-medium {
  color: var(--warn);
}

.risk-low {
  color: var(--ok);
}

.composer-grid,
.result-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: minmax(320px, 1fr) minmax(320px, 0.8fr);
}

.panel {
  background: var(--panel);
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
}

.panel-head h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-family: "Space Grotesk", "Manrope", sans-serif;
}

.panel-head p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.status-tag {
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  background: #f6f2ec;
  color: var(--muted);
}

.status-tag.ok {
  color: var(--ok);
  background: rgba(42, 157, 75, 0.12);
}

.status-tag.warn {
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
}

.status-tag.error {
  color: var(--err);
  background: rgba(208, 52, 44, 0.12);
}

.status-tag.busy {
  color: var(--info);
  background: rgba(46, 111, 227, 0.12);
}

textarea,
input,
select {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 10px 12px;
  font-size: 13px;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  background: #fff;
}

textarea {
  resize: vertical;
  min-height: 120px;
}

.example-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.chip {
  border: 1px solid rgba(27, 27, 27, 0.12);
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  background: #fff;
}

.constraints {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field.toggle {
  grid-column: span 2;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.toggle-btn {
  border-radius: 999px;
  padding: 6px 16px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #f7f2ec;
  font-size: 12px;
}

.toggle-btn.on {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}

.composer-actions,
.yaml-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.progress-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.progress-item {
  display: grid;
  grid-template-columns: 120px 80px 1fr;
  gap: 10px;
  font-size: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  background: #f9f5f0;
  animation: fadeInUp 0.35s ease;
}

.progress-item .status {
  text-transform: uppercase;
  font-size: 11px;
}

.progress-item .status.error {
  color: var(--err);
}

.progress-item .status.done {
  color: var(--ok);
}

.step-card {
  border-radius: 14px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 12px;
  background: #fff;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.step-card.active {
  border-color: rgba(46, 111, 227, 0.4);
  box-shadow: 0 0 0 2px rgba(46, 111, 227, 0.15);
}

.step-card.error {
  border-color: rgba(208, 52, 44, 0.45);
  background: rgba(208, 52, 44, 0.06);
}

.step-card:focus-visible {
  outline: 2px solid rgba(46, 111, 227, 0.4);
  outline-offset: 2px;
}

.step-name {
  font-weight: 600;
}

.step-meta,
.step-targets {
  font-size: 12px;
  color: var(--muted);
}

.history {
  border-top: 1px dashed rgba(27, 27, 27, 0.12);
  padding-top: 12px;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-item {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: #fff;
  padding: 8px 12px;
  font-size: 12px;
  text-align: left;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.history-title {
  font-weight: 600;
  margin-bottom: 2px;
}

.history-diff {
  font-size: 11px;
  color: var(--muted);
}

.history-restore {
  font-size: 11px;
  color: var(--info);
}

.alert {
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 12px;
  background: rgba(46, 111, 227, 0.08);
}

.alert.warn {
  background: rgba(230, 167, 0, 0.12);
}

.alert.ok {
  background: rgba(42, 157, 75, 0.12);
}

.alert.error {
  background: rgba(208, 52, 44, 0.12);
  color: var(--err);
}

.issues {
  margin: 0;
  padding-left: 18px;
  font-size: 12px;
  color: var(--err);
}

.summary {
  display: grid;
  gap: 8px;
  background: #f9f5f0;
  border-radius: 12px;
  padding: 10px 12px;
  font-size: 12px;
}

.human-gate {
  display: grid;
  gap: 10px;
  padding: 12px;
  border-radius: 12px;
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
  font-size: 12px;
}

.gate-reason {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--muted);
}

.gate-reason input {
  background: #fff;
}

.gate-actions {
  display: flex;
  justify-content: flex-end;
}

.gate-copy {
  color: var(--warn);
}

.summary-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.execution-result {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 12px;
  font-size: 12px;
}

.execution-result.failed {
  border-color: rgba(208, 52, 44, 0.4);
}

.result-title {
  font-weight: 600;
  margin-bottom: 8px;
}

.result-io pre {
  margin: 0;
  font-size: 11px;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 980px) {
  .hero,
  .composer-grid,
  .result-grid {
    grid-template-columns: 1fr;
  }

  .constraints {
    grid-template-columns: 1fr;
  }

  .progress-item {
    grid-template-columns: 1fr;
  }
}
</style>
