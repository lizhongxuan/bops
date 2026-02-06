<template>
  <section class="studio">
    <header class="panel workflow-bar fade-in">
      <div class="workflow-meta">
        <div class="title-row">
          <input
            class="title-input"
            :value="workflowTitle"
            placeholder="工作流名称"
            @focus="metaEditing = true"
            @blur="applyMetaChanges"
            @input="onMetaTitleInput"
          />
          <button class="btn ghost" type="button" @click="toggleInventory">
            主机/分组
          </button>
          <RouterLink class="btn ghost" :to="flowLink">流程视图</RouterLink>
        </div>
        <textarea
          class="desc-input"
          :value="workflowDescription"
          placeholder="一句话描述这个工作流"
          rows="2"
          @focus="metaEditing = true"
          @blur="applyMetaChanges"
          @input="onMetaDescInput"
        ></textarea>
        <div class="meta-hint">
          目标主机/分组必填，环境变量包可选，其它高级配置可在右侧展开。
        </div>
      </div>

      <div class="workflow-actions">
        <div class="env-block">
          <div class="label">环境变量包 (可选)</div>
          <div class="env-select">
            <select v-model="selectedEnvPackage">
              <option value="">选择变量包</option>
              <option v-for="item in envPackages" :key="item.name" :value="item.name">
                {{ item.name }}
              </option>
            </select>
            <button class="btn" type="button" @click="addEnvPackage">关联</button>
            <button
              class="btn ghost"
              type="button"
              :disabled="!selectedEnvPackage"
              @click="previewEnvPackage"
            >
              预览
            </button>
          </div>
          <div class="chip-row" v-if="envPackageNames.length">
            <span class="chip" v-for="name in envPackageNames" :key="name">
              {{ name }}
              <button class="chip-remove" type="button" @click="removeEnvPackage(name)">
                ×
              </button>
            </span>
          </div>
          <div v-else class="empty">未关联环境变量包</div>
        </div>

        <div class="action-row">
          <button class="btn" type="button" :disabled="runBusy" @click="planRun">
            计划
          </button>
          <button class="btn primary" type="button" :disabled="runBusy" @click="applyRun">
            执行
          </button>
          <div v-if="runMessage" class="run-message">{{ runMessage }}</div>
          <div class="save">{{ savedLabel }}</div>
        </div>
      </div>
    </header>

    <div class="studio-body">
      <section class="panel steps-panel fade-in">
        <div class="steps-header">
          <div>
            <h2>步骤列表</h2>
            <p>用最简单的表单配置执行步骤，按顺序运行。</p>
          </div>
          <div class="steps-actions">
            <button class="btn" type="button" @click="openYamlEditor">编辑器</button>
            <button class="btn primary" type="button" @click="openAddStep">
              新增步骤
            </button>
          </div>
        </div>
        <div v-if="unsupportedActions.length" class="warning-banner">
          检测到未支持动作: {{ unsupportedActions.join("、") }}。请在右侧高级编辑器中修改。
        </div>

        <div v-if="steps.length === 0" class="empty">
          还没有步骤，点击“新增步骤”开始编排。
        </div>

        <div class="steps-list" v-else>
          <div
            class="step-card"
            v-for="(step, index) in steps"
            :key="`${step.name}-${index}`"
            :class="{
              dragging: draggingIndex === index,
              over: dragOverIndex === index,
              highlight: highlightedIndex === index
            }"
            :data-step-index="index"
            @dragover.prevent="onDragOver(index)"
            @dragleave="onDragLeave"
            @drop="onDrop(index, $event)"
          >
            <div class="step-index">
              <span>{{ index + 1 }}</span>
              <button
                class="drag-handle"
                type="button"
                title="拖拽排序"
                draggable="true"
                @dragstart="onDragStart(index, $event)"
                @dragend="onDragEnd"
              >
                ⋮⋮
              </button>
            </div>
            <div class="step-body">
              <div class="step-row">
                <div class="field">
                  <span>步骤名</span>
                  <input
                    :value="step.name"
                    type="text"
                    placeholder="步骤名称"
                    :class="{ error: hasStepIssue(index, 'step.name') }"
                    @blur="updateStepName(index, ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div class="field">
                  <span>动作</span>
                  <select
                    :value="step.action"
                    :class="{ error: hasStepIssue(index, 'step.action') }"
                    @change="updateStepAction(index, ($event.target as HTMLSelectElement).value)"
                  >
                  <option
                    v-if="step.action && !isSupportedAction(step.action)"
                    :value="step.action"
                  >
                    {{ step.action }} (未支持)
                  </option>
                  <option value="cmd.run">cmd.run</option>
                  <option value="script.shell">script.shell</option>
                  <option value="script.python">script.python</option>
                  <option value="pkg.install">pkg.install</option>
                    <option value="template.render">template.render</option>
                    <option value="service.ensure">service.ensure</option>
                    <option value="service.restart">service.restart</option>
                    <option value="env.set">env.set</option>
                  </select>
                </div>
                <div class="field">
                  <span>目标主机/分组</span>
                  <div class="target-input">
                    <input
                      :value="getTargetInputValue(index, step.targets)"
                      type="text"
                      placeholder="web, db"
                      :class="{ error: hasStepIssue(index, 'step.targets') }"
                      :key="`targets-${step.targets}-${index}`"
                      @blur="updateStepTargets(index, ($event.target as HTMLInputElement).value)"
                    />
                    <button class="btn ghost" type="button" @click="openTargetPicker(index)">
                      选择
                    </button>
                  </div>
                </div>
              </div>
              <div v-if="step.action === 'cmd.run'" class="cmd-panel">
                <label class="field">
                  <span>命令</span>
                  <textarea
                    :value="step.cmd || ''"
                    rows="3"
                    placeholder="df -h"
                    :class="{ error: hasStepIssue(index, 'with.cmd') }"
                    @blur="updateStepWithMultiline(index, 'cmd', ($event.target as HTMLTextAreaElement).value)"
                  ></textarea>
                </label>
                <label class="field">
                  <span>工作目录</span>
                  <input
                    :value="step.dir || ''"
                    type="text"
                    placeholder="/var/log"
                    @blur="updateStepWithScalar(index, 'dir', ($event.target as HTMLInputElement).value)"
                  />
                </label>
              </div>
              <div v-else-if="step.action.startsWith('script.')" class="script-panel">
                <div class="inline-choice">
                  <label>
                    <input
                      type="radio"
                      :name="`script-source-${index}`"
                      :checked="getScriptSource(step) === 'inline'"
                      @change="setScriptSource(index, 'inline')"
                    />
                    内联脚本
                  </label>
                  <label>
                    <input
                      type="radio"
                      :name="`script-source-${index}`"
                      :checked="getScriptSource(step) === 'library'"
                      @change="setScriptSource(index, 'library')"
                    />
                    脚本库
                  </label>
                </div>
                <textarea
                  v-if="getScriptSource(step) === 'inline'"
                  :value="step.script || ''"
                  rows="4"
                  placeholder="echo 'hello'"
                  :class="{ error: hasStepIssue(index, 'with.script') }"
                  @blur="updateStepWithMultiline(index, 'script', ($event.target as HTMLTextAreaElement).value)"
                ></textarea>
                <div v-else class="script-library">
                  <label class="field">
                    <span>脚本库</span>
                    <select
                      :value="step.scriptRef || ''"
                      :class="{ error: hasStepIssue(index, 'with.script_ref') }"
                      @change="updateStepWithScalarAllowEmpty(index, 'script_ref', ($event.target as HTMLSelectElement).value)"
                    >
                      <option value="">选择脚本</option>
                      <option
                        v-for="item in getScriptOptions(step.action, step.scriptRef)"
                        :key="item.name"
                        :value="item.name"
                      >
                        {{ item.name }}
                      </option>
                    </select>
                  </label>
                  <div class="script-actions">
                    <button class="btn ghost" type="button" @click="loadScripts">刷新</button>
                    <button
                      class="btn"
                      type="button"
                      :disabled="!step.scriptRef"
                      @click="openScriptPreview(step.scriptRef || '')"
                    >
                      预览
                    </button>
                  </div>
                  <div v-if="scriptsLoading" class="muted">脚本库加载中...</div>
                  <div v-else-if="scriptsError" class="muted">{{ scriptsError }}</div>
                  <div
                    v-else-if="getScriptOptions(step.action, step.scriptRef).length === 0"
                    class="muted"
                  >
                    当前没有可用脚本
                  </div>
                </div>
              </div>
              <div
                v-if="step.action === 'env.set'"
                class="env-set-panel"
                :class="{ error: hasStepIssue(index, 'with.env') }"
              >
                <div class="env-set-header">
                  <span class="muted">环境变量设置</span>
                  <button class="btn ghost" type="button" @click="toggleEnvEditor(index)">
                    {{ envEditorIndex === index ? "收起编辑" : "编辑变量" }}
                  </button>
                </div>
                <div v-if="step.env && Object.keys(step.env).length" class="env-set-preview">
                  <span class="chip" v-for="(value, key) in step.env" :key="key">
                    {{ key }}={{ value }}
                  </span>
                </div>
                <div v-else class="empty">未配置环境变量</div>
                <div v-if="envEditorIndex === index" class="env-editor">
                  <div class="env-grid">
                    <div class="env-header">
                      <span>变量名</span>
                      <span>变量值</span>
                      <span></span>
                    </div>
                    <div class="env-row" v-for="(row, rowIndex) in envRows" :key="rowIndex">
                      <input v-model="row.key" type="text" placeholder="KEY" />
                      <input v-model="row.value" type="text" placeholder="VALUE" />
                      <button
                        class="btn ghost danger"
                        type="button"
                        @click="removeEnvRow(rowIndex)"
                      >
                        删除
                      </button>
                    </div>
                  </div>
                  <div class="env-actions">
                    <button class="btn ghost" type="button" @click="addEnvRow">添加变量</button>
                    <button class="btn primary" type="button" @click="saveEnvRows(index)">
                      保存
                    </button>
                    <span v-if="envSaveMessage[index]" class="muted">
                      {{ envSaveMessage[index] }}
                    </span>
                  </div>
                </div>
              </div>
              <div class="step-summary">
                <span class="required" v-if="step.required">
                  {{ step.required }}
                </span>
                <span class="warning" v-if="step.action && !isSupportedAction(step.action)">
                  未支持动作
                </span>
                <button class="btn ghost" type="button" @click="duplicateStep(index)">复制</button>
                <button class="btn ghost danger" type="button" @click="deleteStep(index)">删除</button>
                <button class="btn ghost" type="button" @click="openStepDetail(index)">
                  详情
                </button>
              </div>
              <div v-if="stepMessages(index).length" class="step-errors">
                {{ stepMessages(index).join("；") }}
              </div>
              <div class="step-advanced-toggle">
                <button class="btn ghost" type="button" @click="toggleAdvanced(index)">
                  {{ isAdvancedOpen(index) ? "收起高级配置" : "高级配置" }}
                </button>
              </div>
              <div v-if="isAdvancedOpen(index)" class="step-advanced">
                <div v-if="isAdvancedAction(step.action)" class="advanced-action">
                  <div class="advanced-title">高级动作参数</div>
                  <div class="advanced-grid">
                    <template v-if="step.action === 'pkg.install'">
                      <label class="field">
                        <span>包名</span>
                        <input
                          :value="step.withName || ''"
                          type="text"
                          placeholder="nginx"
                          :class="{ error: hasStepIssue(index, 'with.name') }"
                          @blur="updateStepWithScalar(index, 'name', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                    </template>
                    <template v-else-if="step.action === 'template.render'">
                      <label class="field">
                        <span>模板路径</span>
                        <input
                          :value="step.src || ''"
                          type="text"
                          placeholder="nginx.conf.j2"
                          :class="{ error: hasStepIssue(index, 'with.src') }"
                          @blur="updateStepWithScalar(index, 'src', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                      <label class="field">
                        <span>目标路径</span>
                        <input
                          :value="step.dest || ''"
                          type="text"
                          placeholder="/etc/nginx/nginx.conf"
                          :class="{ error: hasStepIssue(index, 'with.dest') }"
                          @blur="updateStepWithScalar(index, 'dest', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                    </template>
                    <template v-else-if="step.action === 'service.ensure'">
                      <label class="field">
                        <span>服务名</span>
                        <input
                          :value="step.withName || ''"
                          type="text"
                          placeholder="nginx"
                          :class="{ error: hasStepIssue(index, 'with.name') }"
                          @blur="updateStepWithScalar(index, 'name', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                      <label class="field">
                        <span>状态</span>
                        <input
                          :value="step.state || ''"
                          type="text"
                          placeholder="started"
                          :class="{ error: hasStepIssue(index, 'with.state') }"
                          @blur="updateStepWithScalar(index, 'state', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                    </template>
                    <template v-else-if="step.action === 'service.restart'">
                      <label class="field">
                        <span>服务名</span>
                        <input
                          :value="step.withName || ''"
                          type="text"
                          placeholder="nginx"
                          :class="{ error: hasStepIssue(index, 'with.name') }"
                          @blur="updateStepWithScalar(index, 'name', ($event.target as HTMLInputElement).value)"
                        />
                      </label>
                    </template>
                  </div>
                </div>
                <div class="advanced-grid">
                  <label class="field">
                    <span>when</span>
                    <input
                      :value="step.when || ''"
                      type="text"
                      placeholder="如: inventory.hostname == 'web1'"
                      @blur="updateStepMeta(index, 'when', ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                  <label class="field">
                    <span>retries</span>
                    <input
                      :value="step.retries || ''"
                      type="text"
                      placeholder="如: 3"
                      @blur="updateStepMeta(index, 'retries', ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                  <label class="field">
                    <span>timeout</span>
                    <input
                      :value="step.timeout || ''"
                      type="text"
                      placeholder="如: 60s"
                      @blur="updateStepMeta(index, 'timeout', ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                  <label class="field">
                    <span>loop</span>
                    <input
                      :value="step.loop || ''"
                      type="text"
                      placeholder="如: a,b,c"
                      @blur="updateStepMeta(index, 'loop', ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                  <label class="field">
                    <span>notify</span>
                    <input
                      :value="step.notify || ''"
                      type="text"
                      placeholder="如: handler-a, handler-b"
                      @blur="updateStepMeta(index, 'notify', ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                </div>
                <div class="advanced-env" v-if="step.action !== 'env.set'">
                  <div class="env-meta">
                    <span class="muted">步骤环境变量</span>
                    <span v-if="Object.keys(step.env || {}).length" class="pill">
                      已配置 {{ Object.keys(step.env || {}).length }} 项
                    </span>
                    <span v-else class="muted">未配置</span>
                  </div>
                  <button class="btn ghost" type="button" @click="toggleEnvEditor(index)">
                    {{ envEditorIndex === index ? "收起环境变量" : "查看/编辑" }}
                  </button>
                </div>
                <div v-if="envEditorIndex === index" class="env-editor">
                  <div class="env-grid">
                    <div class="env-header">
                      <span>变量名</span>
                      <span>变量值</span>
                      <span></span>
                    </div>
                    <div class="env-row" v-for="(row, rowIndex) in envRows" :key="rowIndex">
                      <input v-model="row.key" type="text" placeholder="KEY" />
                      <input v-model="row.value" type="text" placeholder="VALUE" />
                      <button
                        class="btn ghost danger"
                        type="button"
                        @click="removeEnvRow(rowIndex)"
                      >
                        删除
                      </button>
                    </div>
                  </div>
                  <div class="env-actions">
                    <button class="btn ghost" type="button" @click="addEnvRow">添加变量</button>
                    <button class="btn primary" type="button" @click="saveEnvRows(index)">
                      保存
                    </button>
                    <span v-if="envSaveMessage[index]" class="muted">
                      {{ envSaveMessage[index] }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <aside class="panel aux-panel fade-in">
        <details class="aux-section" open>
          <summary>结构预览</summary>
          <div class="flow-canvas">
            <div class="flow-node" v-for="step in steps" :key="step.name">
              <div class="node-title">{{ step.name }}</div>
              <div class="node-meta">{{ step.action || "未指定动作" }}</div>
              <div v-if="step.targets" class="node-targets">目标: {{ step.targets }}</div>
            </div>
            <div v-if="steps.length === 0" class="empty">未检测到步骤</div>
          </div>
        </details>
      </aside>
    </div>

    <div v-if="showAddStep" class="modal" @click.self="closeAddStep">
      <div class="modal-card">
        <h3>新增步骤</h3>
        <label class="field">
          <span>步骤名称</span>
          <input v-model="draftStep.name" type="text" placeholder="例如：检查磁盘" />
        </label>
        <label class="field">
          <span>动作</span>
          <select v-model="draftStep.action">
            <option value="cmd.run">cmd.run</option>
            <option value="script.shell">script.shell</option>
            <option value="script.python">script.python</option>
            <option value="pkg.install">pkg.install</option>
            <option value="template.render">template.render</option>
            <option value="service.ensure">service.ensure</option>
            <option value="service.restart">service.restart</option>
            <option value="env.set">env.set</option>
          </select>
        </label>
        <label class="field">
          <span>目标主机/分组</span>
          <input v-model="draftStep.targets" type="text" placeholder="web, db" />
        </label>

        <div v-if="draftStep.action === 'cmd.run'" class="field">
          <span>命令</span>
          <textarea v-model="draftStep.cmd" rows="4" placeholder="df -h"></textarea>
        </div>

        <div v-else-if="draftStep.action.startsWith('script.')" class="field">
          <span>脚本来源</span>
          <div class="inline-choice">
            <label>
              <input type="radio" value="inline" v-model="draftStep.scriptSource" />
              内联脚本
            </label>
            <label>
              <input type="radio" value="library" v-model="draftStep.scriptSource" />
              脚本库
            </label>
          </div>
          <textarea
            v-if="draftStep.scriptSource === 'inline'"
            v-model="draftStep.script"
            rows="4"
            placeholder="echo 'hello'"
          ></textarea>
          <input
            v-else
            v-model="draftStep.scriptRef"
            type="text"
            placeholder="脚本库名称"
          />
        </div>

        <div class="modal-actions">
          <button class="btn ghost" type="button" @click="closeAddStep">取消</button>
          <button class="btn primary" type="button" @click="confirmAddStep">添加</button>
        </div>
      </div>
    </div>

    <div v-if="showYamlEditor" class="modal" @click.self="closeYamlEditor">
      <div class="modal-card editor-modal">
        <div class="editor-header">
          <h3>YAML 编辑器</h3>
          <div class="toolbar">
            <button class="btn" type="button" @click="formatYaml">格式化</button>
            <button class="btn" type="button" @click="validateYaml">校验</button>
            <div class="dropdown">
              <button class="btn" type="button" @click="toggleTemplates">插入模板</button>
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
          </div>
        </div>
        <div class="editor-panel">
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
          <div class="editor-validation">
            <div class="validation" :class="validation.ok ? 'ok' : 'warn'">
              {{ statusText }}
            </div>
            <ul class="errors">
              <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
            </ul>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn primary" type="button" @click="closeYamlEditor">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="showInventory" class="modal" @click.self="toggleInventory">
      <div class="modal-card">
        <h3>主机/分组管理</h3>
        <div class="inventory-toolbar">
          <input
            v-model="inventorySearch"
            type="text"
            placeholder="搜索分组或主机"
            class="inventory-search"
          />
          <div class="inventory-add">
            <input
              v-model="newGroupName"
              type="text"
              placeholder="新增分组名"
              class="inventory-input"
            />
            <button class="btn" type="button" @click="addInventoryGroup">新增分组</button>
          </div>
        </div>
        <div class="inventory-section">
          <div class="section-title">独立主机</div>
          <div v-if="filteredInventoryHosts.length === 0" class="empty">
            暂无独立主机
          </div>
          <div v-else class="inventory-hosts">
            <div class="host-row" v-for="host in filteredInventoryHosts" :key="host.index">
              <input
                :value="host.name"
                type="text"
                class="inventory-input"
                placeholder="主机名"
                @blur="updateInventoryHostNameByIndex(host.index, ($event.target as HTMLInputElement).value)"
              />
              <input
                :value="host.address"
                type="text"
                class="inventory-input"
                placeholder="地址"
                @blur="updateInventoryHostAddressByIndex(host.index, ($event.target as HTMLInputElement).value)"
              />
              <button
                class="btn ghost danger"
                type="button"
                @click="removeInventoryHostItem(host.index)"
              >
                删除
              </button>
            </div>
          </div>
          <div class="inventory-host-add">
            <input
              v-model="newHostName"
              type="text"
              class="inventory-input"
              placeholder="新增主机名"
            />
            <input
              v-model="newHostAddress"
              type="text"
              class="inventory-input"
              placeholder="地址/主机"
            />
            <button class="btn ghost" type="button" @click="addInventoryHostItem">
              添加主机
            </button>
          </div>
        </div>
        <div class="inventory-section">
          <div class="section-title">分组</div>
          <div v-if="filteredInventoryGroups.length === 0" class="empty">
            暂无分组，请先新增分组。
          </div>
          <div v-else class="inventory-list">
            <div class="inventory-group" v-for="group in filteredInventoryGroups" :key="group.name">
              <div class="group-header">
                <input
                  :value="group.name"
                  type="text"
                  class="inventory-input"
                  @blur="updateInventoryGroupName(group.index, ($event.target as HTMLInputElement).value)"
                />
                <button
                  class="btn ghost danger"
                  type="button"
                  @click="removeInventoryGroup(group.index)"
                >
                  删除分组
                </button>
              </div>
              <div class="host-list">
                <div class="host-row" v-for="host in group.hosts" :key="host.index">
                  <input
                    :value="host.name"
                    type="text"
                    class="inventory-input"
                    @blur="updateInventoryHost(group.index, host.index, ($event.target as HTMLInputElement).value)"
                  />
                  <button
                    class="btn ghost danger"
                    type="button"
                    @click="removeInventoryHost(group.index, host.index)"
                  >
                    删除
                  </button>
                </div>
                <div class="host-add">
                  <input
                    v-model="hostDrafts[group.index]"
                    type="text"
                    class="inventory-input"
                    placeholder="新增主机名"
                  />
                  <button class="btn ghost" type="button" @click="addInventoryHost(group.index)">
                    添加主机
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn primary" type="button" @click="toggleInventory">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="envPreview" class="modal" @click.self="closeEnvPreview">
      <div class="modal-card">
        <h3>变量包预览: {{ envPreview.name }}</h3>
        <div v-if="envPreviewLoading" class="muted">加载中...</div>
        <div v-else class="env-preview">
          <div v-if="envPreview.description" class="muted">{{ envPreview.description }}</div>
          <div v-if="Object.keys(envPreview.env).length === 0" class="empty">
            变量包为空
          </div>
          <div v-else class="env-grid">
            <div class="env-header">
              <span>变量名</span>
              <span>变量值</span>
            </div>
            <div class="env-row" v-for="(value, key) in envPreview.env" :key="key">
              <span>{{ key }}</span>
              <span class="muted">{{ value }}</span>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn primary" type="button" @click="closeEnvPreview">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="stepDetail" class="modal" @click.self="closeStepDetail">
      <div class="modal-card">
        <h3>步骤详情: {{ stepDetail.name || "未命名" }}</h3>
        <div class="detail-meta">
          <span class="pill">{{ stepDetail.action || "未指定动作" }}</span>
          <span class="muted">目标: {{ formatTargetsForInput(stepDetail.targets) || "未设置" }}</span>
        </div>
        <div class="detail-section">
          <div class="detail-title">步骤配置</div>
          <pre class="yaml-preview">{{ stepDetail.yaml }}</pre>
        </div>
        <div v-if="stepDetailLoading" class="muted">脚本文本加载中...</div>
        <div v-else-if="stepDetail.scriptContent" class="detail-section">
          <div class="detail-title">脚本文本</div>
          <pre class="yaml-preview">{{ stepDetail.scriptContent }}</pre>
        </div>
        <div v-else-if="stepDetail.scriptRef" class="detail-section">
          <div class="detail-title">脚本文本</div>
          <div class="muted">脚本库引用: {{ stepDetail.scriptRef }}</div>
        </div>
        <div class="modal-actions">
          <button class="btn primary" type="button" @click="closeStepDetail">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="scriptPreview" class="modal" @click.self="closeScriptPreview">
      <div class="modal-card">
        <h3>脚本预览: {{ scriptPreview.name }}</h3>
        <div v-if="scriptPreviewLoading" class="muted">加载中...</div>
        <div v-else class="detail-section">
          <div class="detail-meta">
            <span class="pill">{{ scriptPreview.language || "未知语言" }}</span>
            <span v-if="scriptPreview.tags && scriptPreview.tags.length" class="muted">
              {{ scriptPreview.tags.join(" · ") }}
            </span>
          </div>
          <div v-if="scriptPreview.description" class="muted">
            {{ scriptPreview.description }}
          </div>
          <pre class="yaml-preview">{{ scriptPreview.content }}</pre>
        </div>
        <div class="modal-actions">
          <button class="btn primary" type="button" @click="closeScriptPreview">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="targetPickerIndex !== null" class="modal" @click.self="closeTargetPicker">
      <div class="modal-card">
        <h3>选择目标</h3>
        <div class="target-picker" v-if="inventoryGroups.length || inventoryHosts.length">
          <div class="target-group" v-for="group in inventoryGroups" :key="group.name">
            <label class="target-option">
              <input type="checkbox" :value="group.name" v-model="targetSelections" />
              <span class="target-tag">分组</span>
              <span>{{ group.name }}</span>
            </label>
            <div class="target-hosts">
              <label class="target-option" v-for="host in group.hosts" :key="host.name">
                <input type="checkbox" :value="host.name" v-model="targetSelections" />
                <span class="target-tag ghost">主机</span>
                <span>{{ host.name }}</span>
              </label>
            </div>
          </div>
          <div v-if="inventoryHosts.length" class="target-group">
            <div class="target-section-title">独立主机</div>
            <div class="target-hosts">
              <label class="target-option" v-for="host in inventoryHosts" :key="host.name">
                <input type="checkbox" :value="host.name" v-model="targetSelections" />
                <span class="target-tag ghost">主机</span>
                <span>{{ host.name }}</span>
              </label>
            </div>
          </div>
        </div>
        <div v-else class="empty">请先在主机/分组中添加目标。</div>
        <div class="modal-actions">
          <button class="btn ghost" type="button" @click="closeTargetPicker">取消</button>
          <button class="btn primary" type="button" @click="applyTargetSelection">应用</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
import { ApiError, request } from "../lib/api";
import { parseSteps, type StepSummary } from "../lib/workflowSteps";

type EnvPackageSummary = {
  name: string;
  description: string;
};

type EnvPackageDetail = {
  name: string;
  description?: string;
  env: Record<string, string>;
};

type ScriptSummary = {
  name: string;
  language: string;
  description?: string;
  tags?: string[];
  updated_at?: string;
};

type ScriptDetail = {
  name: string;
  language: string;
  description?: string;
  tags?: string[];
  content: string;
};

type InventoryHost = {
  name: string;
  index: number;
};

type InventoryGroup = {
  name: string;
  index: number;
  hosts: InventoryHost[];
};

type InventoryGroupData = {
  name: string;
  hosts: string[];
};

type InventoryHostData = {
  name: string;
  address: string;
};

type InventoryHostEntry = {
  name: string;
  address: string;
  index: number;
};

type StepMetaField = "when" | "retries" | "timeout" | "loop" | "notify";

type EnvRow = {
  key: string;
  value: string;
};

type StepValidation = {
  missing: string[];
  messages: string[];
};

type StepDetail = {
  name: string;
  action: string;
  targets: string;
  yaml: string;
  scriptRef?: string;
  scriptContent?: string;
};

type DraftStep = {
  name: string;
  action: string;
  targets: string;
  cmd: string;
  scriptSource: "inline" | "library";
  script: string;
  scriptRef: string;
};

const route = useRoute();
const router = useRouter();
const workflowName = computed(() => String(route.params.name || "workflow"));
const flowLink = computed(() => `/workflows/${workflowName.value}/flow`);
const yaml = ref(defaultStepsYaml(workflowName.value));
const inventoryYaml = ref(defaultInventoryYaml());
const workflowTitle = ref("");
const workflowDescription = ref("");
const metaEditing = ref(false);
const showTemplates = ref(false);
const savedAt = ref<Date | null>(null);
const saving = ref(false);
const pendingSave = ref(false);
const lastSaveOk = ref(true);
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
const scripts = ref<ScriptSummary[]>([]);
const scriptsLoading = ref(false);
const scriptsError = ref("");
const showAddStep = ref(false);
const showYamlEditor = ref(false);
const showInventory = ref(false);
const inventorySearch = ref("");
const newGroupName = ref("");
const newHostName = ref("");
const newHostAddress = ref("");
const hostDrafts = ref<Record<number, string>>({});
const runBusy = ref(false);
const runMessage = ref("");
const draftStep = ref<DraftStep>(resetDraftStep());
const envPreview = ref<EnvPackageDetail | null>(null);
const envPreviewLoading = ref(false);
const scriptPreview = ref<ScriptDetail | null>(null);
const scriptPreviewLoading = ref(false);
const stepDetail = ref<StepDetail | null>(null);
const stepDetailLoading = ref(false);
const draggingIndex = ref<number | null>(null);
const dragOverIndex = ref<number | null>(null);
const expandedSteps = ref<number[]>([]);
const envEditorIndex = ref<number | null>(null);
const envRows = ref<EnvRow[]>([]);
const targetPickerIndex = ref<number | null>(null);
const targetSelections = ref<string[]>([]);
const highlightedIndex = ref<number | null>(null);
const envSaveMessage = ref<Record<number, string>>({});

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

const supportedActions = [
  "cmd.run",
  "script.shell",
  "script.python",
  "env.set",
  "template.render",
  "pkg.install",
  "service.ensure",
  "service.restart"
];

const steps = computed(() => parseSteps(yaml.value));
const inventoryGroups = computed(() => parseInventoryGroups(inventoryYaml.value));
const inventoryHosts = computed(() => parseInventoryHosts(inventoryYaml.value));
const unsupportedActions = computed(() => {
  const actions = new Set<string>();
  for (const step of steps.value) {
    if (step.action && !isSupportedAction(step.action)) {
      actions.add(step.action);
    }
  }
  return Array.from(actions);
});
const stepValidations = computed(() => steps.value.map((step) => buildStepValidation(step)));
const filteredInventoryGroups = computed(() => {
  const query = inventorySearch.value.trim().toLowerCase();
  if (!query) {
    return inventoryGroups.value;
  }
  return inventoryGroups.value
    .map((group) => {
      const groupMatch = group.name.toLowerCase().includes(query);
      const hostsMatch = group.hosts.filter((host) =>
        host.name.toLowerCase().includes(query)
      );
      if (groupMatch) {
        return group;
      }
      if (hostsMatch.length) {
        return { ...group, hosts: hostsMatch };
      }
      return null;
    })
    .filter((group): group is InventoryGroup => Boolean(group));
});
const filteredInventoryHosts = computed(() => {
  const query = inventorySearch.value.trim().toLowerCase();
  if (!query) {
    return inventoryHosts.value;
  }
  return inventoryHosts.value.filter((host) => {
    return (
      host.name.toLowerCase().includes(query) ||
      host.address.toLowerCase().includes(query)
    );
  });
});
const statusText = computed(() => (validation.value.ok ? "校验通过" : "校验未通过"));
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

function onMetaTitleInput(event: Event) {
  workflowTitle.value = (event.target as HTMLInputElement).value;
}

function onMetaDescInput(event: Event) {
  workflowDescription.value = (event.target as HTMLTextAreaElement).value;
}

function applyMetaChanges() {
  metaEditing.value = false;
  const nextTitle = workflowTitle.value.trim();
  if (nextTitle) {
    yaml.value = replaceTopLevelValue(yaml.value, "name", nextTitle);
  }
  yaml.value = replaceTopLevelValue(yaml.value, "description", workflowDescription.value.trim());
}

function syncMetaFromYaml() {
  if (metaEditing.value) return;
  workflowTitle.value = readTopLevelValue(yaml.value, "name") || workflowName.value;
  workflowDescription.value = readTopLevelValue(yaml.value, "description");
}

function toggleInventory() {
  showInventory.value = !showInventory.value;
  if (!showInventory.value) {
    inventorySearch.value = "";
    newGroupName.value = "";
    newHostName.value = "";
    newHostAddress.value = "";
    hostDrafts.value = {};
  }
}

function addInventoryGroup() {
  const base = newGroupName.value.trim() || "new-group";
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) => {
    const name = ensureUniqueName(base, groups.map((group) => group.name));
    return [...groups, { name, hosts: [] }];
  });
  newGroupName.value = "";
}

