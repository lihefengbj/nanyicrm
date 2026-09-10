import { defineStore } from 'pinia'
import type { TokenPair, UserInfo } from '@/types/api'

const ACCESS_KEY = 'nanyicrm_access_token'
const REFRESH_KEY = 'nanyicrm_refresh_token'

interface UserState {
  accessToken: string
  refreshToken: string
  profile: UserInfo | null
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    accessToken: localStorage.getItem(ACCESS_KEY) ?? '',
    refreshToken: localStorage.getItem(REFRESH_KEY) ?? '',
    profile: null,
  }),
  getters: {
    isLoggedIn: (s) => !!s.accessToken,
  },
  actions: {
    setTokens(pair: TokenPair) {
      this.accessToken = pair.accessToken
      this.refreshToken = pair.refreshToken
      localStorage.setItem(ACCESS_KEY, pair.accessToken)
      localStorage.setItem(REFRESH_KEY, pair.refreshToken)
    },
    setProfile(profile: UserInfo) {
      this.profile = profile
    },
    hasPerm(perm: string): boolean {
      if (!this.profile) return false
      if (this.profile.isAdmin || this.profile.isSuper) return true
      return (this.profile.perms || []).includes(perm)
    },
    logout() {
      this.accessToken = ''
      this.refreshToken = ''
      this.profile = null
      localStorage.removeItem(ACCESS_KEY)
      localStorage.removeItem(REFRESH_KEY)
    },
  },
})

