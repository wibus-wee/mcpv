// Input: motion/react, lucide-react icons, UI components, server hooks, core state, UI settings, analytics, mcpvmcp config helpers
// Output: ConnectIdeSheet component for IDE connection presets with comprehensive configuration options
// Position: Shared UI component for configuring IDE connections in the app

import {
  CheckIcon,
  ChevronDownIcon,
  ClipboardCopyIcon,
  ClipboardIcon,
  Code2Icon,
  CogIcon,
  DownloadIcon,
  GlobeIcon,
  MessageCircleIcon,
  MousePointerClickIcon,
  RocketIcon,
  ServerIcon,
  SettingsIcon,
  TagIcon,
  ZapIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import type * as React from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useCoreState } from '@/hooks/use-core-state'
import { useMcpvmcpPath } from '@/hooks/use-mcpvmcp-path'
import { useRpcAddress } from '@/hooks/use-rpc-address'
import { useUISettings } from '@/hooks/use-ui-settings'
import { AnalyticsEvents, track } from '@/lib/analytics'
import type { BuildOptions, SelectorMode, TransportType } from '@/lib/mcpvmcp'
import { buildClientConfig, buildCliSnippet, buildTomlConfig } from '@/lib/mcpvmcp'
import { parseEnvironmentVariables } from '@/lib/parsers'
import { useServers } from '@/modules/servers/hooks'
import { buildEndpointPreview, toGatewayFormState } from '@/modules/settings/lib/gateway-config'

import { useSidebar } from '../ui/sidebar'

type ClientTab = 'cursor' | 'claude' | 'vscode' | 'codex'

type DisplayType = 'textarea' | 'input'
type PresetBlock = { title: string, value: string, displayType: DisplayType }

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

function CopyButton({
  text,
  client,
  blockTitle,
}: {
  text: string
  client: string
  blockTitle: string
}) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
      track(AnalyticsEvents.CONNECT_IDE_COPY, {
        client,
        block: blockTitle,
        result: 'success',
      })
    }
    catch {
      track(AnalyticsEvents.CONNECT_IDE_COPY, {
        client,
        block: blockTitle,
        result: 'error',
      })
      console.error('[ConnectIde] Failed to copy preset')
    }
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

function InlineCopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    }
    catch (error) {
      console.error('[ConnectIde] Failed to copy value', error)
    }
  }

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className="shrink-0"
      onClick={handleCopy}
      aria-label="Copy"
    >
      {copied ? <CheckIcon className="size-4 text-emerald-500" /> : <ClipboardCopyIcon className="size-4" />}
    </Button>
  )
}

function InstallInCursorButton({
  serverName,
  config,
}: {
  serverName: string
  config: string
}) {
  const handleInstall = () => {
    try {
      const deepLink = generateCursorDeepLink(serverName, config)
      window.location.href = deepLink
      track(AnalyticsEvents.CONNECT_IDE_INSTALL_CURSOR, { result: 'success', client: 'cursor' })
    }
    catch (error) {
      track(AnalyticsEvents.CONNECT_IDE_INSTALL_CURSOR, { result: 'error', client: 'cursor' })
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

function AutoHeightTextarea({ value }: { value: string }) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto'
      ref.current.style.height = `${ref.current.scrollHeight}px`
    }
  }, [value])

  return (
    <Textarea
      ref={ref}
      readOnly
      value={value}
      className="font-mono text-xs resize-none"
      style={{ minHeight: '6rem', overflow: 'hidden' }}
    />
  )
}

