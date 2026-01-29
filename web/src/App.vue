<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();
const routeName = computed(() => String(route.name || ""));
const activeWorkflow = computed(() => {
  if (typeof route.params.name === "string") return route.params.name;
  if (typeof route.query.workflow === "string") return route.query.workflow;
  return "";
});
const hasWorkflow = computed(() => activeWorkflow.value.length > 0);

const isFullHeightPage = computed(() =>
  ["run-console", "validation-console", "workflow-flow"].includes(routeName.value)
);

const hideTopbarTitle = computed(() =>
  [
    "home",
    "workflows",
    "runs",
    "workflow-runs",
    "run-console",
    "validation-console",
    "envs",
    "validation-envs",
    "scripts",
    "settings"
  ].includes(routeName.value)
);

const showTopbarTitle = computed(() => !hideTopbarTitle.value);
const isWorkbench = computed(() => routeName.value === "workbench");
const showTopbar = computed(() => showTopbarTitle.value || showSwitch.value || isWorkbench.value);

const pageTitle = computed(() => {
  if (routeName.value === "workbench") return activeWorkflow.value || "工作区";
  if (hasWorkflow.value) return activeWorkflow.value;
  if (routeName.value === "workflows") return "选择工作区";
  if (routeName.value === "runs") return "运行记录";
  if (routeName.value === "run-console") return "运行控制台";
  if (routeName.value === "validation-console") return "验证终端";
  if (routeName.value === "validation-envs") return "验证环境";
  if (routeName.value === "scripts") return "脚本库";
  if (routeName.value === "settings") return "设置";
  return "BOPS";
});

const showMeta = computed(() =>
  hasWorkflow.value && ["workflow", "workflow-flow"].includes(routeName.value)
);
const showSwitch = computed(() => hasWorkflow.value);
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="logo">BOPS</div>
        <div class="subtitle">工作流控制台</div>
      </div>
      <nav class="nav">
        <RouterLink class="nav-item" active-class="active" to="/">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <rect x="4" y="4" width="7" height="7" rx="1.5" />
              <rect x="13" y="4" width="7" height="7" rx="1.5" />
              <rect x="4" y="13" width="7" height="7" rx="1.5" />
              <rect x="13" y="13" width="7" height="7" rx="1.5" />
            </svg>
          </span>
          <span class="nav-label">工作区</span>
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/envs">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M5 7h14v10H5z" />
              <path d="M9 7V5h6v2" />
              <path d="M8 12h8" />
            </svg>
          </span>
          <span class="nav-label">环境变量包</span>
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/validation-envs">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 4l7 3v5c0 4-3 7-7 8-4-1-7-4-7-8V7z" />
              <path d="M9 12l2 2 4-4" />
            </svg>
          </span>
          <span class="nav-label">验证环境</span>
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/scripts">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M8 9l-3 3 3 3" />
              <path d="M16 9l3 3-3 3" />
              <path d="M13 7l-2 10" />
            </svg>
          </span>
          <span class="nav-label">脚本库</span>
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/runs">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M7 5h10a2 2 0 0 1 2 2v10l-7-3-7 3V7a2 2 0 0 1 2-2z" />
            </svg>
          </span>
          <span class="nav-label">运行记录</span>
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/settings">
          <span class="nav-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="12" cy="12" r="3.2" />
              <path d="M4.8 12h2.2M17 12h2.2M12 4.8v2.2M12 17v2.2M6.3 6.3l1.6 1.6M16.1 16.1l1.6 1.6M6.3 17.7l1.6-1.6M16.1 7.9l1.6-1.6" />
            </svg>
          </span>
          <span class="nav-label">设置</span>
        </RouterLink>
      </nav>
      <div class="workflow-group">
        <div class="group-title">当前工作流</div>
        <div v-if="hasWorkflow" class="workflow-card">
          <div class="workflow-name">{{ activeWorkflow }}</div>
          <div class="workflow-meta">最近保存 2 分钟前</div>
        </div>
        <RouterLink v-else class="workflow-empty" to="/workflows">
          选择一个工作流
        </RouterLink>
        <div v-if="hasWorkflow" class="workflow-actions">
          <RouterLink class="nav-item" active-class="active" :to="`/workflows/${activeWorkflow}`">
            工作流编排
          </RouterLink>
          <RouterLink class="nav-item" active-class="active" :to="`/workflows/${activeWorkflow}/flow`">
            流程视图
          </RouterLink>
          <RouterLink class="nav-item" active-class="active" :to="`/workflows/${activeWorkflow}/runs`">
            运行记录
          </RouterLink>
        </div>
      </div>
    </aside>

    <div class="main">
      <header
        v-if="showTopbar"
        class="topbar"
        :class="{ 'topbar-workbench': isWorkbench && !showTopbarTitle }"
      >
        <div v-if="showTopbarTitle" class="title-block">
          <div class="title">{{ pageTitle }}</div>
          <div v-if="showMeta" class="meta">
            上次保存 2 分钟前
          </div>
        </div>
        <div v-else class="title-spacer"></div>
        <div class="topbar-right">
          <div id="topbar-extra" class="topbar-extra"></div>
          <div class="actions">
            <RouterLink v-if="showSwitch" class="btn ghost" to="/workflows">
              切换工作区
            </RouterLink>
          </div>
        </div>
      </header>

      <main class="content" :class="{ 'content-tight': isFullHeightPage }">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  display: grid;
  grid-template-columns: 60px 1fr;
  min-height: 100vh;
  height: 100vh;
}

