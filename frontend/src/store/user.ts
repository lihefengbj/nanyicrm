import { defineStore } from 'pinia'
import type { TokenPair, UserInfo } from '@/types/api'

interface UserState {
  authenticated: boolean
  profile: UserInfo | null
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    authenticated: false,
    profile: null,
  }),
  getters: {
    isLoggedIn: (s) => s.authenticated,
  },
  actions: {
    setTokens(pair: TokenPair) {
      // Kept as a compatibility shim for callers compiled against the old
      // API. Tokens are now HttpOnly cookies and never enter JavaScript.
      void pair
      this.authenticated = true
    },
    setAuthenticated() {
      this.authenticated = true
    },
    setProfile(profile: UserInfo) {
      this.profile = profile
      this.authenticated = true
    },
    hasPerm(perm: string): boolean {
      if (!this.profile) return false
      if (this.profile.isAdmin || this.profile.isSuper) return true
      return (this.profile.perms || []).includes(perm)
    },
    logout() {
      this.authenticated = false
      this.profile = null
      // Remove tokens written by versions before the HttpOnly-cookie
      // migration.
      localStorage.removeItem('nanyicrm_access_token')
      localStorage.removeItem('nanyicrm_refresh_token')
    },
  },
})

