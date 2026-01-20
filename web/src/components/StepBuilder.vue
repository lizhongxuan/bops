<template>
  <div class="panel steps-panel">
    <div class="panel-title">步骤构建器</div>
    <div class="steps-actions">
      <select v-model="selectedAction">
        <option value="cmd.run">cmd.run</option>
        <option value="pkg.install">pkg.install</option>
        <option value="template.render">template.render</option>
        <option value="service.ensure">service.ensure</option>
        <option value="script.shell">script.shell</option>
        <option value="script.python">script.python</option>
        <option value="env.set">env.set</option>
      </select>
      <button class="btn" type="button" @click="addStep">新增步骤</button>
      <button class="btn ghost" type="button" @click="$emit('apply-targets')">批量应用目标</button>
    </div>

    <div v-if="steps.length === 0" class="empty">还没有步骤，先生成草稿或手动添加。</div>

    <div v-else class="step-list">
      <div
        v-for="(step, index) in steps"
        :key="step.id"
        class="step-card"
        :class="{ active: activeStepId === step.id, error: stepIssueIndexes.includes(index) }"
        @click="$emit('select-step', step.id)"
      >
        <div class="step-head">
          <div>
            <div class="step-title">{{ step.name }}</div>
            <div class="step-meta">
              {{ step.action }} · {{ step.targets || globalTargets || "未设置目标" }}
            </div>
          </div>
          <div class="step-status" :class="stepStatus(index).toLowerCase()">
            {{ stepStatus(index) }}
          </div>
        </div>
        <div class="step-body">
          <div class="step-summary">{{ stepSummary(step) }}</div>
        </div>
        <div class="step-actions">
          <button class="btn" type="button" @click.stop="$emit('open-step', index)">详情</button>
          <button class="btn ghost" type="button" @click.stop="$emit('duplicate-step', index)">复制</button>
          <button class="btn" type="button" @click.stop="$emit('open-step-yaml', index)">定位 YAML</button>
          <button class="btn danger" type="button" @click.stop="$emit('remove-step', index)">删除</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { DraftStep } from "../lib/draft";

const props = defineProps<{
  steps: DraftStep[];
  activeStepId: string;
  stepIssueIndexes: number[];
  globalTargets: string;
  validationOk: boolean;
  validationTouched: boolean;
}>();

const emit = defineEmits([
  "add-step",
  "apply-targets",
  "duplicate-step",
  "remove-step",
  "open-step",
  "open-step-yaml",
  "select-step"
]);

const selectedAction = ref("cmd.run");

function addStep() {
  emit("add-step", selectedAction.value);
}

function stepStatus(index: number) {
  if (props.stepIssueIndexes.includes(index)) return "Failed";
  if (props.validationTouched && props.validationOk) return "Validated";
  return "Draft";
}

function stepSummary(step: DraftStep) {
  switch (step.action) {
    case "cmd.run": {
      const cmd = (step.with.cmd || "").trim();
      const dir = (step.with.dir || "").trim();
      if (!cmd) return "未填写命令";
      return dir ? `${cmd} (dir: ${dir})` : cmd;
    }
    case "pkg.install": {
      const pkg = (step.with.packages || "").trim();
      return pkg ? `安装: ${pkg}` : "未填写包名";
    }
    case "template.render": {
      const src = (step.with.src || "").trim();
      const dest = (step.with.dest || "").trim();
      if (!src && !dest) return "未填写模板参数";
      return `${src || "模板"} → ${dest || "目标"}`;
    }
    case "service.ensure": {
      const name = (step.with.name || "").trim();
      const state = (step.with.state || "").trim();
      if (!name) return "未填写服务名";
      return `${name} (${state || "state"})`;
    }
    case "script.shell":
    case "script.python": {
      const ref = (step.with.scriptRef || "").trim();
      if (ref) return `脚本库: ${ref}`;
      const script = (step.with.script || "").trim();
      if (!script) return "未填写脚本";
      return script.length > 48 ? `${script.slice(0, 48)}...` : script;
    }
    case "env.set": {
      const envText = (step.with.envText || "").trim();
      if (!envText) return "未填写环境变量";
      const count = envText.split(/\r?\n/).filter(Boolean).length;
      return `环境变量 ${count} 条`;
    }
    default:
      return "待补充参数";
  }
}
</script>
