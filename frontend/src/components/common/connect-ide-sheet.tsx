// Input: motion/react, lucide-react icons, UI components, server hooks, mcpvmcp config helpers
// Output: ConnectIdeSheet component for IDE connection presets
// Position: Shared UI component for configuring IDE connections in the app

import {
  CheckIcon,
  ClipboardIcon,
  Code2Icon,
  CogIcon,
  DownloadIcon,
  MessageCircleIcon,
  MousePointerClickIcon,
  RocketIcon,
  ServerIcon,
  TagIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import type * as React from 'react'
import { useMemo, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useMcpvmcpPath } from '@/hooks/use-mcpvmcp-path'
import { useRpcAddress } from '@/hooks/use-rpc-address'
import type { SelectorMode } from '@/lib/mcpvmcp'
import { buildClientConfig, buildCliSnippet, buildTomlConfig } from '@/lib/mcpvmcp'
import { useServers } from '@/modules/servers/hooks'

import { useSidebar } from '../ui/sidebar'

type ClientTab = 'cursor' | 'claude' | 'vscode' | 'codex'

type PresetBlock = { title: string, value: string }

const clientMeta: Record<ClientTab, { title: string, Icon: React.ComponentType<any> }> = {
  cursor: { title: 'Cursor', Icon: MousePointerClickIcon },
  claude: { title: 'Claude Desktop', Icon: MessageCircleIcon },
  vscode: { title: 'VS Code', Icon: Code2Icon },
  codex: { title: 'Codex', Icon: CogIcon },
}

function generateCursorDeepLink(name: string, config: string): string {
  const configObj = JSON.parse(config)
  const mcpServers = configObj.mcpServers || {}
  const serverConfig = mcpServers[name]

  if (!serverConfig) {
    throw new Error(`Server ${name} not found in config`)
  }

  const configJson = JSON.stringify(serverConfig)
  const base64Config = btoa(configJson)

  return `cursor://anysphere.cursor-deeplink/mcp/install?name=${encodeURIComponent(name)}&config=${encodeURIComponent(base64Config)}`
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className="shrink-0"
      onClick={handleCopy}
      aria-label="Copy"
    >
      {copied ? <CheckIcon className="size-4 text-emerald-500" /> : <ClipboardIcon className="size-4" />}
    </Button>
  )
}

function InstallInCursorButton({ serverName, config }: { serverName: string, config: string }) {
  const handleInstall = () => {
    try {
      const deepLink = generateCursorDeepLink(serverName, config)
      window.location.href = deepLink
    }
    catch (error) {
      console.error('Failed to generate Cursor deep link:', error)
    }
  }

  return (
    <Button
      variant="default"
      size="sm"
      onClick={handleInstall}
      className="gap-2"
    >
      <DownloadIcon className="size-4" />
      Install in Cursor
    </Button>
  )
}

