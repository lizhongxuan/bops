<template>
  <section class="home-ai">
    <div class="main-grid">
      <section class="panel chat-panel">
        <div class="panel-head chat-head">
          <div>
            <h2>工作流AI助手</h2>
            <p>AI 负责拆解需求并生成草稿，你只需确认关键细节。当前会话：{{ chatSessionTitle || '新会话' }}</p>
          </div>
          <div class="status-tag" :class="streamError ? 'error' : busy || chatPending ? 'busy' : 'idle'">
            {{ streamError ? '异常' : busy ? '生成中' : chatPending ? '对话中' : '就绪' }}
          </div>
        </div>

        <div class="chat-body">
          <ul class="timeline">
            <li v-for="entry in timelineEntries" :key="entry.id" :class="['timeline-item', entry.type]">
              <div class="timeline-header">
                <span class="timeline-badge" :class="entry.type">{{ entry.label }}</span>
                <small v-if="entry.extra">{{ entry.extra }}</small>
              </div>
              <p v-if="entry.body">{{ entry.body }}</p>
              <div v-if="entry.actionLabel" class="timeline-actions">
                <button class="btn ghost btn-sm" type="button" @click="handleEntryAction(entry.action)">
                  {{ entry.actionLabel }}
                </button>
              </div>
            </li>
            <li v-if="chatPending" class="timeline-item typing">
              <div class="timeline-header">
                <span class="timeline-badge ai">AI</span>
                <small>...</small>
              </div>
              <p>正在回复...</p>
            </li>
          </ul>
        </div>

        <div v-if="pendingQuestions.length" class="pending-questions">
          <div class="pending-title">还需要确认</div>
          <div class="pending-chips">
            <button
              v-for="question in pendingQuestions"
              :key="question"
              class="chip"
              type="button"
              @click="applySuggestion(question)"
            >
              {{ question }}
            </button>
          </div>
        </div>

        <div class="composer">
          <div class="chat-toolbar">
            <button class="btn ghost btn-sm" type="button" @click="createChatSession">
              新会话
            </button>
            <button class="btn ghost btn-sm" type="button" @click="showConfigModal = true">
              环境配置
            </button>
            <button class="btn btn-sm" type="button" :disabled="busy || !canFix" @click="runFix">
              修复
            </button>
            <button class="btn btn-sm" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
              沙箱验证
            </button>
          </div>
          <textarea
            v-model="prompt"
            placeholder="描述需求，例如：在 web1/web2 上安装 nginx，渲染配置并启动服务"
            rows="4"
          ></textarea>
          <div v-if="showExamples" class="example-row">
            <button
              v-for="item in examples"
              :key="item"
              class="chip"
              type="button"
              @click="applyExample(item)"
            >
              {{ item }}
            </button>
          </div>
          <div class="composer-footer">
            <button
              class="btn primary btn-sm"
              type="button"
              :disabled="busy || !prompt.trim()"
              @click="startStream"
            >
              发送
            </button>
            <button class="btn ghost btn-sm" type="button" @click="toggleExamples">示例</button>
            <button class="btn ghost btn-sm" type="button" :disabled="busy" @click="clearPrompt">
              清空
            </button>
          </div>
        </div>
      </section>

      <section class="panel workspace-panel">
        <div class="workspace-tabs">
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'visual' }"
            @click="workspaceTab = 'visual'"
          >
            可视化
          </button>
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'yaml' }"
            @click="workspaceTab = 'yaml'"
          >
            YAML
          </button>
          <button
            type="button"
            class="tab"
            :class="{ active: workspaceTab === 'validate' }"
            @click="workspaceTab = 'validate'"
          >
            校验与执行
          </button>
        </div>
        <div class="workspace-toolbar">
          <div class="sync-controls">
            <span class="sync-label">同步</span>
            <button
              class="toggle-btn"
              type="button"
              :class="autoSync ? 'on' : 'off'"
              @click="toggleAutoSync"
            >
              {{ autoSync ? '自动同步' : '同步关闭' }}
            </button>
            <button v-if="!autoSync && visualDirty" class="btn btn-sm" type="button" @click="applyVisualToYaml">
              应用到 YAML
            </button>
            <button v-if="!autoSync && yamlDirty" class="btn btn-sm" type="button" @click="syncVisualFromYaml">
              从 YAML 更新
            </button>
            <span v-if="!autoSync && visualDirty" class="sync-tag warn">视觉未同步</span>
            <span v-if="!autoSync && yamlDirty" class="sync-tag warn">YAML 已更新</span>
          </div>
          <button
            class="btn ghost btn-sm"
            type="button"
            :disabled="!historyTimeline.length"
            @click="showHistoryModal = true"
          >
            草稿历史
          </button>
          <div class="status-tag" :class="validation.ok ? 'ok' : 'warn'">
            {{ validation.ok ? '校验通过' : '待修复' }}
          </div>
        </div>

        <div v-if="workspaceTab === 'visual'" class="tab-panel">
          <div class="visual-grid">
            <div class="steps-section">
              <div class="steps-head">
                <div class="steps-head-left">
                  <button class="btn secondary btn-sm" type="button" @click="appendStep(newStepAction)">
                    新增步骤
                  </button>
                </div>
                <div class="steps-head-right">
                  <span class="step-count">{{ steps.length }} 步</span>
                  <select v-model="newStepAction" class="step-action-select">
                    <option value="cmd.run">cmd.run</option>
                    <option value="pkg.install">pkg.install</option>
                    <option value="template.render">template.render</option>
                    <option value="service.ensure">service.ensure</option>
                    <option value="script.shell">script.shell</option>
                    <option value="script.python">script.python</option>
                    <option value="env.set">env.set</option>
                  </select>
                </div>
              </div>
              <div v-if="steps.length" class="steps-list">
                <div
                  class="step-card"
                  v-for="(step, index) in steps"
                  :key="step.name || `step-${index}`"
                  :class="{
                    active: selectedStepIndex === index,
                    error: canShowIssues && stepIssueIndexes.includes(index)
                  }"
                  role="button"
                  tabindex="0"
                  @click="selectStep(index)"
                  @dblclick="openStepYamlModal(index)"
                >
                  <div class="step-card-head">
                    <div>
                      <div class="step-name">{{ step.name }}</div>
                      <div class="step-meta">{{ step.action || '未指定动作' }}</div>
                    </div>
                    <span class="step-status" :class="stepStatusClass(index)">
                      {{ stepStatus(index) }}
                    </span>
                  </div>
                  <div class="step-summary" v-if="step.required">{{ step.required }}</div>
                  <div class="step-actions">
                    <button class="btn btn-sm" type="button" @click.stop="openStepDetailModal(index)">
                      详情
                    </button>
                    <button class="btn ghost btn-sm" type="button" @click.stop="duplicateStep(index)">
                      复制
                    </button>
                    <button class="btn btn-sm" type="button" @click.stop="openStepYamlModal(index)">
                      编辑 YAML
                    </button>
                    <button class="btn danger btn-sm" type="button" @click.stop="removeStep(index)">
                      删除
                    </button>
                  </div>
                </div>
              </div>
              <div v-else class="empty">尚未解析到步骤，生成草稿获取可视化内容。</div>
            </div>
          </div>
        </div>

        <div v-else-if="workspaceTab === 'yaml'" class="tab-panel">
          <textarea ref="yamlRef" v-model="yaml" spellcheck="false" class="code" rows="20"></textarea>
          <div class="yaml-actions">
            <button class="btn" type="button" :disabled="validationBusy || !yaml.trim()" @click="validateDraft">
              校验
            </button>
            <button class="btn" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
              沙箱验证
            </button>
            <button
              class="btn primary"
              type="button"
              :disabled="saveBusy || !yaml.trim() || requiresConfirm"
              @click="openSaveModal"
            >
              保存为工作流
            </button>
          </div>
        </div>

        <div v-else class="tab-panel validation-panel">
          <div class="validation-actions">
            <button class="btn" type="button" :disabled="validationBusy || !yaml.trim()" @click="validateDraft">
              校验
            </button>
            <button class="btn" type="button" :disabled="executeBusy || !yaml.trim()" @click="runExecution">
              沙箱验证
            </button>
          </div>
          <div class="alert" :class="validation.ok ? 'ok' : 'warn'">
            {{ validation.ok ? '校验通过' : '校验未通过' }}
          </div>
          <ul class="issues" v-if="validation.issues.length">
            <li v-for="issue in validation.issues" :key="issue">{{ issue }}</li>
          </ul>

          <div class="human-gate" v-if="summary.needsReview">
            <div class="gate-copy">检测到风险或校验失败，需要人工确认后才能保存。</div>
            <div v-if="requiresReason" class="gate-reason">
              <label>确认原因</label>
              <input v-model="confirmReason" type="text" placeholder="填写原因" />
            </div>
            <div class="gate-actions">
              <button
                class="btn ghost"
                type="button"
                :disabled="requiresReason && !confirmReason.trim() && !humanConfirmed"
                @click="humanConfirmed = !humanConfirmed"
              >
                {{ humanConfirmed ? '已确认' : '人工确认' }}
              </button>
            </div>
          </div>

          <div class="progress-list compact">
            <div v-if="progressEvents.length === 0" class="empty">等待生成…</div>
            <div
              class="progress-item"
              v-else
              v-for="(evt, index) in progressEvents"
              :key="`${evt.node}-${index}`"
            >
              <div class="node">{{ formatNode(evt.node) }}</div>
              <div class="status" :class="evt.status">{{ evt.status }}</div>
              <div class="message" v-if="evt.message">{{ evt.message }}</div>
            </div>
          </div>

          <div v-if="executeResult" class="execution-result" :class="executeResult.status">
            <div class="result-title">
              执行结果: {{ executeResult.status }}
              <span v-if="executeResult.code">(code {{ executeResult.code }})</span>
            </div>
            <div v-if="executeResult.error" class="result-error">{{ executeResult.error }}</div>
            <div class="result-io">
              <div v-if="executeResult.stdout" class="result-block">
                <div class="result-label">stdout</div>
                <pre>{{ executeResult.stdout }}</pre>
              </div>
              <div v-if="executeResult.stderr" class="result-block">
                <div class="result-label">stderr</div>
                <pre>{{ executeResult.stderr }}</pre>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
    <div v-if="showConfigModal" class="modal-backdrop" @click.self="showConfigModal = false">
      <div class="config-modal">
        <div class="modal-head">
          <h3>环境配置</h3>
          <button class="modal-close" type="button" @click="showConfigModal = false">&#10005;</button>
        </div>
        <div class="modal-grid form-grid">
          <div class="form-field">
            <label>目标环境</label>
            <input v-model="environmentNote" type="text" placeholder="例如 Ubuntu 22.04 / macOS M1" />
          </div>
          <div class="form-field">
            <label>执行策略</label>
            <select v-model="planMode">
              <option value="manual-approve">manual-approve</option>
              <option value="auto">auto</option>
            </select>
          </div>
          <div class="form-field">
            <label>环境变量包</label>
            <div class="select-row">
              <div class="select-value">
                <div v-if="selectedEnvPackages.length" class="chip-row">
                  <span class="chip" v-for="name in selectedEnvPackages" :key="name">
                    {{ name }}
                    <button class="chip-remove" type="button" @click="removeEnvPackage(name)">×</button>
                  </span>
                </div>
                <span v-else class="empty">无</span>
              </div>
              <button class="btn btn-sm" type="button" @click="openEnvPackageModal">选择</button>
            </div>
          </div>
          <div class="form-field">
            <label>最大修复次数</label>
            <input v-model.number="maxRetries" type="number" min="0" max="5" />
          </div>
          <div class="form-field">
            <label>验证环境</label>
            <div class="select-row">
              <span class="select-value">{{ selectedValidationEnv || "无" }}</span>
              <button
                class="btn btn-sm"
                type="button"
                :disabled="!validationEnvs.length"
                @click="openValidationEnvModal"
              >
                选择
              </button>
            </div>
          </div>
        </div>
        <div class="toggle-row">
          <span>自动执行验证</span>
          <button
            class="toggle-btn"
            type="button"
            :class="executeEnabled ? 'on' : 'off'"
            @click="executeEnabled = !executeEnabled"
          >
            {{ executeEnabled ? '启用' : '关闭' }}
          </button>
        </div>
        <div class="modal-actions">
          <button class="btn primary btn-sm" type="button" @click="showConfigModal = false">完成</button>
        </div>
      </div>
    </div>
    <div v-if="showEnvPackageModal" class="modal-backdrop" @click.self="closeEnvPackageModal">
      <div class="config-modal">
        <div class="modal-head">
          <h3>选择环境变量包</h3>
          <button class="modal-close" type="button" @click="closeEnvPackageModal">&#10005;</button>
        </div>
        <div v-if="envPackageOptions.length" class="option-list">
          <label class="option-item" v-for="pkg in envPackageOptions" :key="pkg.name">
            <input type="checkbox" :value="pkg.name" v-model="envPackageDraft" />
            <div>
              <div class="option-title">{{ pkg.name }}</div>
              <div v-if="pkg.description" class="option-desc">{{ pkg.description }}</div>
            </div>
          </label>
        </div>
        <div v-else class="empty">暂无环境变量包</div>
        <div class="modal-actions">
          <button class="btn ghost btn-sm" type="button" @click="closeEnvPackageModal">取消</button>
          <button class="btn primary btn-sm" type="button" @click="applyEnvPackageSelection">确认</button>
        </div>
      </div>
    </div>
    <div v-if="showValidationEnvModal" class="modal-backdrop" @click.self="closeValidationEnvModal">
      <div class="config-modal">
        <div class="modal-head">
          <h3>选择验证环境</h3>
          <button class="modal-close" type="button" @click="closeValidationEnvModal">&#10005;</button>
        </div>
        <div v-if="validationEnvs.length" class="option-list">
          <label class="option-item">
            <input type="radio" value="" v-model="validationEnvDraft" />
            <div class="option-title">无</div>
          </label>
          <label class="option-item" v-for="env in validationEnvs" :key="env.name">
            <input type="radio" :value="env.name" v-model="validationEnvDraft" />
            <div class="option-title">{{ env.name }}</div>
          </label>
        </div>
        <div v-else class="empty">暂无验证环境</div>
        <div class="modal-actions">
          <button class="btn ghost btn-sm" type="button" @click="closeValidationEnvModal">取消</button>
          <button class="btn primary btn-sm" type="button" @click="applyValidationEnvSelection">确认</button>
        </div>
      </div>
    </div>
    <div v-if="showHistoryModal" class="modal-backdrop" @click.self="showHistoryModal = false">
      <div class="history-modal">
        <div class="modal-head">
          <h3>草稿历史</h3>
          <button class="modal-close" type="button" @click="showHistoryModal = false">&#10005;</button>
        </div>
        <div v-if="historyTimeline.length" class="history-list">
          <button
            class="history-item"
            v-for="item in historyTimeline"
            :key="item.index"
            type="button"
            @click="restoreHistory(item.index)"
          >
            <div>
              <div class="history-title">{{ item.label }}</div>
              <div class="history-diff">{{ item.diff }}</div>
            </div>
            <span class="history-restore">恢复</span>
          </button>
        </div>
        <div v-else class="empty">暂无草稿历史</div>
      </div>
    </div>
    <div v-if="showSessionModal" class="modal-backdrop" @click.self="showSessionModal = false">
      <div class="history-modal session-modal">
        <div class="modal-head">
          <h3>聊天会话</h3>
          <button class="modal-close" type="button" @click="showSessionModal = false">&#10005;</button>
        </div>
        <div class="session-actions">
          <button class="btn primary btn-sm" type="button" @click="createChatSession">
            新建会话
          </button>
        </div>
        <div v-if="chatSessions.length" class="session-list">
          <button
            class="history-item"
            v-for="session in chatSessions"
            :key="session.id"
            type="button"
            :class="{ active: session.id === chatSessionId }"
            @click="selectChatSession(session.id)"
          >
            <div>
              <div class="history-title">{{ session.title || '新会话' }}</div>
              <div class="session-meta">
                {{ formatSessionTime(session.updated_at) }} · {{ session.message_count || 0 }} 条
              </div>
            </div>
            <span class="history-restore">恢复</span>
          </button>
        </div>
        <div v-else class="empty">暂无聊天会话</div>
      </div>
    </div>
    <div v-if="showStepDetailModal" class="modal-backdrop" @click.self="closeStepDetailModal">
      <div class="detail-modal">
        <div class="modal-head">
          <div class="detail-title">
            <h3>步骤详情</h3>
            <span v-if="detailStepIndex !== null" class="step-status" :class="stepStatusClass(detailStepIndex)">
              {{ stepStatus(detailStepIndex) }}
            </span>
          </div>
          <button class="modal-close" type="button" @click="closeStepDetailModal">&#10005;</button>
        </div>
        <div v-if="detailStepIndex !== null && draftSteps[detailStepIndex]" class="detail-body">
          <StepDetailForm
            :step="draftSteps[detailStepIndex]"
            @update-step="updateStepFromDraft(detailStepIndex, $event)"
          />
        </div>
        <div v-else class="empty">选择一个步骤进行编辑。</div>
        <div class="detail-actions">
          <button class="btn btn-sm" type="button" :disabled="detailStepIndex === null" @click="openStepYamlFromDetail">
            编辑 YAML
          </button>
          <button class="btn ghost btn-sm" type="button" :disabled="detailStepIndex === null" @click="duplicateStepFromDetail">
            复制
          </button>
          <button class="btn danger btn-sm" type="button" :disabled="detailStepIndex === null" @click="removeStepFromDetail">
            删除
          </button>
        </div>
      </div>
    </div>
    <div v-if="showStepYamlModal" class="modal-backdrop" @click.self="closeStepYamlModal">
      <div class="yaml-modal">
        <div class="modal-head">
          <h3>步骤 YAML</h3>
          <button class="modal-close" type="button" @click="closeStepYamlModal">&#10005;</button>
        </div>
        <p class="modal-summary">
          编辑步骤片段后保存，会同步回可视化{{ autoSync ? "与 YAML" : "" }}。
        </p>
        <div v-if="!autoSync" class="sync-note">自动同步已关闭，仅更新可视化副本。</div>
        <textarea v-model="stepYamlText" spellcheck="false" rows="12" class="code"></textarea>
        <div v-if="stepYamlError" class="alert warn">{{ stepYamlError }}</div>
        <div class="modal-actions">
          <button class="btn ghost btn-sm" type="button" @click="focusYamlFromModal">定位到 YAML</button>
          <button class="btn primary btn-sm" type="button" @click="applyStepYamlChanges">应用</button>
        </div>
      </div>
    </div>
    <div v-if="showSaveModal" class="modal-backdrop" @click.self="closeSaveModal">
      <form class="save-modal" @submit.prevent="saveWorkflow(saveName)">
        <div class="modal-head">
          <h3>保存为工作流</h3>
          <button class="modal-close" type="button" @click="closeSaveModal">&#10005;</button>
        </div>
        <div class="form-field">
          <label>工作流名称</label>
          <input v-model="saveName" type="text" placeholder="例如：web-nginx-setup" />
          <span class="field-hint">仅支持字母、数字、短横线、下划线。</span>
        </div>
        <div v-if="saveError" class="alert warn">{{ saveError }}</div>
        <div class="modal-actions">
          <button class="btn ghost btn-sm" type="button" @click="closeSaveModal">取消</button>
          <button class="btn primary btn-sm" type="submit" :disabled="saveBusy">
            {{ saveBusy ? "保存中..." : "保存" }}
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { ApiError, apiBase, request } from "../lib/api";
import StepDetailForm from "../components/StepDetailForm.vue";
import { normalizeQuestions, resolveQuestions } from "../lib/ai-questions";
import { createDefaultStepWith, type DraftStep } from "../lib/draft";
import { parseSteps, type StepSummary } from "../lib/workflowSteps";

