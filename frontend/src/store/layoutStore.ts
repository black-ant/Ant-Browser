import { create } from 'zustand'

interface LayoutState {
  sidebarCollapsed: boolean
  appUpdateAvailable: boolean
  toggleSidebar: () => void
  setSidebarCollapsed: (collapsed: boolean) => void
  setAppUpdateAvailable: (available: boolean) => void
}

export const useLayoutStore = create<LayoutState>((set) => ({
  sidebarCollapsed: false,
  appUpdateAvailable: false,
  toggleSidebar: () => set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
  setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
  setAppUpdateAvailable: (available) => set({ appUpdateAvailable: available })
}))

