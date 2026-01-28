<template>
  <section class="validation-console">
    <div class="console-header">
      <div>
        <h1>验证终端</h1>
        <div v-if="payload" class="console-meta">
          <span class="badge" :class="payload.status">{{ statusText }}</span>
          <span v-if="payload.env">环境: {{ payload.env }}</span>
          <span v-if="payload.code !== undefined">code {{ payload.code }}</span>
          <span v-if="payload.created_at">{{ formatTime(payload.created_at) }}</span>
        </div>
      </div>
      <div class="actions">
        <button class="btn ghost" type="button" @click="goBack">返回</button>
        <RouterLink class="btn" to="/">回到首页</RouterLink>
      </div>
    </div>

    <div v-if="!payload" class="empty">暂无验证记录</div>

    <section v-else class="panel terminal">
      <div class="terminal-window">
        <div class="terminal-head">
          <span class="dot red"></span>
          <span class="dot yellow"></span>
          <span class="dot green"></span>
          <span class="title">
            {{ payload.env ? `验证环境: ${payload.env}` : "验证终端" }}
          </span>
        </div>
        <div class="terminal-body">
          <div v-if="payload.error" class="block error">
            <div class="block-title">error</div>
            <pre>{{ payload.error }}</pre>
          </div>
          <div v-if="payload.stdout" class="block">
            <div class="block-title">stdout</div>
            <pre>{{ payload.stdout }}</pre>
          </div>
          <div v-if="payload.stderr" class="block error">
            <div class="block-title">stderr</div>
            <pre>{{ payload.stderr }}</pre>
          </div>
          <div v-if="!payload.stdout && !payload.stderr && !payload.error" class="line muted">
            暂无输出
          </div>
        </div>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

type ValidationConsolePayload = {
  status: string;
  stdout?: string;
  stderr?: string;
  code?: number;
  error?: string;
  env?: string;
  created_at?: string;
};

const VALIDATION_CONSOLE_KEY = "bops.validation-console";
const payload = ref<ValidationConsolePayload | null>(null);
const router = useRouter();

const statusText = computed(() => {
  if (!payload.value) return "";
  if (payload.value.status === "success") return "成功";
  if (payload.value.status === "failed") return "失败";
  return payload.value.status || "未知";
});

function formatTime(iso: string) {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString();
}

function goBack() {
  router.back();
}

onMounted(() => {
  const raw = sessionStorage.getItem(VALIDATION_CONSOLE_KEY);
  if (!raw) return;
  try {
    payload.value = JSON.parse(raw) as ValidationConsolePayload;
  } catch (err) {
    payload.value = null;
  }
});
</script>

<style scoped>
.validation-console {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 12px;
  color: var(--ink);
}

.console-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.console-header h1 {
  margin: 0 0 6px;
  font-size: 22px;
}

.console-meta {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
  flex-wrap: wrap;
}

.badge {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 11px;
  background: rgba(27, 27, 27, 0.08);
  color: var(--muted);
}

.badge.success {
  color: var(--ok);
  background: rgba(42, 157, 75, 0.12);
}

.badge.failed {
  color: var(--err);
  background: rgba(208, 52, 44, 0.12);
}

.actions {
  display: flex;
  gap: 10px;
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
}

.terminal-window {
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: #fff;
}

.terminal-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(27, 27, 27, 0.08);
  background: rgba(27, 27, 27, 0.03);
  font-size: 12px;
  color: var(--muted);
}

.terminal-head .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.terminal-head .dot.red {
  background: #e36b5b;
}

.terminal-head .dot.yellow {
  background: #f2c94c;
}

.terminal-head .dot.green {
  background: #6fcf97;
}

.terminal-head .title {
  margin-left: 6px;
}

.terminal-body {
  padding: 12px;
  font-size: 12px;
  background: #fafafa;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block {
  border-radius: 10px;
  background: #fff;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 10px;
}

.block.error {
  border-color: rgba(208, 52, 44, 0.3);
}

.block-title {
  font-weight: 600;
  margin-bottom: 6px;
  font-size: 12px;
  color: var(--muted);
}

pre {
  margin: 0;
  white-space: pre-wrap;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  font-size: 12px;
  color: var(--ink);
}

.line.muted {
  color: var(--muted);
}

.btn {
  border: 1px solid rgba(27, 27, 27, 0.16);
  background: #fff;
  border-radius: 10px;
  padding: 8px 14px;
  font-size: 12px;
  cursor: pointer;
}

.btn.ghost {
  background: transparent;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}
</style>
