<script setup>
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  Menu, X, LayoutDashboard, Package, Database, Users, KeyRound, ScrollText, Settings, LogOut, Sun, Moon,
} from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import AptuaryLogo from '@/components/AptuaryLogo.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route = useRoute()
const auth = useAuthStore()
const theme = useThemeStore()
const sidebarOpen = ref(false)

const nav = [
  { to: '/', label: 'Overview', icon: LayoutDashboard, match: (p) => p === '/' },
  { to: '/packages', label: 'Packages', icon: Package, match: (p) => p.startsWith('/packages') },
  { to: '/repositories', label: 'Repositories', icon: Database, match: (p) => p.startsWith('/repositories') },
  { to: '/users', label: 'Users', icon: Users, match: (p) => p.startsWith('/users') },
  { to: '/apikeys', label: 'API Keys', icon: KeyRound, match: (p) => p.startsWith('/apikeys') },
  { to: '/audit', label: 'Audit', icon: ScrollText, match: (p) => p.startsWith('/audit') },
  { to: '/settings', label: 'Settings', icon: Settings, match: (p) => p.startsWith('/settings') },
]

const pageTitle = computed(() => nav.find((n) => n.match(route.path))?.label || 'Aptuary')

const versionLabel = computed(() => {
  const v = (auth.version || 'dev').trim()
  if (v === 'dev' || v.startsWith('v')) return v
  return `v${v}`
})

async function logout() {
  await auth.logout()
  window.location.href = '/login'
}
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-surface">
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 bg-black/50 lg:hidden" @click="sidebarOpen = false" />
    <aside
      class="fixed inset-y-0 left-0 z-50 flex h-screen w-64 shrink-0 flex-col border-r border-default bg-surface-raised transition-transform lg:static lg:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex items-center gap-3 border-b border-default p-4">
        <AptuaryLogo size="md" />
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-heading">Aptuary</div>
          <div class="text-xs text-muted">{{ versionLabel }}</div>
        </div>
        <button type="button" class="text-muted lg:hidden" @click="sidebarOpen = false"><X class="h-5 w-5" /></button>
      </div>
      <nav class="flex-1 overflow-y-auto p-2">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="mb-0.5 flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition"
          :class="item.match(route.path) ? 'nav-item-active' : 'nav-item-inactive'"
          @click="sidebarOpen = false"
        >
          <component :is="item.icon" class="h-4 w-4 shrink-0" />
          {{ item.label }}
        </RouterLink>
      </nav>
      <div class="border-t border-default p-3">
        <div class="flex items-stretch gap-1.5">
          <button type="button" class="btn-secondary shrink-0 px-2.5 py-2" @click="theme.toggle()">
            <Sun v-if="theme.dark" class="h-4 w-4" />
            <Moon v-else class="h-4 w-4" />
          </button>
          <button v-if="auth.authRequired" type="button" class="btn-secondary min-w-0 flex-1 px-2.5 py-2 text-xs" @click="logout">
            <LogOut class="h-4 w-4 shrink-0" />
            <span>Sign out</span>
          </button>
        </div>
      </div>
    </aside>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <header class="flex shrink-0 items-center gap-3 border-b border-default bg-surface-raised px-4 py-3 lg:hidden">
        <button type="button" class="text-muted" @click="sidebarOpen = true"><Menu class="h-5 w-5" /></button>
        <span class="truncate font-semibold text-heading">{{ pageTitle }}</span>
      </header>
      <main class="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto p-4 md:p-6 text-heading">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
  </div>
</template>
