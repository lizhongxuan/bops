<template>
  <aside :class="['chat-drawer', collapsed ? 'collapsed' : 'expanded']">
    <div class="drawer-handle" @click="toggle">
      <span class="handle-icon">💬</span>
    </div>
    <div v-if="!collapsed" class="drawer-body">
      <div class="drawer-status-row">
        <div class="status-tag" :class="statusTag.cls">{{ statusTag.text }}</div>
        <button class="icon-btn" type="button" @click="toggle" aria-label="折叠">›</button>
        <button class="icon-btn" type="button" @click="toggleRunModal" aria-label="运行状态">📈</button>
      </div>

      <div class="chat-body">
        <ul class="timeline">
          <li v-for="entry in timeline" :key="entry.id" :class="['timeline-item', entry.type]">
            <div class="timeline-header">
              <span class="timeline-badge" :class="entry.type">{{ entry.label }}</span>
              <small>{{ entry.time }}</small>
            </div>
            <p>{{ entry.body }}</p>
          </li>
          <li v-if="!timeline.length" class="timeline-item muted">
            <p>这里会显示对话、生成与修复进度。</p>
          </li>
        </ul>
      </div>

      <div class="composer">
        <textarea
          v-model="prompt"
          class="drawer-input"
          placeholder="描述需求，例如：在 web1/web2 上安装 nginx，渲染配置并启动服务"
          rows="4"
        ></textarea>
        <div class="drawer-actions">
          <button class="btn btn-sm primary" type="button" :disabled="busy" @click="emitGenerate">
            生成流程
          </button>
          <button class="btn btn-sm ghost" type="button" :disabled="busy" @click="emitFix">
            修复错误
          </button>
          <button
            class="btn btn-sm ghost"
            type="button"
            :disabled="busy || !selectedNode"
            @click="emitRegenerate"
          >
            重生成节点
          </button>
        </div>
      </div>

    </div>
    <teleport to="body">
      <div v-if="runModalOpen" class="run-modal-backdrop" @click="toggleRunModal">
        <div class="run-modal" @click.stop>
          <div class="run-modal-head">
            <span>运行状态与日志</span>
            <button class="icon-btn" type="button" @click="toggleRunModal" aria-label="关闭">✕</button>
          </div>
          <div class="run-panel">
            <div class="run-head">
              <span>运行状态</span>
              <span class="run-status">{{ runStatus || "idle" }}</span>
            </div>
            <div v-if="runSummary" class="run-summary">
              <div class="summary-row">
                <span>状态</span>
                <strong>{{ runSummary.status || "finished" }}</strong>
              </div>
              <div class="summary-row">
                <span>步骤</span>
                <span>{{ runSummary.successSteps }}/{{ runSummary.totalSteps }} 成功</span>
              </div>
              <div class="summary-row">
                <span>失败</span>
                <span>{{ runSummary.failedSteps }}</span>
              </div>
              <div class="summary-row">
                <span>耗时</span>
                <span>{{ formatDuration(runSummary.durationMs) }}</span>
              </div>
              <div v-if="runSummary.issues.length" class="summary-issues">
                <div class="summary-label">问题列表</div>
                <ul>
                  <li v-for="(issue, idx) in runSummary.issues" :key="idx">{{ issue }}</li>
                </ul>
              </div>
              <div v-else-if="runSummary.message" class="summary-issues">
                <div class="summary-label">信息</div>
                <div class="summary-message">{{ runSummary.message }}</div>
              </div>
            </div>
            <div class="run-logs">
              <div v-if="!runLogs.length" class="muted">暂无日志</div>
              <div v-for="(line, idx) in runLogs" :key="idx" class="log-line">{{ line }}</div>
            </div>
          </div>
        </div>
      </div>
    </teleport>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

type RunSummary = {
  status: string;
  totalSteps: number;
  successSteps: number;
  failedSteps: number;
  durationMs: number;
  issues: string[];
  message?: string;
};

