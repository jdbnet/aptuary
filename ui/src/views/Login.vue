<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import AptuaryLogo from '@/components/AptuaryLogo.vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

function isSafeRedirect(path) {
  return typeof path === 'string' && path.startsWith('/') && !path.startsWith('//')
}

async function login() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    const redirect = route.query.redirect
    router.push(isSafeRedirect(redirect) ? redirect : '/')
  } catch {
    error.value = 'Invalid username or password'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-surface px-4">
    <div class="card w-full max-w-md p-8 shadow-xl">
      <div class="mb-6 text-center">
        <AptuaryLogo size="lg" class="mx-auto mb-4" />
        <h1 class="text-2xl font-semibold text-heading">Aptuary</h1>
        <p class="mt-1 text-sm text-muted">Sign in to manage your APT repository</p>
      </div>
      <form class="space-y-4" @submit.prevent="login">
        <div v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-center text-sm text-red-600 dark:text-red-300">
          {{ error }}
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Username</label>
          <input v-model="username" type="text" required autofocus class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Password</label>
          <input v-model="password" type="password" required class="input-field" />
        </div>
        <button type="submit" class="btn-primary w-full py-2" :disabled="loading">
          {{ loading ? 'Signing in...' : 'Sign in' }}
        </button>
      </form>
    </div>
  </div>
</template>