function updateInventoryGroupName(groupIndex: number, value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) => {
    if (!groups[groupIndex]) return groups;
    const otherNames = groups
      .map((group, index) => (index === groupIndex ? "" : group.name))
      .filter(Boolean);
    groups[groupIndex].name = ensureUniqueName(trimmed, otherNames);
    return groups;
  });
}

function removeInventoryGroup(groupIndex: number) {
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) =>
    groups.filter((_, index) => index !== groupIndex)
  );
}

function addInventoryHost(groupIndex: number) {
  const draft = hostDrafts.value[groupIndex] || "";
  const trimmed = draft.trim();
  if (!trimmed) return;
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) => {
    if (!groups[groupIndex]) return groups;
    const hosts = groups[groupIndex].hosts;
    const nextName = ensureUniqueName(trimmed, hosts);
    groups[groupIndex].hosts = [...hosts, nextName];
    return groups;
  });
  hostDrafts.value = { ...hostDrafts.value, [groupIndex]: "" };
}

function updateInventoryHost(groupIndex: number, hostIndex: number, value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) => {
    if (!groups[groupIndex]) return groups;
    const hosts = groups[groupIndex].hosts;
    if (!hosts[hostIndex]) return groups;
    const otherHosts = hosts.filter((_, idx) => idx !== hostIndex);
    hosts[hostIndex] = ensureUniqueName(trimmed, otherHosts);
    groups[groupIndex].hosts = [...hosts];
    return groups;
  });
}

