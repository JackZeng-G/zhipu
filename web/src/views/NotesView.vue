<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NGrid,
  NGi,
  NTree,
  NList,
  NListItem,
  NInput,
  NButton,
  NCard,
  NSpace,
  NDivider,
  NPagination,
  NSpin,
  NText,
  NEmpty,
  NSkeleton,
  useMessage
} from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { useNotesStore, type Notebook } from '@/stores/notes'

const router = useRouter()
const message = useMessage()
const notesStore = useNotesStore()

const searchQuery = ref('')
const summary = ref<string | null>(null)
const editInstruction = ref('')
const editSelectedText = ref('')
const editResult = ref<string | null>(null)
const summarizing = ref(false)
const editing = ref(false)
const detailLoading = ref(false)

const filteredNotes = computed(() => {
  if (!searchQuery.value) return notesStore.notes
  const q = searchQuery.value.toLowerCase()
  return notesStore.notes.filter((n) => n.title.toLowerCase().includes(q))
})

function notebooksToTreeOptions(notebooks: Notebook[]): TreeOption[] {
  return notebooks.map((nb) => ({
    key: nb.id,
    label: nb.name,
    children: nb.children ? notebooksToTreeOptions(nb.children) : undefined
  }))
}

const treeOptions = computed(() => notebooksToTreeOptions(notesStore.notebooks))

function handleTreeSelect(keys: string[]) {
  if (keys.length > 0) {
    notesStore.fetchNotes(keys[0], 1, notesStore.pagination.pageSize)
  }
}

async function handlePageChange(page: number) {
  await notesStore.fetchNotes(notesStore.currentNotebookId || undefined, page, notesStore.pagination.pageSize)
}

async function selectNote(id: string) {
  detailLoading.value = true
  await notesStore.fetchNote(id)
  summary.value = null
  editResult.value = null
  detailLoading.value = false
}

async function handleSummarize() {
  if (!notesStore.currentNote) return
  summarizing.value = true
  summary.value = null
  try {
    const result = await notesStore.summarize(notesStore.currentNote.id)
    if (result) {
      summary.value = result
    } else {
      message.error(notesStore.error || '\u751F\u6210\u6458\u8981\u5931\u8D25')
    }
  } finally {
    summarizing.value = false
  }
}

async function handleEdit() {
  if (!notesStore.currentNote || !editInstruction.value) return
  editing.value = true
  editResult.value = null
  try {
    const result = await notesStore.edit(
      notesStore.currentNote.id,
      editInstruction.value,
      editSelectedText.value || undefined
    )
    if (result) {
      editResult.value = result
    } else {
      message.error(notesStore.error || 'AI \u7F16\u8F91\u5931\u8D25')
    }
  } finally {
    editing.value = false
  }
}

function startChat() {
  if (!notesStore.currentNote) return
  router.push({ path: '/chat', query: { noteId: notesStore.currentNote.id } })
}

onMounted(async () => {
  await Promise.all([
    notesStore.fetchNotebooks(),
    notesStore.fetchNotes(undefined, 1, 20)
  ])
})
</script>

