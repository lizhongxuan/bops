<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute } from "vue-router";
import { activeWorkflow as storedWorkflow, refreshActiveWorkflow, setActiveWorkflow } from "./lib/activeWorkflow";

const route = useRoute();
const routeName = computed(() => String(route.name || ""));
const activeWorkflowName = computed(() => {
  if (typeof route.params.name === "string") return route.params.name;
  if (typeof route.query.workflow === "string") return route.query.workflow;
  return storedWorkflow.value;
});
const hasWorkflow = computed(() => activeWorkflowName.value.length > 0);

watch(
  () => [route.params.name, route.query.workflow],
  ([paramName, queryName]) => {
    const next =
      typeof paramName === "string"
        ? paramName
        : typeof queryName === "string"
          ? queryName
          : "";
    if (next) {
      setActiveWorkflow(next);
      return;
    }
    refreshActiveWorkflow();
  },
  { immediate: true }
);

const isFullHeightPage = computed(() =>
  ["run-console", "validation-console", "workflow-flow"].includes(routeName.value)
);

const isWorkspaceRoute = computed(() => ["home", "workflows"].includes(routeName.value));
const showTopbar = computed(() => routeName.value === "home");
const workspaceLink = computed(() => (hasWorkflow.value ? "/" : "/workflows"));

const displayWorkflowName = computed(() =>
  hasWorkflow.value ? activeWorkflowName.value : "未选择工作流"
);
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="logo">BOPS</div>
        <div class="subtitle">工作流控制台</div>
      </div>
      <nav class="nav">
        <RouterLink class="nav-item" :class="{ active: isWorkspaceRoute }" :to="workspaceLink">
          工作区
        </RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/envs">环境变量包</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/validation-envs">验证环境</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/scripts">脚本库</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/runs">运行记录</RouterLink>
        <RouterLink class="nav-item" active-class="active" to="/settings">设置</RouterLink>
      </nav>
      <div class="workflow-group">
        <div class="group-title">当前工作流</div>
        <div v-if="hasWorkflow" class="workflow-card">
          <div class="workflow-name">{{ activeWorkflowName }}</div>
          <div class="workflow-meta">最近保存 2 分钟前</div>
        </div>
        <RouterLink v-else class="workflow-empty" to="/workflows">
          选择一个工作流
        </RouterLink>
      </div>
    </aside>

    <div class="main">
      <header v-if="showTopbar" class="topbar">
        <div class="topbar-workflow">
          <span class="topbar-label">当前工作流</span>
          <span class="topbar-name">{{ displayWorkflowName }}</span>
        </div>
        <div class="topbar-actions">
          <RouterLink class="btn ghost" to="/workflows">切换工作区</RouterLink>
          <RouterLink v-if="hasWorkflow" class="btn ghost" :to="`/workflows/${activeWorkflowName}`">
            工作流编排
          </RouterLink>
          <RouterLink v-if="hasWorkflow" class="btn ghost" :to="`/workflows/${activeWorkflowName}/flow`">
            流程视图
          </RouterLink>
          <RouterLink v-if="hasWorkflow" class="btn ghost" :to="`/workflows/${activeWorkflowName}/runs`">
            运行记录
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
  height: 100vh;
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
  min-height: 0;
  overflow: hidden;
}

  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 16px 6px;
    gap: 16px;
  }

.title {
  font-family: "Space Grotesk", sans-serif;
  font-size: 20px;
}

.topbar-workflow {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 13px;
  color: var(--muted);
}

.topbar-label {
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-size: 14px;
  font-weight: 600;
}

.topbar-name {
  color: var(--ink);
  font-weight: 600;
}

.topbar-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
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
  padding: 1px 1px 1px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
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