type ValidationEnvSummary = {
  name: string;
};

type EnvPackageSummary = {
  name: string;
  description?: string;
};

type ValidationState = {
  ok: boolean;
  issues: string[];
};

type ExecutionResult = {
  status: string;
  stdout?: string;
  stderr?: string;
  code?: number;
  error?: string;
};

type ProgressEvent = {
  node: string;
  status: string;
  message?: string;
};

type ChatEntry = {
  id: string;
  label: string;
  body: string;
  type: string;
  extra?: string;
  action?: "fix";
  actionLabel?: string;
};

type ChatSessionMessage = {
  role: string;
  content: string;
};

type ChatSession = {
  id: string;
  title: string;
  created_at?: string;
  updated_at?: string;
  messages: ChatSessionMessage[];
};

type ChatSessionSummary = {
  id: string;
  title: string;
  updated_at?: string;
  message_count?: number;
};

type SummaryState = {
  summary: string;
  steps: number;
  riskLevel: string;
  riskNotes: string[];
  issues: string[];
  needsReview: boolean;
};

type SummaryResponse = {
  summary?: string;
  steps?: number;
  risk_level?: string;
  riskLevel?: string;
  risk_notes?: string[];
  riskNotes?: string[];
  issues?: string[];
  needs_review?: boolean;
  needsReview?: boolean;
};

