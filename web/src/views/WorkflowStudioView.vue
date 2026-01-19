<template>
  <section class="studio">
    <section class="panel editor-panel fade-in">
      <div class="toolbar">
        <button class="btn" type="button" @click="formatYaml">
          格式化
        </button>
        <button class="btn" type="button" @click="validateYaml">
          校验
        </button>
        <div class="dropdown">
          <button class="btn" type="button" @click="toggleTemplates">
            插入模板
          </button>
          <div v-if="showTemplates" class="menu">
            <button
              class="menu-item"
              type="button"
              v-for="item in templates"
              :key="item.label"
              @click="insertTemplate(item.value)"
            >
              {{ item.label }}
            </button>
          </div>
        </div>
        <div class="status" :class="validation.ok ? 'ok' : 'warn'">
          {{ statusText }}
        </div>
        <div class="save">{{ savedLabel }}</div>
      </div>
      <div class="editor-wrap">
        <div class="editor-highlight" aria-hidden="true">
          <div class="editor-highlight-inner" ref="highlightInnerRef">
            <span
              v-for="(line, index) in highlightLines"
              :key="index"
              class="line"
              :class="{ error: errorLineSet.has(index + 1) }"
            >
              {{ line.length ? line : " " }}
            </span>
          </div>
        </div>
        <textarea
          ref="editorRef"
          class="editor"
          v-model="yaml"
          spellcheck="false"
          wrap="off"
          @scroll="syncScroll"
          @input="syncScroll"
        ></textarea>
      </div>
    </section>

    <aside class="panel preview fade-in">
      <div class="panel-title">结构预览</div>
      <div class="preview-body">
        <div class="section flow-section">
        <div class="section-title-row">
          <div class="section-title">流程视图</div>
          <RouterLink class="link-btn" :to="flowLink">
            进入流程
          </RouterLink>
        </div>
          <div class="flow-canvas">
            <div class="flow-node" v-for="step in steps" :key="step.name">
              <div class="node-title">{{ step.name }}</div>
              <div class="node-meta">{{ step.action || "未指定动作" }}</div>
              <div v-if="step.targets" class="node-targets">
                目标: {{ step.targets }}
              </div>
            </div>
            <div v-if="steps.length === 0" class="empty">
              未检测到步骤
            </div>
          </div>
        </div>
        <div class="section">
        <div class="section-title">主机</div>
          <div class="tag" v-for="host in hosts" :key="host">{{ host }}</div>
          <div v-if="hosts.length === 0" class="empty">
          未检测到主机
          </div>
        </div>
        <div class="section">
        <div class="section-title">变量</div>
          <div class="tag" v-for="item in vars" :key="item">{{ item }}</div>
          <div v-if="vars.length === 0" class="empty">
          未检测到变量
          </div>
        </div>
        <div class="section">
        <div class="section-title">环境变量包</div>
          <div class="env-select">
            <select v-model="selectedEnvPackage">
              <option value="">选择变量包</option>
              <option v-for="item in envPackages" :key="item.name" :value="item.name">
                {{ item.name }}
              </option>
            </select>
            <button class="btn" type="button" @click="addEnvPackage">关联</button>
          </div>
          <div class="chip-row" v-if="envPackageNames.length">
            <span class="chip" v-for="name in envPackageNames" :key="name">
              {{ name }}
              <button class="chip-remove" type="button" @click="removeEnvPackage(name)">×</button>
            </span>
          </div>
          <div v-if="envPackageNames.length === 0" class="empty">
          未关联环境变量包
          </div>
        </div>
        <div class="section">
        <div class="section-title">校验结果</div>
          <ul class="errors">
            <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
          </ul>
        </div>
      </div>
    </aside>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute } from "vue-router";
import { ApiError, request } from "../lib/api";

type EnvPackageSummary = {
  name: string;
  description: string;
};

const route = useRoute();
const workflowName = computed(() => String(route.params.name || "workflow"));
const flowLink = computed(() => `/workflows/${workflowName.value}/flow`);
const yaml = ref(defaultYaml(workflowName.value));
const showTemplates = ref(false);
const savedAt = ref<Date | null>(null);
const saving = ref(false);
const loading = ref(false);
const validation = ref({ ok: true, issues: [] as string[] });
const isDirty = ref(false);
let autoSaveTimer: number | null = null;
const errorLines = ref<number[]>([]);
const editorRef = ref<HTMLTextAreaElement | null>(null);
const highlightInnerRef = ref<HTMLDivElement | null>(null);
const envPackages = ref<EnvPackageSummary[]>([]);
const selectedEnvPackage = ref("");
const envPackageNames = ref<string[]>([]);

