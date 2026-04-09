<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NCard,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NButton,
  NSpace,
  NAlert,
  useMessage
} from 'naive-ui'
import { useNASStore } from '@/stores/nas'
import { getSettings, updateSettings } from '@/api'

const message = useMessage()
const nasStore = useNASStore()

const nasForm = ref({
  host: '',
  port: 5000,
  username: '',
  password: ''
})

const ollamaForm = ref({
  address: 'http://localhost:11434',
  model: 'qwen2.5'
})

const settingsLoading = ref(false)

onMounted(async () => {
  await nasStore.checkStatus()
  try {
    const res = await getSettings()
    if (res.data) {
      ollamaForm.value.address = res.data.ollama_address || 'http://localhost:11434'
      ollamaForm.value.model = res.data.ollama_model || 'qwen2.5'
      nasForm.value.host = res.data.nas_host || ''
      nasForm.value.port = parseInt(res.data.nas_port || '5000')
      nasForm.value.username = res.data.nas_username || ''
    }
  } catch (e) {
    // ignore
  }
})

async function handleConnect() {
  await nasStore.connect(nasForm.value)
  if (nasStore.connected) {
    message.success('\u8FDE\u63A5\u6210\u529F')
  } else {
    message.error(nasStore.error || '\u8FDE\u63A5\u5931\u8D25')
  }
}

async function handleDisconnect() {
  await nasStore.disconnect()
  message.success('\u5DF2\u65AD\u5F00\u8FDE\u63A5')
}

async function handleSync() {
  await nasStore.sync()
  if (nasStore.error) {
    message.error(nasStore.error)
  } else {
    message.success('\u540C\u6B65\u5B8C\u6210')
  }
}

async function handleSaveOllama() {
  settingsLoading.value = true
  try {
    await updateSettings({
      ollama_address: ollamaForm.value.address,
      ollama_model: ollamaForm.value.model
    })
    message.success('\u4FDD\u5B58\u6210\u529F')
  } catch (e: any) {
    message.error(e.message || '\u4FDD\u5B58\u5931\u8D25')
  } finally {
    settingsLoading.value = false
  }
}
</script>

<template>
  <div style="padding: 24px; max-width: 800px">
    <h1 style="margin-bottom: 24px">\u8BBE\u7F6E</h1>

    <n-card title="NAS \u8FDE\u63A5\u8BBE\u7F6E" style="margin-bottom: 24px">
      <n-form :model="nasForm" label-width="100px">
        <n-form-item label="NAS \u5730\u5740">
          <n-input v-model:value="nasForm.host" placeholder="\u4F8B\u5982: 192.168.1.100" />
        </n-form-item>
        <n-form-item label="\u7AEF\u53E3">
          <n-input-number v-model:value="nasForm.port" :min="1" :max="65535" style="width: 100%" />
        </n-form-item>
        <n-form-item label="\u7528\u6237\u540D">
          <n-input v-model:value="nasForm.username" placeholder="\u8F93\u5165\u7528\u6237\u540D" />
        </n-form-item>
        <n-form-item label="\u5BC6\u7801">
          <n-input v-model:value="nasForm.password" type="password" placeholder="\u8F93\u5165\u5BC6\u7801" />
        </n-form-item>
        <n-form-item>
          <n-space>
            <n-button type="primary" :loading="nasStore.loading" @click="handleConnect">\u8FDE\u63A5</n-button>
            <n-button :disabled="!nasStore.connected" @click="handleDisconnect">\u65AD\u5F00</n-button>
            <n-button :disabled="!nasStore.connected" @click="handleSync">\u7ACB\u5373\u540C\u6B65</n-button>
          </n-space>
        </n-form-item>
      </n-form>

      <n-alert v-if="nasStore.error" type="error" :show-icon="false" style="margin-top: 16px">
        {{ nasStore.error }}
      </n-alert>

      <div v-if="nasStore.connected" style="margin-top: 16px">
        <n-alert type="success" :show-icon="false">
          \u5DF2\u8FDE\u63A5\u5230: {{ nasStore.host }}
          <br />
          \u6700\u540E\u540C\u6B65: {{ nasStore.lastSync || '\u672A\u540C\u6B65' }}
        </n-alert>
      </div>
    </n-card>

    <n-card title="Ollama \u8BBE\u7F6E">
      <n-form :model="ollamaForm" label-width="100px">
        <n-form-item label="Ollama \u5730\u5740">
          <n-input v-model:value="ollamaForm.address" placeholder="http://localhost:11434" />
        </n-form-item>
        <n-form-item label="\u6A21\u578B\u540D\u79F0">
          <n-input v-model:value="ollamaForm.model" placeholder="\u4F8B\u5982: qwen2.5" />
        </n-form-item>
        <n-form-item>
          <n-button type="primary" :loading="settingsLoading" @click="handleSaveOllama">\u4FDD\u5B58</n-button>
        </n-form-item>
      </n-form>
    </n-card>
  </div>
</template>
