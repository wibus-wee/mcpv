// Input: TanStack Router, theme utilities, analytics
// Output: Appearance settings page
// Position: /settings/appearance route

import { createFileRoute } from '@tanstack/react-router'
import { MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'

import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/settings/appearance')({
  component: AppearanceSettingsPage,
})

function AppearanceSettingsPage() {
  const { theme, setTheme } = useTheme()

  const themes = [
    { value: 'light', label: 'Light', icon: SunIcon },
    { value: 'dark', label: 'Dark', icon: MoonIcon },
    { value: 'system', label: 'System', icon: MonitorIcon },
  ]

  return (
    <div className="p-3">
      <CardHeader>
        <CardTitle className="text-sm">Theme</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex gap-2">
          {themes.map((t) => {
            const Icon = t.icon
            const isActive = theme === t.value
            return (
              <Button
                key={t.value}
                variant={isActive ? 'secondary' : 'outline'}
                onClick={() => {
                  setTheme(t.value)
                  track(AnalyticsEvents.SETTINGS_THEME_CHANGE, { theme: t.value })
                }}
                className={cn('flex-1 gap-2', isActive && 'ring-2 ring-primary/20')}
              >
                <Icon className="size-4" />
                {t.label}
              </Button>
            )
          })}
        </div>
      </CardContent>
    </div>
  )
}