const templates = [
  {
    label: "cmd.run",
    value: "  - name: run command\n    targets: [web]\n    action: cmd.run\n    with:\n      cmd: \"echo hello\"\n"
  },
  {
    label: "template.render",
    value:
      "  - name: render config\n    targets: [web]\n    action: template.render\n    with:\n      src: nginx.conf.j2\n      dest: /etc/nginx/nginx.conf\n"
  },
  {
    label: "service.ensure",
    value:
      "  - name: ensure service\n    targets: [web]\n    action: service.ensure\n    with:\n      name: nginx\n      state: started\n"
  },
  {
    label: "pkg.install",
    value:
      "  - name: install package\n    targets: [web]\n    action: pkg.install\n    with:\n      name: nginx\n"
  },
  {
    label: "env.set",
    value:
      "  - name: set env\n    targets: [local]\n    action: env.set\n    with:\n      env:\n        TOKEN: \"\"\n"
  },
  {
    label: "script.shell",
    value:
      "  - name: run shell script\n    targets: [web]\n    action: script.shell\n    with:\n      script: |\n        echo \"hello from shell\"\n"
  },
  {
    label: "script.python",
    value:
      "  - name: run python script\n    targets: [web]\n    action: script.python\n    with:\n      script: |\n        import platform\n        print(platform.platform())\n"
  }
];

const steps = computed(() => parseSteps(yaml.value));
const hosts = computed(() => parseHosts(yaml.value));
const vars = computed(() => parseVars(yaml.value));
const statusText = computed(() =>
  validation.value.ok ? "校验通过" : "校验未通过"
);
const highlightLines = computed(() => yaml.value.split(/\r?\n/));
const errorLineSet = computed(() => new Set(errorLines.value));

const savedLabel = computed(() => {
  if (loading.value) {
    return "加载中";
  }
  if (saving.value) {
    return "保存中";
  }
  if (isDirty.value) {
    return "待保存";
  }
  if (!savedAt.value) {
    return "未保存";
  }
  return `已保存 ${formatTime(savedAt.value)}`;
});

function toggleTemplates() {
  showTemplates.value = !showTemplates.value;
}

function insertTemplate(snippet: string) {
  yaml.value = `${yaml.value.trim()}\n\n${snippet}`;
  showTemplates.value = false;
}

async function loadEnvPackages() {
  try {
    const data = await request<{ items: EnvPackageSummary[] }>("/envs");
    envPackages.value = data.items || [];
  } catch (err) {
    envPackages.value = [];
  }
}

function addEnvPackage() {
  if (!selectedEnvPackage.value) return;
  const next = Array.from(new Set([...envPackageNames.value, selectedEnvPackage.value]));
  yaml.value = replaceEnvPackagesBlock(yaml.value, next);
  selectedEnvPackage.value = "";
}

function removeEnvPackage(name: string) {
  const next = envPackageNames.value.filter((item) => item !== name);
  yaml.value = replaceEnvPackagesBlock(yaml.value, next);
}

async function validateYaml() {
  try {
    const data = await request<{ ok: boolean; issues?: string[] }>(
      `/workflows/${workflowName.value}/validate`,
      { method: "POST", body: { yaml: yaml.value } }
    );
    validation.value = {
      ok: data.ok,
      issues: data.issues || []
    };
    errorLines.value = data.ok ? [] : deriveErrorLines(validation.value.issues, yaml.value);
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [
        apiErr.message ? `校验失败: ${apiErr.message}` : "校验失败，请检查服务是否启动"
      ]
    };
    errorLines.value = deriveErrorLines(validation.value.issues, yaml.value);
  }
}

function formatYaml() {
  const lines = yaml.value.split(/\r?\n/);
  const formatted = lines.map((line) => line.replace(/\t/g, "  ").replace(/\s+$/g, ""));
  yaml.value = formatted.join("\n").trimEnd() + "\n";
  void validateYaml();
}