function removeInventoryHost(groupIndex: number, hostIndex: number) {
  inventoryYaml.value = updateInventoryGroups(inventoryYaml.value, (groups) => {
    if (!groups[groupIndex]) return groups;
    groups[groupIndex].hosts = groups[groupIndex].hosts.filter((_, idx) => idx !== hostIndex);
    return groups;
  });
}

function addInventoryHostItem() {
  const name = newHostName.value.trim();
  if (!name) return;
  const address = newHostAddress.value.trim();
  inventoryYaml.value = updateInventoryHosts(inventoryYaml.value, (hosts) => {
    const nextName = ensureUniqueName(name, hosts.map((host) => host.name));
    return [...hosts, { name: nextName, address }];
  });
  newHostName.value = "";
  newHostAddress.value = "";
}

function updateInventoryHostNameByIndex(index: number, value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  inventoryYaml.value = updateInventoryHosts(inventoryYaml.value, (hosts) => {
    if (!hosts[index]) return hosts;
    const other = hosts.filter((_, idx) => idx !== index).map((host) => host.name);
    hosts[index].name = ensureUniqueName(trimmed, other);
    return hosts;
  });
}

function updateInventoryHostAddressByIndex(index: number, value: string) {
  const trimmed = value.trim();
  inventoryYaml.value = updateInventoryHosts(inventoryYaml.value, (hosts) => {
    if (!hosts[index]) return hosts;
    hosts[index].address = trimmed;
    return hosts;
  });
}

