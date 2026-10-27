import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

export function InlineAlert({ tone = 'error', children }: { tone?: 'error' | 'info'; children: ReactNode }) {
  return (
    <div className={cn('alert', tone)} role={tone === 'error' ? 'alert' : 'status'}>
      {children}
    </div>
  )
}