.sidebar {
  padding: 18px 8px;
  background: #f9f6f0;
  border-right: 1px solid var(--grid);
  position: sticky;
  top: 0;
  height: 100vh;
  overflow: auto;
}

.brand {
  margin-bottom: 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.logo {
  font-family: "Space Grotesk", sans-serif;
  font-weight: 700;
  letter-spacing: 0.16em;
  font-size: 13px;
}

.subtitle {
  display: none;
}

.nav {
  display: grid;
  gap: 8px;
  margin-bottom: 24px;
}

.nav-item {
  padding: 8px 4px;
  border-radius: 12px;
  border: 1px solid transparent;
  color: var(--muted);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  text-align: center;
  line-height: 1.2;
}

.nav-item.active {
  color: var(--ink);
  border-color: transparent;
  background: transparent;
}

.nav-icon {
  width: 26px;
  height: 26px;
  border-radius: 10px;
  border: 1px solid var(--grid);
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
}

.nav-item.active .nav-icon {
  color: var(--ink);
  border-color: rgba(232, 93, 42, 0.35);
  box-shadow: 0 10px 18px rgba(232, 93, 42, 0.12);
}

.nav-label {
  max-width: 52px;
  word-break: break-all;
}

.nav-icon svg {
  width: 16px;
  height: 16px;
  stroke: currentColor;
  fill: none;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.workflow-group {
  display: none;
}

.group-title {
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
}

.workflow-actions {
  display: grid;
  gap: 8px;
}

.workflow-card {
  border: 1px solid var(--grid);
  border-radius: 14px;
  padding: 12px 14px;
  background: #fff;
  box-shadow: 0 12px 20px rgba(232, 93, 42, 0.06);
}

.workflow-name {
  font-weight: 600;
}

.workflow-meta {
  font-size: 11px;
  color: var(--muted);
  margin-top: 4px;
}

.workflow-empty {
  display: block;
  border-radius: 14px;
  padding: 12px 14px;
  font-size: 12px;
  color: var(--muted);
  border: 1px dashed var(--grid);
  background: #faf8f4;
  text-align: center;
}

.workflow-empty:hover {
  background: #fff;
}

.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px 12px;
  gap: 16px;
  max-width: 980px;
  margin: 0 auto;
  width: 100%;
}

.title-block {
  display: flex;
  flex-direction: column;
}

.title-spacer {
  flex: 1;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
}

.topbar-extra {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.topbar-workbench {
  justify-content: flex-end;
}

.topbar-workbench .title-spacer {
  display: none;
}

.title {
  font-family: "Space Grotesk", sans-serif;
  font-size: 20px;
}

.meta {
  color: var(--muted);
  font-size: 12px;
  margin-top: 4px;
}

.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 16px;
  cursor: pointer;
  font-size: 13px;
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  box-shadow: 0 14px 24px rgba(232, 93, 42, 0.24);
}

.btn.ghost {
  border-color: var(--grid);
  color: var(--muted);
}

.content {
  padding: 16px 16px 16px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
  align-items: stretch;
}

.content > * {
  flex: 1;
  min-height: 0;
}

.content.content-tight {
  padding-bottom: 0;
}

@media (max-width: 980px) {
  .app-shell {
    grid-template-columns: 1fr;
  }

  .sidebar {
    position: static;
    height: auto;
    border-right: none;
    border-bottom: 1px solid var(--grid);
    padding: 18px 16px;
  }

  .topbar {
    padding: 14px 16px 10px;
  }

  .content {
    padding: 16px 16px 16px;
  }
}
</style>
