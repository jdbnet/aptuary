<script setup>
import { onMounted, ref } from 'vue'
import { Trash2, Pencil } from '@lucide/vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'

const items = ref([])
const form = ref({ username: '', password: '', role: 'viewer' })
const editing = ref(null)
const error = ref('')

async function load() {
  const { data } = await api.get('/users')
  items.value = data || []
}

async function save() {
  error.value = ''
  try {
    if (editing.value) {
      await api.put(`/users/${editing.value}`, {
        password: form.value.password || undefined,
        role: form.value.role,
      })
      editing.value = null
    } else {
      await api.post('/users', form.value)
    }
    form.value = { username: '', password: '', role: 'viewer' }
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

function startEdit(u) {
  editing.value = u.id
  form.value = { username: u.username, password: '', role: u.role }
}

async function remove(u) {
  if (!await confirm({ title: 'Delete user?', message: `Remove ${u.username}?` })) return
  await api.delete(`/users/${u.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Users</h1>
      <p class="mt-1 text-sm text-muted">Dashboard accounts (admin, operator, viewer)</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <form class="card flex flex-wrap items-end gap-3" @submit.prevent="save">
      <div class="min-w-[140px]">
        <label class="mb-1 block text-sm text-muted">Username</label>
        <input v-model="form.username" class="input-field" :disabled="editing" required />
      </div>
      <div class="min-w-[140px]">
        <label class="mb-1 block text-sm text-muted">Password</label>
        <input v-model="form.password" type="password" class="input-field" :required="!editing" />
      </div>
      <div>
        <label class="mb-1 block text-sm text-muted">Role</label>
        <select v-model="form.role" class="input-field">
          <option value="admin">admin</option>
          <option value="operator">operator</option>
          <option value="viewer">viewer</option>
        </select>
      </div>
      <button type="submit" class="btn-primary">{{ editing ? 'Update' : 'Create' }}</button>
    </form>
    <div class="card table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th>Username</th>
            <th>Role</th>
            <th>Created</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in items" :key="u.id">
            <td>{{ u.username }}</td>
            <td>{{ u.role }}</td>
            <td class="text-muted">{{ u.created_at }}</td>
            <td>
              <div class="table-actions">
                <button type="button" class="btn-row btn-row-edit" title="Edit" @click="startEdit(u)">
                  <Pencil class="h-3.5 w-3.5" />
                </button>
                <button type="button" class="btn-row btn-row-danger" title="Delete" @click="remove(u)">
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
