import { ref } from "vue";

const ACTIVE_WORKFLOW_KEY = "bops-last-workflow";
const activeWorkflow = ref("");

function readStoredWorkflow() {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(ACTIVE_WORKFLOW_KEY) || "";
}

function refreshActiveWorkflow() {
  activeWorkflow.value = readStoredWorkflow();
  return activeWorkflow.value;
}

function setActiveWorkflow(name: string) {
  activeWorkflow.value = name;
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(ACTIVE_WORKFLOW_KEY, name);
  } catch {
    // ignore storage errors
  }
}

refreshActiveWorkflow();

export { ACTIVE_WORKFLOW_KEY, activeWorkflow, refreshActiveWorkflow, setActiveWorkflow };