<template>
  <div style="height: 100vh; display: flex; overflow: hidden">
    <!-- Left column: Notebook tree -->
    <div style="width: 300px; border-right: 1px solid #e0e0e0; overflow: auto; padding: 16px">
      <h3 style="margin-top: 0; margin-bottom: 12px">\u7B14\u8BB0\u672C</h3>
      <n-tree
        :data="treeOptions"
        block-line
        selectable
        @update:selected-keys="handleTreeSelect"
        :default-expand-all="true"
      />
    </div>

    <!-- Middle column: Note list -->
    <div style="flex: 1; border-right: 1px solid #e0e0e0; display: flex; flex-direction: column; overflow: hidden">
      <div style="padding: 16px">
        <n-input
          v-model:value="searchQuery"
          placeholder="\u641C\u7D22\u7B14\u8BB0\u6807\u9898..."
          clearable
        />
      </div>
      <div style="flex: 1; overflow: auto">
        <n-spin :show="notesStore.loading">
          <n-list v-if="filteredNotes.length > 0" hoverable clickable>
            <n-list-item
              v-for="note in filteredNotes"
              :key="note.id"
              @click="selectNote(note.id)"
              :class="{ 'note-selected': notesStore.currentNote?.id === note.id }"
              style="cursor: pointer; padding: 12px 16px"
            >
              <n-text strong>{{ note.title }}</n-text>
              <br />
              <n-text depth="3" style="font-size: 12px">{{ note.modified_time }}</n-text>
            </n-list-item>
          </n-list>
          <n-empty v-else description="\u6682\u65E0\u7B14\u8BB0" style="padding: 48px" />
        </n-spin>
      </div>
      <div style="padding: 12px; display: flex; justify-content: center">
        <n-pagination
          :page="notesStore.pagination.page"
          :page-count="Math.ceil(notesStore.pagination.total / notesStore.pagination.pageSize)"
          @update:page="handlePageChange"
        />
      </div>
    </div>

    <!-- Right column: Note detail + AI panel -->
    <div v-if="notesStore.currentNote" style="width: 500px; overflow: auto; display: flex; flex-direction: column">
      <n-spin :show="detailLoading" style="flex: 1">
        <div style="padding: 16px">
          <h2 style="margin-top: 0">{{ notesStore.currentNote.title }}</h2>
          <n-text depth="3" style="font-size: 12px">
            \u521B\u5EFA: {{ notesStore.currentNote.created_time }} |
            \u4FEE\u6539: {{ notesStore.currentNote.modified_time }}
          </n-text>
          <n-divider />
          <div class="note-content" v-html="notesStore.currentNote.content"></div>
        </div>
      </n-spin>

      <n-divider style="margin: 0" />

      <div style="padding: 16px; overflow: auto">
        <h3 style="margin-top: 0">AI \u5DE5\u5177</h3>

        <!-- Summarize -->
        <n-space vertical>
          <n-button :loading="summarizing" @click="handleSummarize">\u751F\u6210\u6458\u8981</n-button>
          <n-card v-if="summary" size="small" title="\u6458\u8981">
            {{ summary }}
          </n-card>

          <n-divider />

          <!-- Edit assist -->
          <h4 style="margin: 0">\u7F16\u8F91\u8F85\u52A9</h4>
          <n-input
            v-model:value="editSelectedText"
            type="textarea"
            placeholder="\u9009\u4E2D\u7684\u6587\u672C\uFF08\u53EF\u9009\uFF09"
            :rows="2"
          />
          <n-input
            v-model:value="editInstruction"
            placeholder="\u8F93\u5165\u7F16\u8F91\u6307\u4EE4\uFF0C\u4F8B\u5982: \u5C06\u8FD9\u6BB5\u8BDD\u6539\u5199\u4E3A\u66F4\u7B80\u6D01\u7684\u7248\u672C"
            :rows="2"
          />
          <n-button :loading="editing" @click="handleEdit" :disabled="!editInstruction">\u63D0\u4EA4\u7F16\u8F91</n-button>
          <n-card v-if="editResult" size="small" title="\u7F16\u8F91\u7ED3\u679C">
            {{ editResult }}
          </n-card>

          <n-divider />

          <!-- Start chat -->
          <n-button @click="startChat">\u5F00\u59CB\u5BF9\u8BDD</n-button>
        </n-space>
      </div>
    </div>
    <div v-else style="width: 500px; display: flex; align-items: center; justify-content: center; border-left: 1px solid #e0e0e0">
      <n-empty description="\u9009\u62E9\u4E00\u7BC7\u7B14\u8BB0\u67E5\u770B\u8BE6\u60C5" />
    </div>
  </div>
</template>

<style scoped>
.note-content {
  line-height: 1.8;
  font-size: 14px;
}

.note-content :deep(img) {
  max-width: 100%;
}

.note-content :deep(pre) {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
}

.note-content :deep(code) {
  background: #f5f5f5;
  padding: 2px 4px;
  border-radius: 2px;
}

.note-selected {
  background-color: #e8f4fd;
}
</style>