const props = defineProps<{
  selectedNode?: { id: string; name: string } | null;
  status?: string;
  error?: string;
  busy?: boolean;
  runStatus?: string;
  runSummary?: RunSummary | null;
  runLogs?: string[];
}>();
const emit = defineEmits<{
  (event: "generate", prompt: string): void;
  (event: "fix"): void;
  (event: "regenerate", prompt: string): void;
}>();

const collapsed = ref(false);
const storageKey = "bops-workbench-chat-drawer";
const prompt = ref("");

const selectedNode = computed(() => props.selectedNode || null);
const status = computed(() => props.status || "");
const error = computed(() => props.error || "");
const busy = computed(() => Boolean(props.busy));
const runStatus = computed(() => props.runStatus || "");
const runSummary = computed(() => props.runSummary || null);
const runLogs = computed(() => props.runLogs || []);
const runModalOpen = ref(false);

type TimelineEntry = {
  id: string;
  type: "user" | "assistant" | "error";
  label: string;
  body: string;
  time: string;
};

const timeline = ref<TimelineEntry[]>([]);
let timelineIndex = 0;
let lastStatus = "";
let lastError = "";

function toggle() {
  collapsed.value = !collapsed.value;
}

function loadState() {
  try {
    const raw = localStorage.getItem(storageKey);
    if (raw !== null) {
      collapsed.value = raw === "collapsed";
    }
  } catch {
    // ignore
  }
}

function persistState() {
  try {
    localStorage.setItem(storageKey, collapsed.value ? "collapsed" : "expanded");
  } catch {
    // ignore
  }
}

function emitGenerate() {
  const text = prompt.value.trim();
  if (text) {
    pushEntry("user", "我想要：" + text);
  }
  emit("generate", text);
}

function emitFix() {
  pushEntry("user", "请修复当前流程的问题。");
  emit("fix");
}

function emitRegenerate() {
  const text = prompt.value.trim();
  pushEntry("user", text ? `重生成节点：${text}` : "重生成当前节点");
  emit("regenerate", text);
}

function toggleRunModal() {
  runModalOpen.value = !runModalOpen.value;
}

onMounted(loadState);
watch(collapsed, persistState);
watch(status, (value) => {
  if (!value || value === lastStatus) return;
  lastStatus = value;
  pushEntry("assistant", value);
});
watch(error, (value) => {
  if (!value || value === lastError) return;
  lastError = value;
  pushEntry("error", value);
});

const statusTag = computed(() => {
  if (error.value) return { text: "异常", cls: "error" };
  if (busy.value) return { text: "生成中", cls: "busy" };
  if (status.value) return { text: "更新", cls: "busy" };
  return { text: "就绪", cls: "idle" };
});

function pushEntry(type: TimelineEntry["type"], body: string) {
  const now = new Date();
  const time = now.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  timeline.value = [
    ...timeline.value,
    {
      id: `entry-${timelineIndex++}`,
      type,
      label: type === "user" ? "你" : type === "error" ? "异常" : "AI",
      body,
      time
    }
  ];
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
</script>

<style scoped>
.chat-drawer {
  position: fixed;
  right: 0;
  top: 0;
  background: var(--panel);
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  box-shadow: var(--shadow);
  border: 1px solid #e3ded7;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
  transition: width 0.2s ease;
  z-index: 120;
}

.chat-drawer.collapsed {
  width: 32px;
  height: 32px;
  border-radius: 999px;
  align-items: center;
  box-shadow: 0 12px 22px rgba(27, 27, 27, 0.16);
  top: 72px;
  right: 20px;
}

.chat-drawer.collapsed .drawer-handle {
  flex: 1;
  padding: 0;
  border-bottom: none;
  border-radius: 999px;
}

.chat-drawer.expanded {
  width: 520px;
  height: 100%;
  top: 0;
  right: 0;
}

.drawer-handle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 12px 10px;
  cursor: pointer;
  background: #f6f3ef;
  border-bottom: 1px solid #e3ded7;
}

.chat-drawer.expanded .drawer-handle {
  display: none;
}

.handle-icon {
  font-size: 16px;
}

.drawer-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  flex: 1;
}

.drawer-status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: flex-end;
}

.status-tag {
  font-size: 11px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  color: var(--muted);
  background: #f6f3ef;
  white-space: nowrap;
}

