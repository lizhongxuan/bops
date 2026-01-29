<template>
  <section class="node-library-panel">
    <header class="panel-head">
      <h3>节点库</h3>
      <div class="search-row">
        <input
          v-model="searchText"
          class="search-input"
          type="search"
          placeholder="搜索节点模板"
        />
      </div>
    </header>

    <div class="panel-body">
      <div v-if="loading" class="panel-state">加载中...</div>
      <div v-else-if="error" class="panel-state error">{{ error }}</div>
      <div v-else class="template-groups">
        <div v-for="group in groupedTemplates" :key="group.category" class="template-group">
          <div class="group-title">{{ group.category || "未分类" }}</div>
          <div class="template-list">
            <div
              v-for="item in group.items"
              :key="item.name"
              class="template-card"
              draggable="true"
              @dragstart="(event) => onDragStart(event, item)"
            >
              <div class="template-name">{{ item.name }}</div>
              <div class="template-desc">{{ item.description || item.action || "" }}</div>
              <div class="template-tags" v-if="item.tags && item.tags.length">
                <span v-for="tag in item.tags" :key="tag" class="tag">{{ tag }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-if="!groupedTemplates.length" class="panel-state">暂无模板</div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { request } from "../lib/api";

export type TemplateSummary = {
  name: string;
  category: string;
  description: string;
  tags?: string[];
  action?: string;
  node?: {
    type?: string;
    name?: string;
    action?: string;
    with?: Record<string, unknown>;
    targets?: string[];
  };
};

type TemplateListResponse = {
  items: TemplateSummary[];
  total: number;
};

const templates = ref<TemplateSummary[]>([]);
const loading = ref(false);
const error = ref("");
const searchText = ref("");

const filteredTemplates = computed(() => {
  const query = searchText.value.trim().toLowerCase();
  if (!query) return templates.value;
  return templates.value.filter((item) => {
    const haystack = [
      item.name,
      item.category,
      item.description,
      item.action,
      ...(item.tags || [])
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return haystack.includes(query);
  });
});

const groupedTemplates = computed(() => {
  const groups = new Map<string, TemplateSummary[]>();
  filteredTemplates.value.forEach((item) => {
    const key = item.category || "";
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(item);
  });
  return Array.from(groups.entries()).map(([category, items]) => ({
    category,
    items
  }));
});

async function loadTemplates() {
  loading.value = true;
  error.value = "";
  try {
    const data = await request<TemplateListResponse>("/node-templates");
    templates.value = data.items || [];
  } catch (err) {
    error.value = "模板加载失败";
    templates.value = [];
  } finally {
    loading.value = false;
  }
}

function onDragStart(event: DragEvent, item: TemplateSummary) {
  if (!event.dataTransfer) return;
  event.dataTransfer.effectAllowed = "copy";
  event.dataTransfer.setData("application/json", JSON.stringify(item));
  event.dataTransfer.setData("text/plain", item.name);
}

onMounted(() => {
  void loadTemplates();
});
</script>

<style scoped>
.node-library-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: var(--panel);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  padding: 16px;
  height: 100%;
  min-width: 280px;
}

.panel-head {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.panel-head h3 {
  margin: 0;
  font-size: 16px;
  color: var(--ink);
}

.search-row {
  display: flex;
  gap: 8px;
}

.search-input {
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--grid);
  background: #fff;
  font-size: 12px;
}

.panel-body {
  flex: 1;
  overflow: auto;
}

.panel-state {
  color: var(--muted);
  font-size: 12px;
  padding: 8px 0;
}

.panel-state.error {
  color: var(--err);
}

.template-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.template-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.group-title {
  font-size: 12px;
  color: var(--muted);
}

.template-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
}

.template-card {
  background: #f6f3ef;
  border: 1px solid #e3ded7;
  border-radius: var(--radius-md);
  padding: 10px 12px;
  cursor: grab;
}

.template-card:active {
  cursor: grabbing;
}

.template-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}

.template-desc {
  font-size: 12px;
  color: var(--muted);
  margin-top: 4px;
}

.template-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.tag {
  font-size: 10px;
  background: #fff;
  border: 1px solid #e3ded7;
  border-radius: 999px;
  padding: 2px 6px;
  color: #5a5a5a;
}
</style>
