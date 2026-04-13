<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NButton,
  NSpace,
  NAlert,
  NDivider,
  NSelect,
  NTag,
  useMessage
} from 'naive-ui'
import { useNASStore } from '@/stores/nas'
import {
  getSettings,
  getAIConfigs, createAIConfig, activateAIConfig, deleteAIConfig, testAIConfig
} from '@/api'

const message = useMessage()
const nasStore = useNASStore()

const nasForm = ref({
  host: '',
  port: 5001,
  username: '',
  password: ''
})

const settingsLoading = ref(false)

onMounted(async () => {
  await nasStore.checkStatus()
  try {
    const res = await getSettings()
    if (res.data) {
      nasForm.value.host = res.data.nas_host || ''
      nasForm.value.username = res.data.nas_username || ''
      nasForm.value.password = ''
      if (res.data.nas_port) {
        nasForm.value.port = parseInt(res.data.nas_port)
      }
    }
  } catch (e) {
    // ignore
  }
  await loadAIConfigs()
})

// === NAS ===
const otpCode = ref('')

const formattedLastSync = computed(() => {
  if (!nasStore.lastSync) return '未同步'
  const ts = typeof nasStore.lastSync === 'number' ? nasStore.lastSync : parseInt(nasStore.lastSync)
  if (isNaN(ts)) return '未同步'
  return new Date(ts * 1000).toLocaleString('zh-CN')
})

async function handleConnect() {
  await nasStore.connect({ ...nasForm.value, otp_code: otpCode.value })
  if (nasStore.connected) {
    message.success('连接成功')
  } else {
    message.error(nasStore.error || '连接失败')
  }
}

async function handleDisconnect() {
  await nasStore.disconnect()
  message.success('已断开连接')
}

async function handleSync() {
  await nasStore.sync()
  if (nasStore.error) {
    message.error(nasStore.error)
  } else {
    message.success('同步完成')
  }
}

// === AI Config ===
interface AIConfig {
  id: number
  provider: string
  name: string
  base_url: string | null
  model: string
  is_active: boolean
  created_at: number
}

const aiConfigs = ref<AIConfig[]>([])
const showAddConfig = ref(false)
const newConfig = ref({ provider: 'ollama', name: '', base_url: 'http://localhost:11434', model: 'qwen2.5', api_key: '' })
const testingId = ref<number | null>(null)

const providerOptions = [
  { label: 'Claude (本地CLI)', value: 'claude' },
  { label: 'Ollama (本地)', value: 'ollama' },
]

async function loadAIConfigs() {
  try {
    const res = await getAIConfigs()
    aiConfigs.value = res.data.configs || []
  } catch (e) {
    // ignore
  }
}

async function handleAddConfig() {
  try {
    await createAIConfig(newConfig.value)
    message.success('配置已添加')
    showAddConfig.value = false
    newConfig.value = { provider: 'ollama', name: '', base_url: 'http://localhost:11434', model: 'qwen2.5', api_key: '' }
    await loadAIConfigs()
  } catch (e: any) {
    message.error(e.response?.data?.error || e.message || '添加失败')
  }
}

async function handleActivate(id: number) {
  try {
    await activateAIConfig(id)
    message.success('已切换激活配置')
    await loadAIConfigs()
  } catch (e: any) {
    message.error(e.response?.data?.error || '切换失败')
  }
}

async function handleDelete(id: number) {
  try {
    await deleteAIConfig(id)
    message.success('已删除')
    await loadAIConfigs()
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  }
}

async function handleTest(id: number) {
  testingId.value = id
  try {
    const res = await testAIConfig(id)
    message.success(`连接成功: ${res.data.response || 'OK'}`)
  } catch (e: any) {
    message.error(e.response?.data?.error || '连接失败')
  } finally {
    testingId.value = null
  }
}
</script>