type HistoryEntry = {
  index: number;
  label: string;
  diff: string;
};

const prompt = ref("");
const yaml = ref("");
const yamlRef = ref<HTMLTextAreaElement | null>(null);
const autoSync = ref(true);
const visualYaml = ref("");
const visualDirty = ref(false);
const yamlDirty = ref(false);
const busy = ref(false);
const streamError = ref("");
const progressEvents = ref<ProgressEvent[]>([]);
const chatEntries = ref<ChatEntry[]>([
  {
    id: "welcome",
    label: "AI",
    body: "你好！告诉我你的需求，我会拆解成可执行工作流，并主动追问缺失细节。",
    type: "ai"
  }
]);
const chatPending = ref(false);
const chatSessions = ref<ChatSessionSummary[]>([]);
const chatSessionId = ref("");
const chatSessionTitle = ref("");
const selectedStepIndex = ref<number | null>(null);
const stepIssueIndexes = ref<number[]>([]);
const draftId = ref("");
const history = ref<string[]>([]);
const validation = ref<ValidationState>({ ok: true, issues: [] });
const validationTouched = ref(false);
const validationBusy = ref(false);
const executeBusy = ref(false);
const executeResult = ref<ExecutionResult | null>(null);
const summary = ref<SummaryState>({
  summary: "",
  steps: 0,
  riskLevel: "",
  riskNotes: [],
  issues: [],
  needsReview: false
});
const questionOverrides = ref<string[]>([]);
const humanConfirmed = ref(false);
const confirmReason = ref("");

const validationEnvs = ref<ValidationEnvSummary[]>([]);
const selectedValidationEnv = ref("");
const envPackageOptions = ref<EnvPackageSummary[]>([]);
const selectedEnvPackages = ref<string[]>([]);
const showEnvPackageModal = ref(false);
const envPackageDraft = ref<string[]>([]);
const showValidationEnvModal = ref(false);
const validationEnvDraft = ref("");
const executeEnabled = ref(false);
const maxRetries = ref(2);
const planMode = ref("manual-approve");
const environmentNote = ref("");
const router = useRouter();
const SESSION_STORAGE_KEY = "bops_chat_session_id";

const examples = [
  "在 web1/web2 上安装 nginx，渲染配置并启动服务",
  "检查磁盘空间，超过 80% 则告警",
  "拉取脚本库中的备份脚本并执行"
];

const showExamples = ref(false);
const showConfigModal = ref(false);
const showHistoryModal = ref(false);
const showSessionModal = ref(false);
const showStepDetailModal = ref(false);
const detailStepIndex = ref<number | null>(null);
const showStepYamlModal = ref(false);
const stepYamlIndex = ref<number | null>(null);
const stepYamlText = ref("");
const stepYamlError = ref("");
const showSaveModal = ref(false);
const saveName = ref("");
const saveError = ref("");
const saveBusy = ref(false);
const newStepAction = ref("cmd.run");

const workspaceTab = ref<"visual" | "yaml" | "validate">("visual");
const visualYamlSource = computed(() => (autoSync.value ? yaml.value : visualYaml.value));
const steps = computed<StepSummary[]>(() => parseSteps(visualYamlSource.value));
const draftSteps = computed<DraftStep[]>(() => steps.value.map((step, index) => buildDraftStep(step, index)));
const timelineEntries = computed(() => {
  return chatEntries.value;
});
const requiresReason = computed(() => summary.value.riskLevel === "high");
const requiresConfirm = computed(() => {
  if (!summary.value.needsReview) return false;
  if (!humanConfirmed.value) return true;
  if (requiresReason.value && !confirmReason.value.trim()) return true;
  return false;
});
const historyTimeline = computed<HistoryEntry[]>(() => buildHistoryTimeline());
const canFix = computed(() => {
  if (!yaml.value.trim()) return false;
  return summary.value.issues.length > 0 || validation.value.issues.length > 0;
});
const pendingQuestions = computed(() => {
  const issues = summary.value.issues.length ? summary.value.issues : validation.value.issues;
  return resolveQuestions(questionOverrides.value, issues, 6);
});
const syncBlocked = computed(() => !autoSync.value && (visualDirty.value || yamlDirty.value));
const canShowIssues = computed(() => !syncBlocked.value);

let chatIndex = 0;
let summaryTimer: number | null = null;
watch(
  yaml,
  (next, prev) => {
    if (autoSync.value) {
      visualYaml.value = next;
      visualDirty.value = false;
      yamlDirty.value = false;
      selectedStepIndex.value = null;
    } else if (next !== prev) {
      yamlDirty.value = next !== visualYaml.value;
    }
    if (summaryTimer) {
      window.clearTimeout(summaryTimer);
    }
    stepIssueIndexes.value = [];
    humanConfirmed.value = false;
    confirmReason.value = "";
    validationTouched.value = false;
    summaryTimer = window.setTimeout(() => {
      void refreshSummary();
    }, 600);
  },
  { immediate: true }
);

watch(saveName, () => {
  if (saveError.value) {
    saveError.value = "";
  }
});

onMounted(() => {
  loadValidationEnvs();
  loadEnvPackages();
  void initChatSession();
});

async function loadValidationEnvs() {
  try {
    const data = await request<{ items: ValidationEnvSummary[] }>("/validation-envs");
    validationEnvs.value = data.items || [];
  } catch (err) {
    validationEnvs.value = [];
  }
}

async function loadEnvPackages() {
  try {
    const data = await request<{ items: EnvPackageSummary[] }>("/envs");
    envPackageOptions.value = data.items || [];
  } catch (err) {
    envPackageOptions.value = [];
  }
}

function pushChatEntry(entry: Omit<ChatEntry, "id">) {
  const id = `chat-${chatIndex++}`;
  chatEntries.value = [...chatEntries.value, { id, ...entry }];
}

function setChatEntriesFromSession(session: ChatSession) {
  const messages = Array.isArray(session.messages) ? session.messages : [];
  if (!messages.length) {
    chatEntries.value = [
      {
        id: "welcome",
        label: "AI",
        body: "你好！告诉我你的需求，我会拆解成可执行工作流，并主动追问缺失细节。",
        type: "ai"
      }
    ];
    return;
  }
  chatEntries.value = messages.map((msg, index) => ({
    id: `session-${index}`,
    label: msg.role === "user" ? "用户" : "AI",
    body: msg.content,
    type: msg.role === "user" ? "user" : "ai"
  }));
}

async function loadChatSessions() {
  try {
    const data = await request<{ items: ChatSessionSummary[] }>("/ai/chat/sessions");
    chatSessions.value = data.items || [];
  } catch (err) {
    chatSessions.value = [];
  }
}

async function restoreChatSession(id: string) {
  try {
    const data = await request<{ session: ChatSession }>(`/ai/chat/sessions/${id}`);
    chatSessionId.value = data.session.id;
    chatSessionTitle.value = data.session.title || "新会话";
    window.localStorage.setItem(SESSION_STORAGE_KEY, chatSessionId.value);
    setChatEntriesFromSession(data.session);
  } catch (err) {
    chatSessionId.value = "";
    chatSessionTitle.value = "";
  }
}

