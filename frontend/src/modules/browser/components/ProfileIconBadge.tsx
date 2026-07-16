import { Globe2 } from 'lucide-react'

interface ProfileIconBadgeProps {
  badge?: string
  color?: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const sizes = {
  sm: { shell: 'h-8 w-8', icon: 'h-5 w-5', badge: 'min-h-4 min-w-4 px-1 text-[8px] -bottom-1 -right-1' },
  md: { shell: 'h-10 w-10', icon: 'h-6 w-6', badge: 'min-h-5 min-w-5 px-1 text-[9px] -bottom-1 -right-1' },
  lg: { shell: 'h-16 w-16', icon: 'h-9 w-9', badge: 'min-h-7 min-w-7 px-1.5 text-[11px] -bottom-1.5 -right-1.5' },
}

export function ProfileIconBadge({ badge, color = '#2563EB', size = 'md', className = '' }: ProfileIconBadgeProps) {
  const classes = sizes[size]
  return (
    <div className={`relative shrink-0 ${classes.shell} ${className}`} aria-label={`系统图标角标 ${badge || '自动编号'}`}>
      <div className="flex h-full w-full items-center justify-center rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] shadow-sm">
        <Globe2 className={classes.icon} strokeWidth={1.7} />
      </div>
      <span
        className={`absolute flex items-center justify-center rounded-full border-2 border-[var(--color-bg-surface)] font-bold leading-none tracking-tight text-white shadow-sm ${classes.badge}`}
        style={{ backgroundColor: color }}
      >
        {badge || '··'}
      </span>
    </div>
  )
}
