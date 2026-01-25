// Input: motion, icons, spring helper
// Output: Settings header block
// Position: Settings page header

import { SettingsIcon } from 'lucide-react'
import { m } from 'motion/react'

import { Spring } from '@/lib/spring'

export const SettingsHeader = () => (
  <m.div
    className="p-6 pb-0"
    initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
    transition={Spring.smooth(0.4)}
  >
    <div className="flex items-center gap-2">
      <SettingsIcon className="size-4 text-muted-foreground" />
      <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
    </div>
    <p className="text-muted-foreground text-sm">
      Runtime defaults shared across all servers
    </p>
  </m.div>
)
