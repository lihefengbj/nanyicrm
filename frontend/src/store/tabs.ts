import { defineStore } from "pinia"

export interface TabItem {
  path: string
  title: string
  name: string // route/component name, used by keep-alive include
  closable: boolean
}

export const HOME_TAB: TabItem = { path: "/dashboard", title: "工作台", name: "Dashboard", closable: false }

export const useTabsStore = defineStore("tabs", {
  state: () => ({
    tabs: [{ ...HOME_TAB }] as TabItem[],
  }),
  getters: {
    cachedNames: (s) => s.tabs.map((t) => t.name),
  },
  actions: {
    add(tab: TabItem) {
      if (!this.tabs.some((t) => t.path === tab.path)) {
        this.tabs.push(tab)
      }
    },
    // returns the path to navigate to after closing, when the closed tab was active
    close(path: string, activePath: string): string | null {
      const idx = this.tabs.findIndex((t) => t.path === path)
      if (idx < 0 || !this.tabs[idx].closable) return null
      this.tabs.splice(idx, 1)
      if (path !== activePath) return null
      const next = this.tabs[Math.min(idx, this.tabs.length - 1)]
      return next.path
    },
    closeOthers(path: string) {
      this.tabs = this.tabs.filter((t) => !t.closable || t.path === path)
    },
    closeAll() {
      this.tabs = [{ ...HOME_TAB }]
    },
    reset() {
      this.tabs = [{ ...HOME_TAB }]
    },
  },
})
