<template>
  <section class="home-ai">
    <div class="main-grid">
      <section class="panel chat-panel">
        <div class="panel-head chat-head">
          <div>
            <h2>工作流AI助手</h2>
            <p>AI 负责拆解需求并生成草稿，你只需确认关键细节。</p>
          </div>
          <div class="status-tag" :class="streamError ? 'error' : busy ? 'busy' : 'idle'">
            {{ streamError ? '异常' : busy ? '生成中' : '就绪' }}
          </div>
        </div>

        <div class="draft-stats">
          <div class="draft-stat">
            <span>草稿</span>
            <strong>{{ draftStatus }}</strong>
          </div>
          <div class="draft-stat">
            <span>风险</span>
            <strong :class="`risk-${summary.riskLevel || 'low'}`">{{ summary.riskLevel || 'low' }}</strong>
          </div>
          <div class="draft-stat">
            <span>步骤</span>
            <strong>{{ steps.length }}</strong>
          </div>
          <div class="draft-stat">
            <span>确认</span>
            <strong>{{ confirmStatus }}</strong>
          </div>
        </div>

        <div class="chat-body">
          <ul class="timeline">
            <li v-for="entry in timelineEntries" :key="entry.id" class="timeline-item">
              <div class="timeline-header">
                <span class="timeline-badge" :class="entry.type">{{ entry.label }}</span>
                <small v-if="entry.extra">{{ entry.extra }}</small>
              </div>
              <p>{{ entry.body }}</p>
            </li>
          </ul>
        </div>

        <div class="composer">
          <div class="chat-toolbar">
            <button class="btn ghost btn-sm" type="button" @click="showConfigModal = true">
              选择目标主机/分组
            </button>
            <button
              class="btn primary btn-sm"
              type="button"
              :disabled="busy || !prompt.trim()"
              @click="startStream"
            >
              生成草稿
            </button>
            <button class="btn btn-sm" type="button" :disabled="busy || !yaml.trim()" @click="validateDraft">
              校验
            </button>
            <button class="btn btn-sm" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
              沙箱验证
            </button>
          </div>
          <textarea
            v-model="prompt"
            placeholder="描述需求，例如：在 web1/web2 上安装 nginx，渲染配置并启动服务"
            rows="4"
          ></textarea>
          <div v-if="showExamples" class="example-row">
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
          <div class="composer-footer">
            <button
              class="btn primary btn-sm"
              type="button"
              :disabled="busy || !prompt.trim()"
              @click="startStream"
            >
              发送
            </button>
            <button class="btn ghost btn-sm" type="button" @click="toggleExamples">示例</button>
            <button class="btn ghost btn-sm" type="button" :disabled="busy" @click="clearPrompt">
              清空
            </button>
          </div>
        </div>
      </section>

      <section class="panel workspace-panel">
        <div class="panel-head workspace-head">
          <div class="workspace-title">
            <h2>{{ draftTitle }}</h2>
            <p>通过聊天不断完善细节</p>
            <div class="workspace-tags">
              <span class="chip subtle">{{ planMode }}</span>
              <span class="chip" :class="`risk-${summary.riskLevel || 'low'}`">
                {{ summary.riskLevel || 'low' }}
              </span>
            </div>
          </div>
          <div class="panel-actions">
            <button class="btn ghost btn-sm" type="button" @click="showSummaryModal = true">
              需求摘要
            </button>
            <button
              class="btn ghost btn-sm"
              type="button"
              :disabled="!historyTimeline.length"
              @click="showHistoryModal = true"
            >
              草稿历史
            </button>
            <div class="status-tag" :class="validation.ok ? 'ok' : 'warn'">
              {{ validation.ok ? '校验通过' : '待修复' }}
            </div>
          </div>
        </div>

        <div class="workspace-tabs">
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'visual' }"
            @click="workspaceTab = 'visual'"
          >
            可视化
          </button>
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'yaml' }"
            @click="workspaceTab = 'yaml'"
          >
            YAML
          </button>
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'validate' }"
            @click="workspaceTab = 'validate'"
          >
            校验与执行
          </button>
        </div>

        <div v-if="workspaceTab === 'visual'" class="tab-panel">
          <div class="steps-section">
            <div class="steps-head">
              <h3>步骤构建器</h3>
              <div class="steps-head-actions">
                <span class="step-count">{{ steps.length }} 步</span>
                <button class="btn secondary btn-sm" type="button" @click="appendStep">
                  新增步骤
                </button>
              </div>
            </div>
            <div v-if="steps.length" class="steps-list">
              <button
                class="step-card"
                v-for="(step, index) in steps"
                :key="step.name || `step-${index}`"
                :class="{ active: selectedStep === step.name, error: stepIssueIndexes.includes(index) }"
                type="button"
                @click="focusStep(step)"
              >
                <div class="step-name">{{ step.name }}</div>
                <div class="step-meta">{{ step.action || '未指定动作' }}</div>
                <div class="step-targets" v-if="step.targets">目标: {{ step.targets }}</div>
              </button>
            </div>
            <div v-else class="empty">尚未解析到步骤，生成草稿获取可视化内容。</div>
          </div>
        </div>

        <div v-else-if="workspaceTab === 'yaml'" class="tab-panel">
          <textarea ref="yamlRef" v-model="yaml" spellcheck="false" class="code" rows="20"></textarea>
          <div class="yaml-actions">
            <button class="btn" type="button" :disabled="validationBusy || !yaml.trim()" @click="validateDraft">
              校验
            </button>
            <button class="btn" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
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
        </div>

        <div v-else class="tab-panel validation-panel">
          <div class="validation-actions">
            <button class="btn" type="button" :disabled="validationBusy || !yaml.trim()" @click="validateDraft">
              校验
            </button>
            <button class="btn" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
              沙箱验证
            </button>
          </div>
          <div class="alert" :class="validation.ok ? 'ok' : 'warn'">
            {{ validation.ok ? '校验通过' : '校验未通过' }}
          </div>
          <ul class="issues" v-if="validation.issues.length">
            <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
          </ul>

          <div class="human-gate" v-if="summary.needsReview">
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
                {{ humanConfirmed ? '已确认' : '人工确认' }}
              </button>
            </div>
          </div>

          <div class="progress-list compact">
            <div v-if="progressEvents.length === 0" class="empty">等待生成…</div>
            <div
              class="progress-item"
              v-else
              v-for="(evt, index) in progressEvents"
              :key="`${evt.node}-${index}`"
            >
              <div class="node">{{ formatNode(evt.node) }}</div>
              <div class="status" :class="evt.status">{{ evt.status }}</div>
              <div class="message" v-if="evt.message">{{ evt.message }}</div>
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
      </section>
    </div>
    <div v-if="showSummaryModal" class="modal-backdrop" @click.self="showSummaryModal = false">
      <div class="summary-modal">
        <div class="modal-head">
          <h3>需求摘要</h3>
          <button class="modal-close" type="button" @click="showSummaryModal = false">&#10005;</button>
        </div>
        <p class="modal-summary">{{ summary.summary || 'AI 还在构建草稿...' }}</p>
        <div class="modal-grid">
          <div v-if="environmentNote" class="modal-row">
            <span>目标环境</span>
            <strong>{{ environmentNote }}</strong>
          </div>
          <div v-if="targetHint" class="modal-row">
            <span>目标主机/分组</span>
            <strong>{{ targetHint }}</strong>
          </div>
          <div class="modal-row">
            <span>执行策略</span>
            <strong>{{ planMode }}</strong>
          </div>
          <div class="modal-row">
            <span>验证环境</span>
            <strong>{{ selectedValidationEnv || '默认' }}</strong>
          </div>
          <div class="modal-row">
            <span>重试次数</span>
            <strong>{{ maxRetries }}</strong>
          </div>
        </div>
        <div v-if="summary.issues.length" class="modal-issues">
          <span class="chip secondary" v-for="issue in summary.issues" :key="issue">
            {{ issue }}
          </span>
        </div>
        <button class="btn primary" type="button" @click="showSummaryModal = false">知道了</button>
      </div>
    </div>
    <div v-if="showConfigModal" class="modal-backdrop" @click.self="showConfigModal = false">
      <div class="config-modal">
        <div class="modal-head">
          <h3>目标与执行参数</h3>
          <button class="modal-close" type="button" @click="showConfigModal = false">&#10005;</button>
        </div>
        <div class="modal-grid form-grid">
          <div class="form-field">
            <label>目标主机/分组</label>
            <input v-model="targetHint" type="text" placeholder="例如 web, db" />
          </div>
          <div class="form-field">
            <label>目标环境</label>
            <input v-model="environmentNote" type="text" placeholder="例如 Ubuntu 22.04 / macOS M1" />
          </div>
          <div class="form-field">
            <label>执行策略</label>
            <select v-model="planMode">
              <option value="manual-approve">manual-approve</option>
              <option value="auto">auto</option>
            </select>
          </div>
          <div class="form-field">
            <label>环境变量包</label>
            <input v-model="envPackages" type="text" placeholder="prod-env, staging" />
          </div>
          <div class="form-field">
            <label>最大修复次数</label>
            <input v-model.number="maxRetries" type="number" min="0" max="5" />
          </div>
          <div class="form-field">
            <label>验证环境</label>
            <select v-model="selectedValidationEnv">
              <option value="">默认</option>
              <option v-for="env in validationEnvs" :key="env.name" :value="env.name">
                {{ env.name }}
              </option>
            </select>
          </div>
        </div>
        <div class="toggle-row">
          <span>自动执行验证</span>
          <button
            class="toggle-btn"
            type="button"
            :class="executeEnabled ? 'on' : 'off'"
            @click="executeEnabled = !executeEnabled"
          >
            {{ executeEnabled ? '启用' : '关闭' }}
          </button>
        </div>
        <div class="modal-actions">
          <button class="btn primary btn-sm" type="button" @click="showConfigModal = false">完成</button>
        </div>
      </div>
    </div>
    <div v-if="showHistoryModal" class="modal-backdrop" @click.self="showHistoryModal = false">
      <div class="history-modal">
        <div class="modal-head">
          <h3>草稿历史</h3>
          <button class="modal-close" type="button" @click="showHistoryModal = false">&#10005;</button>
        </div>
        <div v-if="historyTimeline.length" class="history-list">
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
        <div v-else class="empty">暂无草稿历史</div>
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

