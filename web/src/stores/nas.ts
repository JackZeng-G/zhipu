import { defineStore } from 'pinia'
import { ref } from 'vue'
import { connectNAS, disconnectNAS, getNASStatus, syncNAS } from '@/api'

export const useNASStore = defineStore('nas', () => {
  const connected = ref(false)
  const host = ref('')
  const lastSync = ref<string | null>(null)
  const loading = ref(false)
  const error = ref('')

  async function checkStatus() {
    try {
      const res = await getNASStatus()
      connected.value = res.data.connected
      host.value = res.data.host || ''
      lastSync.value = res.data.last_sync || null
      error.value = ''
    } catch (e: any) {
      error.value = e.message || 'Failed to check NAS status'
    }
  }

  async function connect(data: { host: string; port: number; username: string; password: string }) {
    loading.value = true
    error.value = ''
    try {
      const res = await connectNAS(data)
      connected.value = res.data.connected
      host.value = data.host
      lastSync.value = res.data.last_sync || null
    } catch (e: any) {
      error.value = e.response?.data?.error || e.message || 'Failed to connect'
      connected.value = false
    } finally {
      loading.value = false
    }
  }

  async function disconnect() {
    loading.value = true
    try {
      await disconnectNAS()
      connected.value = false
      host.value = ''
      lastSync.value = null
    } catch (e: any) {
      error.value = e.message || 'Failed to disconnect'
    } finally {
      loading.value = false
    }
  }

  async function sync() {
    loading.value = true
    error.value = ''
    try {
      await syncNAS()
      await checkStatus()
    } catch (e: any) {
      error.value = e.message || 'Failed to sync'
    } finally {
      loading.value = false
    }
  }

  return {
    connected,
    host,
    lastSync,
    loading,
    error,
    checkStatus,
    connect,
    disconnect,
    sync
  }
})