.status-tag.busy {
  color: var(--ink);
  border-color: rgba(232, 93, 42, 0.3);
  background: rgba(232, 93, 42, 0.12);
}

.status-tag.error {
  color: var(--err);
  border-color: rgba(208, 52, 44, 0.3);
  background: rgba(208, 52, 44, 0.08);
}

.status-tag.idle {
  color: var(--muted);
}

.icon-btn {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid #e3ded7;
  background: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 12px;
}

.icon-btn:hover {
  box-shadow: 0 6px 12px rgba(27, 27, 27, 0.12);
}

.drawer-input {
  border: 1px solid #e3ded7;
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  font-size: 13px;
  resize: vertical;
}

.chat-body {
  flex: 1;
  min-height: 0;
  border: 1px solid #eee6de;
  border-radius: var(--radius-md);
  background: #faf8f4;
  padding: 12px;
  overflow: auto;
}

.timeline {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 12px;
}

.timeline-item {
  background: #fff;
  border: 1px solid #e9e0d6;
  border-radius: var(--radius-md);
  padding: 10px 12px;
  font-size: 12px;
  color: var(--ink);
  box-shadow: 0 8px 18px rgba(27, 27, 27, 0.06);
}

.timeline-item.muted {
  background: transparent;
  border: 1px dashed #e1d8cd;
  color: var(--muted);
}

.muted {
  font-size: 12px;
  color: var(--muted);
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  color: var(--muted);
  font-size: 11px;
}

.timeline-badge {
  padding: 4px 8px;
  border-radius: 999px;
  background: #f2ede7;
  border: 1px solid #e5ddd3;
  font-size: 11px;
  color: var(--muted);
}

.timeline-badge.user {
  color: var(--ink);
  background: rgba(46, 111, 227, 0.1);
  border-color: rgba(46, 111, 227, 0.2);
}

.timeline-badge.assistant {
  color: var(--ink);
  background: rgba(232, 93, 42, 0.12);
  border-color: rgba(232, 93, 42, 0.2);
}

.timeline-badge.error {
  color: var(--err);
  background: rgba(208, 52, 44, 0.1);
  border-color: rgba(208, 52, 44, 0.2);
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.drawer-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.btn {
  border: 1px solid #e3ded7;
  background: #fff;
  color: var(--ink);
  padding: 8px 12px;
  font-size: 12px;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 16px rgba(27, 27, 27, 0.12);
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  box-shadow: 0 12px 20px rgba(232, 93, 42, 0.2);
}

.btn.ghost {
  background: #f6f3ef;
  border-color: #e5ddd3;
  color: var(--muted);
  box-shadow: none;
}

.drawer-actions .btn {
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 11px;
}

.run-panel {
  background: #f9f6f2;
  border-radius: var(--radius-lg);
  border: 1px solid #eee4db;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 240px;
  overflow: hidden;
}

.run-head {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
}

.run-status {
  color: var(--ink);
}

.run-summary {
  border: 1px solid var(--grid);
  border-radius: var(--radius-md);
  padding: 8px 10px;
  background: #fff;
  font-size: 11px;
  color: var(--muted);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.run-summary strong {
  color: var(--ink);
  font-weight: 600;
}

.summary-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.summary-issues ul {
  margin: 4px 0 0;
  padding-left: 14px;
}

.summary-issues li {
  line-height: 1.4;
}

.summary-label {
  font-size: 11px;
  color: var(--muted);
}

.summary-message {
  color: var(--ink);
}

.run-logs {
  max-height: 160px;
  overflow: auto;
  font-size: 11px;
  color: var(--muted);
}

.log-line {
  padding: 2px 0;
}

.run-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 15, 15, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}

.run-modal {
  width: 360px;
  max-width: 90vw;
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid #e7dfd6;
  box-shadow: 0 24px 40px rgba(27, 27, 27, 0.2);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.run-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: var(--ink);
}

@media (max-width: 1200px) {
  .chat-drawer.expanded {
    width: 420px;
  }
}

@media (max-width: 980px) {
  .chat-drawer {
    position: fixed;
    top: 0;
    height: 100vh;
  }
}
</style>
