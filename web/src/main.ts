import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import HomeView from "./views/HomeView.vue";
import RunsView from "./views/RunsView.vue";
import WorkflowPickerView from "./views/WorkflowPickerView.vue";
import WorkflowStudioView from "./views/WorkflowStudioView.vue";
import FlowView from "./views/FlowView.vue";
import RunConsoleView from "./views/RunConsoleView.vue";
import EnvPackagesView from "./views/EnvPackagesView.vue";
import ValidationEnvsView from "./views/ValidationEnvsView.vue";
import ScriptsView from "./views/ScriptsView.vue";
import "./styles/tokens.css";
import "./styles/global.css";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
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
    }
  ]
});

createApp(App).use(router).mount("#app");
