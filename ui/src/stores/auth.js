import { defineStore } from 'pinia'
import api from '@/api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    authRequired: false,
    authenticated: false,
    checked: false,
    me: null,
    version: '',
  }),
  actions: {
    async check() {
      const { data } = await api.get('/auth/status')
      this.authRequired = data.auth_required
      this.authenticated = data.authenticated
      this.version = data.version || ''
      this.checked = true
      if (this.authenticated) {
        try {
          const me = await api.get('/auth/me')
          this.me = me.data
        } catch {
          this.me = null
        }
      }
    },
    async login(username, password) {
      await api.post('/auth/login', { username, password })
      this.authRequired = true
      this.authenticated = true
      this.checked = true
    },
    async logout() {
      await api.post('/auth/logout')
      this.authenticated = false
      this.me = null
    },
  },
})