<template>
  <div class="settings-page">
    <div class="settings-container">
      <h1 class="page-title">设置</h1>

      <!-- NAS 连接 -->
      <section class="settings-section">
        <h2 class="section-title">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="section-icon">
            <rect x="2" y="4" width="20" height="6" rx="2" stroke="currentColor" stroke-width="1.5"/>
            <rect x="2" y="14" width="20" height="6" rx="2" stroke="currentColor" stroke-width="1.5"/>
            <circle cx="6" cy="7" r="1" fill="currentColor"/>
            <circle cx="6" cy="17" r="1" fill="currentColor"/>
          </svg>
          NAS 连接设置
        </h2>

        <div class="settings-card">
          <NForm :model="nasForm" label-placement="left" label-width="100px">
            <NFormItem label="NAS 地址">
              <NInput v-model:value="nasForm.host" placeholder="例如: 192.168.1.100" />
            </NFormItem>
            <NFormItem label="端口">
              <NInputNumber v-model:value="nasForm.port" :min="1" :max="65535" style="width: 100%" />
            </NFormItem>
            <NFormItem label="用户名">
              <NInput v-model:value="nasForm.username" placeholder="输入用户名" />
            </NFormItem>
            <NFormItem label="密码">
              <NInput v-model:value="nasForm.password" type="password" placeholder="输入密码" />
            </NFormItem>
            <NFormItem v-if="nasStore.otpRequired" label="验证码">
              <NInput v-model:value="otpCode" placeholder="输入二次验证码（OTP）" />
            </NFormItem>
            <NFormItem>
              <NSpace>
                <NButton type="primary" :loading="nasStore.loading" @click="handleConnect" class="btn-primary">
                  连接
                </NButton>
                <NButton :disabled="!nasStore.connected" @click="handleDisconnect">断开</NButton>
                <NButton :disabled="!nasStore.connected" @click="handleSync">立即同步</NButton>
              </NSpace>
            </NFormItem>
          </NForm>

          <NAlert v-if="nasStore.error" type="error" :show-icon="false" class="status-alert">
            {{ nasStore.error }}
          </NAlert>

          <div v-if="nasStore.connected" class="connection-status connected">
            <div class="status-indicator">
              <span class="status-dot-connected" />
              已连接到: {{ nasStore.host }}
            </div>
            <div class="status-detail">最后同步: {{ formattedLastSync }}</div>
          </div>
        </div>
      </section>

      <NDivider />

      <!-- AI 模型配置 -->
      <section class="settings-section">
        <h2 class="section-title">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="section-icon">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          AI 模型配置
        </h2>

        <!-- 已有配置列表 -->
        <div class="config-list">
          <div v-for="cfg in aiConfigs" :key="cfg.id" class="config-item" :class="{ 'config-active': cfg.is_active }">
            <div class="config-info">
              <div class="config-name">
                {{ cfg.name }}
                <NTag :type="cfg.provider === 'claude' ? 'info' : 'default'" size="small" class="provider-tag">
                  {{ cfg.provider }}
                </NTag>
                <NTag v-if="cfg.is_active" type="success" size="small">激活</NTag>
              </div>
              <div class="config-model">{{ cfg.model }}</div>
            </div>
            <NSpace size="small">
              <NButton v-if="!cfg.is_active" size="small" @click="handleActivate(cfg.id)">激活</NButton>
              <NButton size="small" :loading="testingId === cfg.id" @click="handleTest(cfg.id)">测试</NButton>
              <NButton size="small" type="error" quaternary @click="handleDelete(cfg.id)">删除</NButton>
            </NSpace>
          </div>

          <div v-if="aiConfigs.length === 0" class="empty-config">
            暂无 AI 配置，请添加一个
          </div>
        </div>

        <!-- 添加新配置 -->
        <NButton v-if="!showAddConfig" block @click="showAddConfig = true" style="margin-top: 12px">
          + 添加 AI 配置
        </NButton>

        <div v-if="showAddConfig" class="settings-card" style="margin-top: 12px">
          <NForm :model="newConfig" label-placement="left" label-width="100px">
            <NFormItem label="提供商">
              <NSelect v-model:value="newConfig.provider" :options="providerOptions" />
            </NFormItem>
            <NFormItem label="名称">
              <NInput v-model:value="newConfig.name" :placeholder="newConfig.provider === 'claude' ? '例如: Claude 本地' : '例如: Ollama 本地'" />
            </NFormItem>
            <NFormItem v-if="newConfig.provider === 'ollama'" label="Base URL">
              <NInput v-model:value="newConfig.base_url" placeholder="http://localhost:11434" />
            </NFormItem>
            <NFormItem label="模型">
              <NInput v-model:value="newConfig.model" :placeholder="newConfig.provider === 'ollama' ? 'qwen2.5' : 'claude-sonnet-4-20250514'" />
            </NFormItem>
            <NFormItem>
              <NSpace>
                <NButton type="primary" @click="handleAddConfig" class="btn-primary">保存</NButton>
                <NButton @click="showAddConfig = false">取消</NButton>
              </NSpace>
            </NFormItem>
          </NForm>
        </div>
      </section>

    </div>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: 100vh;
  background: var(--color-bg-primary);
  overflow: auto;
}

.settings-container {
  max-width: 700px;
  margin: 0 auto;
  padding: 40px 32px 64px;
}

.page-title {
  font-family: var(--font-heading);
  font-size: 2rem;
  font-weight: 600;
  background: var(--gradient-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 40px;
}

.settings-section {
  margin-bottom: 8px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 1.1rem;
  font-weight: 500;
  color: var(--color-text-primary);
  margin-bottom: 16px;
}

.section-icon {
  color: var(--color-text-tertiary);
}

.settings-card {
  background: rgba(30, 30, 46, 0.4);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 24px;
}

.status-alert {
  margin-top: 16px;
}

.connection-status {
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid;
}

.connection-status.connected {
  background: rgba(74, 222, 128, 0.06);
  border-color: rgba(74, 222, 128, 0.2);
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--color-text-primary);
  margin-bottom: 4px;
}

.status-dot-connected {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #4ade80;
  box-shadow: 0 0 8px rgba(74, 222, 128, 0.4);
}

.status-detail {
  font-size: 12px;
  color: var(--color-text-tertiary);
  padding-left: 16px;
}

.btn-primary {
  background: var(--button-primary-bg) !important;
  border: none !important;
}

/* === AI Config === */
.config-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.config-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(30, 30, 46, 0.3);
  transition: all var(--transition-fast);
}

.config-item.config-active {
  border-color: rgba(74, 222, 128, 0.3);
  background: rgba(74, 222, 128, 0.04);
}

.config-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.provider-tag {
  font-size: 11px;
}

.config-model {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 2px;
}

.empty-config {
  padding: 24px;
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 14px;
}
</style>