function removeInventoryHostItem(index: number) {
  inventoryYaml.value = updateInventoryHosts(inventoryYaml.value, (hosts) =>
    hosts.filter((_, idx) => idx !== index)
  );
}

function ensureUniqueName(name: string, existing: string[]) {
  if (!existing.includes(name)) return name;
  let counter = 1;
  let next = `${name}-${counter}`;
  while (existing.includes(next)) {
    counter += 1;
    next = `${name}-${counter}`;
  }
  return next;
}

function validateInventoryBeforeRun() {
  const hostNames = new Set<string>();
  for (const host of inventoryHosts.value) {
    if (host.name) hostNames.add(host.name);
  }
  for (const group of inventoryGroups.value) {
    for (const host of group.hosts) {
      if (host) hostNames.add(host);
    }
  }
  if (hostNames.size === 0) {
    return "请先在“主机/分组”中配置至少一个主机。";
  }

  const groupMap = new Map<string, string[]>();
  for (const group of inventoryGroups.value) {
    groupMap.set(group.name, group.hosts || []);
  }

  const issues: string[] = [];
  for (const step of steps.value) {
    const targets = parseTargetList(step.targets);
    if (targets.length === 0) {
      continue;
    }
    for (const target of targets) {
      if (hostNames.has(target)) continue;
      if (groupMap.has(target)) {
        const hosts = groupMap.get(target) || [];
        if (!hosts.length) {
          issues.push(`分组 ${target} 没有配置主机`);
        }
        continue;
      }
      issues.push(`目标 ${target} 未在 inventory 中定义`);
    }
  }

  if (!issues.length) return "";
  return `请检查主机/分组配置：${issues.join("；")}`;
}

function parseTargetList(raw: string) {
  const trimmed = (raw || "").trim();
  if (!trimmed) return [];
  if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
    const inner = trimmed.slice(1, -1).trim();
    if (!inner) return [];
    return inner
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }
  if (trimmed.includes(",")) {
    return trimmed
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return [trimmed];
}

function toggleTemplates() {
  showTemplates.value = !showTemplates.value;
}

function insertTemplate(snippet: string) {
  yaml.value = `${yaml.value.trim()}\n\n${snippet}`;
  showTemplates.value = false;
}

function openAddStep() {
  draftStep.value = resetDraftStep();
  showAddStep.value = true;
}

function closeAddStep() {
  showAddStep.value = false;
}

function openYamlEditor() {
  showYamlEditor.value = true;
}

function closeYamlEditor() {
  showYamlEditor.value = false;
  showTemplates.value = false;
}

function confirmAddStep() {
  const snippet = buildStepSnippet(draftStep.value);
  yaml.value = appendStepBlock(yaml.value, snippet);
  showAddStep.value = false;
}

function updateStepName(index: number, value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  yaml.value = updateStepField(yaml.value, index, "name", trimmed);
}

function updateStepAction(index: number, value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  yaml.value = updateStepField(yaml.value, index, "action", trimmed);
}

function updateStepTargets(index: number, value: string) {
  yaml.value = updateStepField(yaml.value, index, "targets", value);
}

function formatTargetsForInput(value: string) {
  return value.replace(/[\[\]]/g, "").replace(/['"]/g, "").trim();
}

function getTargetInputValue(index: number, raw: string) {
  if (targetPickerIndex.value === index && targetSelections.value.length) {
    return targetSelections.value.join(", ");
  }
  return parseTargets(raw).join(", ");
}

function updateStepWithScalar(index: number, key: string, value: string) {
  yaml.value = updateStepWithField(yaml.value, index, key, value, false);
}

function updateStepWithScalarAllowEmpty(index: number, key: string, value: string) {
  yaml.value = updateStepWithField(yaml.value, index, key, value, false, true);
}

function updateStepWithMultiline(index: number, key: string, value: string) {
  yaml.value = updateStepWithField(yaml.value, index, key, value, true);
}

function getScriptSource(step: StepSummary) {
  if (step.scriptRefPresent) {
    return "library";
  }
  return "inline";
}

function setScriptSource(index: number, source: "inline" | "library") {
  if (source === "inline") {
    yaml.value = updateStepWithField(yaml.value, index, "script_ref", "", false);
  } else {
    const withoutScript = updateStepWithField(yaml.value, index, "script", "", true);
    yaml.value = updateStepWithField(withoutScript, index, "script_ref", "", false, true);
  }
}

function openTargetPicker(index: number) {
  const step = steps.value[index];
  targetSelections.value = step ? parseTargets(step.targets) : [];
  targetPickerIndex.value = index;
}

function closeTargetPicker() {
  targetPickerIndex.value = null;
  targetSelections.value = [];
}

function applyTargetSelection() {
  if (targetPickerIndex.value === null) return;
  const unique = normalizeTargets(targetSelections.value);
  const value = unique.join(", ");
  yaml.value = updateStepField(yaml.value, targetPickerIndex.value, "targets", value);
  closeTargetPicker();
}

function getQueryStepName() {
  const raw = route.query.step;
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw)) return raw[0] || "";
  return "";
}

function focusStepFromQuery() {
  const stepName = getQueryStepName();
  if (!stepName) {
    highlightedIndex.value = null;
    return;
  }
  const index = steps.value.findIndex((step) => step.name === stepName);
  if (index < 0) return;
  highlightedIndex.value = index;
  void nextTick(() => {
    const el = document.querySelector(`[data-step-index="${index}"]`) as HTMLElement | null;
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  });
}

function duplicateStep(index: number) {
  yaml.value = duplicateStepBlock(yaml.value, index);
}

function deleteStep(index: number) {
  yaml.value = deleteStepBlock(yaml.value, index);
}

function updateStepMeta(index: number, key: StepMetaField, value: string) {
  const formatter = key === "loop" || key === "notify" ? formatListLiteral : formatScalar;
  yaml.value = updateStepMetaField(yaml.value, index, key, value, formatter);
}

function toggleAdvanced(index: number) {
  const set = new Set(expandedSteps.value);
  if (set.has(index)) {
    set.delete(index);
    if (envEditorIndex.value === index) {
      envEditorIndex.value = null;
      envRows.value = [];
    }
  } else {
    set.add(index);
  }
  expandedSteps.value = Array.from(set);
}

function isAdvancedOpen(index: number) {
  return expandedSteps.value.includes(index);
}

function toggleEnvEditor(index: number) {
  if (envEditorIndex.value === index) {
    envEditorIndex.value = null;
    envRows.value = [];
    return;
  }
  const step = steps.value[index];
  envEditorIndex.value = index;
  envRows.value = buildEnvRows(step?.env);
}

function addEnvRow() {
  envRows.value = [...envRows.value, { key: "", value: "" }];
}

function removeEnvRow(rowIndex: number) {
  envRows.value = envRows.value.filter((_, index) => index !== rowIndex);
}

async function saveEnvRows(index: number) {
  if (envEditorIndex.value !== index) return;
  const env: Record<string, string> = {};
  for (const row of envRows.value) {
    const key = row.key.trim();
    if (!key) continue;
    env[key] = row.value;
  }
  const nextYaml = updateStepEnvBlock(yaml.value, index, env);
  if (nextYaml !== yaml.value) {
    yaml.value = nextYaml;
  }
  envRows.value = buildEnvRows(env);
  envSaveMessage.value = { ...envSaveMessage.value, [index]: "保存中..." };
  const saved = await saveYamlAndWait();
  envSaveMessage.value = {
    ...envSaveMessage.value,
    [index]: saved ? "已保存" : "保存失败"
  };
  window.setTimeout(() => {
    const next = { ...envSaveMessage.value };
    delete next[index];
    envSaveMessage.value = next;
  }, 1500);
}

function onDragStart(index: number, event: DragEvent) {
  draggingIndex.value = index;
  dragOverIndex.value = index;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", String(index));
  }
}

function onDragOver(index: number) {
  if (dragOverIndex.value !== index) {
    dragOverIndex.value = index;
  }
}

function onDragLeave() {
  dragOverIndex.value = null;
}

function onDragEnd() {
  draggingIndex.value = null;
  dragOverIndex.value = null;
}

function onDrop(index: number, event: DragEvent) {
  event.preventDefault();
  const fromRaw = event.dataTransfer?.getData("text/plain");
  const from = draggingIndex.value ?? (fromRaw ? Number(fromRaw) : Number.NaN);
  if (Number.isNaN(from)) {
    draggingIndex.value = null;
    dragOverIndex.value = null;
    return;
  }
  if (from !== index) {
    yaml.value = moveStepBlock(yaml.value, from, index);
  }
  draggingIndex.value = null;
  dragOverIndex.value = null;
}

