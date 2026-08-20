<script setup>
import { onMounted, ref } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'
import { ALL_SCOPES, SCOPE_LABELS } from '@/lib/scopes'

const items = ref([])
const editing = ref(false)
const form = ref({ id: null, name: '', scopes: ['packages:write'] })
const created = ref('')
const error = ref('')

function reset() {
  editing.value = false
  form.value = { id: null, name: '', scopes: ['packages:write'] }
}

function toggleScope(scope) {
  const set = new Set(form.value.scopes)
  if (set.has(scope)) set.delete(scope)
  else set.add(scope)
  form.value.scopes = [...set]
}

async function load() {
  const { data } = await api.get('/apikeys')
  items.value = data || []
}

async function save() {
  error.value = ''
  created.value = ''
  if (!form.value.scopes.length) {
    error.value = 'Select at least one scope'
    return
  }
  try {
    if (editing.value) {
      await api.put(`/apikeys/${form.value.id}`, { scopes: form.value.scopes })
      reset()
    } else {
      const { data } = await api.post('/apikeys', {
        name: form.value.name,
        scopes: form.value.scopes,
      })
      created.value = data.token
      form.value = { id: null, name: '', scopes: [...form.value.scopes] }
    }
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

function edit(k) {
  editing.value = true
  created.value = ''
  error.value = ''
  form.value = { id: k.id, name: k.name, scopes: [...(k.scopes || [])] }
}

async function askRemove(k) {
  if (!await confirm({
    title: 'Delete API key?',
    message: `Revoke "${k.name}"? CI uploads using this key will fail.`,
  })) return
  await api.delete(`/apikeys/${k.id}`)
  if (editing.value && form.value.id === k.id) reset()
  await load()
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">API Keys</h1>
      <p class="mt-1 text-sm text-muted">Keys for CI uploads on the public endpoint (prefix apk_)</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="created" class="card text-sm">Copy now, it is not shown again: <code class="break-all">{{ created }}</code></p>
    <form class="card space-y-4" @submit.prevent="save">
      <div>
        <label class="mb-1 block text-sm text-muted">Name</label>
        <input v-model="form.name" class="input-field max-w-md" :disabled="editing" required />
      </div>
      <div>
        <label class="mb-2 block text-sm text-muted">Scopes</label>
        <div class="flex flex-wrap gap-2">
          <label v-for="scope in ALL_SCOPES" :key="scope" class="flex items-center gap-2 text-sm">
            <input type="checkbox" :checked="form.scopes.includes(scope)" @change="toggleScope(scope)" />
            {{ SCOPE_LABELS[scope] || scope }}
          </label>
        </div>
      </div>
      <div class="flex gap-2">
        <button type="submit" class="btn-primary">{{ editing ? 'Update scopes' : 'Create key' }}</button>
        <button v-if="editing" type="button" class="btn-ghost" @click="reset">Cancel</button>
      </div>
    </form>
    <div class="card table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th>Name</th>
            <th>Prefix</th>
            <th>Scopes</th>
            <th>Last used</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in items" :key="k.id">
            <td>{{ k.name }}</td>
            <td class="font-mono text-xs">{{ k.prefix }}...</td>
            <td class="max-w-xs truncate">{{ (k.scopes || []).join(', ') }}</td>
            <td class="text-muted">{{ k.last_used_at || '—' }}</td>
            <td>
              <div class="table-actions">
                <button type="button" class="btn-row btn-row-edit" title="Edit scopes" @click="edit(k)">
                  <Pencil class="h-3.5 w-3.5" />
                </button>
                <button type="button" class="btn-row btn-row-danger" title="Delete" @click="askRemove(k)">
                  <Trash2 class="h-3.5 w-3.5" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