async function createChatSession() {
  try {
    const data = await request<{ session: ChatSession }>("/ai/chat/sessions", {
      method: "POST",
      body: { title: "新会话" }
    });
    chatSessionId.value = data.session.id;
    chatSessionTitle.value = data.session.title || "新会话";
    window.localStorage.setItem(SESSION_STORAGE_KEY, chatSessionId.value);
    setChatEntriesFromSession(data.session);
    await loadChatSessions();
    showSessionModal.value = false;
  } catch (err) {
    pushChatEntry({
      label: "系统",
      body: "新建会话失败，请检查服务是否启动",
      type: "error",
      extra: "ERROR"
    });
  }
}

async function initChatSession() {
  const stored = window.localStorage.getItem(SESSION_STORAGE_KEY);
  if (stored) {
    await restoreChatSession(stored);
    if (chatSessionId.value) {
      return;
    }
  }
  await loadChatSessions();
  if (chatSessions.value.length) {
    await restoreChatSession(chatSessions.value[0].id);
    return;
  }
  await createChatSession();
}

function selectChatSession(id: string) {
  void restoreChatSession(id);
  showSessionModal.value = false;
}

function formatSessionTime(value?: string) {
  if (!value) return "未知时间";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "未知时间";
  return date.toLocaleString();
}

function applyExample(text: string) {
  prompt.value = text;
  showExamples.value = false;
}

async function applySuggestion(text: string) {
  const trimmed = prompt.value.trim();
  prompt.value = trimmed ? `${trimmed}\n${text}` : text;
  if (busy.value) return;
  await startStream();
}

function toggleExamples() {
  showExamples.value = !showExamples.value;
}

function clearPrompt() {
  prompt.value = "";
  showExamples.value = false;
}

function formatNode(node: string) {
  return node.replace(/_/g, " ");
}

function getIndent(line: string) {
  const match = line.match(/^(\s*)/);
  return match ? match[1].length : 0;
}

function formatTargetsForInput(value: string) {
  return value.replace(/[\[\]]/g, "").replace(/['"]/g, "").trim();
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

function envMapToText(env?: Record<string, string>) {
  if (!env) return "";
  return Object.entries(env)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}

function envTextToMap(text: string) {
  const result: Record<string, string> = {};
  const lines = text.split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const splitIndex = trimmed.indexOf("=");
    if (splitIndex <= 0) continue;
    const key = trimmed.slice(0, splitIndex).trim();
    if (!key) continue;
    result[key] = trimmed.slice(splitIndex + 1).trim();
  }
  return result;
}

function getVisualYaml() {
  return autoSync.value ? yaml.value : visualYaml.value;
}

function setVisualYaml(next: string, markDirty = true) {
  visualYaml.value = next;
  if (autoSync.value) {
    yaml.value = next;
    visualDirty.value = false;
    yamlDirty.value = false;
  } else if (markDirty) {
    visualDirty.value = next !== yaml.value;
  }
}

function applyVisualToYaml() {
  yaml.value = visualYaml.value;
  visualDirty.value = false;
  yamlDirty.value = false;
}

function syncVisualFromYaml() {
  visualYaml.value = yaml.value;
  visualDirty.value = false;
  yamlDirty.value = false;
  selectedStepIndex.value = null;
}

function toggleAutoSync() {
  if (autoSync.value) {
    autoSync.value = false;
    visualYaml.value = yaml.value;
    visualDirty.value = false;
    yamlDirty.value = false;
    return;
  }
  if (visualDirty.value) {
    const useVisual = window.confirm("可视化有未同步修改，是否应用到 YAML？");
    if (useVisual) {
      applyVisualToYaml();
    } else {
      syncVisualFromYaml();
    }
  } else if (yamlDirty.value) {
    syncVisualFromYaml();
  }
  autoSync.value = true;
}

function ensureYamlSynced() {
  if (autoSync.value || !visualDirty.value) return true;
  const confirmSync = window.confirm("可视化有未同步修改，是否先应用到 YAML？");
  if (!confirmSync) return false;
  applyVisualToYaml();
  return true;
}

function openEnvPackageModal() {
  envPackageDraft.value = [...selectedEnvPackages.value];
  showEnvPackageModal.value = true;
  if (!envPackageOptions.value.length) {
    void loadEnvPackages();
  }
}

function closeEnvPackageModal() {
  showEnvPackageModal.value = false;
  envPackageDraft.value = [];
}

function applyEnvPackageSelection() {
  selectedEnvPackages.value = normalizeTargets(envPackageDraft.value);
  closeEnvPackageModal();
}

function removeEnvPackage(name: string) {
  selectedEnvPackages.value = selectedEnvPackages.value.filter((item) => item !== name);
}

function openValidationEnvModal() {
  validationEnvDraft.value = selectedValidationEnv.value;
  showValidationEnvModal.value = true;
  if (!validationEnvs.value.length) {
    void loadValidationEnvs();
  }
}

function closeValidationEnvModal() {
  showValidationEnvModal.value = false;
}

function applyValidationEnvSelection() {
  selectedValidationEnv.value = validationEnvDraft.value;
  closeValidationEnvModal();
}

function validateWorkflowName(name: string) {
  if (!name) {
    return "请输入工作流名称";
  }
  if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
    return "名称格式不正确，仅支持字母、数字、短横线、下划线";
  }
  return "";
}

function openSaveModal() {
  showSaveModal.value = true;
  saveName.value = "";
  saveError.value = "";
}

function closeSaveModal() {
  showSaveModal.value = false;
  saveError.value = "";
}

function buildDraftStep(step: StepSummary, index: number): DraftStep {
  const action = step.action || "cmd.run";
  const withData = createDefaultStepWith(action);
  withData.cmd = step.cmd || "";
  withData.dir = step.dir || "";
  withData.src = step.src || "";
  withData.dest = step.dest || "";
  withData.state = step.state || (action === "service.ensure" ? "started" : "");
  withData.script = step.script || "";
  withData.scriptRef = step.scriptRef || "";
  withData.envText = envMapToText(step.env);
  withData.vars = step.vars || "";
  if (action === "pkg.install") {
    withData.packages = step.withName ? formatTargetsForInput(step.withName) : "";
  }
  if (action === "service.ensure") {
    withData.name = step.withName || "";
  }
  return {
    id: step.name ? `${step.name}-${index}` : `step-${index}`,
    name: step.name || "",
    action,
    targets: formatTargetsForInput(step.targets || ""),
    with: withData
  };
}

function stepStatus(index: number) {
  if (syncBlocked.value) {
    return "Unsynced";
  }
  if (stepIssueIndexes.value.includes(index)) {
    return "Failed";
  }
  if (summary.value.riskLevel === "high") {
    return "Risky";
  }
  if (validationTouched.value && validation.value.ok) {
    return "Validated";
  }
  return "Draft";
}

function stepStatusClass(index: number) {
  if (syncBlocked.value) {
    return "unsynced";
  }
  if (stepIssueIndexes.value.includes(index)) {
    return "failed";
  }
  if (summary.value.riskLevel === "high") {
    return "risky";
  }
  if (validationTouched.value && validation.value.ok) {
    return "validated";
  }
  return "draft";
}

function clearStepWithFields(content: string, index: number) {
  let next = content;
  next = updateStepWithField(next, index, "cmd", "", true);
  next = updateStepWithField(next, index, "dir", "", false);
  next = updateStepWithField(next, index, "name", "", false);
  next = updateStepWithField(next, index, "names", "", false);
  next = updateStepWithField(next, index, "src", "", false);
  next = updateStepWithField(next, index, "dest", "", false);
  next = updateStepWithField(next, index, "state", "", false);
  next = updateStepWithField(next, index, "script", "", true);
  next = updateStepWithField(next, index, "script_ref", "", false);
  next = updateStepEnvBlock(next, index, {});
  next = updateStepVarsBlock(next, index, "");
  return next;
}

function updateStepFromDraft(index: number | null, draftStep: DraftStep) {
  if (index === null) return;
  const current = steps.value[index];
  const previousAction = current?.action || "";
  const nextAction = draftStep.action.trim();
  let next = getVisualYaml();

  const name = draftStep.name.trim();
  if (name) {
    next = updateStepField(next, index, "name", name);
  }
  if (nextAction) {
    next = updateStepField(next, index, "action", nextAction);
  }

  if (nextAction && nextAction !== previousAction) {
    next = clearStepWithFields(next, index);
  }

  const withData = draftStep.with;
  if (nextAction === "cmd.run") {
    next = updateStepWithField(next, index, "cmd", withData.cmd || "", true);
    next = updateStepWithField(next, index, "dir", withData.dir || "", false);
    next = updateStepEnvBlock(next, index, envTextToMap(withData.envText || ""));
  } else if (nextAction === "pkg.install") {
    const packages = withData.packages || "";
    const items = parseTargets(packages);
    if (items.length > 1) {
      const formatted = `[${items.map((item) => formatScalar(item)).join(", ")}]`;
      next = updateStepWithField(next, index, "name", "", false);
      next = updateStepWithField(next, index, "names", formatted, false);
    } else {
      const value = items[0] || "";
      next = updateStepWithField(next, index, "names", "", false);
      next = updateStepWithField(next, index, "name", value, false);
    }
  } else if (nextAction === "template.render") {
    next = updateStepWithField(next, index, "src", withData.src || "", false);
    next = updateStepWithField(next, index, "dest", withData.dest || "", false);
    next = updateStepVarsBlock(next, index, withData.vars || "");
  } else if (nextAction === "service.ensure") {
    next = updateStepWithField(next, index, "name", withData.name || "", false);
    next = updateStepWithField(next, index, "state", withData.state || "", false);
  } else if (nextAction === "env.set") {
    next = updateStepEnvBlock(next, index, envTextToMap(withData.envText || ""));
  } else if (nextAction.startsWith("script.")) {
    const scriptRef = withData.scriptRef?.trim() || "";
    const script = withData.script || "";
    if (scriptRef) {
      next = updateStepWithField(next, index, "script", "", true);
      next = updateStepWithField(next, index, "script_ref", scriptRef, false);
    } else if (script) {
      next = updateStepWithField(next, index, "script_ref", "", false);
      next = updateStepWithField(next, index, "script", script, true);
    } else {
      next = updateStepWithField(next, index, "script_ref", "", false);
      next = updateStepWithField(next, index, "script", "", true);
    }
  }

  setVisualYaml(next);
}

function duplicateStep(index: number | null) {
  if (index === null) return;
  if (index < 0 || index >= steps.value.length) return;
  setVisualYaml(duplicateStepBlock(getVisualYaml(), index));
  selectedStepIndex.value = index + 1;
}

function removeStep(index: number | null) {
  if (index === null) return;
  const total = steps.value.length;
  if (index < 0 || index >= total) return;
  setVisualYaml(deleteStepBlock(getVisualYaml(), index));
  if (selectedStepIndex.value === null) return;
  if (selectedStepIndex.value === index) {
    const nextIndex = index < total - 1 ? index : index - 1;
    selectedStepIndex.value = nextIndex >= 0 ? nextIndex : null;
  } else if (selectedStepIndex.value > index) {
    selectedStepIndex.value -= 1;
  }
}

function buildMultilineField(key: string, value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return [`    ${key}: ""`];
  }
  if (trimmed.includes("\n")) {
    const payload = trimmed.split(/\r?\n/).map((line) => `      ${line}`);
    return [`    ${key}: |`, ...payload];
  }
  return [`    ${key}: ${formatScalar(trimmed)}`];
}

