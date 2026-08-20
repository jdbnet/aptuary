<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import api from '@/api/client'

const stats = ref({})
let timer

function formatBytes(n) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function sourcesLine(distro, component) {
  const base = (stats.value.public_url || '').replace(/\/$/, '')
  return `deb [signed-by=/usr/share/keyrings/aptuary.gpg] ${base} ${distro} ${component}`
}

const publicBase = computed(() => (stats.value.public_url || '').replace(/\/$/, ''))

async function refresh() {
  const { data } = await api.get('/stats')
  stats.value = data
}

onMounted(async () => {
  await refresh()
  timer = setInterval(refresh, 10000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Overview</h1>
      <p class="mt-1 text-sm text-muted">APT repository status and client configuration</p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="card">
        <div class="text-sm text-muted">Total packages</div>
        <div class="mt-1 text-2xl font-semibold">{{ stats.total_packages ?? 0 }}</div>
      </div>
      <div class="card">
        <div class="text-sm text-muted">Package data</div>
        <div class="mt-1 text-2xl font-semibold">{{ formatBytes(stats.disk_bytes) }}</div>
      </div>
      <div class="card">
        <div class="text-sm text-muted">Repo on disk</div>
        <div class="mt-1 text-2xl font-semibold">{{ formatBytes(stats.repo_disk_bytes) }}</div>
      </div>
      <div class="card">
        <div class="text-sm text-muted">GPG key</div>
        <div class="mt-1 text-sm font-mono truncate">{{ stats.gpg_key_id || '—' }}</div>
      </div>
    </div>

    <div v-if="stats.by_distribution" class="card">
      <h2 class="font-semibold">By distribution</h2>
      <div class="mt-3 flex flex-wrap gap-2">
        <span v-for="(count, dist) in stats.by_distribution" :key="dist" class="badge badge-running">
          {{ dist }}: {{ count }}
        </span>
      </div>
    </div>

    <div v-if="stats.repositories?.length" class="card space-y-4">
      <h2 class="font-semibold">Client setup</h2>
      <p class="text-sm text-muted">
        One script per distribution (architecture is chosen automatically by apt). Requires curl, gpg, and root.
      </p>
      <div v-for="repo in stats.repositories" :key="repo.name" class="space-y-2">
        <div class="text-sm font-medium">{{ repo.name }}</div>
        <pre class="rounded-lg bg-canvas-inset dark:bg-canvas-inset-dark p-3 text-xs overflow-x-auto">curl -fsSL {{ publicBase }}/install/{{ repo.name }}.sh | sudo bash</pre>
      </div>
      <div class="space-y-3">
        <div class="text-sm font-medium">Manual sources.list</div>
        <template v-for="repo in stats.repositories" :key="'src-' + repo.name">
          <pre
            v-for="comp in repo.components"
            :key="comp"
            class="rounded-lg bg-canvas-inset dark:bg-canvas-inset-dark p-3 text-xs overflow-x-auto"
          >{{ sourcesLine(repo.name, comp) }}</pre>
        </template>
      </div>
    </div>

    <div v-if="stats.recent_uploads?.length" class="card">
      <h2 class="font-semibold">Recent uploads</h2>
      <div class="table-scroll mt-3">
        <table class="data-table">
          <thead class="text-muted">
            <tr>
              <th>Package</th>
              <th>Version</th>
              <th>Arch</th>
              <th>Distro</th>
              <th>When</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in stats.recent_uploads" :key="p.id">
              <td>{{ p.name }}</td>
              <td>{{ p.version }}</td>
              <td>{{ p.architecture }}</td>
              <td>{{ p.distribution }}</td>
              <td class="text-muted">{{ p.uploaded_at }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