export function ConnectIdeSheet() {
  const [open, setOpen] = useState(false)
  const [selectorMode, setSelectorMode] = useState<SelectorMode>('server')
  const [selectorValue, setSelectorValue] = useState('')
  const { path } = useMcpvmcpPath()
  const { rpcAddress } = useRpcAddress()
  const { data: servers } = useServers()
  const sidebar = useSidebar()

  const serverOptions = useMemo(
    () => (servers ?? []).map(server => server.name).sort((a, b) => a.localeCompare(b)),
    [servers],
  )
  const tagOptions = useMemo(() => {
    const set = new Set<string>()
      ; (servers ?? []).forEach((server) => {
        server.tags?.forEach(tag => set.add(tag))
      })
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  }, [servers])

  const defaultSelectorValue = useMemo(() =>
    selectorMode === 'server'
      ? serverOptions[0] ?? ''
      : tagOptions[0] ?? '', [selectorMode, serverOptions, tagOptions])

  const effectiveSelectorValue = useMemo(() =>
    selectorValue || defaultSelectorValue, [selectorValue, defaultSelectorValue])

  const selector = useMemo(() => ({
    mode: selectorMode,
    value: effectiveSelectorValue,
  }), [selectorMode, effectiveSelectorValue])

  const configServerName = useMemo(() =>
    selector.mode === 'server'
      ? selector.value
      : `mcpv-${selector.value || 'tag'}`, [selector])

  const configByClient = useMemo<Record<ClientTab, PresetBlock[]>>(
    () => ({
      cursor: [
        {
          title: 'Cursor config (json)',
          value: buildClientConfig('cursor', path, selector, rpcAddress),
        },
      ],
      vscode: [
        {
          title: 'VS Code config (json)',
          value: buildClientConfig('vscode', path, selector, rpcAddress),
        },
      ],
      claude: [
        {
          title: 'Claude CLI (stdio)',
          value: buildCliSnippet(path, selector, rpcAddress, 'claude'),
        },
        {
          title: 'Claude config (json)',
          value: buildClientConfig('claude', path, selector, rpcAddress),
        },
      ],
      codex: [
        {
          title: 'Codex CLI (stdio)',
          value: buildCliSnippet(path, selector, rpcAddress, 'codex'),
        },
        {
          title: 'Codex config (config.toml)',
          value: buildTomlConfig(path, selector, rpcAddress),
        },
      ],
    }),
    [path, selector, rpcAddress],
  )

  const suggestions = useMemo(() =>
    selectorMode === 'server' ? serverOptions : tagOptions, [selectorMode, serverOptions, tagOptions])

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
        <RocketIcon className="size-4" />
        <m.span
          initial={{ opacity: 0, width: 0 }}
          animate={{ opacity: sidebar.open ? 1 : 0, width: sidebar.open ? 'auto' : 0 }}
          transition={{ duration: 0.2 }}
        >
          Connect IDE
        </m.span>
      </Button>
      <SheetContent side="right" showCloseButton>
        <SheetHeader>
          <SheetTitle>Connect your IDE</SheetTitle>
          <SheetDescription>Copy ready-to-use commands or config snippets for common clients.</SheetDescription>
        </SheetHeader>
        <SheetPanel className="space-y-6">
          <div className="space-y-3">
            <p className="text-sm font-medium">Connection target</p>
            <ToggleGroup
              multiple={false}
              value={[selectorMode]}
              onValueChange={(values) => {
                const next = values[0] as SelectorMode | undefined
                setSelectorMode(next ?? 'server')
              }}
              className="flex flex-wrap gap-2"
            >
              <ToggleGroupItem value="server" size="sm" variant="outline">
                <ServerIcon className="size-3" />
                Server
              </ToggleGroupItem>
              <ToggleGroupItem value="tag" size="sm" variant="outline">
                <TagIcon className="size-3" />
                Tag
              </ToggleGroupItem>
            </ToggleGroup>
            <div className="space-y-2">
              <Input
                value={effectiveSelectorValue}
                onChange={event => setSelectorValue(event.target.value)}
                placeholder={selectorMode === 'server' ? 'Server name' : 'Tag name'}
              />
              {suggestions.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {suggestions.slice(0, 6).map(option => (
                    <Button
                      key={option}
                      type="button"
                      variant="secondary"
                      size="xs"
                      onClick={() => setSelectorValue(option)}
                    >
                      {option}
                    </Button>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="space-y-3">
            <p className="text-sm font-medium">Client presets</p>
            <Tabs defaultValue="cursor">
              <TabsList>
                <TabsTrigger value="cursor" className="gap-1.5">
                  <MousePointerClickIcon className="size-3.5" />
                  Cursor
                </TabsTrigger>
                <TabsTrigger value="claude" className="gap-1.5">
                  <MessageCircleIcon className="size-3.5" />
                  Claude
                </TabsTrigger>
                <TabsTrigger value="vscode" className="gap-1.5">
                  <Code2Icon className="size-3.5" />
                  VS Code
                </TabsTrigger>
                <TabsTrigger value="codex" className="gap-1.5">
                  <CogIcon className="size-3.5" />
                  Codex
                </TabsTrigger>
              </TabsList>

              {(Object.keys(clientMeta) as ClientTab[]).map((key) => {
                const { Icon } = clientMeta[key]
                return (
                  <TabsContent key={key} value={key} className="mt-4 space-y-3">
                    {key === 'cursor' && selector.value && (
                      <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg border">
                        <p className="text-sm text-muted-foreground">
                          Install MCP server directly in Cursor via deep link
                        </p>
                        <InstallInCursorButton serverName={configServerName} config={configByClient[key][0].value} />
                      </div>
                    )}
                    {configByClient[key].map(block => (
                      <Card key={`${key}-${block.title}`}>
                        <CardHeader className="flex flex-row items-center justify-between space-y-0">
                          <div className="flex items-center gap-2">
                            <Icon className="size-4 text-muted-foreground" />
                            <CardTitle className="text-sm">{block.title}</CardTitle>
                          </div>
                          <CopyButton text={block.value} />
                        </CardHeader>
                        <CardContent className="space-y-2">
                          <Textarea readOnly value={block.value} className="font-mono text-xs min-h-[160px]" />
                        </CardContent>
                      </Card>
                    ))}
                  </TabsContent>
                )
              })}
            </Tabs>
          </div>
        </SheetPanel>
        <SheetFooter variant="bare">
          <p className="text-xs text-muted-foreground text-left">
            Tags let one client access multiple servers. Use server mode for a single server.
          </p>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
