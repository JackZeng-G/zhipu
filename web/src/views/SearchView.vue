<script setup lang="ts">
import { ref } from 'vue'
import {
  NInput,
  NButton,
  NCard,
  NSpace,
  NSpin,
  NEmpty,
  NText,
  useMessage
} from 'naive-ui'
import { aiSearch } from '@/api'

interface SearchResult {
  note_id: string
  title: string
  reason: string
}

const message = useMessage()
const query = ref('')
const results = ref<SearchResult[]>([])
const searching = ref(false)
const hasSearched = ref(false)

async function handleSearch() {
  if (!query.value.trim()) return

  searching.value = true
  hasSearched.value = true
  results.value = []

  try {
    const res = await aiSearch(query.value)
    results.value = res.data.results || []
  } catch (e: any) {
    message.error(e.response?.data?.error || e.message || '\u641C\u7D22\u5931\u8D25')
  } finally {
    searching.value = false
  }
}
</script>

<template>
  <div style="padding: 24px; max-width: 900px; margin: 0 auto">
    <h1 style="margin-bottom: 24px">AI \u641C\u7D22</h1>

    <n-space vertical :size="16">
      <n-space>
        <n-input
          v-model:value="query"
          placeholder="\u7528\u81EA\u7136\u8BED\u8A00\u641C\u7D22\u7B14\u8BB0..."
          size="large"
          style="width: 700px"
          @keydown.enter="handleSearch"
        />
        <n-button type="primary" size="large" :loading="searching" @click="handleSearch">\u641C\u7D22</n-button>
      </n-space>

      <n-spin :show="searching">
        <div v-if="results.length > 0">
          <n-card
            v-for="result in results"
            :key="result.note_id"
            style="margin-bottom: 12px; cursor: pointer"
            hoverable
          >
            <n-text strong style="font-size: 16px">{{ result.title }}</n-text>
            <br />
            <n-text depth="3">{{ result.reason }}</n-text>
          </n-card>
        </div>
        <n-empty
          v-else-if="hasSearched && !searching"
          description="\u672A\u627E\u5230\u76F8\u5173\u7B14\u8BB0"
        />
      </n-spin>
    </n-space>
  </div>
</template>