function stripQuotes(value: string) {
  return value.replace(/^['"]|['"]$/g, "");
}

function isSupportedAction(action: string) {
  return supportedActions.includes(action);
}

function isAdvancedAction(action: string) {
  return (
    action === "template.render" ||
    action === "pkg.install" ||
    action === "service.ensure" ||
    action === "service.restart"
  );
}

function buildStepValidation(step: StepSummary): StepValidation {
  const missing: string[] = [];
  const messages: string[] = [];

  const addMissing = (key: string, message: string) => {
    if (!missing.includes(key)) {
      missing.push(key);
    }
    if (!messages.includes(message)) {
      messages.push(message);
    }
  };

  if (!step.name.trim()) {
    addMissing("step.name", "步骤名必填");
  }

  if (!formatTargetsForInput(step.targets)) {
    addMissing("step.targets", "目标必填");
  }

  if (!step.action) {
    addMissing("step.action", "动作必选");
    return { missing, messages };
  }

  if (!isSupportedAction(step.action)) {
    if (!messages.includes("未支持动作，请在高级编辑器修改")) {
      messages.push("未支持动作，请在高级编辑器修改");
    }
    return { missing, messages };
  }

  if (step.action === "cmd.run") {
    if (!step.cmd || !step.cmd.trim()) {
      addMissing("with.cmd", "命令必填");
    }
  } else if (step.action.startsWith("script.")) {
    if (step.scriptRefPresent) {
      if (!step.scriptRef) {
        addMissing("with.script_ref", "脚本库必选");
      }
    } else if (!step.script || !step.script.trim()) {
      addMissing("with.script", "脚本文本必填");
    }
  } else if (step.action === "env.set") {
    const count = step.env ? Object.keys(step.env).length : 0;
    if (!count) {
      addMissing("with.env", "环境变量必填");
    }
  } else if (step.action === "pkg.install") {
    if (!step.withName) {
      addMissing("with.name", "包名必填");
    }
  } else if (step.action === "template.render") {
    if (!step.src) {
      addMissing("with.src", "模板路径必填");
    }
    if (!step.dest) {
      addMissing("with.dest", "目标路径必填");
    }
  } else if (step.action === "service.ensure") {
    if (!step.withName) {
      addMissing("with.name", "服务名必填");
    }
    if (!step.state) {
      addMissing("with.state", "状态必填");
    }
  } else if (step.action === "service.restart") {
    if (!step.withName) {
      addMissing("with.name", "服务名必填");
    }
  }

  return { missing, messages };
}

function hasStepIssue(index: number, key: string) {
  const issues = stepValidations.value[index];
  if (!issues) return false;
  return issues.missing.includes(key);
}

function stepMessages(index: number) {
  return stepValidations.value[index]?.messages || [];
}

function buildEnvRows(env?: Record<string, string>) {
  if (!env) {
    return [{ key: "", value: "" }];
  }
  const entries = Object.entries(env);
  if (!entries.length) {
    return [{ key: "", value: "" }];
  }
  return entries.map(([key, value]) => ({ key, value }));
}

async function loadEnvPackages() {
  try {
    const data = await request<{ items: EnvPackageSummary[] }>("/envs");
    envPackages.value = data.items || [];
  } catch (err) {
    envPackages.value = [];
  }
}

async function loadScripts() {
  scriptsLoading.value = true;
  scriptsError.value = "";
  try {
    const data = await request<{ items: ScriptSummary[] }>("/scripts");
    scripts.value = data.items || [];
  } catch (err) {
    scriptsError.value = "脚本库加载失败";
    scripts.value = [];
  } finally {
    scriptsLoading.value = false;
  }
}

function getScriptOptions(action: string, selected?: string) {
  let language = "";
  if (action === "script.shell") {
    language = "shell";
  } else if (action === "script.python") {
    language = "python";
  }
  const list = scripts.value.filter((item) => !language || item.language === language);
  if (selected && !list.some((item) => item.name === selected)) {
    return [{ name: selected, language }, ...list];
  }
  return list;
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

async function previewEnvPackage() {
  if (!selectedEnvPackage.value) return;
  envPreviewLoading.value = true;
  envPreview.value = {
    name: selectedEnvPackage.value,
    env: {}
  };
  try {
    const data = await request<EnvPackageDetail>(`/envs/${selectedEnvPackage.value}`);
    envPreview.value = {
      name: data.name,
      description: data.description,
      env: data.env || {}
    };
  } catch (err) {
    envPreview.value = {
      name: selectedEnvPackage.value,
      env: {}
    };
  } finally {
    envPreviewLoading.value = false;
  }
}

function closeEnvPreview() {
  envPreview.value = null;
}

async function openScriptPreview(name: string) {
  if (!name) return;
  scriptPreviewLoading.value = true;
  scriptPreview.value = {
    name,
    language: "",
    content: ""
  };
  try {
    const data = await request<ScriptDetail>(`/scripts/${name}`);
    scriptPreview.value = {
      name: data.name,
      language: data.language || "",
      description: data.description,
      tags: data.tags || [],
      content: data.content || ""
    };
  } catch (err) {
    scriptPreview.value = {
      name,
      language: "",
      content: ""
    };
  } finally {
    scriptPreviewLoading.value = false;
  }
}

function closeScriptPreview() {
  scriptPreview.value = null;
  scriptPreviewLoading.value = false;
}

async function openStepDetail(index: number) {
  const step = steps.value[index];
  if (!step) return;
  let block = getStepBlock(yaml.value, index);
  if (!block && step.name) {
    block = getStepBlockByName(yaml.value, step.name);
  }
  if (!block) {
    block = "未找到步骤配置，请检查 YAML。";
  }
  stepDetailLoading.value = !!(step.scriptRef && !step.script);
  stepDetail.value = {
    name: step.name,
    action: step.action,
    targets: step.targets,
    yaml: block,
    scriptRef: step.scriptRef,
    scriptContent: step.script || ""
  };
  if (step.scriptRef && !step.script) {
    try {
      const data = await request<{ content?: string }>(`/scripts/${step.scriptRef}`);
      if (stepDetail.value) {
        stepDetail.value.scriptContent = data.content || "";
      }
    } catch (err) {
      if (stepDetail.value) {
        stepDetail.value.scriptContent = "";
      }
    }
    stepDetailLoading.value = false;
  }
}

function closeStepDetail() {
  stepDetail.value = null;
  stepDetailLoading.value = false;
}

async function planRun() {
  if (runBusy.value) return;
  const inventoryIssue = validateInventoryBeforeRun();
  if (inventoryIssue) {
    runMessage.value = inventoryIssue;
    return;
  }
  runBusy.value = true;
  runMessage.value = "正在生成计划...";
  try {
    await request(`/workflows/${workflowName.value}/plan`, { method: "POST" });
    runMessage.value = "计划完成";
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `计划失败: ${apiErr.message}` : "计划失败";
  } finally {
    runBusy.value = false;
  }
}

async function applyRun() {
  if (runBusy.value) return;
  const inventoryIssue = validateInventoryBeforeRun();
  if (inventoryIssue) {
    runMessage.value = inventoryIssue;
    return;
  }
  runBusy.value = true;
  runMessage.value = "执行中...";
  try {
    const data = await request<{ run_id: string }>(`/workflows/${workflowName.value}/apply`, {
      method: "POST"
    });
    runMessage.value = "执行已启动";
    if (data.run_id) {
      router.push(`/runs/${data.run_id}`);
    }
  } catch (err) {
    const apiErr = err as ApiError;
    runMessage.value = apiErr.message ? `执行失败: ${apiErr.message}` : "执行失败";
  } finally {
    runBusy.value = false;
  }
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

async function saveYaml(): Promise<boolean> {
  if (saving.value) {
    pendingSave.value = true;
    return false;
  }
  saving.value = true;
  let saved = false;
  try {
    await request(`/workflows/${workflowName.value}/steps`, {
      method: "PUT",
      body: { yaml: yaml.value }
    });
    await request(`/workflows/${workflowName.value}/inventory`, {
      method: "PUT",
      body: { yaml: inventoryYaml.value },
      headers: { "X-Workflow-Editor": "manual" }
    });
    savedAt.value = new Date();
    isDirty.value = false;
    saved = true;
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [
        apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败，请检查服务是否启动"
      ]
    };
    saved = false;
  } finally {
    saving.value = false;
  }
  lastSaveOk.value = saved;
  if (pendingSave.value) {
    pendingSave.value = false;
    return saveYaml();
  }
  return saved;
}

function delay(ms: number) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

async function waitForSaveIdle(timeoutMs = 3000) {
  const start = Date.now();
  while ((saving.value || pendingSave.value) && Date.now() - start < timeoutMs) {
    await delay(80);
  }
  return !saving.value && !pendingSave.value;
}

async function saveYamlAndWait() {
  await saveYaml();
  await waitForSaveIdle();
  return lastSaveOk.value;
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
  loadScripts();
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
  syncMetaFromYaml();
});

watch(inventoryYaml, () => {
  scheduleAutoSave();
});

onBeforeRouteLeave(async () => {
  cancelAutoSave();
  if (isDirty.value) {
    await saveYaml();
  }
});

watch(
  () => [steps.value.length, route.query.step],
  () => {
    focusStepFromQuery();
  }
);

async function loadYaml() {
  loading.value = true;
  try {
    const stepsData = await request<{ yaml: string }>(`/workflows/${workflowName.value}/steps`);
    yaml.value = stepsData.yaml || defaultStepsYaml(workflowName.value);
  } catch (err) {
    const apiErr = err as ApiError;
    if (apiErr.status === 404) {
      const stepsFallback = defaultStepsYaml(workflowName.value);
      yaml.value = stepsFallback;
      try {
        await request(`/workflows/${workflowName.value}/steps`, {
          method: "PUT",
          body: { yaml: stepsFallback }
        });
      } catch {
        validation.value = {
          ok: false,
          issues: ["创建默认 steps 失败，请检查服务是否启动"]
        };
      }
    } else {
      validation.value = { ok: false, issues: ["加载 steps 失败，请检查服务是否启动"] };
    }
  }

  try {
    const invData = await request<{ yaml: string }>(`/workflows/${workflowName.value}/inventory`, {
      headers: { "X-Workflow-Editor": "manual" }
    });
    inventoryYaml.value = invData.yaml || defaultInventoryYaml();
  } catch (err) {
    const apiErr = err as ApiError;
    if (apiErr.status === 404) {
      const invFallback = defaultInventoryYaml();
      inventoryYaml.value = invFallback;
      try {
        await request(`/workflows/${workflowName.value}/inventory`, {
          method: "PUT",
          body: { yaml: invFallback },
          headers: { "X-Workflow-Editor": "manual" }
        });
      } catch {
        validation.value = {
          ok: false,
          issues: ["创建默认 inventory 失败，请检查服务是否启动"]
        };
      }
    } else {
      validation.value = { ok: false, issues: ["加载 inventory 失败，请检查服务是否启动"] };
    }
  }

  savedAt.value = new Date();
  isDirty.value = false;
  errorLines.value = [];
  loading.value = false;
  await validateYaml();
}

function syncScroll() {
  if (!editorRef.value || !highlightInnerRef.value) return;
  const top = editorRef.value.scrollTop;
  const left = editorRef.value.scrollLeft;
  highlightInnerRef.value.style.transform = `translate(${-left}px, ${-top}px)`;
}

function buildStepSnippet(step: DraftStep) {
  const name = step.name.trim() || "new step";
  const targets = parseTargets(step.targets);
  const lines = [`  - name: ${name}`];
  if (targets.length) {
    lines.push(`    targets: [${targets.join(", ")}]`);
  }
  lines.push(`    action: ${step.action}`);
  lines.push("    with:");

  if (step.action === "cmd.run") {
    lines.push(...buildMultilineField("cmd", step.cmd || "echo hello"));
  } else if (step.action.startsWith("script.")) {
    if (step.scriptSource === "library") {
      const ref = step.scriptRef.trim() || "script-name";
      lines.push(`      script_ref: ${ref}`);
    } else {
      lines.push(...buildMultilineField("script", step.script || "echo \"hello\""));
    }
  } else if (step.action === "env.set") {
    lines.push("      env:");
    lines.push("        KEY: VALUE");
  } else if (step.action === "pkg.install") {
    lines.push("      name: package-name");
  } else if (step.action === "template.render") {
    lines.push("      src: template.j2");
    lines.push("      dest: /etc/example.conf");
  } else if (step.action.startsWith("service.")) {
    lines.push("      name: service-name");
    if (step.action === "service.ensure") {
      lines.push("      state: started");
    }
  }

  return lines;
}

function buildMultilineField(key: string, value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return [`      ${key}: ""`];
  }
  if (trimmed.includes("\n")) {
    const payload = trimmed.split(/\r?\n/).map((line) => `        ${line}`);
    return [`      ${key}: |`, ...payload];
  }
  return [`      ${key}: ${formatScalar(trimmed)}`];
}

function buildWithFieldLines(
  key: string,
  value: string,
  propIndent: string,
  multiline: boolean,
  allowEmpty = false
) {
  const fieldIndent = `${propIndent}  `;
  if (!value) {
    return allowEmpty ? [`${fieldIndent}${key}: ""`] : [];
  }
  if (multiline && value.includes("\n")) {
    const payload = value.split(/\r?\n/).map((line) => `${fieldIndent}  ${line}`);
    return [`${fieldIndent}${key}: |`, ...payload];
  }
  return [`${fieldIndent}${key}: ${formatScalar(value)}`];
}

function parseTargets(raw: string) {
  const cleaned = formatTargetsForInput(raw);
  return cleaned
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeTargets(values: string[]) {
  const unique: string[] = [];
  const seen = new Set<string>();
  for (const value of values) {
    const trimmed = value.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    unique.push(trimmed);
  }
  return unique;
}

function appendStepBlock(content: string, snippetLines: string[]) {
  const lines = content.split(/\r?\n/);
  const section = findStepsSection(lines);
  const snippet = [...snippetLines, ""];

  if (!section) {
    const next = [...lines, "", "steps:", ...snippet];
    return next.join("\n").trimEnd() + "\n";
  }

  const insertAt = section.end;
  const before = lines.slice(0, insertAt);
  const after = lines.slice(insertAt);
  const next = [...before, "", ...snippet, ...after];
  return next.join("\n").trimEnd() + "\n";
}

function updateStepField(content: string, index: number, field: "name" | "action" | "targets", value: string) {
  const lines = content.split(/\r?\n/);
  const section = findStepsSection(lines);
  if (!section) {
    return content;
  }
  const stepLines = collectStepLines(lines);
  if (index < 0 || index >= stepLines.length) {
    return content;
  }

  const start = stepLines[index] - 1;
  const end = index + 1 < stepLines.length ? stepLines[index + 1] - 1 : section.end;
  const block = lines.slice(start, end);
  const baseIndentMatch = block[0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const propIndent = `${baseIndent}  `;

  if (field === "name") {
    block[0] = `${baseIndent}- name: ${value}`;
  }

  if (field === "action") {
    const actionIndex = block.findIndex((line) => new RegExp(`^${propIndent}action\s*:`).test(line));
    if (actionIndex >= 0) {
      block[actionIndex] = `${propIndent}action: ${value}`;
    } else {
      block.splice(1, 0, `${propIndent}action: ${value}`);
    }
  }

  if (field === "targets") {
    const targetsIndex = block.findIndex((line) => new RegExp(`^${propIndent}targets\s*:`).test(line));
    const formatted = formatTargetsForInput(value);
    if (targetsIndex >= 0) {
      let removeCount = 0;
      for (let i = targetsIndex + 1; i < block.length; i += 1) {
        const line = block[i];
        if (line.trim() === "") {
          removeCount += 1;
          continue;
        }
        const indent = getIndent(line);
        if (indent <= propIndent.length) {
          break;
        }
        removeCount += 1;
      }
      if (removeCount) {
        block.splice(targetsIndex + 1, removeCount);
      }
    }
    if (!formatted) {
      if (targetsIndex >= 0) {
        block.splice(targetsIndex, 1);
      }
    } else {
      const targetsLine = `${propIndent}targets: [${formatted}]`;
      if (targetsIndex >= 0) {
        block[targetsIndex] = targetsLine;
      } else {
        block.splice(1, 0, targetsLine);
      }
    }
  }

  const next = [...lines.slice(0, start), ...block, ...lines.slice(end)];
  return next.join("\n");
}

function updateStepWithField(
  content: string,
  index: number,
  key: string,
  rawValue: string,
  multiline: boolean,
  allowEmpty = false
) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;

  const block = [...data.blocks[index]];
  const baseIndentMatch = block[0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const propIndent = `${baseIndent}  `;
  const withIndex = block.findIndex((line) => new RegExp(`^${propIndent}with\\s*:$`).test(line));
  const trimmed = rawValue.trim();
  const nextLines = buildWithFieldLines(key, trimmed, propIndent, multiline, allowEmpty);

  if (withIndex === -1) {
    if (!nextLines.length) return content;
    block.push(`${propIndent}with:`);
    block.push(...nextLines);
    const nextBlocks = [...data.blocks];
    nextBlocks[index] = block;
    return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
  }

  const withIndent = propIndent.length;
  let withEnd = withIndex + 1;
  while (withEnd < block.length) {
    const line = block[withEnd];
    if (line.trim() === "") {
      withEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= withIndent) {
      break;
    }
    withEnd += 1;
  }

  const fieldIndent = `${propIndent}  `;
  const fieldRegex = new RegExp(`^${fieldIndent}${escapeRegex(key)}\\s*:`);
  const fieldIndex = block.findIndex((line, idx) => idx > withIndex && idx < withEnd && fieldRegex.test(line));

  if (fieldIndex === -1) {
    if (nextLines.length) {
      block.splice(withEnd, 0, ...nextLines);
    }
  } else {
    let fieldEnd = fieldIndex + 1;
    while (fieldEnd < withEnd) {
      const line = block[fieldEnd];
      if (line.trim() === "") {
        fieldEnd += 1;
        continue;
      }
      const indent = getIndent(line);
      if (indent <= fieldIndent.length) {
        break;
      }
      fieldEnd += 1;
    }

    if (nextLines.length) {
      block.splice(fieldIndex, fieldEnd - fieldIndex, ...nextLines);
    } else {
      block.splice(fieldIndex, fieldEnd - fieldIndex);
    }
  }

  const nextBlocks = [...data.blocks];
  nextBlocks[index] = block;
  return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
}

function updateStepMetaField(
  content: string,
  index: number,
  key: StepMetaField,
  rawValue: string,
  formatter: (value: string) => string = formatScalar
) {
  const lines = content.split(/\r?\n/);
  const section = findStepsSection(lines);
  if (!section) {
    return content;
  }
  const stepLines = collectStepLines(lines);
  if (index < 0 || index >= stepLines.length) {
    return content;
  }

  const start = stepLines[index] - 1;
  const end = index + 1 < stepLines.length ? stepLines[index + 1] - 1 : section.end;
  const block = lines.slice(start, end);
  const baseIndentMatch = block[0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const propIndent = `${baseIndent}  `;
  const propRegex = new RegExp(`^${propIndent}${key}\\s*:`);
  const propIndex = block.findIndex((line) => propRegex.test(line));
  const value = rawValue.trim();

  if (!value) {
    if (propIndex >= 0) {
      block.splice(propIndex, 1);
    }
  } else {
    const formatted = formatter(value);
    if (formatted) {
      const line = `${propIndent}${key}: ${formatted}`;
      if (propIndex >= 0) {
        block[propIndex] = line;
      } else {
        const withIndex = block.findIndex((lineItem) =>
          new RegExp(`^${propIndent}with\\s*:`).test(lineItem)
        );
        const insertIndex = withIndex >= 0 ? withIndex : block.length;
        block.splice(insertIndex, 0, line);
      }
    }
  }

  const next = [...lines.slice(0, start), ...block, ...lines.slice(end)];
  return next.join("\n");
}

function updateStepEnvBlock(content: string, index: number, env: Record<string, string>) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;

  const block = [...data.blocks[index]];
  const baseIndentMatch = block[0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const propIndent = `${baseIndent}  `;
  const withIndent = propIndent.length;
  const withIndex = block.findIndex((line) => new RegExp(`^${propIndent}with\\s*:$`).test(line));

  const envEntries = Object.entries(env).filter(([key]) => key.trim() !== "");
  const envLines = envEntries.map(
    ([key, value]) => `${propIndent}    ${key}: ${formatScalar(value)}`
  );
  const envBlock = envEntries.length
    ? [`${propIndent}  env:`, ...envLines]
    : [];

  if (withIndex === -1) {
    if (!envBlock.length) return content;
    block.push(`${propIndent}with:`);
    block.push(...envBlock);
    const nextBlocks = [...data.blocks];
    nextBlocks[index] = block;
    return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
  }

  let withEnd = withIndex + 1;
  while (withEnd < block.length) {
    const line = block[withEnd];
    if (line.trim() === "") {
      withEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= withIndent) {
      break;
    }
    withEnd += 1;
  }

  const envIndent = `${propIndent}  `;
  const envIndex = block.findIndex(
    (line, idx) =>
      idx > withIndex &&
      idx < withEnd &&
      new RegExp(`^${envIndent}env\\s*:$`).test(line)
  );

  if (envIndex === -1) {
    if (envBlock.length) {
      block.splice(withEnd, 0, ...envBlock);
    }
  } else {
    let envEnd = envIndex + 1;
    while (envEnd < withEnd) {
      const line = block[envEnd];
      if (line.trim() === "") {
        envEnd += 1;
        continue;
      }
      const indent = getIndent(line);
      if (indent <= envIndent.length) {
        break;
      }
      envEnd += 1;
    }

    if (envBlock.length) {
      block.splice(envIndex, envEnd - envIndex, ...envBlock);
    } else {
      block.splice(envIndex, envEnd - envIndex);
    }
  }

  const nextBlocks = [...data.blocks];
  nextBlocks[index] = block;
  return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
}

function getStepBlock(content: string, index: number) {
  const lines = content.split(/\r?\n/);
  const section = findStepsSection(lines);
  if (!section) return "";
  const stepLines = collectStepLines(lines);
  if (index < 0 || index >= stepLines.length) {
    return "";
  }
  const start = stepLines[index] - 1;
  const end = index + 1 < stepLines.length ? stepLines[index + 1] - 1 : section.end;
  return lines.slice(start, end).join("\n").trimEnd();
}

function getStepBlockByName(content: string, name: string) {
  const lines = content.split(/\r?\n/);
  const nameRegex = new RegExp(`^\\s*-\\s*name\\s*:\\s*${escapeRegex(name)}\\s*$`);
  let start = -1;
  for (let i = 0; i < lines.length; i += 1) {
    if (nameRegex.test(lines[i])) {
      start = i;
      break;
    }
  }
  if (start < 0) return "";
  const baseIndent = getIndent(lines[start]);
  let end = start + 1;
  while (end < lines.length) {
    const line = lines[end];
    if (line.trim() === "") {
      end += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= baseIndent && /^\s*-\s*name\s*:/i.test(line)) {
      break;
    }
    end += 1;
  }
  return lines.slice(start, end).join("\n").trimEnd();
}

function duplicateStepBlock(content: string, index: number) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;
  const blocks = [...data.blocks];
  const copy = [...blocks[index]];
  blocks.splice(index + 1, 0, copy);
  return rebuildStepsSection(data.lines, blocks, data.sectionStart, data.sectionEnd);
}

function deleteStepBlock(content: string, index: number) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;
  const blocks = [...data.blocks];
  blocks.splice(index, 1);
  return rebuildStepsSection(data.lines, blocks, data.sectionStart, data.sectionEnd);
}

function moveStepBlock(content: string, fromIndex: number, toIndex: number) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (fromIndex < 0 || fromIndex >= data.blocks.length) return content;
  if (toIndex < 0 || toIndex >= data.blocks.length) return content;
  const blocks = [...data.blocks];
  const [block] = blocks.splice(fromIndex, 1);
  blocks.splice(toIndex, 0, block);
  return rebuildStepsSection(data.lines, blocks, data.sectionStart, data.sectionEnd);
}

function getStepBlocks(content: string) {
  const lines = content.split(/\r?\n/);
  const section = findStepsSection(lines);
  if (!section) return null;
  const stepLines = collectStepLines(lines);
  const blocks = stepLines.map((startLine, idx) => {
    const start = startLine - 1;
    const end = idx + 1 < stepLines.length ? stepLines[idx + 1] - 1 : section.end;
    return lines.slice(start, end);
  });
  return {
    lines,
    blocks,
    sectionStart: section.start,
    sectionEnd: section.end
  };
}

function rebuildStepsSection(
  lines: string[],
  blocks: string[][],
  sectionStart: number,
  sectionEnd: number
) {
  const trimmedBlocks = blocks.map((block) => trimBlock(block));
  if (trimmedBlocks.length === 0) {
    const next = [...lines.slice(0, sectionStart + 1), ...lines.slice(sectionEnd)];
    return next.join("\n").trimEnd() + "\n";
  }

  const body: string[] = [];
  trimmedBlocks.forEach((block, idx) => {
    if (idx > 0) {
      body.push("");
    }
    body.push(...block);
  });

  const next = [...lines.slice(0, sectionStart + 1), ...body, ...lines.slice(sectionEnd)];
  return next.join("\n").trimEnd() + "\n";
}

function trimBlock(block: string[]) {
  const next = [...block];
  while (next.length && next[0].trim() === "") {
    next.shift();
  }
  while (next.length && next[next.length - 1].trim() === "") {
    next.pop();
  }
  return next;
}

function getIndent(line: string) {
  const match = line.match(/^(\s*)/);
  return match ? match[1].length : 0;
}

function findStepsSection(lines: string[]) {
  const startIndex = lines.findIndex((line) => /^\s*steps\s*:\s*$/.test(line));
  if (startIndex === -1) return null;
  const sectionIndent = getIndent(lines[startIndex]);
  let endIndex = startIndex + 1;
  while (endIndex < lines.length) {
    const line = lines[endIndex];
    if (line.trim() === "") {
      endIndex += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= sectionIndent && /^\s*[a-zA-Z0-9_-]+\s*:/i.test(line)) {
      break;
    }
    endIndex += 1;
  }
  return { start: startIndex, end: endIndex };
}

function replaceTopLevelValue(content: string, key: string, value: string) {
  const lines = content.split(/\r?\n/);
  const index = lines.findIndex((line) => new RegExp(`^${key}\s*:`).test(line));
  const formatted = `${key}: ${formatScalar(value)}`;

  if (index === -1) {
    const insertIndex = findTopLevelKeyIndex(lines, "version") ?? 0;
    const next = [...lines.slice(0, insertIndex + 1), formatted, ...lines.slice(insertIndex + 1)];
    return next.join("\n").trimEnd() + "\n";
  }

  lines[index] = formatted;
  return lines.join("\n");
}

function readTopLevelValue(content: string, key: string) {
  const match = content.match(new RegExp(`^${key}\s*:\s*(.*)$`, "m"));
  if (!match) return "";
  return match[1].replace(/^"|"$/g, "").trim();
}

function formatScalar(value: string) {
  if (value === "") {
    return '""';
  }
  if (/[:#]/.test(value) || value.includes("\"")) {
    return JSON.stringify(value);
  }
  return value;
}

function formatListLiteral(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("[") || trimmed.startsWith("{")) {
    return trimmed;
  }
  if (trimmed.includes(",")) {
    const items = trimmed
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => formatScalar(item));
    if (!items.length) {
      return "";
    }
    return `[${items.join(", ")}]`;
  }
  return formatScalar(trimmed);
}