type ChatEntry = {
  id: string;
  label: string;
  body: string;
  type: string;
  extra?: string;
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
const chatEntries = ref<ChatEntry[]>([
  {
    id: "welcome",
    label: "AI",
    body: "你好！告诉我你的需求，我会拆解成可执行工作流，并主动追问缺失细节。",
    type: "ai"
  }
]);
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

const showExamples = ref(false);
const showConfigModal = ref(false);
const showSummaryModal = ref(false);
const showHistoryModal = ref(false);

const workspaceTab = ref<"visual" | "yaml" | "validate">("visual");
const steps = computed<StepSummary[]>(() => parseSteps(yaml.value));
const timelineEntries = computed(() => {
  return chatEntries.value;
});
const requiresReason = computed(() => summary.value.riskLevel === "high");
const requiresConfirm = computed(() => {
  if (!summary.value.needsReview) return false;
  if (!humanConfirmed.value) return true;
  if (requiresReason.value && !confirmReason.value.trim()) return true;
  return false;
});
const historyTimeline = computed<HistoryEntry[]>(() => buildHistoryTimeline());
const draftTitle = computed(() => {
  if (draftId.value) return `ai-${draftId.value.slice(0, 6)}`;
  return "ai-draft";
});
const draftStatus = computed(() => {
  if (draftId.value) return "已保存";
  if (yaml.value.trim()) return "未保存";
  return "未生成";
});
const confirmStatus = computed(() => (validation.value.ok ? "正常" : "需处理"));

let chatIndex = 0;
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

function pushChatEntry(entry: Omit<ChatEntry, "id">) {
  const id = `chat-${chatIndex++}`;
  chatEntries.value = [...chatEntries.value, { id, ...entry }];
}

function applyExample(text: string) {
  prompt.value = text;
  showExamples.value = false;
}

function toggleExamples() {
  showExamples.value = !showExamples.value;
}

function clearPrompt() {
  prompt.value = "";
  showExamples.value = false;
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

function appendStep() {
  const baseName = "新建步骤";
  const existingNames = steps.value.map((step) => step.name).filter(Boolean);
  let suffix = 1;
  let stepName = baseName;
  while (existingNames.includes(stepName)) {
    suffix += 1;
    stepName = `${baseName} ${suffix}`;
  }
  const baseLines = [
    `- name: ${stepName}`,
    "  action: cmd.run",
    "  targets: []",
    "  cmd: echo '待补充'"
  ];
  const trimmed = yaml.value.trim();
  if (!trimmed) {
    const indented = baseLines.map((line) => `  ${line}`).join("\n");
    yaml.value = `steps:\n${indented}`;
    return;
  }
  const lines = yaml.value.split(/\r?\n/);
  const stepsIndex = lines.findIndex((line) => /^\s*steps\s*:/.test(line));
  if (stepsIndex < 0) {
    const indented = baseLines.map((line) => `  ${line}`).join("\n");
    yaml.value = `${trimmed}\n\nsteps:\n${indented}`;
    return;
  }
  const stepsIndent = lines[stepsIndex].match(/^(\s*)/)[1].length;
  if (/^\s*steps\s*:\s*\[\s*\]\s*$/.test(lines[stepsIndex])) {
    const prefix = lines[stepsIndex].match(/^(\s*)/)[1];
    lines[stepsIndex] = `${prefix}steps:`;
  }
  const stepIndent = " ".repeat(stepsIndent + 2);
  const stepLines = baseLines.map((line) => `${stepIndent}${line}`);
  let insertAt = lines.length;
  for (let i = stepsIndex + 1; i < lines.length; i += 1) {
    const line = lines[i];
    if (line.trim() === "") {
      continue;
    }
    const indent = line.match(/^(\s*)/)[1].length;
    if (indent <= stepsIndent) {
      insertAt = i;
      break;
    }
  }
  lines.splice(insertAt, 0, ...stepLines);
  yaml.value = lines.join("\n");
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
  pushChatEntry({ label: "用户", body: prompt.value.trim(), type: "user" });
  showExamples.value = false;
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
    pushChatEntry({
      label: "系统",
      body: streamError.value,
      type: "error",
      extra: "ERROR"
    });
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
      const evt = payload as ProgressEvent;
      if (evt.status === "error" && evt.message) {
        pushChatEntry({
          label: formatNode(evt.node || "AI"),
          body: evt.message,
          type: "error",
          extra: "ERROR"
        });
      }
    } else if (eventName === "result") {
      applyResult(payload);
    } else if (eventName === "error") {
      streamError.value = payload.error || "生成失败";
      pushChatEntry({
        label: "系统",
        body: streamError.value,
        type: "error",
        extra: "ERROR"
      });
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
  const summaryText = typeof payload.summary === "string" && payload.summary.trim()
    ? payload.summary.trim()
    : "草稿已生成";
  const riskText = payload.risk_level ? `风险 ${payload.risk_level}` : "";
  const issueCount = Array.isArray(payload.issues) ? payload.issues.length : 0;
  const issueText = issueCount ? `问题 ${issueCount}` : "";
  const resultBody = [summaryText, riskText, issueText].filter(Boolean).join(" · ");
  pushChatEntry({
    label: "AI",
    body: resultBody || "草稿已生成",
    type: issueCount ? "warn" : "ai",
    extra: "DONE"
  });
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
    const issueText = issues.length ? issues.slice(0, 2).join(" · ") : "未发现问题";
    pushChatEntry({
      label: "校验",
      body: data.ok ? `校验通过：${issueText}` : `校验失败：${issueText}`,
      type: data.ok ? "ai" : "warn",
      extra: data.ok ? "OK" : "WARN"
    });
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [apiErr.message ? `校验失败: ${apiErr.message}` : "校验失败，请检查服务是否启动"]
    };
    stepIssueIndexes.value = [];
    pushChatEntry({
      label: "校验",
      body: validation.value.issues[0],
      type: "error",
      extra: "ERROR"
    });
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
    const codeText = typeof data.code === "number" ? ` (code ${data.code})` : "";
    const isSuccess = data.status === "success";
    pushChatEntry({
      label: "执行",
      body: `沙箱验证完成：${data.status}${codeText}`,
      type: isSuccess ? "ai" : "warn",
      extra: data.status?.toUpperCase()
    });
  } catch (err) {
    const apiErr = err as ApiError;
    executeResult.value = {
      status: "failed",
      error: apiErr.message ? `验证失败: ${apiErr.message}` : "验证失败，请检查服务是否启动"
    };
    pushChatEntry({
      label: "执行",
      body: executeResult.value.error || "验证失败",
      type: "error",
      extra: "ERROR"
    });
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
  padding: 24px;
  color: var(--ink);
  flex: 1;
  min-height: 0;
}

.main-grid {
  display: grid;
  grid-template-columns: minmax(360px, 1.25fr) minmax(320px, 0.95fr);
  gap: 20px;
  flex: 1;
  min-height: 0;
  grid-template-rows: minmax(0, 1fr);
}

.panel {
  background: var(--panel);
  border-radius: 20px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.panel-head h2 {
  margin: 0;
  font-size: 20px;
  font-family: "Space Grotesk", "Manrope", sans-serif;
}

.panel-head p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.chat-head {
  align-items: flex-start;
}

.panel-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.draft-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(27, 27, 27, 0.06);
  background: rgba(255, 255, 255, 0.65);
}

.draft-stat {
  display: flex;
  justify-content: space-between;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.draft-stat strong {
  color: var(--ink);
  font-weight: 600;
}

.risk-low {
  color: var(--ok);
}

.risk-medium {
  color: var(--warn);
}

.risk-high {
  color: var(--err);
}

.status-tag {
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--muted);
  background: #f6f2ec;
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

.chat-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.chat-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.timeline {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.timeline-item {
  padding: 12px 14px;
  background: rgba(255, 255, 255, 0.4);
  border-radius: 14px;
  border: 1px solid rgba(27, 27, 27, 0.08);
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.timeline-badge {
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.timeline-badge.user {
  background: rgba(46, 111, 227, 0.12);
  color: var(--info);
}

.timeline-badge.ai {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}

.timeline-badge.warn {
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
}

.timeline-badge.error {
  background: rgba(208, 52, 44, 0.12);
  color: var(--err);
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.chat-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
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
  width: 100%;
  box-sizing: border-box;
}

textarea {
  resize: vertical;
  min-height: 90px;
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

.chip.subtle {
  background: #f3eee7;
  color: var(--muted);
  border-color: rgba(27, 27, 27, 0.1);
}

.chip.secondary {
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
  border-color: rgba(230, 167, 0, 0.3);
}

.composer-footer {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.btn {
  border: 1px solid rgba(27, 27, 27, 0.16);
  background: #fff;
  border-radius: 10px;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  box-shadow: none;
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  box-shadow: 0 12px 22px rgba(232, 93, 42, 0.24);
}

.btn.secondary {
  background: #f7f2ec;
  border-color: rgba(27, 27, 27, 0.12);
  color: var(--ink);
}

.btn.ghost {
  background: transparent;
  color: var(--muted);
}

.btn.btn-sm {
  padding: 5px 10px;
  font-size: 12px;
}

.workspace-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.workspace-head {
  align-items: flex-start;
}

.workspace-title h2 {
  margin: 0;
}

.workspace-title p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.workspace-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.workspace-tabs {
  display: flex;
  gap: 10px;
}

.tab {
  flex: 1;
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: rgba(255, 255, 255, 0.75);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.tab.active {
  background: #fff;
  border-color: rgba(46, 111, 227, 0.35);
  box-shadow: 0 1px 6px rgba(46, 111, 227, 0.18);
}

.tab-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.requirement-card {
  background: rgba(255, 255, 255, 0.7);
  border-radius: 16px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.card-head h3 {
  margin: 0;
  font-size: 18px;
}

.card-head p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}

.card-grid {
  display: grid;
  gap: 10px;
}

.card-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
}

.card-row strong {
  color: var(--ink);
}

.chip-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.steps-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.steps-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.step-count {
  font-size: 12px;
  color: var(--muted);
}

.steps-list {
  display: grid;
  gap: 10px;
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

.code {
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  min-height: 200px;
}

.yaml-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}


.validation-panel .validation-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
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

.issues {
  margin: 0;
  padding-left: 18px;
  font-size: 12px;
  color: var(--err);
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

.progress-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.progress-item {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: #fff;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-item .node {
  font-weight: 600;
}

.progress-item .status {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--muted);
}

.progress-item .status.error {
  color: var(--err);
}

.progress-item .status.done {
  color: var(--ok);
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
  font-size: 12px;
  background: #f4f4f4;
  border-radius: 8px;
  padding: 8px;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(24, 24, 24, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 20;
}

.summary-modal,
.config-modal,
.history-modal {
  width: min(560px, 100%);
  background: #fff;
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: 0 24px 40px rgba(27, 27, 27, 0.18);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.modal-head h3 {
  margin: 0;
  font-size: 18px;
}

.modal-close {
  border: none;
  background: transparent;
  font-size: 18px;
  cursor: pointer;
  color: var(--muted);
}

.modal-summary {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}

.modal-grid {
  display: grid;
  gap: 10px;
}

.modal-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.modal-row strong {
  color: var(--ink);
}

.modal-issues {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.form-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
  padding: 8px 10px;
  border-radius: 12px;
  border: 1px dashed rgba(27, 27, 27, 0.16);
  background: rgba(250, 246, 240, 0.6);
}

.toggle-btn {
  border-radius: 999px;
  padding: 6px 14px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #f7f2ec;
  font-size: 12px;
}

.toggle-btn.on {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 980px) {
  .main-grid {
    grid-template-columns: 1fr;
    grid-template-rows: auto;
  }

  .draft-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
