<template>
  <div class="chat-timeline" ref="timelineRef">
    <div v-if="messages.length === 0" class="chat-empty">
      先描述需求，AI 会主动追问与确认细节。
    </div>
    <div v-else>
      <div
        v-for="message in messages"
        :key="message.id"
        class="chat-bubble"
        :class="message.role"
      >
        <div class="bubble-role">{{ message.role === "user" ? "你" : "AI" }}</div>
        <div class="bubble-content">{{ message.content }}</div>
      </div>
    </div>
    <div v-if="assistantTyping" class="chat-bubble assistant typing">
      <div class="bubble-role">AI</div>
      <div class="bubble-content">正在生成草稿…</div>
    </div>
    <div v-if="pendingQuestions.length" class="question-list">
      <div class="question-title">还需要确认</div>
      <div class="question-chips">
        <button
          v-for="question in pendingQuestions"
          :key="question"
          class="chip"
          type="button"
          @click="emit('suggestion', question)"
        >
          {{ question }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";

const props = defineProps<{
  messages: Array<{ id: number; role: "user" | "assistant"; content: string }>;
  assistantTyping: boolean;
  pendingQuestions: string[];
}>();

const emit = defineEmits<{
  (event: "suggestion", value: string): void;
}>();

const timelineRef = ref<HTMLDivElement | null>(null);

function scrollToBottom() {
  const el = timelineRef.value;
  if (el) {
    requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight;
    });
  }
}

watch(
  () => props.messages,
  () => {
    scrollToBottom();
  },
  { deep: true }
);

onMounted(() => {
  scrollToBottom();
});
</script>

<style scoped>
.chat-timeline {
  margin: 16px 0;
  padding-right: 6px;
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.chat-empty {
  color: var(--muted);
  font-size: 13px;
  padding: 16px;
  border: 1px dashed rgba(27, 27, 27, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.6);
}

.chat-bubble {
  padding: 12px 14px;
  border-radius: 14px;
  background: #ffffff;
  border: 1px solid rgba(27, 27, 27, 0.08);
  box-shadow: 0 6px 18px rgba(27, 27, 27, 0.05);
  display: flex;
  flex-direction: column;
}

.chat-bubble.user {
  align-self: flex-end;
  background: #fff0e5;
}

.chat-bubble.assistant {
  align-self: flex-start;
}

.chat-bubble.typing {
  opacity: 0.7;
  font-style: italic;
}

.bubble-role {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--muted);
  margin-bottom: 6px;
}

.bubble-content {
  font-size: 13px;
  white-space: pre-line;
}

.question-list {
  background: rgba(255, 255, 255, 0.8);
  border-radius: 12px;
  padding: 10px;
  border: 1px solid rgba(27, 27, 27, 0.08);
}

.question-title {
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 6px;
}

.question-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chip {
  border: 1px solid rgba(27, 27, 27, 0.12);
  background: #fff;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
}
</style>