function buildStepSnippet(name: string, action: string) {
  const trimmedName = name.trim() || "new step";
  const lines = [`- name: ${trimmedName}`];
  lines.push(`  action: ${action}`);
  lines.push("  with:");

  if (action === "cmd.run") {
    lines.push(...buildMultilineField("cmd", "echo \"hello\""));
  } else if (action === "pkg.install") {
    lines.push("    name: package-name");
  } else if (action === "template.render") {
    lines.push("    src: template.j2");
    lines.push("    dest: /etc/example.conf");
    lines.push("    vars:");
    lines.push("      key: value");
  } else if (action === "service.ensure") {
    lines.push("    name: service-name");
    lines.push("    state: started");
  } else if (action === "env.set") {
    lines.push("    env:");
    lines.push("      KEY: VALUE");
  } else if (action.startsWith("script.")) {
    lines.push(...buildMultilineField("script", "echo \"hello\""));
  }

  return lines;
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

function escapeRegex(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
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

function collectStepLines(lines: string[]) {
  const stepLines: number[] = [];
  let inSteps = false;
  let stepsIndent = 0;

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const stepsMatch = line.match(/^(\s*)steps\s*:\s*$/);
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
      if (/^\s*-\s*name\s*:/i.test(line)) {
        stepLines.push(i + 1);
      }
    }
  }
  return stepLines;
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

function getStepBlock(content: string, index: number) {
  const data = getStepBlocks(content);
  if (!data) return "";
  if (index < 0 || index >= data.blocks.length) return "";
  return data.blocks[index].join("\n").trimEnd();
}

function normalizeStepBlock(blockText: string, baseIndent: string) {
  const trimmed = blockText.trimEnd();
  if (!trimmed) return [];
  const lines = trimBlock(trimmed.split(/\r?\n/));
  const firstLine = lines.find((line) => line.trim() !== "") || "";
  const needsIndent = firstLine.trimStart() === firstLine && firstLine.startsWith("-");
  if (!needsIndent) return lines;
  return lines.map((line) => (line.trim() ? `${baseIndent}${line}` : line));
}

function replaceStepBlock(content: string, index: number, blockText: string) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;
  const baseIndentMatch = data.blocks[index][0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const nextBlock = normalizeStepBlock(blockText, baseIndent);
  if (!nextBlock.length) return content;
  const nextBlocks = [...data.blocks];
  nextBlocks[index] = nextBlock;
  return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
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
    const actionIndex = block.findIndex((line) => new RegExp(`^${propIndent}action\\s*:`).test(line));
    if (actionIndex >= 0) {
      block[actionIndex] = `${propIndent}action: ${value}`;
    } else {
      block.splice(1, 0, `${propIndent}action: ${value}`);
    }
  }

  if (field === "targets") {
    const targetsIndex = block.findIndex((line) => new RegExp(`^${propIndent}targets\\s*:`).test(line));
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

function stripTargetsFromYaml(content: string) {
  const data = getStepBlocks(content);
  if (!data) return content;
  let next = content;
  for (let index = 0; index < data.blocks.length; index += 1) {
    next = updateStepField(next, index, "targets", "");
  }
  return next;
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

function updateStepVarsBlock(content: string, index: number, rawVars: string) {
  const data = getStepBlocks(content);
  if (!data) return content;
  if (index < 0 || index >= data.blocks.length) return content;

  const block = [...data.blocks[index]];
  const baseIndentMatch = block[0]?.match(/^(\s*)-/);
  const baseIndent = baseIndentMatch ? baseIndentMatch[1] : "  ";
  const propIndent = `${baseIndent}  `;
  const withIndent = propIndent.length;
  const withIndex = block.findIndex((line) => new RegExp(`^${propIndent}with\\s*:$`).test(line));

  const lines = rawVars.split(/\r?\n/);
  const hasVars = lines.some((line) => line.trim() !== "");
  const varsBlock = hasVars
    ? [`${propIndent}  vars:`, ...lines.map((line) => `${propIndent}    ${line}`)]
    : [];

  if (withIndex === -1) {
    if (!varsBlock.length) return content;
    block.push(`${propIndent}with:`);
    block.push(...varsBlock);
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

  const varsIndent = `${propIndent}  `;
  const varsIndex = block.findIndex(
    (line, idx) =>
      idx > withIndex &&
      idx < withEnd &&
      new RegExp(`^${varsIndent}vars\\s*:`).test(line)
  );

  if (varsIndex === -1) {
    if (varsBlock.length) {
      block.splice(withEnd, 0, ...varsBlock);
    }
  } else {
    let varsEnd = varsIndex + 1;
    while (varsEnd < withEnd) {
      const line = block[varsEnd];
      if (line.trim() === "") {
        varsEnd += 1;
        continue;
      }
      const indent = getIndent(line);
      if (indent <= varsIndent.length) {
        break;
      }
      varsEnd += 1;
    }

    if (varsBlock.length) {
      block.splice(varsIndex, varsEnd - varsIndex, ...varsBlock);
    } else {
      block.splice(varsIndex, varsEnd - varsIndex);
    }
  }

  const nextBlocks = [...data.blocks];
  nextBlocks[index] = block;
  return rebuildStepsSection(data.lines, nextBlocks, data.sectionStart, data.sectionEnd);
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

function handleEntryAction(action?: ChatEntry["action"]) {
  if (!action) return;
  if (action === "fix") {
    void runFix();
  }
}

async function ensureChatSession() {
  if (chatSessionId.value) return;
  await createChatSession();
}

async function sendChatMessage(content: string) {
  if (!chatSessionId.value) return;
  chatPending.value = true;
  try {
    const data = await request<{ reply: ChatSessionMessage; session: ChatSession }>(
      `/ai/chat/sessions/${chatSessionId.value}/messages`,
      {
        method: "POST",
        body: { content }
      }
    );
    if (data.reply?.content) {
      pushChatEntry({
        label: "AI",
        body: data.reply.content,
        type: "ai"
      });
    }
    if (data.session?.title) {
      chatSessionTitle.value = data.session.title;
    }
  } catch (err) {
    pushChatEntry({
      label: "系统",
      body: "聊天回复失败，请检查服务是否启动",
      type: "error",
      extra: "ERROR"
    });
  } finally {
    chatPending.value = false;
  }
}

function focusStepInYaml(step: StepSummary) {
  const textarea = yamlRef.value;
  if (!textarea) return;
  const lines = yaml.value.split(/\r?\n/);
  let lineIndex = typeof step.line === "number" ? step.line : -1;
  if (lineIndex < 0) {
    lineIndex = lines.findIndex((line) => line.trim() === `- name: ${step.name}`);
  }
  if (lineIndex < 0 || lineIndex >= lines.length) return;
  let start = 0;
  for (let i = 0; i < lineIndex; i++) {
    start += lines[i].length + 1;
  }
  const end = start + lines[lineIndex].length;
  textarea.focus();
  textarea.setSelectionRange(start, end);
  const style = window.getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight || "") || 18;
  textarea.scrollTop = Math.max(0, lineIndex * lineHeight - lineHeight);
}

function openStepYamlModal(index: number) {
  const content = getVisualYaml();
  const block = getStepBlock(content, index);
  stepYamlIndex.value = index;
  stepYamlText.value = block || "";
  stepYamlError.value = "";
  showStepYamlModal.value = true;
}

function closeStepYamlModal() {
  showStepYamlModal.value = false;
  stepYamlIndex.value = null;
  stepYamlText.value = "";
  stepYamlError.value = "";
}

function applyStepYamlChanges() {
  if (stepYamlIndex.value === null) return;
  const raw = stepYamlText.value;
  const trimmed = raw.trim();
  if (!trimmed) {
    stepYamlError.value = "步骤 YAML 不能为空";
    return;
  }
  if (!/^\s*-\s*name\s*:/m.test(raw)) {
    stepYamlError.value = "步骤 YAML 必须包含 - name";
    return;
  }
  const next = replaceStepBlock(getVisualYaml(), stepYamlIndex.value, raw);
  setVisualYaml(next);
  closeStepYamlModal();
}

function focusYamlFromModal() {
  const index = stepYamlIndex.value;
  if (index === null) return;
  const step = steps.value[index];
  if (!step) return;
  closeStepYamlModal();
  workspaceTab.value = "yaml";
  void nextTick(() => {
    focusStepInYaml(step);
  });
}

function selectStep(index: number) {
  selectedStepIndex.value = index;
}

function openStepDetailModal(index: number) {
  selectedStepIndex.value = index;
  detailStepIndex.value = index;
  showStepDetailModal.value = true;
}

function closeStepDetailModal() {
  showStepDetailModal.value = false;
  detailStepIndex.value = null;
}

function openStepYamlFromDetail() {
  if (detailStepIndex.value === null) return;
  closeStepDetailModal();
  openStepYamlModal(detailStepIndex.value);
}

function duplicateStepFromDetail() {
  if (detailStepIndex.value === null) return;
  const current = detailStepIndex.value;
  duplicateStep(current);
  detailStepIndex.value = current + 1;
}

function removeStepFromDetail() {
  if (detailStepIndex.value === null) return;
  removeStep(detailStepIndex.value);
  closeStepDetailModal();
}

function appendStep(action = "cmd.run") {
  const baseName = "新建步骤";
  const existingNames = steps.value.map((step) => step.name).filter(Boolean);
  let suffix = 1;
  let stepName = baseName;
  while (existingNames.includes(stepName)) {
    suffix += 1;
    stepName = `${baseName} ${suffix}`;
  }
  const baseLines = buildStepSnippet(stepName, action);
  const content = getVisualYaml();
  const trimmed = content.trim();
  if (!trimmed) {
    const defaultName = draftId.value ? `ai-${draftId.value.slice(0, 6)}` : "draft-workflow";
    const indented = baseLines.map((line) => `  ${line}`).join("\n");
    const seed = [
      "version: v0.1",
      `name: ${defaultName}`,
      "description: ",
      "",
      "inventory:",
      "  hosts:",
      "    local:",
      "      address: 127.0.0.1",
      "",
      "plan:",
      `  mode: ${planMode.value}`,
      "  strategy: sequential",
      "",
      "steps:",
      indented
    ].join("\n");
    setVisualYaml(seed);
    return;
  }
  const lines = content.split(/\r?\n/);
  const stepsIndex = lines.findIndex((line) => /^\s*steps\s*:/.test(line));
  if (stepsIndex < 0) {
    const indented = baseLines.map((line) => `  ${line}`).join("\n");
    setVisualYaml(`${trimmed}\n\nsteps:\n${indented}`);
    return;
  }
  const stepsIndent = getIndent(lines[stepsIndex]);
  if (/^\s*steps\s*:\s*\[\s*\]\s*$/.test(lines[stepsIndex])) {
    const prefix = lines[stepsIndex].match(/^(\s*)/)?.[1] ?? "";
    lines[stepsIndex] = `${prefix}steps:`;
  }
  const stepIndent = " ".repeat(stepsIndent + 2);
  const stepLines = baseLines.map((line) => `${stepIndent}${line}`);
  let insertAt = lines.length;
  for (let i = stepsIndex + 1; i < lines.length; i += 1) {
    const line = lines[i];
    if (line.trim() === "") {
      continue;
    }
    const indent = getIndent(line);
    if (indent <= stepsIndent) {
      insertAt = i;
      break;
    }
  }
  lines.splice(insertAt, 0, ...stepLines);
  setVisualYaml(lines.join("\n"));
}

function buildContext() {
  const packages = selectedEnvPackages.value;
  const payload: Record<string, unknown> = {
    plan_mode: planMode.value,
    max_retries: maxRetries.value
  };
  if (environmentNote.value.trim()) {
    payload.environment = environmentNote.value.trim();
  }
  if (packages.length) {
    payload.env_packages = packages;
  }
  if (selectedValidationEnv.value) {
    payload.validation_env = selectedValidationEnv.value;
  }
  return payload;
}

async function startStream() {
  const message = prompt.value.trim();
  if (!message) return;
  await ensureChatSession();
  pushChatEntry({ label: "用户", body: message, type: "user" });
  showExamples.value = false;
  questionOverrides.value = [];
  void sendChatMessage(message);
  busy.value = true;
  streamError.value = "";
  progressEvents.value = [];
  executeResult.value = null;
  const payload = {
    mode: "generate",
    prompt: message,
    context: buildContext(),
    env: selectedValidationEnv.value || undefined,
    execute: executeEnabled.value,
    max_retries: maxRetries.value,
    draft_id: draftId.value || undefined
  };
  try {
    await streamWorkflow(payload);
  } finally {
    busy.value = false;
  }
}

async function streamWorkflow(payload: Record<string, unknown>) {
  const url = `${apiBase()}/ai/workflow/stream`;
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  if (!response.ok || !response.body) {
    streamError.value = "流式连接失败";
    pushChatEntry({
      label: "系统",
      body: streamError.value,
      type: "error",
      extra: "ERROR"
    });
    return;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const chunk = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      handleSSEChunk(chunk);
      boundary = buffer.indexOf("\n\n");
    }
  }
}

function handleSSEChunk(chunk: string) {
  const lines = chunk.split("\n");
  let eventName = "message";
  let data = "";
  for (const line of lines) {
    if (line.startsWith("event:")) {
      eventName = line.replace("event:", "").trim();
    } else if (line.startsWith("data:")) {
      data += line.replace("data:", "").trim();
    }
  }
  if (!data) return;
  try {
    const payload = JSON.parse(data);
    if (eventName === "status") {
      progressEvents.value = [...progressEvents.value, payload].slice(-40);
      const evt = payload as ProgressEvent;
      if (evt.status === "error" && evt.message) {
        pushChatEntry({
          label: formatNode(evt.node || "AI"),
          body: evt.message,
          type: "error",
          extra: "ERROR"
        });
      }
    } else if (eventName === "result") {
      applyResult(payload);
    } else if (eventName === "error") {
      streamError.value = payload.error || "生成失败";
      pushChatEntry({
        label: "系统",
        body: streamError.value,
        type: "error",
        extra: "ERROR"
      });
    }
  } catch (err) {
    streamError.value = "解析流式数据失败";
  }
}

function applyResult(payload: Record<string, unknown>) {
  const nextYaml = typeof payload.yaml === "string" ? payload.yaml : "";
  if (nextYaml) {
    yaml.value = stripTargetsFromYaml(nextYaml);
  }
  questionOverrides.value = normalizeQuestions(payload.questions);
  if (typeof payload.summary === "string") {
    summary.value.summary = payload.summary;
  }
  summary.value.riskLevel = String(payload.risk_level || "");
  summary.value.needsReview = Boolean(payload.needs_review);
  summary.value.issues = Array.isArray(payload.issues) ? payload.issues : [];
  const issueCount = Array.isArray(payload.issues) ? payload.issues.length : 0;
  pushChatEntry({
    label: "AI",
    body: issueCount ? `已更新，待确认 ${issueCount} 项。` : "已更新草稿。",
    type: issueCount ? "warn" : "ai",
    extra: "DONE"
  });
  if (Array.isArray(payload.history)) {
    history.value = payload.history.filter((item) => typeof item === "string");
  }
  if (typeof payload.draft_id === "string") {
    draftId.value = payload.draft_id;
  }
  humanConfirmed.value = false;
  confirmReason.value = "";
  selectedStepIndex.value = null;
  void refreshSummary();
}

async function refreshSummary() {
  if (!yaml.value.trim()) return;
  try {
    const data = await request<SummaryResponse>("/ai/workflow/summary", {
      method: "POST",
      body: { yaml: yaml.value }
    });
    summary.value = {
      summary: data.summary || "",
      steps: data.steps || 0,
      riskLevel: data.risk_level || data.riskLevel || "",
      riskNotes: data.risk_notes || data.riskNotes || [],
      issues: data.issues || [],
      needsReview: Boolean(data.needs_review ?? data.needsReview)
    };
    stepIssueIndexes.value = summary.value.issues.length ? deriveStepIssues(summary.value.issues) : [];
    if (!summary.value.needsReview) {
      humanConfirmed.value = false;
      confirmReason.value = "";
    }
  } catch (err) {
    summary.value.summary = "概览获取失败";
  }
}

async function validateDraft() {
  if (!yaml.value.trim()) return;
  if (!ensureYamlSynced()) return;
  validationTouched.value = true;
  validationBusy.value = true;
  try {
    const payloadYaml = stripTargetsFromYaml(yaml.value);
    const data = await request<{ ok: boolean; issues?: string[] }>("/workflows/_draft/validate", {
      method: "POST",
      body: { yaml: payloadYaml }
    });
    const issues = data.issues || [];
    validation.value = { ok: data.ok, issues };
    stepIssueIndexes.value = data.ok ? [] : deriveStepIssues(issues);
    const issueText = issues.length ? issues.slice(0, 2).join(" · ") : "未发现问题";
    pushChatEntry({
      label: "校验",
      body: data.ok ? `校验通过：${issueText}` : `校验失败：${issueText}`,
      type: data.ok ? "ai" : "warn",
      extra: data.ok ? "OK" : "WARN",
      action: data.ok ? undefined : "fix",
      actionLabel: data.ok ? undefined : "一键修复"
    });
  } catch (err) {
    const apiErr = err as ApiError;
    validation.value = {
      ok: false,
      issues: [apiErr.message ? `校验失败: ${apiErr.message}` : "校验失败，请检查服务是否启动"]
    };
    stepIssueIndexes.value = [];
    pushChatEntry({
      label: "校验",
      body: validation.value.issues[0],
      type: "error",
      extra: "ERROR"
    });
  } finally {
    validationBusy.value = false;
  }
}

async function runExecution() {
  if (!yaml.value.trim()) return;
  if (!ensureYamlSynced()) return;
  executeBusy.value = true;
  executeResult.value = null;
  try {
    const payloadYaml = stripTargetsFromYaml(yaml.value);
    const data = await request<ExecutionResult>("/ai/workflow/execute", {
      method: "POST",
      body: { yaml: payloadYaml, env: selectedValidationEnv.value || undefined }
    });
    executeResult.value = data;
    const codeText = typeof data.code === "number" ? ` (code ${data.code})` : "";
    const isSuccess = data.status === "success";
    pushChatEntry({
      label: "执行",
      body: `沙箱验证完成：${data.status}${codeText}`,
      type: isSuccess ? "ai" : "warn",
      extra: data.status?.toUpperCase()
    });
  } catch (err) {
    const apiErr = err as ApiError;
    executeResult.value = {
      status: "failed",
      error: apiErr.message ? `验证失败: ${apiErr.message}` : "验证失败，请检查服务是否启动"
    };
    pushChatEntry({
      label: "执行",
      body: executeResult.value.error || "验证失败",
      type: "error",
      extra: "ERROR"
    });
  } finally {
    executeBusy.value = false;
  }
}

async function runFix() {
  if (!yaml.value.trim()) return;
  if (!ensureYamlSynced()) return;
  const issues = validation.value.issues.length ? validation.value.issues : summary.value.issues;
  if (!issues.length) {
    pushChatEntry({
      label: "系统",
      body: "暂无可修复的问题。",
      type: "warn",
      extra: "INFO"
    });
    return;
  }
  busy.value = true;
  streamError.value = "";
  progressEvents.value = [];
  executeResult.value = null;
  pushChatEntry({
    label: "系统",
    body: "开始修复草稿...",
    type: "ai",
    extra: "FIX"
  });
  const payload = {
    mode: "fix",
    yaml: yaml.value,
    issues,
    context: buildContext(),
    env: selectedValidationEnv.value || undefined,
    execute: executeEnabled.value,
    max_retries: maxRetries.value,
    draft_id: draftId.value || undefined
  };
  try {
    await streamWorkflow(payload);
  } finally {
    busy.value = false;
  }
}

async function saveWorkflow(name?: string) {
  if (requiresConfirm.value) {
    window.alert(requiresReason.value ? "需要人工确认并填写原因后才能保存" : "需要人工确认后才能保存");
    return;
  }
  if (!ensureYamlSynced()) return;
  const trimmed = (name || saveName.value).trim();
  const validationError = validateWorkflowName(trimmed);
  if (validationError) {
    saveError.value = validationError;
    return;
  }
  saveError.value = "";
  const reason = confirmReason.value.trim();
  saveBusy.value = true;
  const payloadYaml = stripTargetsFromYaml(yaml.value);
  try {
    await request(`/workflows/${trimmed}`, {
      method: "PUT",
      body: { yaml: payloadYaml, confirm_reason: reason || undefined }
    });
    draftId.value = "";
    confirmReason.value = "";
    saveError.value = "";
    showSaveModal.value = false;
    await router.push({ name: "workflow", params: { name: trimmed } });
  } catch (err) {
    const apiErr = err as ApiError;
    saveError.value = apiErr.message ? `保存失败: ${apiErr.message}` : "保存失败，请检查服务是否启动";
  } finally {
    saveBusy.value = false;
  }
}

function restoreHistory(index: number) {
  const snapshot = history.value[index];
  if (snapshot) {
    yaml.value = stripTargetsFromYaml(snapshot);
    humanConfirmed.value = false;
    confirmReason.value = "";
    selectedStepIndex.value = null;
  }
}

function deriveStepIssues(issues: string[]) {
  const indexes = new Set<number>();
  for (const issue of issues) {
    const indexMatch = issue.match(/steps\[(\d+)\]/i);
    if (indexMatch) {
      const idx = Number(indexMatch[1]);
      if (!Number.isNaN(idx)) {
        indexes.add(idx);
      }
      continue;
    }
    const nameMatch = issue.match(/step name \"([^\"]+)\"/i);
    if (nameMatch) {
      const name = nameMatch[1];
      const idx = steps.value.findIndex((step) => step.name === name);
      if (idx >= 0) {
        indexes.add(idx);
      }
    }
  }
  return Array.from(indexes).sort((a, b) => a - b);
}

function buildHistoryTimeline(): HistoryEntry[] {
  if (!history.value.length) return [];
  const snapshots = [...history.value, yaml.value];
  const items = history.value.map((entry, index) => {
    const next = snapshots[index + 1] || "";
    const diff = diffSummary(entry, next);
    const label = index === 0 ? "初版" : `修复 ${index}`;
    return { index, label, diff };
  });
  return items.reverse();
}

function diffSummary(prev: string, next: string) {
  if (!prev.trim()) return "initial";
  const prevLines = prev.split(/\r?\n/);
  const nextLines = next.split(/\r?\n/);
  let added = 0;
  let removed = 0;
  const max = Math.max(prevLines.length, nextLines.length);
  for (let i = 0; i < max; i += 1) {
    if (i >= prevLines.length) {
      added += 1;
    } else if (i >= nextLines.length) {
      removed += 1;
    } else if (prevLines[i] !== nextLines[i]) {
      added += 1;
      removed += 1;
    }
  }
  return `+${added}/-${removed}`;
}

</script>

<style scoped>
.home-ai {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 12px;
  color: var(--ink);
  flex: 1;
  height: 100%;
  min-height: 0;
  background: transparent;
}

.main-grid {
  display: grid;
  grid-template-columns: minmax(360px, 1.25fr) minmax(320px, 0.95fr);
  gap: 12px;
  flex: 1;
  min-height: 0;
  grid-template-rows: minmax(0, 1fr);
  height: 100%;
}

.panel {
  background: #fff;
  border-radius: 20px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: var(--shadow);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.panel-head h2 {
  margin: 0;
  font-size: 20px;
  font-family: "Space Grotesk", "Manrope", sans-serif;
}

.panel-head p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.chat-head {
  align-items: flex-start;
}

.panel-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.sync-controls {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.sync-label {
  font-size: 12px;
  color: var(--muted);
}

.sync-tag {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 11px;
  background: rgba(27, 27, 27, 0.08);
  color: var(--muted);
}

.sync-tag.warn {
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
}

.draft-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(27, 27, 27, 0.06);
  background: rgba(255, 255, 255, 0.65);
}

.draft-stat {
  display: flex;
  justify-content: space-between;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.draft-stat strong {
  color: var(--ink);
  font-weight: 600;
}

.risk-low {
  color: var(--ok);
}

.risk-medium {
  color: var(--warn);
}

.risk-high {
  color: var(--err);
}

.status-tag {
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--muted);
  background: #f6f2ec;
}

.status-tag.ok {
  color: var(--ok);
  background: rgba(42, 157, 75, 0.12);
}

.status-tag.warn {
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
}

.status-tag.error {
  color: var(--err);
  background: rgba(208, 52, 44, 0.12);
}

.status-tag.busy {
  color: var(--info);
  background: rgba(46, 111, 227, 0.12);
}

.chat-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  overflow: hidden;
}

.chat-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.timeline {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.timeline-item {
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.68);
  border-radius: 14px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  max-width: 70%;
  font-size: 12px;
  animation: fade-up 0.35s ease;
}

.timeline-item.user {
  align-self: flex-end;
  background: rgba(46, 111, 227, 0.08);
  border-color: rgba(46, 111, 227, 0.2);
}

.timeline-item.ai {
  align-self: flex-start;
  background: rgba(42, 157, 75, 0.08);
  border-color: rgba(42, 157, 75, 0.2);
}

.timeline-item.warn {
  align-self: flex-start;
  background: rgba(230, 167, 0, 0.12);
  border-color: rgba(230, 167, 0, 0.2);
}

.timeline-item.error {
  align-self: flex-start;
  background: rgba(208, 52, 44, 0.12);
  border-color: rgba(208, 52, 44, 0.2);
}

.timeline-item.typing {
  opacity: 0.7;
  font-style: italic;
}

.timeline-actions {
  margin-top: 8px;
  display: flex;
  justify-content: flex-start;
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.timeline-badge {
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.timeline-badge.user {
  background: rgba(46, 111, 227, 0.12);
  color: var(--info);
}

.timeline-badge.ai {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}

.timeline-badge.warn {
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
}

.timeline-badge.error {
  background: rgba(208, 52, 44, 0.12);
  color: var(--err);
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.pending-questions {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px dashed rgba(27, 27, 27, 0.12);
  background: rgba(255, 255, 255, 0.7);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pending-title {
  font-size: 12px;
  color: var(--muted);
}

.pending-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chat-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.chat-status {
  margin-left: auto;
}

textarea,
input,
select {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 10px 12px;
  font-size: 13px;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  background: #fff;
  width: 100%;
  box-sizing: border-box;
}

textarea {
  resize: vertical;
  min-height: 90px;
}

.example-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.chip {
  border: 1px solid rgba(27, 27, 27, 0.12);
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  background: #fff;
}

.chip.subtle {
  background: #f3eee7;
  color: var(--muted);
  border-color: rgba(27, 27, 27, 0.1);
}

.chip.secondary {
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
  border-color: rgba(230, 167, 0, 0.3);
}

.composer-footer {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.btn {
  border: 1px solid rgba(27, 27, 27, 0.16);
  background: #fff;
  border-radius: 10px;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  box-shadow: none;
}

.btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
  box-shadow: 0 12px 22px rgba(232, 93, 42, 0.24);
}

.btn.secondary {
  background: #f7f2ec;
  border-color: rgba(27, 27, 27, 0.12);
  color: var(--ink);
}

.btn.ghost {
  background: transparent;
  color: var(--muted);
}

.btn.danger {
  background: rgba(208, 52, 44, 0.12);
  border-color: rgba(208, 52, 44, 0.2);
  color: var(--err);
}

.btn.btn-sm {
  padding: 5px 10px;
  font-size: 12px;
}

.workspace-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
  overflow: hidden;
}

.workspace-tabs {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.tab {
  flex: 1;
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: rgba(255, 255, 255, 0.75);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.tab.active {
  background: #fff;
  border-color: rgba(46, 111, 227, 0.35);
  box-shadow: 0 1px 6px rgba(46, 111, 227, 0.18);
}

.tab-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.validation-panel {
  overflow: auto;
}

.workspace-toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: space-between;
}

.workspace-toolbar .status-tag {
  margin-left: auto;
}

.requirement-card {
  background: rgba(255, 255, 255, 0.7);
  border-radius: 16px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.card-head h3 {
  margin: 0;
  font-size: 18px;
}

.card-head p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}

.card-grid {
  display: grid;
  gap: 10px;
}

.card-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
}

.card-row strong {
  color: var(--ink);
}

.chip-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.visual-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 16px;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.steps-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
}

.steps-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.steps-head-left,
.steps-head-right {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.steps-head-right {
  justify-content: flex-end;
}

.step-action-select {
  min-width: 140px;
}

.step-count {
  font-size: 12px;
  color: var(--muted);
}

.steps-list {
  display: grid;
  gap: 8px;
  overflow-y: auto;
  min-height: 0;
  flex: 1;
  padding-right: 2px;
}

.step-card {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  padding: 10px;
  background: #fff;
  cursor: pointer;
  text-align: left;
  height: 120px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
  animation: fade-up 0.35s ease;
}

.step-card.active {
  border-color: rgba(46, 111, 227, 0.4);
  box-shadow: 0 0 0 2px rgba(46, 111, 227, 0.15);
}

.step-card:hover {
  transform: translateY(-1px);
}

.step-card.error {
  border-color: rgba(208, 52, 44, 0.45);
  background: rgba(208, 52, 44, 0.06);
}

.step-card:focus-visible {
  outline: 2px solid rgba(46, 111, 227, 0.4);
  outline-offset: 2px;
}

.step-card-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
}

.step-name {
  font-weight: 600;
  font-size: 13px;
}

.step-meta {
  font-size: 11px;
  color: var(--muted);
}

.step-status {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  border: 1px solid transparent;
}

.step-status.draft {
  color: var(--muted);
  background: rgba(27, 27, 27, 0.06);
  border-color: rgba(27, 27, 27, 0.08);
}

.step-status.validated {
  color: var(--ok);
  background: rgba(42, 157, 75, 0.12);
  border-color: rgba(42, 157, 75, 0.2);
}

.step-status.failed {
  color: var(--err);
  background: rgba(208, 52, 44, 0.12);
  border-color: rgba(208, 52, 44, 0.2);
}

.step-status.risky {
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
  border-color: rgba(230, 167, 0, 0.2);
}

.step-status.unsynced {
  color: var(--info);
  background: rgba(46, 111, 227, 0.12);
  border-color: rgba(46, 111, 227, 0.2);
}

.step-summary {
  margin-top: 4px;
  font-size: 11px;
  color: var(--muted);
}

.step-actions {
  margin-top: auto;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.detail-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.detail-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.code {
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
  min-height: 200px;
  flex: 1;
}

.yaml-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}


.validation-panel .validation-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.alert {
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 12px;
  background: rgba(46, 111, 227, 0.08);
}

.alert.warn {
  background: rgba(230, 167, 0, 0.12);
}

.alert.ok {
  background: rgba(42, 157, 75, 0.12);
}

.issues {
  margin: 0;
  padding-left: 18px;
  font-size: 12px;
  color: var(--err);
}

.history {
  border-top: 1px dashed rgba(27, 27, 27, 0.12);
  padding-top: 12px;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-item {
  border-radius: 10px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: #fff;
  padding: 8px 12px;
  font-size: 12px;
  text-align: left;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.history-item.active {
  border-color: rgba(46, 111, 227, 0.4);
  box-shadow: 0 0 0 2px rgba(46, 111, 227, 0.12);
}

.history-title {
  font-weight: 600;
  margin-bottom: 2px;
}

.history-diff {
  font-size: 11px;
  color: var(--muted);
}

.session-meta {
  font-size: 11px;
  color: var(--muted);
}

.session-actions {
  display: flex;
  justify-content: flex-end;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-restore {
  font-size: 11px;
  color: var(--info);
}

.human-gate {
  display: grid;
  gap: 10px;
  padding: 12px;
  border-radius: 12px;
  background: rgba(230, 167, 0, 0.12);
  color: var(--warn);
  font-size: 12px;
}

.gate-reason {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--muted);
}

.gate-reason input {
  background: #fff;
}

.gate-actions {
  display: flex;
  justify-content: flex-end;
}

.gate-copy {
  color: var(--warn);
}

.progress-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.progress-item {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: #fff;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-item .node {
  font-weight: 600;
}

.progress-item .status {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--muted);
}

.progress-item .status.error {
  color: var(--err);
}

.progress-item .status.done {
  color: var(--ok);
}

.execution-result {
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  padding: 12px;
  font-size: 12px;
}

.execution-result.failed {
  border-color: rgba(208, 52, 44, 0.4);
}

.result-title {
  font-weight: 600;
  margin-bottom: 8px;
}

.result-io pre {
  margin: 0;
  font-size: 12px;
  background: #f4f4f4;
  border-radius: 8px;
  padding: 8px;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(24, 24, 24, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 20;
}

.summary-modal,
.config-modal,
.history-modal,
.yaml-modal,
.save-modal,
.detail-modal {
  width: min(560px, 100%);
  background: #fff;
  border-radius: 18px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: 0 24px 40px rgba(27, 27, 27, 0.18);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.modal-head h3 {
  margin: 0;
  font-size: 18px;
}

.detail-modal {
  width: min(640px, 100%);
  max-height: 85vh;
  overflow: hidden;
}

.detail-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

.modal-close {
  border: none;
  background: transparent;
  font-size: 18px;
  cursor: pointer;
  color: var(--muted);
}

.modal-summary {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}

.sync-note {
  padding: 8px 10px;
  border-radius: 10px;
  font-size: 12px;
  color: var(--warn);
  background: rgba(230, 167, 0, 0.12);
}

.modal-grid {
  display: grid;
  gap: 10px;
}

.modal-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.modal-row strong {
  color: var(--ink);
}

.modal-issues {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.form-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.select-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.select-value {
  flex: 1;
  font-size: 12px;
  color: var(--ink);
}

.option-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 260px;
  overflow: auto;
  padding-right: 6px;
}

.option-item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.08);
  background: rgba(255, 255, 255, 0.7);
  cursor: pointer;
}

.option-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}

.option-desc {
  font-size: 11px;
  color: var(--muted);
  margin-top: 2px;
}

.field-hint {
  font-size: 11px;
  color: var(--muted);
}

.tag-input {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  padding: 6px;
  border-radius: 12px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #fff;
  min-height: 40px;
}

.tag-input input {
  border: none;
  padding: 4px 6px;
  min-width: 120px;
  flex: 1;
  width: auto;
  background: transparent;
  font-family: "IBM Plex Mono", "Space Grotesk", sans-serif;
}

.tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid rgba(27, 27, 27, 0.1);
  background: #f3eee7;
  font-size: 11px;
  color: var(--ink);
}

.chip-remove {
  border: none;
  background: transparent;
  font-size: 12px;
  cursor: pointer;
  color: var(--muted);
}

.tag-remove {
  border: none;
  background: transparent;
  font-size: 12px;
  cursor: pointer;
  color: var(--muted);
}

.suggestions {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 6px;
}

.suggestions-label {
  font-size: 11px;
  color: var(--muted);
}

.suggestions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
  padding: 8px 10px;
  border-radius: 12px;
  border: 1px dashed rgba(27, 27, 27, 0.16);
  background: rgba(250, 246, 240, 0.6);
}

.toggle-btn {
  border-radius: 999px;
  padding: 6px 14px;
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #f7f2ec;
  font-size: 12px;
}

.toggle-btn.on {
  background: rgba(42, 157, 75, 0.12);
  color: var(--ok);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
}

.empty {
  font-size: 12px;
  color: var(--muted);
}

@keyframes fade-up {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 980px) {
  .main-grid {
    grid-template-columns: 1fr;
    grid-template-rows: auto;
  }

  .visual-grid {
    grid-template-columns: 1fr;
  }

  .draft-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
