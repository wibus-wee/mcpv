// Input: Sheet, Button, Switch, Select, Slider, RadioGroup, Input components
// Output: SettingsSheet component for dashboard settings
// Position: Dashboard settings panel with various controls

import {
  MoonIcon,
  SettingsIcon,
  SunIcon,
} from 'lucide-react'
import { useTheme } from 'next-themes'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Radio, RadioGroup } from '@/components/ui/radio-group'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

export function SettingsSheet() {
  const { theme, setTheme } = useTheme()
  const [refreshInterval, setRefreshInterval] = useState(5)
  const [notifications, setNotifications] = useState(true)
  const [logLevel, setLogLevel] = useState<string | unknown>('info')

  return (
    <Sheet>
      <Tooltip>
        <TooltipTrigger
          render={
            <SheetTrigger
              render={
                <Button variant="outline" size="icon">
                  <SettingsIcon className="size-4" />
                </Button>
              }
            />
          }
        />
        <TooltipContent>Settings</TooltipContent>
      </Tooltip>
      <SheetContent side="right" inset>
        <SheetHeader>
          <SheetTitle>Settings</SheetTitle>
          <SheetDescription>
            Configure your dashboard preferences
          </SheetDescription>
        </SheetHeader>
        <SheetPanel>
          <div className="space-y-6">
            {/* Theme */}
            <div className="space-y-3">
              <Label className="text-sm font-medium">Theme</Label>
              <div className="flex items-center gap-2">
                <Button
                  variant={theme === 'light' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setTheme('light')}
                >
                  <SunIcon className="size-4" />
                  Light
                </Button>
                <Button
                  variant={theme === 'dark' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setTheme('dark')}
                >
                  <MoonIcon className="size-4" />
                  Dark
                </Button>
                <Button
                  variant={theme === 'system' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setTheme('system')}
                >
                  System
                </Button>
              </div>
            </div>

            <Separator />

            {/* Refresh Interval */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="text-sm font-medium">Refresh Interval</Label>
                <span className="text-muted-foreground text-sm">{refreshInterval}s</span>
              </div>
              <Slider
                value={[refreshInterval]}
                onValueChange={(v) => setRefreshInterval(Array.isArray(v) ? v[0] : v)}
                min={1}
                max={30}
              />
              <p className="text-muted-foreground text-xs">
                How often to refresh dashboard data
              </p>
            </div>

            <Separator />

            {/* Notifications */}
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label className="text-sm font-medium">Notifications</Label>
                <p className="text-muted-foreground text-xs">
                  Show desktop notifications for errors
                </p>
              </div>
              <Switch
                checked={notifications}
                onCheckedChange={setNotifications}
              />
            </div>

            <Separator />

            {/* Log Level */}
            <div className="space-y-3">
              <Label className="text-sm font-medium">Minimum Log Level</Label>
              <RadioGroup value={logLevel} onValueChange={setLogLevel}>
                <div className="flex items-center gap-2">
                  <Radio value="debug" />
                  <Label className="text-sm">Debug</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Radio value="info" />
                  <Label className="text-sm">Info</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Radio value="warn" />
                  <Label className="text-sm">Warning</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Radio value="error" />
                  <Label className="text-sm">Error</Label>
                </div>
              </RadioGroup>
            </div>

            <Separator />

            {/* API Endpoint */}
            <div className="space-y-3">
              <Label className="text-sm font-medium">API Endpoint</Label>
              <Input
                placeholder="http://localhost:8080"
                defaultValue="http://localhost:8080"
                size="sm"
              />
              <p className="text-muted-foreground text-xs">
                The mcpd core API endpoint
              </p>
            </div>
          </div>
        </SheetPanel>
        <SheetFooter>
          <Button variant="outline">Reset to Defaults</Button>
          <Button>Save Changes</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
