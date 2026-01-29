import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import HomeView from "./views/HomeView.vue";
import WorkbenchView from "./views/WorkbenchView.vue";
import RunsView from "./views/RunsView.vue";
import WorkflowPickerView from "./views/WorkflowPickerView.vue";
import WorkflowStudioView from "./views/WorkflowStudioView.vue";
import FlowView from "./views/FlowView.vue";
import RunConsoleView from "./views/RunConsoleView.vue";
import EnvPackagesView from "./views/EnvPackagesView.vue";
import ValidationEnvsView from "./views/ValidationEnvsView.vue";
import ScriptsView from "./views/ScriptsView.vue";
import SettingsView from "./views/SettingsView.vue";
import ValidationConsoleView from "./views/ValidationConsoleView.vue";
import "@mcp-ui/client/ui-resource-renderer.wc.js";
import "./styles/tokens.css";
import "./styles/global.css";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "workbench",
      component: WorkbenchView
    },
    {
      path: "/home",
      name: "home",
      component: HomeView
    },
    {
      path: "/workflows",
      name: "workflows",
      component: WorkflowPickerView
    },
    {
      path: "/envs",
      name: "envs",
      component: EnvPackagesView
    },
    {
      path: "/validation-envs",
      name: "validation-envs",
      component: ValidationEnvsView
    },
    {
      path: "/scripts",
      name: "scripts",
      component: ScriptsView
    },
    {
      path: "/workflows/:name/runs",
      name: "workflow-runs",
      component: RunsView
    },
    {
      path: "/workflows/:name",
      name: "workflow",
      component: WorkflowStudioView
    },
    {
      path: "/workflows/:name/flow",
      name: "workflow-flow",
      component: FlowView
    },
    {
      path: "/runs/:id",
      name: "run-console",
      component: RunConsoleView
    },
    {
      path: "/runs",
      name: "runs",
      component: RunsView
    },
    {
      path: "/validation-console",
      name: "validation-console",
      component: ValidationConsoleView
    },
    {
      path: "/settings",
      name: "settings",
      component: SettingsView
    }
  ]
});

router.beforeEach((to) => {
  if (to.name !== "workbench") return;
  const workflow =
    typeof to.query.workflow === "string"
      ? to.query.workflow
      : "";
  const stored = localStorage.getItem("bops-last-workflow") || "";
  if (!workflow) {
    if (stored) {
      return { path: "/", query: { workflow: stored } };
    }
    return { path: "/workflows" };
  }
  localStorage.setItem("bops-last-workflow", workflow);
});

createApp(App).use(router).mount("#app");