async function saveYaml() {
  if (saving.value) return;
  saving.value = true;
  try {
    await request(`/workflows/${workflowName.value}`, {
      method: "PUT",
      body: { yaml: yaml.value }
    });
    savedAt.value = new Date();
    isDirty.value = false;
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [
        apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败，请检查服务是否启动"
      ]
    };
  } finally {
    saving.value = false;
  }
}

function scheduleAutoSave() {
  if (loading.value) return;
  isDirty.value = true;
  if (autoSaveTimer) {
    window.clearTimeout(autoSaveTimer);
  }
  autoSaveTimer = window.setTimeout(() => {
    void saveYaml();
  }, 800);
}

function cancelAutoSave() {
  if (autoSaveTimer) {
    window.clearTimeout(autoSaveTimer);
    autoSaveTimer = null;
  }
}

function handleKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
    event.preventDefault();
    saveYaml();
  }
}

onMounted(() => {
  document.addEventListener("keydown", handleKeydown);
  loadYaml();
  loadEnvPackages();
});

onUnmounted(() => {
  document.removeEventListener("keydown", handleKeydown);
  cancelAutoSave();
});

watch(workflowName, () => {
  loadYaml();
});

watch(yaml, () => {
  scheduleAutoSave();
  envPackageNames.value = parseEnvPackages(yaml.value);
  if (errorLines.value.length) {
    errorLines.value = [];
  }
});

onBeforeRouteLeave(async () => {
  cancelAutoSave();
  if (isDirty.value) {
    await saveYaml();
  }
});

async function loadYaml() {
  loading.value = true;
  try {
    const data = await request<{ yaml: string }>(`/workflows/${workflowName.value}`);
    yaml.value = data.yaml || defaultYaml(workflowName.value);
    savedAt.value = new Date();
    isDirty.value = false;
    errorLines.value = [];
  } catch (err) {
    if ((err as ApiError).status === 404) {
      const fallback = defaultYaml(workflowName.value);
      yaml.value = fallback;
      try {
        await request(`/workflows/${workflowName.value}`, {
          method: "PUT",
          body: { yaml: fallback }
        });
        savedAt.value = new Date();
        isDirty.value = false;
        errorLines.value = [];
      } catch (saveErr) {
        savedAt.value = null;
        validation.value = {
          ok: false,
          issues: ["创建默认工作流失败，请检查服务是否启动"]
        };
      }
    } else {
      validation.value = {
        ok: false,
        issues: ["加载失败，请检查服务是否启动"]
      };
    }
  } finally {
    loading.value = false;
  }
  await validateYaml();
}

function syncScroll() {
  if (!editorRef.value || !highlightInnerRef.value) return;
  const top = editorRef.value.scrollTop;
  const left = editorRef.value.scrollLeft;
  highlightInnerRef.value.style.transform = `translate(${-left}px, ${-top}px)`;
}

