<template>
  <div class="card file-card">
    <div class="card-title">{{ card.title || "创建文件" }}</div>
    <div v-if="!card.files || card.files.length === 0" class="card-empty">暂无文件</div>
    <div v-else class="file-list">
      <div v-for="(file, idx) in card.files" :key="`${file.path}-${idx}`" class="file-item">
        <div class="file-head">
          <span class="path">{{ file.path }}</span>
          <span v-if="file.language" class="lang">{{ file.language }}</span>
        </div>
        <pre v-if="file.content" class="file-content">{{ file.content }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
export type FileCreateCardPayload = {
  card_type: "file_create";
  title?: string;
  files?: Array<{
    path: string;
    language?: string;
    content?: string;
  }>;
  actions?: string[];
};

defineProps<{ card: FileCreateCardPayload }>();
</script>

<style scoped>
.card {
  border: 1px solid rgba(27, 27, 27, 0.08);
  border-radius: 14px;
  background: #fff;
  padding: 12px;
}

.card-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 8px;
}

.card-empty {
  font-size: 12px;
  color: #8c8c8c;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.file-item {
  border: 1px dashed rgba(27, 27, 27, 0.1);
  border-radius: 10px;
  padding: 8px;
}

.file-head {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  margin-bottom: 6px;
}

.path {
  font-weight: 600;
}

.lang {
  color: #6f6f6f;
}

.file-content {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  font-family: "SFMono-Regular", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
</style>