export function ConnectIdeSheet() {
  const [open, setOpen] = useState(false)
  const [selectorMode, setSelectorMode] = useState<SelectorMode>('server')
  const [selectorValue, setSelectorValue] = useState('')
  const [transport, setTransport] = useState<TransportType>('streamable-http')
  const [launchUIOnFail, setLaunchUIOnFail] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [showHttpAdvanced, setShowHttpAdvanced] = useState(false)
  const [httpSettingsTouched, setHttpSettingsTouched] = useState(false)

  // Streamable HTTP settings
  const [httpUrl, setHttpUrl] = useState('')
  const [httpHeaders, setHttpHeaders] = useState('')

  // RPC settings
  const [rpcMaxRecvMsgSize, setRpcMaxRecvMsgSize] = useState<number | undefined>()
  const [rpcMaxSendMsgSize, setRpcMaxSendMsgSize] = useState<number | undefined>()
  const [rpcKeepaliveTime, setRpcKeepaliveTime] = useState<number | undefined>()
  const [rpcKeepaliveTimeout, setRpcKeepaliveTimeout] = useState<number | undefined>()
  const [rpcTLSEnabled, setRpcTLSEnabled] = useState(false)
  const [rpcTLSCertFile, setRpcTLSCertFile] = useState('')
  const [rpcTLSKeyFile, setRpcTLSKeyFile] = useState('')
  const [rpcTLSCAFile, setRpcTLSCAFile] = useState('')

  const { path } = useMcpvmcpPath()
  const { rpcAddress } = useRpcAddress()
  const { coreStatus } = useCoreState()
  const { sections: uiSections } = useUISettings({ scope: 'global' })
  const { data: servers } = useServers()
  const sidebar = useSidebar()
  const wasOpenRef = useRef(false)
  const lastTrackedSelectorRef = useRef<{ mode: SelectorMode, value: string } | null>(null)
  const transportLabel = transport === 'streamable-http' ? 'http' : 'stdio'

  const gatewaySection = uiSections?.gateway
  const gatewaySettings = useMemo(() => toGatewayFormState(gatewaySection), [gatewaySection])
  const gatewayEndpoint = useMemo(
    () => buildEndpointPreview(gatewaySettings.httpAddr, gatewaySettings.httpPath),
    [gatewaySettings.httpAddr, gatewaySettings.httpPath],
  )
  const gatewayHeaderValue = useMemo(
    () => (gatewaySettings.httpToken ? `X-Mcp-Token=${gatewaySettings.httpToken}` : ''),
    [gatewaySettings.httpToken],
  )
  const isGatewayEnabled = gatewaySettings.enabled
  const isGatewayRunning = isGatewayEnabled && coreStatus === 'running'
  const gatewaySummary = useMemo(() => {
    if (isGatewayRunning) {
      return {
        title: 'App-managed gateway is running',
        description: 'This app launches mcpvmcp for you. Clients will connect using Settings > Gateway.',
      }
    }
    if (isGatewayEnabled) {
      if (coreStatus === 'starting') {
        return {
          title: 'Gateway is starting',
          description: 'Core is starting. The app will launch mcpvmcp when ready.',
        }
      }
      if (coreStatus === 'stopping') {
        return {
          title: 'Gateway is stopping',
          description: 'Core is stopping. Start it again to let the app launch mcpvmcp.',
        }
      }
      if (coreStatus === 'error') {
        return {
          title: 'Gateway not running',
          description: 'Core is in an error state. Restart it to let the app launch mcpvmcp.',
        }
      }
      return {
        title: 'Gateway configured but not running',
        description: 'Core is stopped. Start the core to let the app launch mcpvmcp.',
      }
    }
    return {
      title: 'Gateway not managed by the app',
      description: 'Enable the gateway in Settings > Gateway to let the app host mcpvmcp, or customize HTTP settings here.',
    }
  }, [coreStatus, isGatewayEnabled, isGatewayRunning])

  useEffect(() => {
    if (transport !== 'streamable-http' || httpSettingsTouched) return
    setHttpUrl(gatewayEndpoint)
    setHttpHeaders(gatewayHeaderValue)
  }, [gatewayEndpoint, gatewayHeaderValue, httpSettingsTouched, transport])

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

  const buildOptions = useMemo<BuildOptions>(() => {
    const options: BuildOptions = {
      transport,
      launchUIOnFail,
    }

    // Streamable HTTP settings
    if (transport === 'streamable-http') {
      const resolvedUrl = httpUrl.trim() || gatewayEndpoint
      if (resolvedUrl) options.httpUrl = resolvedUrl
      const parsedHeaders = parseEnvironmentVariables(httpHeaders)
      if (Object.keys(parsedHeaders).length > 0) {
        options.httpHeaders = parsedHeaders
      }
    }

    // RPC settings
    if (rpcMaxRecvMsgSize) options.rpcMaxRecvMsgSize = rpcMaxRecvMsgSize
    if (rpcMaxSendMsgSize) options.rpcMaxSendMsgSize = rpcMaxSendMsgSize
    if (rpcKeepaliveTime) options.rpcKeepaliveTime = rpcKeepaliveTime
    if (rpcKeepaliveTimeout) options.rpcKeepaliveTimeout = rpcKeepaliveTimeout
    if (rpcTLSEnabled) {
      options.rpcTLSEnabled = true
      if (rpcTLSCertFile) options.rpcTLSCertFile = rpcTLSCertFile
      if (rpcTLSKeyFile) options.rpcTLSKeyFile = rpcTLSKeyFile
      if (rpcTLSCAFile) options.rpcTLSCAFile = rpcTLSCAFile
    }

    return options
  }, [
    transport,
    launchUIOnFail,
    httpUrl,
    httpHeaders,
    gatewayEndpoint,
    rpcMaxRecvMsgSize,
    rpcMaxSendMsgSize,
    rpcKeepaliveTime,
    rpcKeepaliveTimeout,
    rpcTLSEnabled,
    rpcTLSCertFile,
    rpcTLSKeyFile,
    rpcTLSCAFile,
  ])

  const configByClient = useMemo<Record<ClientTab, PresetBlock[]>>(
    () => ({
      cursor: [
        {
          title: 'Cursor config (json)',
          value: buildClientConfig('cursor', path, selector, rpcAddress, buildOptions),
          displayType: 'textarea',
        },
      ],
      vscode: [
        {
          title: 'VS Code config (json)',
          value: buildClientConfig('vscode', path, selector, rpcAddress, buildOptions),
          displayType: 'textarea',
        },
      ],
      claude: [
        {
          title: `Claude CLI (${transportLabel})`,
          value: buildCliSnippet(path, selector, rpcAddress, 'claude', buildOptions),
          displayType: 'input',
        },
        {
          title: 'Claude config (json)',
          value: buildClientConfig('claude', path, selector, rpcAddress, buildOptions),
          displayType: 'textarea',
        },
      ],
      codex: [
        {
          title: `Codex CLI (${transportLabel})`,
          value: buildCliSnippet(path, selector, rpcAddress, 'codex', buildOptions),
          displayType: 'input',
        },
        {
          title: 'Codex config (config.toml)',
          value: buildTomlConfig(path, selector, rpcAddress, buildOptions),
          displayType: 'textarea',
        },
      ],
    }),
    [path, selector, rpcAddress, buildOptions],
  )

  const suggestions = useMemo(() =>
    selectorMode === 'server' ? serverOptions : tagOptions, [selectorMode, serverOptions, tagOptions])

  useEffect(() => {
    if (open && !wasOpenRef.current) {
      track(AnalyticsEvents.CONNECT_IDE_OPENED, {
        selector_mode: selectorMode,
        server_count: serverOptions.length,
        tag_count: tagOptions.length,
      })
    }
    wasOpenRef.current = open
  }, [open, selectorMode, serverOptions.length, tagOptions.length])

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
        <RocketIcon className="size-4" />
        <m.span
          initial={{ opacity: 0, width: 0 }}
          animate={{ opacity: sidebar.open ? 1 : 0, width: sidebar.open ? 'auto' : 0 }}
          transition={{ duration: 0.2 }}
        >
          Connect Agent
        </m.span>
      </Button>
      <SheetContent side="right" showCloseButton>
        <SheetHeader>
          <SheetTitle>Connect your Agent</SheetTitle>
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
                const mode = next ?? 'server'
                const nextDefault = mode === 'server'
                  ? (serverOptions[0] ?? '')
                  : (tagOptions[0] ?? '')
                const hasValue = Boolean((selectorValue || nextDefault).trim())
                setSelectorMode(mode)
                track(AnalyticsEvents.CONNECT_IDE_TARGET_CHANGE, {
                  mode,
                  value_source: 'toggle',
                  has_value: hasValue,
                })
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
                onBlur={() => {
                  const value = effectiveSelectorValue.trim()
                  const last = lastTrackedSelectorRef.current
                  if (last?.mode === selectorMode && last.value === value) return
                  lastTrackedSelectorRef.current = { mode: selectorMode, value }
                  track(AnalyticsEvents.CONNECT_IDE_TARGET_CHANGE, {
                    mode: selectorMode,
                    value_source: 'manual',
                    has_value: value.length > 0,
                  })
                }}
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
                      onClick={() => {
                        setSelectorValue(option)
                        lastTrackedSelectorRef.current = { mode: selectorMode, value: option }
                        track(AnalyticsEvents.CONNECT_IDE_TARGET_CHANGE, {
                          mode: selectorMode,
                          value_source: 'suggestion',
                          has_value: true,
                        })
                      }}
                    >
                      {option}
                    </Button>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="space-y-3">
            <p className="text-sm font-medium">Transport</p>
            <ToggleGroup
              multiple={false}
              value={[transport]}
              onValueChange={(values) => {
                const next = values[0] as TransportType | undefined
                const mode = next ?? 'stdio'
                setTransport(mode)
                track(AnalyticsEvents.CONNECT_IDE_OPTION_CHANGE, {
                  option: 'transport',
                  value: mode,
                })
              }}
              className="flex flex-wrap gap-2"
            >
              <ToggleGroupItem value="streamable-http" size="sm" variant="outline">
                <GlobeIcon className="size-3" />
                HTTP
              </ToggleGroupItem>
              <ToggleGroupItem value="stdio" size="sm" variant="outline">
                <ZapIcon className="size-3" />
                stdio
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          {transport === 'streamable-http' && (
            <div
              className="space-y-3"
            >
              <div className="space-y-3 rounded-lg border border-border/50 bg-muted/20 p-4">
                <div className="space-y-1">
                  <p className="text-sm font-medium">{gatewaySummary.title}</p>
                  <p className="text-xs text-muted-foreground">{gatewaySummary.description}</p>
                </div>
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3 rounded-md border border-border/40 bg-background/70 px-3 py-2">
                    <div className="space-y-0.5">
                      <p className="text-xs text-muted-foreground">Base endpoint</p>
                      <p className="font-mono text-xs">{gatewayEndpoint}</p>
                    </div>
                    <InlineCopyButton text={gatewayEndpoint} />
                  </div>
                  {gatewayHeaderValue ? (
                    <div className="flex items-center justify-between gap-3 rounded-md border border-border/40 bg-background/70 px-3 py-2">
                      <div className="space-y-0.5">
                        <p className="text-xs text-muted-foreground">Token header</p>
                        <p className="font-mono text-xs">{gatewayHeaderValue}</p>
                      </div>
                      <InlineCopyButton text={gatewayHeaderValue} />
                    </div>
                  ) : null}
                </div>
                <Button
                  variant="link"
                  size="xs"
                  className="h-auto px-0 text-xs"
                  onClick={() => {
                    if (showHttpAdvanced) {
                      setHttpSettingsTouched(false)
                      setHttpUrl(gatewayEndpoint)
                      setHttpHeaders(gatewayHeaderValue)
                    }
                    setShowHttpAdvanced(prev => !prev)
                  }}
                >
                  {showHttpAdvanced ? 'Use app-managed settings' : 'Customize HTTP settings'}
                </Button>
              </div>
              {showHttpAdvanced && (
                <m.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.2 }}
                  className="space-y-3"
                >
                  <p className="text-xs text-muted-foreground">
                    Custom HTTP settings override the app-managed gateway defaults.
                  </p>
                  <div className="space-y-2">
                    <Label htmlFor="http-url" className="text-sm">
                      HTTP base URL
                      <span className="text-xs text-muted-foreground ml-2">(before /server or /tags)</span>
                    </Label>
                    <Input
                      id="http-url"
                      value={httpUrl}
                      onChange={(event) => {
                        setHttpSettingsTouched(true)
                        setHttpUrl(event.target.value)
                      }}
                      placeholder="https://mcp.context7.com/mcp"
                      className="font-mono text-xs"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="http-headers" className="text-sm">
                      HTTP Headers (optional)
                      <span className="text-xs text-muted-foreground ml-2">(one per line, KEY=VALUE)</span>
                    </Label>
                    <Textarea
                      id="http-headers"
                      value={httpHeaders}
                      onChange={(event) => {
                        setHttpSettingsTouched(true)
                        setHttpHeaders(event.target.value)
                      }}
                      placeholder="CONTEXT7_API_KEY=YOUR_API_KEY"
                      className="font-mono text-xs resize-none"
                      rows={3}
                    />
                  </div>
                </m.div>
              )}
            </div>
          )}

          {transport === 'stdio' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="launch-ui-on-fail" className="text-sm font-medium">
                    Launch UI on connection failure
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    Automatically launch MCPV UI when a client fails to connect because the mcpv isn't running.
                  </p>
                </div>
                <Switch
                  id="launch-ui-on-fail"
                  checked={launchUIOnFail}
                  onCheckedChange={(checked) => {
                    setLaunchUIOnFail(checked)
                    track(AnalyticsEvents.CONNECT_IDE_OPTION_CHANGE, {
                      option: 'launchUIOnFail',
                      value: checked,
                    })
                  }}
                />
              </div>
            </div>
          )}

          {transport === 'stdio' && (
            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <CollapsibleTrigger>
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-between px-0 hover:bg-transparent"
                >
                  <div className="flex items-center gap-2">
                    <SettingsIcon className="size-4" />
                    <span className="text-sm font-medium">Advanced Settings</span>
                  </div>
                  <m.div
                    animate={{ rotate: advancedOpen ? 180 : 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <ChevronDownIcon className="size-4 text-muted-foreground" />
                  </m.div>
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-4 pt-4">
                {/* RPC Settings */}
                <div className="space-y-3 rounded-lg border border-border/50 p-4 bg-muted/20">
                  <div className="flex items-center gap-2">
                    <ServerIcon className="size-4 text-muted-foreground" />
                    <p className="text-sm font-medium">RPC Settings</p>
                  </div>
                  <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label htmlFor="rpc-max-recv" className="text-xs">
                          Max Recv Size (bytes)
                        </Label>
                        <Input
                          id="rpc-max-recv"
                          type="number"
                          value={rpcMaxRecvMsgSize ?? ''}
                          onChange={e => setRpcMaxRecvMsgSize(e.target.value ? Number(e.target.value) : undefined)}
                          placeholder="Default"
                          className="font-mono text-xs"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="rpc-max-send" className="text-xs">
                          Max Send Size (bytes)
                        </Label>
                        <Input
                          id="rpc-max-send"
                          type="number"
                          value={rpcMaxSendMsgSize ?? ''}
                          onChange={e => setRpcMaxSendMsgSize(e.target.value ? Number(e.target.value) : undefined)}
                          placeholder="Default"
                          className="font-mono text-xs"
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label htmlFor="rpc-keepalive-time" className="text-xs">
                          Keepalive Time (s)
                        </Label>
                        <Input
                          id="rpc-keepalive-time"
                          type="number"
                          value={rpcKeepaliveTime ?? ''}
                          onChange={e => setRpcKeepaliveTime(e.target.value ? Number(e.target.value) : undefined)}
                          placeholder="Default"
                          className="font-mono text-xs"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="rpc-keepalive-timeout" className="text-xs">
                          Keepalive Timeout (s)
                        </Label>
                        <Input
                          id="rpc-keepalive-timeout"
                          type="number"
                          value={rpcKeepaliveTimeout ?? ''}
                          onChange={e => setRpcKeepaliveTimeout(e.target.value ? Number(e.target.value) : undefined)}
                          placeholder="Default"
                          className="font-mono text-xs"
                        />
                      </div>
                    </div>
                    <div className="flex items-center justify-between pt-2">
                      <Label htmlFor="rpc-tls" className="text-xs">
                        Enable RPC TLS
                      </Label>
                      <Switch
                        id="rpc-tls"
                        checked={rpcTLSEnabled}
                        onCheckedChange={setRpcTLSEnabled}
                      />
                    </div>
                    {rpcTLSEnabled && (
                      <m.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        exit={{ opacity: 0, height: 0 }}
                        transition={{ duration: 0.2 }}
                        className="space-y-3"
                      >
                        <div className="space-y-2">
                          <Label htmlFor="rpc-tls-cert" className="text-xs">
                            TLS Cert File
                          </Label>
                          <Input
                            id="rpc-tls-cert"
                            value={rpcTLSCertFile}
                            onChange={e => setRpcTLSCertFile(e.target.value)}
                            placeholder="/path/to/cert.pem"
                            className="font-mono text-xs"
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="rpc-tls-key" className="text-xs">
                            TLS Key File
                          </Label>
                          <Input
                            id="rpc-tls-key"
                            value={rpcTLSKeyFile}
                            onChange={e => setRpcTLSKeyFile(e.target.value)}
                            placeholder="/path/to/key.pem"
                            className="font-mono text-xs"
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="rpc-tls-ca" className="text-xs">
                            TLS CA File
                          </Label>
                          <Input
                            id="rpc-tls-ca"
                            value={rpcTLSCAFile}
                            onChange={e => setRpcTLSCAFile(e.target.value)}
                            placeholder="/path/to/ca.pem"
                            className="font-mono text-xs"
                          />
                        </div>
                      </m.div>
                    )}
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>
          )}

          <div className="space-y-3">
            <p className="text-sm font-medium">Client presets</p>
            <Tabs
              defaultValue="cursor"
              onValueChange={(value) => {
                if (typeof value === 'string') {
                  track(AnalyticsEvents.CONNECT_IDE_TAB_CHANGE, { client: value })
                }
              }}
            >
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
                          <CopyButton text={block.value} client={key} blockTitle={block.title} />
                        </CardHeader>
                        <CardContent className="space-y-2">
                          {block.displayType === 'input' ? (
                            <Input readOnly value={block.value} className="font-mono text-xs" />
                          ) : (
                            <AutoHeightTextarea value={block.value} />
                          )}
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