function deriveErrorLines(issues: string[], content: string) {
  const lines = content.split(/\r?\n/);
  const errorSet = new Set<number>();
  const stepLines = collectStepLines(lines);

  const addIfValid = (line: number | null) => {
    if (line && line > 0 && line <= lines.length) {
      errorSet.add(line);
    }
  };

  for (const issue of issues) {
    const lineMatch = issue.match(/line\s+(\d+)/i);
    if (lineMatch) {
      addIfValid(Number(lineMatch[1]));
      continue;
    }

    if (/version is required/i.test(issue)) {
      addIfValid(findTopLevelKey(lines, "version") || 1);
      continue;
    }
    if (/name is required/i.test(issue)) {
      addIfValid(findTopLevelKey(lines, "name") || 1);
      continue;
    }
    if (/steps must not be empty/i.test(issue)) {
      addIfValid(findTopLevelKey(lines, "steps") || lines.length);
      continue;
    }
    if (/plan\.mode/i.test(issue)) {
      addIfValid(findSectionKey(lines, "plan", "mode") || findTopLevelKey(lines, "plan"));
      continue;
    }
    if (/plan\.strategy/i.test(issue)) {
      addIfValid(findSectionKey(lines, "plan", "strategy") || findTopLevelKey(lines, "plan"));
      continue;
    }

    const stepIndexMatch = issue.match(/steps\[(\d+)\]/i);
    if (stepIndexMatch) {
      const idx = Number(stepIndexMatch[1]);
      addIfValid(stepLines[idx] || findTopLevelKey(lines, "steps") || 1);
      continue;
    }

    const stepNameMatch = issue.match(/step name \"([^\"]+)\"/i);
    if (stepNameMatch) {
      const name = stepNameMatch[1];
      const matches = findStepNameLines(lines, name);
      if (matches.length) {
        matches.forEach((line) => addIfValid(line));
      }
      continue;
    }

    if (/handler/i.test(issue)) {
      addIfValid(findTopLevelKey(lines, "handlers") || findTopLevelKey(lines, "steps") || 1);
    }
  }

  return Array.from(errorSet).sort((a, b) => a - b);
}

function findTopLevelKey(lines: string[], key: string) {
  const regex = new RegExp(`^${key}\\s*:\\s*`);
  for (let i = 0; i < lines.length; i += 1) {
    if (regex.test(lines[i])) {
      return i + 1;
    }
  }
  return null;
}

function findSectionKey(lines: string[], section: string, key: string) {
  const sectionRegex = new RegExp(`^(\\s*)${section}\\s*:\\s*$`);
  const keyRegex = new RegExp(`^\\s*${key}\\s*:\\s*`);
  let inSection = false;
  let sectionIndent = 0;
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const match = line.match(sectionRegex);
    if (match) {
      inSection = true;
      sectionIndent = match[1].length;
      continue;
    }
    if (inSection) {
      const indent = line.match(/^(\\s*)/)[1].length;
      if (indent <= sectionIndent && line.trim() !== "") {
        inSection = false;
        continue;
      }
      if (keyRegex.test(line)) {
        return i + 1;
      }
    }
  }
  return null;
}

function collectStepLines(lines: string[]) {
  const stepLines: number[] = [];
  let inSteps = false;
  let stepsIndent = 0;

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const stepsMatch = line.match(/^(\\s*)steps\\s*:\\s*$/);
    if (stepsMatch) {
      inSteps = true;
      stepsIndent = stepsMatch[1].length;
      continue;
    }
    if (inSteps) {
      const indent = line.match(/^(\\s*)/)[1].length;
      if (indent <= stepsIndent && line.trim() !== "") {
        inSteps = false;
        continue;
      }
      if (/^\\s*-\\s*name\\s*:/i.test(line)) {
        stepLines.push(i + 1);
      }
    }
  }
  return stepLines;
}

function findStepNameLines(lines: string[], name: string) {
  const matches: number[] = [];
  const regex = new RegExp(`^\\s*-\\s*name\\s*:\\s*${escapeRegex(name)}\\s*$`);
  for (let i = 0; i < lines.length; i += 1) {
    if (regex.test(lines[i])) {
      matches.push(i + 1);
    }
  }
  return matches;
}

function escapeRegex(value: string) {
  return value.replace(/[.*+?^${}()|[\\]\\\\]/g, "\\\\$&");
}

function parseSteps(content: string) {
  const lines = content.split(/\r?\n/);
  const steps: { name: string; action: string; targets: string }[] = [];
  let current: { name: string; action: string; targets: string } | null = null;

  for (const line of lines) {
    const nameMatch = line.match(/^\s*-\s*name\s*:\s*(.+)$/);
    if (nameMatch) {
      current = { name: nameMatch[1].trim(), action: "", targets: "" };
      steps.push(current);
      continue;
    }
    if (current) {
      const actionMatch = line.match(/^\s*action\s*:\s*(.+)$/);
      if (actionMatch) {
        current.action = actionMatch[1].trim();
      }
      const targetsMatch = line.match(/^\s*targets\s*:\s*(.+)$/);
      if (targetsMatch) {
        current.targets = targetsMatch[1].trim();
      }
    }
  }

  return steps;
}

function parseHosts(content: string) {
  const lines = content.split(/\r?\n/);
  const hosts: string[] = [];
  let inHosts = false;
  let hostsIndent = 0;

  for (const line of lines) {
    const hostsMatch = line.match(/^(\s*)hosts\s*:\s*$/);
    if (hostsMatch) {
      inHosts = true;
      hostsIndent = hostsMatch[1].length;
      continue;
    }
    if (inHosts) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (indent <= hostsIndent) {
        inHosts = false;
        continue;
      }
      const hostMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:\s*$/);
      if (hostMatch) {
        hosts.push(hostMatch[1]);
      }
    }
  }

  return Array.from(new Set(hosts));
}

