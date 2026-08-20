<script setup>
import { useConfirm } from '@/lib/confirm'

const { open, title, message, accept, cancel } = useConfirm()
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4">
    <div class="card max-w-md w-full shadow-lg">
      <h3 class="text-heading font-semibold">{{ title }}</h3>
      <p class="mt-2 text-sm text-muted">{{ message }}</p>
      <div class="mt-4 flex justify-end gap-2">
        <button type="button" class="btn-ghost" @click="cancel">Cancel</button>
        <button type="button" class="btn-danger" @click="accept">Confirm</button>
      </div>
    </div>
  </div>
</template>
