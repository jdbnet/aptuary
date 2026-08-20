<script setup>
import { computed, onMounted, ref } from 'vue'
import { Trash2 } from '@lucide/vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'

const items = ref([])
const filter = ref('')
const error = ref('')

const filtered = computed(() => {
  const q = filter.value.toLowerCase().trim()
  if (!q) return items.value
  return items.value.filter((p) =>
    [p.name, p.version, p.architecture, p.distribution, p.component].some((f) =>
      String(f).toLowerCase().includes(q),
    ),
  )
})

async function load() {
  const { data } = await api.get('/packages')
  items.value = data || []
}

async function remove(p) {
  if (!await confirm({
    title: 'Delete package?',
    message: `Remove ${p.name} ${p.version} (${p.architecture}) from ${p.distribution}/${p.component}?`,
  })) return
  error.value = ''
  try {
    await api.delete(`/packages/${p.id}`)
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Packages</h1>
      <p class="mt-1 text-sm text-muted">Uploaded Debian packages across all distributions</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <input v-model="filter" type="search" placeholder="Search packages..." class="input-field max-w-md" />
    <div class="card table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th>Package</th>
            <th>Version</th>
            <th>Arch</th>
            <th>Distribution</th>
            <th>Component</th>
            <th>Size</th>
            <th>Uploaded</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in filtered" :key="p.id">
            <td>{{ p.name }}</td>
            <td>{{ p.version }}</td>
            <td>{{ p.architecture }}</td>
            <td>{{ p.distribution }}</td>
            <td>{{ p.component }}</td>
            <td>{{ Math.round(p.size / 1024) }} KB</td>
            <td class="text-muted">{{ p.uploaded_at }}</td>
            <td>
              <button type="button" class="btn-row btn-row-danger" @click="remove(p)">
                <Trash2 class="h-3.5 w-3.5" />
                Delete
              </button>
            </td>
          </tr>
          <tr v-if="!filtered.length">
            <td colspan="8" class="text-muted">No packages found</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