function parseVars(content: string) {
  const lines = content.split(/\r?\n/);
  const vars: string[] = [];
  let inVars = false;
  let varsIndent = 0;

  for (const line of lines) {
    const varsMatch = line.match(/^(\s*)vars\s*:\s*$/);
    if (varsMatch) {
      inVars = true;
      varsIndent = varsMatch[1].length;
      continue;
    }
    if (inVars) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (indent <= varsIndent) {
        inVars = false;
        continue;
      }
      const varMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:/);
      if (varMatch) {
        vars.push(varMatch[1]);
      }
    }
  }

  return Array.from(new Set(vars));
}

function parseEnvPackages(content: string) {
  const lines = content.split(/\r?\n/);
  const packages: string[] = [];
  let inSection = false;
  let sectionIndent = 0;

  for (const line of lines) {
    const match = line.match(/^(\s*)env_packages\s*:\s*$/);
    if (match) {
      inSection = true;
      sectionIndent = match[1].length;
      continue;
    }
    if (inSection) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (indent <= sectionIndent && line.trim() !== "") {
        inSection = false;
        continue;
      }
      const itemMatch = line.match(/^\s*-\s*([a-zA-Z0-9_-]+)\s*$/);
      if (itemMatch) {
        packages.push(itemMatch[1]);
      }
    }
  }

  return Array.from(new Set(packages));
}

function replaceEnvPackagesBlock(content: string, packages: string[]) {
  const lines = content.split(/\r?\n/);
  const block = packages.length
    ? ["env_packages:", ...packages.map((name) => `  - ${name}`)]
    : [];
  const startIndex = lines.findIndex((line) => /^env_packages\s*:\s*$/.test(line));

  if (startIndex === -1) {
    if (!block.length) {
      return content;
    }
    const insertIndex =
      findTopLevelKeyIndex(lines, "plan") ??
      findTopLevelKeyIndex(lines, "steps") ??
      lines.length;
    const next = [...lines.slice(0, insertIndex), ...block, "", ...lines.slice(insertIndex)];
    return next.join("\n").trimEnd() + "\n";
  }

  let endIndex = startIndex + 1;
  while (endIndex < lines.length) {
    const line = lines[endIndex];
    if (line.trim() === "") {
      endIndex += 1;
      continue;
    }
    if (/^[a-zA-Z0-9_-]+\s*:/i.test(line) && !/^\s/.test(line)) {
      break;
    }
    endIndex += 1;
  }

  if (!block.length) {
    const next = [...lines.slice(0, startIndex), ...lines.slice(endIndex)];
    return next.join("\n").trimEnd() + "\n";
  }

  const next = [...lines.slice(0, startIndex), ...block, ...lines.slice(endIndex)];
  return next.join("\n").trimEnd() + "\n";
}

function findTopLevelKeyIndex(lines: string[], key: string) {
  const regex = new RegExp(`^${key}\\s*:\\s*$`);
  for (let i = 0; i < lines.length; i += 1) {
    if (regex.test(lines[i])) {
      return i;
    }
  }
  return null;
}

function formatTime(date: Date) {
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit"
  });
}

function defaultYaml(name: string) {
  return `version: v0.1
name: ${name}
description: install and config nginx

inventory:
  groups:
    web:
      hosts:
        - web1
        - web2
  vars:
    ssh_user: ops

vars:
  conf_src: nginx.conf.j2

plan:
  mode: manual-approve
  strategy: sequential

steps:
  - name: install package
    targets: [web]
    action: pkg.install
    with:
      name: nginx

  - name: render config
    targets: [web]
    action: template.render
    with:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf

  - name: ensure service
    targets: [web]
    action: service.ensure
    with:
      name: nginx
      state: started
`;
}
</script>

<style scoped>
.studio {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 18px;
  min-height: calc(100vh - 140px);
  align-items: stretch;
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
  height: 100%;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  margin-bottom: 12px;
}

.editor-panel {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 14px;
  cursor: pointer;
  font-size: 12px;
}