function resetDraftStep(): DraftStep {
  return {
    name: "",
    action: "cmd.run",
    targets: "",
    cmd: "",
    scriptSource: "inline",
    script: "",
    scriptRef: ""
  };
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
      const indent = getIndent(line);
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
      const indent = getIndent(line);
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
  return value.replace(/[.*+?^${}()|[\\]\\]/g, "\\$&");
}

function parseInventoryGroups(content: string): InventoryGroup[] {
  const groups = readInventoryGroups(content);
  return groups.map((group, index) => ({
    name: group.name,
    index,
    hosts: group.hosts.map((host, hostIndex) => ({
      name: host,
      index: hostIndex
    }))
  }));
}

function readInventoryGroups(content: string): InventoryGroupData[] {
  const lines = content.split(/\r?\n/);
  const groups: InventoryGroupData[] = [];
  let inInventory = false;
  let inventoryIndent = 0;
  let inGroups = false;
  let groupsIndent = 0;
  let currentGroup: InventoryGroupData | null = null;
  let inHosts = false;
  let hostsIndent = 0;

  for (const line of lines) {
    const inventoryMatch = line.match(/^(\s*)inventory\s*:\s*$/);
    if (inventoryMatch) {
      inInventory = true;
      inventoryIndent = inventoryMatch[1].length;
      inGroups = false;
      currentGroup = null;
      continue;
    }

    if (inInventory) {
      const indent = getIndent(line);
      if (indent <= inventoryIndent && line.trim() !== "") {
        inInventory = false;
        inGroups = false;
        currentGroup = null;
        inHosts = false;
        continue;
      }
    }

    if (!inInventory) {
      continue;
    }

    const groupsMatch = line.match(/^(\s*)groups\s*:\s*$/);
    if (groupsMatch) {
      inGroups = true;
      groupsIndent = groupsMatch[1].length;
      currentGroup = null;
      inHosts = false;
      continue;
    }

    if (inGroups) {
      const indent = getIndent(line);
      if (indent <= groupsIndent && line.trim() !== "") {
        inGroups = false;
        currentGroup = null;
        inHosts = false;
        continue;
      }

      const groupMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:\s*$/);
      if (groupMatch && indent === groupsIndent + 2) {
        currentGroup = { name: groupMatch[1], hosts: [] };
        groups.push(currentGroup);
        inHosts = false;
        continue;
      }

      if (currentGroup) {
        const hostsMatch = line.match(/^\s*hosts\s*:\s*$/);
        if (hostsMatch) {
          inHosts = true;
          hostsIndent = indent;
          continue;
        }

        if (inHosts) {
          if (line.trim() === "") {
            continue;
          }
          if (indent <= hostsIndent) {
            inHosts = false;
            continue;
          }
          const hostMatch = line.match(/^\s*-\s*([a-zA-Z0-9_.-]+)\s*$/);
          if (hostMatch) {
            currentGroup.hosts.push(hostMatch[1]);
          }
        }
      }
    }
  }

  return groups;
}

