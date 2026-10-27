import type { SelectHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

/** A native select with the app's look. Pass `aria-label` — there is no visible label. */
export function Select({ className, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cn('select', className)} {...rest} />
}