.status {
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 12px;
  border: 1px solid var(--grid);
}

.status.ok {
  color: var(--ok);
}

.status.warn {
  color: var(--err);
}

.save {
  margin-left: auto;
  color: var(--muted);
  font-size: 12px;
}

.editor-wrap {
  margin-top: 12px;
  min-height: 360px;
  flex: 1;
  border-radius: var(--radius-md);
  border: 1px solid #111111;
  background: #111111;
  position: relative;
  overflow: hidden;
}

.editor-highlight {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.editor-highlight-inner {
  font-family: "JetBrains Mono", monospace;
  font-size: 13px;
  line-height: 1.5;
  padding: 14px;
  white-space: pre;
  color: transparent;
}

.editor-highlight .line {
  display: block;
}

.editor-highlight .line.error {
  background: rgba(208, 52, 44, 0.2);
  box-shadow: inset 0 0 0 1px rgba(208, 52, 44, 0.6);
  border-radius: 4px;
}

.editor {
  position: relative;
  width: 100%;
  height: 100%;
  border: none;
  background: transparent;
  color: #f4f1ec;
  font-family: "JetBrains Mono", monospace;
  font-size: 13px;
  padding: 14px;
  line-height: 1.5;
  resize: none;
}

.dropdown {
  position: relative;
}

.menu {
  position: absolute;
  top: 36px;
  left: 0;
  background: #ffffff;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 6px;
  min-width: 180px;
  box-shadow: var(--shadow);
  z-index: 10;
}

.menu-item {
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px 10px;
  cursor: pointer;
}

.menu-item:hover {
  background: #f6f3ee;
}


.preview .section {
  margin-bottom: 16px;
}

.preview {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.preview-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow: auto;
  padding-right: 4px;
}

.flow-section {
  margin-bottom: 8px;
}

.flow-canvas {
  position: relative;
  padding: 16px 16px 16px 46px;
  border-radius: var(--radius-md);
  border: 1px dashed var(--grid);
  background: linear-gradient(180deg, #fbfaf7 0%, #f5f1eb 100%);
  min-height: 220px;
}

.flow-canvas::before {
  content: "";
  position: absolute;
  left: 26px;
  top: 18px;
  bottom: 18px;
  width: 2px;
  background: linear-gradient(180deg, rgba(225, 221, 214, 0.4), rgba(225, 221, 214, 1));
}

.flow-node {
  position: relative;
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  background: #ffffff;
  padding: 10px 12px 10px 16px;
  margin-bottom: 16px;
}

.flow-node::before {
  content: "";
  position: absolute;
  left: -30px;
  top: 18px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 0 0 6px rgba(232, 93, 42, 0.16);
}

.flow-node::after {
  content: "";
  position: absolute;
  right: 12px;
  top: 18px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1px solid var(--grid);
  background: #fff;
}

.flow-node:last-child {
  margin-bottom: 0;
}

.node-title {
  font-weight: 600;
}

.node-meta {
  font-size: 12px;
  color: var(--muted);
}

.node-targets {
  margin-top: 6px;
  font-size: 12px;
  color: var(--muted);
}

.section-title {
  font-size: 12px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.12em;
  margin-bottom: 8px;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.link-btn {
  font-size: 12px;
  border: 1px solid var(--grid);
  border-radius: 999px;
  padding: 4px 10px;
  color: var(--ink);
  background: #fff;
}

.step {
  border-radius: var(--radius-md);
  border: 1px solid var(--grid);
  padding: 8px 10px;
  margin-bottom: 8px;
}

.step-name {
  font-weight: 600;
}

.step-meta {
  font-size: 12px;
  color: var(--muted);
}

.tag {
  display: inline-flex;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  font-size: 12px;
  margin: 0 6px 6px 0;
}

.env-select {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.env-select select {
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  padding: 6px 8px;
  font-size: 12px;
}

.chip-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
  border: 1px solid var(--grid);
  padding: 4px 8px;
  font-size: 12px;
  background: #fff;
}

.chip-remove {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  color: var(--muted);
}

.errors {
  padding-left: 18px;
  color: var(--err);
  margin: 0;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 1200px) {
  .studio {
    grid-template-columns: 1fr;
  }
}
</style>
