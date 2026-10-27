import type { ComponentProps } from 'react'
import { cn } from '@/lib/cn'

type Props = ComponentProps<'button'> & {
  variant?: 'default' | 'primary' | 'ghost'
  block?: boolean
}

export function Button({ variant = 'default', block, className, type = 'button', ...rest }: Props) {
  return <button type={type} className={cn('btn', variant !== 'default' && variant, block && 'block', className)} {...rest} />
}
