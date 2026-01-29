<template>
  <aside :class="['chat-drawer', collapsed ? 'collapsed' : 'expanded']">
    <div class="drawer-handle" @click="toggle">
      <span class="handle-icon">💬</span>
      <span v-if="collapsed" class="handle-label">AI</span>
    </div>
    <div v-if="!collapsed" class="drawer-body">
      <header class="drawer-head">
        <h3>AI 助手</h3>
        <button class="btn ghost btn-sm" type="button" @click="toggle">折叠</button>
      </header>
      <textarea
        v-model="prompt"
        class="drawer-input"
        placeholder="输入需求或节点优化意图"
        rows="3"
      ></textarea>
      <div class="drawer-actions">
        <button class="btn btn-sm" type="button" :disabled="busy" @click="emitGenerate">生成流程</button>
        <button
          class="btn btn-sm ghost"
          type="button"
          :disabled="busy || !selectedNode"
          @click="emitRegenerate"
        >
          重生成节点
        </button>
        <button class="btn btn-sm ghost" type="button" :disabled="busy" @click="emitFix">修复错误</button>
      </div>
      <div class="drawer-content">
        <p v-if="status" class="status">{{ status }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <p v-if="!status && !error" class="muted">这里将展示对话与进度信息。</p>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

const props = defineProps<{
  selectedNode?: { id: string; name: string } | null;
  status?: string;
  error?: string;
  busy?: boolean;
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
  emit("generate", prompt.value.trim());
}

function emitFix() {
  emit("fix");
}

function emitRegenerate() {
  emit("regenerate", prompt.value.trim());
}

onMounted(loadState);
watch(collapsed, persistState);
</script>

<style scoped>
.chat-drawer {
  position: relative;
  background: var(--panel);
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  box-shadow: var(--shadow);
  border: 1px solid #e3ded7;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
}

.chat-drawer.collapsed {
  width: 64px;
  align-items: center;
}

.chat-drawer.expanded {
  width: 320px;
}

.drawer-handle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px 12px;
  cursor: pointer;
  background: #f6f3ef;
  border-bottom: 1px solid #e3ded7;
}

.handle-icon {
  font-size: 16px;
}

.handle-label {
  font-size: 12px;
  color: var(--muted);
}

.drawer-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
  flex: 1;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.drawer-head h3 {
  margin: 0;
  font-size: 14px;
}

.drawer-actions {
  display: grid;
  gap: 8px;
}

.drawer-input {
  border: 1px solid #e3ded7;
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 12px;
  resize: vertical;
}

.drawer-content {
  flex: 1;
  border-top: 1px solid #e3ded7;
  padding-top: 10px;
}

.status {
  font-size: 12px;
  color: var(--info);
}

.error {
  font-size: 12px;
  color: var(--err);
}

.muted {
  font-size: 12px;
  color: var(--muted);
}
</style>