function updateInventoryGroups(
  content: string,
  updater: (groups: InventoryGroupData[]) => InventoryGroupData[]
) {
  const current = readInventoryGroups(content);
  const next = updater([...current]);
  return replaceInventoryGroups(content, next);
}

function replaceInventoryGroups(content: string, groups: InventoryGroupData[]) {
  const lines = content.split(/\r?\n/);
  const inventoryIndex = lines.findIndex((line) => /^inventory\s*:\s*$/.test(line));
  const buildGroupsBlock = (indentSize: number) => {
    const indent = " ".repeat(indentSize);
    if (!groups.length) {
      return [`${indent}groups: {}`];
    }
    const block: string[] = [`${indent}groups:`];
    for (const group of groups) {
      block.push(`${indent}  ${group.name}:`);
      if (group.hosts.length) {
        block.push(`${indent}    hosts:`);
        for (const host of group.hosts) {
          block.push(`${indent}      - ${host}`);
        }
      } else {
        block.push(`${indent}    hosts: []`);
      }
    }
    return block;
  };

  if (inventoryIndex === -1) {
    const block = ["inventory:", ...buildGroupsBlock(2)];
    const insertIndex =
      findTopLevelKeyIndex(lines, "vars") ??
      findTopLevelKeyIndex(lines, "plan") ??
      findTopLevelKeyIndex(lines, "steps") ??
      lines.length;
    const next = [...lines.slice(0, insertIndex), ...block, "", ...lines.slice(insertIndex)];
    return next.join("\n").trimEnd() + "\n";
  }

  const inventoryIndent = getIndent(lines[inventoryIndex]);
  let inventoryEnd = inventoryIndex + 1;
  while (inventoryEnd < lines.length) {
    const line = lines[inventoryEnd];
    if (line.trim() === "") {
      inventoryEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= inventoryIndent && /^[a-zA-Z0-9_-]+\s*:/i.test(line)) {
      break;
    }
    inventoryEnd += 1;
  }

  const groupsIndex = lines.findIndex(
    (line, idx) =>
      idx > inventoryIndex &&
      idx < inventoryEnd &&
      new RegExp(`^\\s*groups\\s*:\\s*$`).test(line)
  );
  const groupsIndent = inventoryIndent + 2;
  const nextGroupsBlock = buildGroupsBlock(groupsIndent);

  if (groupsIndex === -1) {
    const insertAt = inventoryIndex + 1;
    const next = [
      ...lines.slice(0, insertAt),
      ...nextGroupsBlock,
      "",
      ...lines.slice(insertAt)
    ];
    return next.join("\n").trimEnd() + "\n";
  }

  let groupsEnd = groupsIndex + 1;
  while (groupsEnd < inventoryEnd) {
    const line = lines[groupsEnd];
    if (line.trim() === "") {
      groupsEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= groupsIndent && line.trim() !== "") {
      break;
    }
    groupsEnd += 1;
  }

  const next = [
    ...lines.slice(0, groupsIndex),
    ...nextGroupsBlock,
    ...lines.slice(groupsEnd)
  ];
  return next.join("\n").trimEnd() + "\n";
}

function parseInventoryHosts(content: string): InventoryHostEntry[] {
  const hosts = readInventoryHosts(content);
  return hosts.map((host, index) => ({
    name: host.name,
    address: host.address,
    index
  }));
}

function readInventoryHosts(content: string): InventoryHostData[] {
  const lines = content.split(/\r?\n/);
  const hosts: InventoryHostData[] = [];
  let inInventory = false;
  let inventoryIndent = 0;
  let inHosts = false;
  let hostsIndent = 0;
  let currentHost: InventoryHostData | null = null;

  for (const line of lines) {
    const inventoryMatch = line.match(/^(\s*)inventory\s*:\s*$/);
    if (inventoryMatch) {
      inInventory = true;
      inventoryIndent = inventoryMatch[1].length;
      inHosts = false;
      currentHost = null;
      continue;
    }

    if (inInventory) {
      const indent = getIndent(line);
      if (indent <= inventoryIndent && line.trim() !== "") {
        inInventory = false;
        inHosts = false;
        currentHost = null;
        continue;
      }
    }

    if (!inInventory) {
      continue;
    }

    const hostsMatch = line.match(/^(\s*)hosts\s*:\s*$/);
    if (hostsMatch && hostsMatch[1].length === inventoryIndent + 2) {
      inHosts = true;
      hostsIndent = hostsMatch[1].length;
      currentHost = null;
      continue;
    }

    if (inHosts) {
      const indent = getIndent(line);
      if (indent <= hostsIndent && line.trim() !== "") {
        inHosts = false;
        currentHost = null;
        continue;
      }

      const hostMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:\s*$/);
      if (hostMatch && indent === hostsIndent + 2) {
        currentHost = { name: hostMatch[1], address: "" };
        hosts.push(currentHost);
        continue;
      }

      if (currentHost) {
        const addressMatch = line.match(/^\s*address\s*:\s*(.+)$/);
        if (addressMatch) {
          currentHost.address = stripQuotes(addressMatch[1].trim());
        }
      }
    }
  }

  return hosts;
}

function updateInventoryHosts(
  content: string,
  updater: (hosts: InventoryHostData[]) => InventoryHostData[]
) {
  const current = readInventoryHosts(content);
  const next = updater([...current]);
  return replaceInventoryHosts(content, next);
}

