<script setup>
import { onMounted, ref } from 'vue'
import api from '@/api/client'

const items = ref([])

async function load() {
  const { data } = await api.get('/audit')
  items.value = data || []
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Audit log</h1>
      <p class="mt-1 text-sm text-muted">Uploads, deletes, and configuration changes</p>
    </div>
    <div class="card table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th>Time</th>
            <th>Actor</th>
            <th>Action</th>
            <th>Resource</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in items" :key="e.id">
            <td class="text-muted">{{ e.at }}</td>
            <td>{{ e.actor_type }}:{{ e.actor_id }}</td>
            <td>{{ e.action }}</td>
            <td>{{ e.resource }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
