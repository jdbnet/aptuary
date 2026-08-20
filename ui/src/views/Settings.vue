<script setup>
import { onMounted, ref } from 'vue'
import api from '@/api/client'

const settings = ref({})
const gpg = ref({})

async function load() {
  const [s, g] = await Promise.all([
    api.get('/settings'),
    api.get('/gpg/status'),
  ])
  settings.value = s.data
  gpg.value = g.data
}

async function downloadKey() {
  const res = await api.get('/gpg/public-key', { responseType: 'blob' })
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = 'aptuary.gpg'
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Settings</h1>
      <p class="mt-1 text-sm text-muted">Server configuration (read-only) and GPG key export</p>
    </div>
    <div class="card space-y-2 text-sm">
      <div><span class="text-muted">Data directory:</span> {{ settings.data_dir }}</div>
      <div><span class="text-muted">Admin listen:</span> {{ settings.admin_listen }}</div>
      <div><span class="text-muted">Public listen:</span> {{ settings.public_listen }}</div>
      <div><span class="text-muted">Public URL:</span> {{ settings.public_url }}</div>
      <div><span class="text-muted">GPG home:</span> {{ settings.gpg_home }}</div>
    </div>
    <div class="card space-y-3">
      <h2 class="font-semibold">GPG signing key</h2>
      <div class="text-sm"><span class="text-muted">Key ID:</span> <code>{{ gpg.key_id }}</code></div>
      <div class="text-sm"><span class="text-muted">Fingerprint:</span> <code>{{ gpg.fingerprint || '—' }}</code></div>
      <button type="button" class="btn-secondary" @click="downloadKey">Download public key</button>
      <p class="text-xs text-muted">
        Clients should use signed-by=/usr/share/keyrings/aptuary.gpg in sources.list
      </p>
    </div>
  </div>
</template>