function replaceInventoryHosts(content: string, hosts: InventoryHostData[]) {
  const lines = content.split(/\r?\n/);
  const inventoryIndex = lines.findIndex((line) => /^inventory\s*:\s*$/.test(line));
  const buildHostsBlock = (indentSize: number) => {
    const indent = " ".repeat(indentSize);
    if (!hosts.length) {
      return [`${indent}hosts: {}`];
    }
    const block: string[] = [`${indent}hosts:`];
    for (const host of hosts) {
      block.push(`${indent}  ${host.name}:`);
      if (host.address) {
        block.push(`${indent}    address: ${formatScalar(host.address)}`);
      } else {
        block.push(`${indent}    address: ""`);
      }
    }
    return block;
  };

  if (inventoryIndex === -1) {
    const block = ["inventory:", ...buildHostsBlock(2)];
    const insertIndex =
      findTopLevelKeyIndex(lines, "vars") ??
      findTopLevelKeyIndex(lines, "plan") ??
      findTopLevelKeyIndex(lines, "steps") ??
      lines.length;
    const next = [...lines.slice(0, insertIndex), ...block, "", ...lines.slice(insertIndex)];
    return next.join("\n").trimEnd() + "\n";
  }

  const inventoryIndent = getIndent(lines[inventoryIndex]);
  let inventoryEnd = inventoryIndex + 1;
  while (inventoryEnd < lines.length) {
    const line = lines[inventoryEnd];
    if (line.trim() === "") {
      inventoryEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= inventoryIndent && /^[a-zA-Z0-9_-]+\s*:/i.test(line)) {
      break;
    }
    inventoryEnd += 1;
  }

  const hostsIndex = lines.findIndex(
    (line, idx) =>
      idx > inventoryIndex &&
      idx < inventoryEnd &&
      new RegExp(`^\\s*hosts\\s*:\\s*$`).test(line)
  );
  const hostsIndent = inventoryIndent + 2;
  const nextHostsBlock = buildHostsBlock(hostsIndent);

  if (hostsIndex === -1) {
    const insertAt = inventoryIndex + 1;
    const next = [
      ...lines.slice(0, insertAt),
      ...nextHostsBlock,
      "",
      ...lines.slice(insertAt)
    ];
    return next.join("\n").trimEnd() + "\n";
  }

  let hostsEnd = hostsIndex + 1;
  while (hostsEnd < inventoryEnd) {
    const line = lines[hostsEnd];
    if (line.trim() === "") {
      hostsEnd += 1;
      continue;
    }
    const indent = getIndent(line);
    if (indent <= hostsIndent && line.trim() !== "") {
      break;
    }
    hostsEnd += 1;
  }

  const next = [
    ...lines.slice(0, hostsIndex),
    ...nextHostsBlock,
    ...lines.slice(hostsEnd)
  ];
  return next.join("\n").trimEnd() + "\n";
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
      const indent = getIndent(line);
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
      findTopLevelKeyIndex(lines, "plan") ?? findTopLevelKeyIndex(lines, "steps") ?? lines.length;
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

function defaultStepsYaml(name: string) {
  return `version: v0.1
name: ${name}
description: install and config nginx

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

function defaultInventoryYaml() {
  return `inventory:
  groups:
    web:
      hosts:
        - web1
        - web2
  vars:
    ssh_user: ops
`;
}
</script>

<style scoped>
.studio {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: calc(100vh - 140px);
}

.panel {
  background: var(--panel);
  border-radius: var(--radius-lg);
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
}

.workflow-bar {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(280px, 0.8fr);
  gap: 18px;
}

.workflow-meta {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.title-input {
  flex: 1;
  min-width: 220px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  border-radius: 12px;
  padding: 10px 12px;
  font-size: 16px;
  font-weight: 600;
  background: #fff;
}

.desc-input {
  border: 1px solid rgba(27, 27, 27, 0.12);
  border-radius: 12px;
  padding: 10px 12px;
  background: #fff;
  font-size: 13px;
  resize: vertical;
}

.meta-hint {
  font-size: 12px;
  color: var(--muted);
}

.workflow-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.env-block {
  border: 1px dashed var(--grid);
  border-radius: 14px;
  padding: 12px;
  background: #fff;
}

.label {
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 8px;
}

.env-select {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.env-select select {
  border-radius: 10px;
  border: 1px solid var(--grid);
  padding: 6px 8px;
  font-size: 12px;
  background: #fff;
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

.action-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.run-message {
  font-size: 12px;
  color: var(--muted);
}

.save {
  margin-left: auto;
  color: var(--muted);
  font-size: 12px;
}

.studio-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 18px;
  flex: 1;
}

.steps-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.steps-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.steps-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.steps-header h2 {
  margin: 0;
}

.steps-header p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.step-card {
  display: grid;
  grid-template-columns: 38px 1fr;
  gap: 12px;
  border-radius: 14px;
  border: 1px solid var(--grid);
  padding: 12px;
  background: #fff;
}

.step-card.highlight {
  border-color: rgba(230, 167, 0, 0.6);
  box-shadow: 0 0 0 2px rgba(230, 167, 0, 0.2);
}

.step-card.over {
  border-color: rgba(232, 93, 42, 0.5);
  box-shadow: 0 0 0 2px rgba(232, 93, 42, 0.15);
}

.step-card.dragging {
  opacity: 0.6;
}

.step-index {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
}

.step-index span {
  width: 32px;
  height: 32px;
  border-radius: 999px;
  background: #fff3e8;
  color: var(--ink);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}

.drag-handle {
  border: 1px dashed rgba(27, 27, 27, 0.2);
  background: #fff;
  color: var(--muted);
  border-radius: 8px;
  padding: 4px 6px;
  font-size: 12px;
  cursor: grab;
}

.drag-handle:active {
  cursor: grabbing;
}

.step-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field input,
.field select,
.field textarea {
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #fff;
  font-size: 13px;
  color: var(--ink);
}

.field input.error,
.field select.error,
.field textarea.error {
  border-color: rgba(208, 52, 44, 0.6);
  box-shadow: 0 0 0 1px rgba(208, 52, 44, 0.3);
}

.target-input {
  display: flex;
  align-items: center;
  gap: 8px;
}

.target-input input {
  flex: 1;
}

.step-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  flex-wrap: wrap;
}

.warning-banner {
  border-radius: 12px;
  border: 1px solid rgba(232, 93, 42, 0.4);
  background: #fff3e8;
  padding: 10px 12px;
  font-size: 12px;
  color: #8f3a10;
}

.warning {
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid rgba(232, 93, 42, 0.5);
  background: #fff3e8;
  color: #8f3a10;
}

.advanced-hint {
  margin-top: -4px;
}

.step-errors {
  font-size: 12px;
  color: var(--err);
}

.env-set-panel {
  border-radius: 12px;
  border: 1px dashed var(--grid);
  padding: 10px;
  background: #fffaf5;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.env-set-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.env-set-panel.error {
  border-color: rgba(208, 52, 44, 0.4);
  box-shadow: inset 0 0 0 1px rgba(208, 52, 44, 0.2);
}

.env-set-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cmd-panel,
.script-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.script-library {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.script-library select {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 13px;
  background: #fff;
  color: var(--ink);
}

.script-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.script-panel textarea {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 13px;
  color: var(--ink);
  background: #fff;
}

.script-panel input {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 13px;
  color: var(--ink);
  background: #fff;
}

.step-advanced-toggle {
  display: flex;
  justify-content: flex-end;
}

.step-advanced {
  border-top: 1px dashed rgba(27, 27, 27, 0.12);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.advanced-action {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.advanced-title {
  font-size: 12px;
  color: var(--muted);
}

.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.advanced-env {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.env-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12px;
}

.env-editor {
  border: 1px dashed var(--grid);
  border-radius: 12px;
  padding: 10px;
  background: #fffaf5;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.env-editor .env-grid {
  gap: 8px;
}

.env-editor .env-header,
.env-editor .env-row {
  grid-template-columns: 1fr 1fr auto;
}

.env-editor input {
  border-radius: 8px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 6px 8px;
  font-size: 12px;
}

.env-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.pill {
  border-radius: 999px;
  padding: 4px 10px;
  border: 1px solid var(--grid);
  background: #faf6f0;
  font-size: 12px;
}

.required {
  padding: 4px 8px;
  border-radius: 10px;
  background: #fff3e8;
  border: 1px dashed rgba(232, 93, 42, 0.4);
  color: var(--ink);
}

.muted {
  color: var(--muted);
}

.aux-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: auto;
}

.aux-section {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 10px 12px;
  background: #fff;
}

.aux-section summary {
  cursor: pointer;
  font-weight: 600;
  margin-bottom: 10px;
}

.flow-canvas {
  position: relative;
  padding: 12px 12px 12px 38px;
  border-radius: 12px;
  border: 1px dashed var(--grid);
  background: linear-gradient(180deg, #fbfaf7 0%, #f5f1eb 100%);
  min-height: 180px;
}

.flow-canvas::before {
  content: "";
  position: absolute;
  left: 20px;
  top: 12px;
  bottom: 12px;
  width: 2px;
  background: linear-gradient(180deg, rgba(225, 221, 214, 0.4), rgba(225, 221, 214, 1));
}

.flow-node {
  position: relative;
  border-radius: 12px;
  border: 1px solid var(--grid);
  background: #ffffff;
  padding: 8px 12px;
  margin-bottom: 12px;
}

.flow-node::before {
  content: "";
  position: absolute;
  left: -26px;
  top: 14px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 0 0 6px rgba(232, 93, 42, 0.16);
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

.validation {
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 12px;
  border: 1px solid var(--grid);
  display: inline-flex;
  margin-bottom: 8px;
}

.validation.ok {
  color: var(--ok);
}

.validation.warn {
  color: var(--err);
}

.errors {
  padding-left: 18px;
  color: var(--err);
  margin: 0;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-title {
  font-size: 12px;
  color: var(--muted);
}

.yaml-preview {
  background: #0f0f0f;
  color: #f5f1ec;
  padding: 10px;
  border-radius: 10px;
  font-size: 12px;
  white-space: pre-wrap;
  max-height: 280px;
  min-height: 120px;
  overflow: auto;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.btn {
  border: 1px solid var(--ink);
  background: transparent;
  padding: 8px 14px;
  cursor: pointer;
  font-size: 12px;
  border-radius: 999px;
}

.btn.primary {
  background: var(--brand);
  color: #fff;
  border-color: var(--brand);
}

.btn.ghost {
  border-color: rgba(27, 27, 27, 0.2);
}

.btn.danger {
  border-color: rgba(208, 52, 44, 0.6);
  color: #c0392b;
}

.editor-wrap {
  min-height: 320px;
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

.env-preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.env-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.env-header,
.env-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  font-size: 12px;
}

.env-header {
  color: var(--muted);
}

.inventory-toolbar {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.inventory-search {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 12px;
}

.inventory-add {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.inventory-input {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 8px 10px;
  font-size: 12px;
  flex: 1;
  min-width: 160px;
}

.inventory-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.inventory-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title {
  font-size: 12px;
  color: var(--muted);
  font-weight: 600;
}

.inventory-hosts {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.inventory-group {
  border-radius: 12px;
  border: 1px solid var(--grid);
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #fffaf5;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.host-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.host-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
}

.host-add {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
}

.inventory-hosts .host-row {
  grid-template-columns: 1fr 1fr auto;
}

.inventory-host-add {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

.target-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 360px;
  overflow: auto;
}

.target-group {
  border-radius: 12px;
  border: 1px solid var(--grid);
  padding: 10px;
  background: #fff;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.target-option {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.target-hosts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 6px;
  padding-left: 24px;
}

.target-section-title {
  font-size: 12px;
  color: var(--muted);
  font-weight: 600;
}

.target-tag {
  border-radius: 999px;
  border: 1px solid var(--grid);
  padding: 2px 8px;
  font-size: 11px;
  background: #faf6f0;
  color: var(--muted);
}

.target-tag.ghost {
  background: #f5f1eb;
}

.modal {
  position: fixed;
  inset: 0;
  background: rgba(17, 17, 17, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  z-index: 40;
}

.modal-card {
  background: #fff;
  border-radius: 16px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  padding: 18px;
  width: min(520px, 100%);
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: var(--shadow);
}

.editor-modal {
  width: min(960px, 100%);
  height: min(86vh, 820px);
  max-height: 86vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.editor-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.editor-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.editor-modal .editor-wrap {
  flex: 1;
  min-height: 0;
}

.editor-validation {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 140px;
  overflow: auto;
}

.editor-modal .editor {
  height: 100%;
  min-height: 0;
}

.inline-choice {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 1100px) {
  .workflow-bar {
    grid-template-columns: 1fr;
  }
  .studio-body {
    grid-template-columns: 1fr;
  }
  .step-row {
    grid-template-columns: 1fr;
  }
  .advanced-grid {
    grid-template-columns: 1fr;
  }
}
</style>
