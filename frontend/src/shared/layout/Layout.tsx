import { ReactNode } from 'react'
import { Sidebar } from './Sidebar'

interface LayoutProps {
  children: ReactNode
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="flex h-screen bg-[var(--color-bg-base)]">
      <Sidebar />
      <div className="flex-1 min-w-0 overflow-hidden">
        <main className="h-full min-w-0 overflow-auto p-4">
          {children}
        </main>
      </div>
    </div>
  )
}
