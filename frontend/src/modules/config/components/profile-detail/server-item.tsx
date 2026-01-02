// Input: ServerSpecDetail type, runtime status hooks
// Output: ServerItem accordion component
// Position: Individual server display within profile

import type { ServerSpecDetail } from '@bindings/mcpd/internal/ui'
import {
  ClockIcon,
  CpuIcon,
  TerminalIcon,
  TrashIcon,
} from 'lucide-react'

import {
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import {
  AlertDialog,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import { ServerRuntimeIndicator, ServerRuntimeSummary } from '../server-runtime-status'

export type ServerSpecWithKey = ServerSpecDetail & { specKey?: string }

interface ServerItemProps {
  server: ServerSpecWithKey
  canEdit: boolean
  isBusy: boolean
  disabledHint?: string
  onToggleDisabled: (server: ServerSpecWithKey, disabled: boolean) => void
  onDelete: (server: ServerSpecWithKey) => void
}

/**
 * Displays a single server within an accordion, including runtime status,
 * configuration details, and action controls.
 */
export function ServerItem({
  server,
  canEdit,
  isBusy,
  disabledHint,
  onToggleDisabled,
  onDelete,
}: ServerItemProps) {
  const specKey = server.specKey ?? server.name
  const isDisabled = Boolean(server.disabled)
  const strategyLabel = {
    stateless: 'Stateless',
    stateful: 'Stateful',
    persistent: 'Persistent',
    singleton: 'Singleton',
  }[server.strategy] ?? server.strategy
  const showStrategyBadge = server.strategy !== 'stateless'
  const sessionTTLSeconds = server.sessionTTLSeconds
  const sessionTTLLabel =
    sessionTTLSeconds > 0
      ? `Session TTL ${sessionTTLSeconds}s`
      : 'Session TTL Off'

  return (
    <AccordionItem
      value={`server-${server.name}`}
      className={cn('border-none', isDisabled && 'opacity-70')}
    >
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className="font-mono text-sm truncate">{server.name}</span>
          <div className="flex items-center gap-1.5 ml-auto mr-2">
            <ServerRuntimeIndicator specKey={specKey} />
            {isDisabled && (
              <Badge variant="warning" size="sm">Disabled</Badge>
            )}
            {showStrategyBadge && (
              <Badge variant="secondary" size="sm">{strategyLabel}</Badge>
            )}
            {server.strategy === 'stateful' && (
              <Badge variant="outline" size="sm">{sessionTTLLabel}</Badge>
            )}
          </div>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-3">
        <div className="space-y-3">
          {/* Controls */}
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-2.5 py-2">
            <div className="flex items-center gap-2">
              <Switch
                checked={!isDisabled}
                onCheckedChange={checked => onToggleDisabled(server, !checked)}
                disabled={!canEdit || isBusy}
                title={disabledHint}
              />
              <span className="text-xs text-muted-foreground">
                {isDisabled ? 'Disabled' : 'Enabled'}
              </span>
            </div>
            <AlertDialog>
              <AlertDialogTrigger
                disabled={!canEdit || isBusy}
                render={(
                  <Button
                    variant="ghost"
                    size="xs"
                    title={disabledHint}
                  >
                    <TrashIcon className="size-3.5" />
                    Delete
                  </Button>
                )}
              />
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete server</AlertDialogTitle>
                  <AlertDialogDescription>
                    This removes the server from the profile configuration.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogClose
                    render={<Button variant="ghost">Cancel</Button>}
                  />
                  <AlertDialogClose
                    render={(
                      <Button
                        variant="destructive"
                        onClick={() => onDelete(server)}
                      >
                        Delete server
                      </Button>
                    )}
                  />
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>

          {/* Runtime Status */}
          <ServerRuntimeSummary
            specKey={specKey}
            className="rounded-lg border bg-muted/10 px-2.5 py-2"
          />

          {/* Command */}
          <div>
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-1.5">
              <TerminalIcon className="size-3" />
              Command
            </div>
            <div className="bg-muted/40 rounded px-2.5 py-2 font-mono text-xs overflow-x-auto">
              {server.cmd.join(' ')}
            </div>
          </div>

          {/* Working Directory */}
          {server.cwd && (
            <div>
              <p className="text-muted-foreground text-xs mb-1">Working Directory</p>
              <p className="font-mono text-xs">{server.cwd}</p>
            </div>
          )}

          {/* Environment Variables */}
          {Object.keys(server.env).length > 0 && (
            <div>
              <p className="text-muted-foreground text-xs mb-1.5">Environment Variables</p>
              <div className="space-y-1">
                {Object.entries(server.env).map(([key, value]) => (
                  <div key={key} className="flex items-center gap-2 text-xs">
                    <Badge variant="outline" size="sm" className="font-mono shrink-0">
                      {key}
                    </Badge>
                    <span className="text-muted-foreground truncate">
                      {value.startsWith('${') ? value : '••••••'}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <Separator className="my-2" />

          {/* Settings Grid */}
          <div className="grid grid-cols-3 gap-3 text-xs">
            <div className="flex items-center gap-1.5">
              <ClockIcon className="size-3 text-muted-foreground" />
              <div>
                <p className="text-muted-foreground">Idle</p>
                <p className="font-mono">{server.idleSeconds}s</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <CpuIcon className="size-3 text-muted-foreground" />
              <div>
                <p className="text-muted-foreground">Max</p>
                <p className="font-mono">{server.maxConcurrent}</p>
              </div>
            </div>
            <div>
              <p className="text-muted-foreground">Min Ready</p>
              <p className="font-mono">{server.minReady}</p>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <p className="text-muted-foreground">Strategy</p>
              <p className="font-mono">{strategyLabel}</p>
            </div>
            {server.strategy === 'stateful' && (
              <div>
                <p className="text-muted-foreground">Session TTL</p>
                <p className="font-mono">
                  {sessionTTLSeconds > 0 ? `${sessionTTLSeconds}s` : 'Off'}
                </p>
              </div>
            )}
          </div>

          {/* Exposed Tools */}
          {server.exposeTools && server.exposeTools.length > 0 && (
            <div>
              <p className="text-muted-foreground text-xs mb-1.5">Exposed Tools</p>
              <div className="flex flex-wrap gap-1">
                {server.exposeTools.map(tool => (
                  <Badge key={tool} variant="secondary" size="sm" className="font-mono">
                    {tool}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}
