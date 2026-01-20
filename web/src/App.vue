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
  ["run-console", "workflow-flow"].includes(routeName.value)
);

const hideTopbarTitle = computed(() =>
  [
    "home",
    "workflows",
    "runs",
    "workflow-runs",
    "run-console",
    "envs",
    "validation-envs",
    "scripts"
  ].includes(routeName.value)
);

const showTopbarTitle = computed(() => !hideTopbarTitle.value);

const pageTitle = computed(() => {
  if (hasWorkflow.value) return activeWorkflow.value;
  if (routeName.value === "workflows") return "选择工作流";
  if (routeName.value === "runs") return "运行记录";
  if (routeName.value === "run-console") return "运行控制台";
  if (routeName.value === "validation-envs") return "验证环境";
  if (routeName.value === "scripts") return "脚本库";
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
        <RouterLink class="nav-item" active-class="active" to="/">首页</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/workflows">工作流</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/envs">环境变量包</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/validation-envs">验证环境</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/scripts">脚本库</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/runs">运行记录</RouterLink>
        <a class="nav-item" href="#">设置</a>
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
      <header class="topbar">
        <div v-if="showTopbarTitle">
          <div class="title">{{ pageTitle }}</div>
          <div v-if="showMeta" class="meta">
            上次保存 2 分钟前
          </div>
        </div>
        <div v-else class="title-spacer"></div>
        <div class="actions">
          <RouterLink v-if="showSwitch" class="btn ghost" to="/workflows">
            切换工作流
          </RouterLink>
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
  grid-template-columns: 260px 1fr;
  min-height: 100vh;
}

.sidebar {
  padding: 28px 22px;
  background: #f9f6f0;
  border-right: 1px solid var(--grid);
  position: sticky;
  top: 0;
  height: 100vh;
  overflow: auto;
}

.brand {
  margin-bottom: 32px;
}

.logo {
  font-family: "Space Grotesk", sans-serif;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.subtitle {
  font-size: 12px;
  color: var(--muted);
  margin-top: 6px;
}

.nav {
  display: grid;
  gap: 10px;
  margin-bottom: 24px;
}

.nav-item {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid transparent;
  color: var(--muted);
}

.nav-item.active {
  color: var(--ink);
  border-color: var(--grid);
  background: #ffffff;
  box-shadow: var(--shadow);
}

.workflow-group {
  display: grid;
  gap: 12px;
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
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px 32px 16px;
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
  padding: 0 32px 48px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
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
  }
}
</style>
