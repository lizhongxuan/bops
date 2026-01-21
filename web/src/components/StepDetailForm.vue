<template>
  <div class="step-detail">
    <div class="field">
      <label>步骤名称</label>
      <input :value="step.name" @input="updateField('name', readValue($event))" type="text" placeholder="例如：安装 nginx" />
    </div>

    <div class="field">
      <label>动作</label>
      <select :value="step.action" @change="handleActionChange">
        <option value="cmd.run">cmd.run</option>
        <option value="pkg.install">pkg.install</option>
        <option value="template.render">template.render</option>
        <option value="service.ensure">service.ensure</option>
        <option value="script.shell">script.shell</option>
        <option value="script.python">script.python</option>
        <option value="env.set">env.set</option>
      </select>
    </div>

    <template v-if="step.action === 'cmd.run'">
      <div class="field">
        <label>命令</label>
        <textarea :value="step.with.cmd" @input="updateWithField('cmd', readValue($event))" rows="3" placeholder="echo hello" />
      </div>
      <div class="field">
        <label>工作目录</label>
        <input :value="step.with.dir" @input="updateWithField('dir', readValue($event))" type="text" placeholder="/opt/app" />
      </div>
      <div class="field">
        <label>环境变量 (KEY=VALUE 每行一条)</label>
        <textarea
          :value="step.with.envText"
          @input="updateWithField('envText', readValue($event))"
          rows="3"
          placeholder="TOKEN=abc123"
        />
      </div>
    </template>

    <template v-else-if="step.action === 'pkg.install'">
      <div class="field">
        <label>包名</label>
        <input
          :value="step.with.packages"
          @input="updateWithField('packages', readValue($event))"
          type="text"
          placeholder="nginx, curl"
        />
      </div>
    </template>

    <template v-else-if="step.action === 'template.render'">
      <div class="field">
        <label>模板路径</label>
        <input :value="step.with.src" @input="updateWithField('src', readValue($event))" type="text" placeholder="nginx.conf.j2" />
      </div>
      <div class="field">
        <label>输出路径</label>
        <input :value="step.with.dest" @input="updateWithField('dest', readValue($event))" type="text" placeholder="/etc/nginx/nginx.conf" />
      </div>
      <div class="field">
        <label>模板变量 (YAML/JSON)</label>
        <textarea
          :value="step.with.vars"
          @input="updateWithField('vars', readValue($event))"
          rows="3"
          placeholder="key: value"
        />
      </div>
    </template>

    <template v-else-if="step.action === 'service.ensure'">
      <div class="field">
        <label>服务名</label>
        <input :value="step.with.name" @input="updateWithField('name', readValue($event))" type="text" placeholder="nginx" />
      </div>
      <div class="field">
        <label>状态</label>
        <select :value="step.with.state" @change="updateWithField('state', readValue($event))">
          <option value="started">started</option>
          <option value="stopped">stopped</option>
        </select>
      </div>
    </template>

    <template v-else-if="step.action === 'script.shell' || step.action === 'script.python'">
      <div class="field">
        <label>脚本库引用</label>
        <input
          :value="step.with.scriptRef"
          @input="updateWithField('scriptRef', readValue($event))"
          type="text"
          placeholder="install-nginx"
        />
      </div>
      <div class="field">
        <label>脚本内容</label>
        <textarea
          :value="step.with.script"
          @input="updateWithField('script', readValue($event))"
          rows="4"
          placeholder="#!/bin/sh"
        />
      </div>
    </template>

    <template v-else-if="step.action === 'env.set'">
      <div class="field">
        <label>环境变量 (KEY=VALUE 每行一条)</label>
        <textarea
          :value="step.with.envText"
          @input="updateWithField('envText', readValue($event))"
          rows="4"
          placeholder="TOKEN=abc123"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { DraftStep, createDefaultStepWith } from "../lib/draft";

const props = defineProps<{
  step: DraftStep;
}>();

const emit = defineEmits<{
  (event: "update-step", value: DraftStep): void;
}>();

function updateStep(partial: Partial<DraftStep>) {
  emit("update-step", {
    ...props.step,
    ...partial,
    with: {
      ...props.step.with,
      ...(partial.with || {})
    }
  });
}

function updateField(field: keyof DraftStep, value: string) {
  if (field === "name" || field === "action") {
    updateStep({ [field]: value } as Partial<DraftStep>);
  }
}

function updateWithField(field: keyof DraftStep["with"], value: string) {
  updateStep({
    with: {
      [field]: value
    }
  });
}

function readValue(event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null;
  return target?.value ?? "";
}

function handleActionChange(event: Event) {
  const nextAction = (event.target as HTMLSelectElement).value;
  updateStep({
    action: nextAction,
    with: createDefaultStepWith(nextAction)
  });
}
</script>

<style scoped>
.step-detail {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field label {
  font-size: 12px;
  color: var(--muted);
}

.field input,
.field textarea,
.field select {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 12px;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  background: #fff;
}

.field textarea {
  resize: vertical;
  min-height: 60px;
}
</style>
