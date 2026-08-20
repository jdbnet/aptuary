<script setup>
import { onMounted, ref } from 'vue'
import api from '@/api/client'

const repos = ref([])
const error = ref('')
const saving = ref(false)

async function load() {
  const { data } = await api.get('/repositories')
  repos.value = data || []
}

async function save() {
  error.value = ''
  saving.value = true
  try {
    await api.put('/repositories', repos.value)
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
  }
}

function addRepo() {
  repos.value.push({ name: '', components: ['main'], architectures: ['amd64'] })
}

function removeRepo(i) {
  repos.value.splice(i, 1)
}

function parseList(s) {
  return s.split(',').map((x) => x.trim()).filter(Boolean)
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Repositories</h1>
      <p class="mt-1 text-sm text-muted">Configure distributions, components, and architectures</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <div class="space-y-4">
      <div v-for="(repo, i) in repos" :key="i" class="card space-y-3">
        <div class="flex items-center justify-between gap-2">
          <label class="text-sm font-medium">Distribution {{ i + 1 }}</label>
          <button type="button" class="btn-ghost text-xs" @click="removeRepo(i)">Remove</button>
        </div>
        <input v-model="repo.name" class="input-field" placeholder="stable" required />
        <div>
          <label class="mb-1 block text-sm text-muted">Components (comma-separated)</label>
          <input
            :value="repo.components.join(', ')"
            class="input-field"
            @input="repo.components = parseList($event.target.value)"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Architectures (comma-separated)</label>
          <input
            :value="repo.architectures.join(', ')"
            class="input-field"
            @input="repo.architectures = parseList($event.target.value)"
          />
        </div>
      </div>
    </div>
    <div class="flex gap-2">
      <button type="button" class="btn-secondary" @click="addRepo">Add distribution</button>
      <button type="button" class="btn-primary" :disabled="saving" @click="save">
        {{ saving ? 'Saving...' : 'Save and republish' }}
      </button>
    </div>
  </div>
</template>
