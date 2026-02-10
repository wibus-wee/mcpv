// Input: motion/react, lucide-react icons, UI components, server hooks, analytics, mcpvmcp config helpers
// Output: ConnectIdeSheet component for IDE connection presets with comprehensive configuration options
// Position: Shared UI component for configuring IDE connections in the app

import {
  CheckIcon,
  ChevronDownIcon,
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
import { useMcpvmcpPath } from '@/hooks/use-mcpvmcp-path'
import { useRpcAddress } from '@/hooks/use-rpc-address'
import { AnalyticsEvents, track } from '@/lib/analytics'
import type { BuildOptions, SelectorMode, TransportType } from '@/lib/mcpvmcp'
import { buildClientConfig, buildCliSnippet, buildTomlConfig } from '@/lib/mcpvmcp'
import { useServers } from '@/modules/servers/hooks'

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
  const [transport, setTransport] = useState<TransportType>('stdio')
  const [launchUIOnFail, setLaunchUIOnFail] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)

  // HTTP settings
  const [httpAddr, setHttpAddr] = useState('127.0.0.1:8090')
  const [httpPath, setHttpPath] = useState('/mcp')
  const [httpToken, setHttpToken] = useState('')
  const [httpAllowedOrigins, setHttpAllowedOrigins] = useState('')
  const [httpJSONResponse, setHttpJSONResponse] = useState(false)
  const [httpSessionTimeout, setHttpSessionTimeout] = useState(0)
  const [httpTLSEnabled, setHttpTLSEnabled] = useState(false)
  const [httpTLSCertFile, setHttpTLSCertFile] = useState('')
  const [httpTLSKeyFile, setHttpTLSKeyFile] = useState('')
  const [httpEventStore, setHttpEventStore] = useState(false)
  const [httpEventStoreBytes, setHttpEventStoreBytes] = useState(0)

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
  const { data: servers } = useServers()
  const sidebar = useSidebar()
  const wasOpenRef = useRef(false)
  const lastTrackedSelectorRef = useRef<{ mode: SelectorMode, value: string } | null>(null)

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

    // HTTP settings
    if (transport === 'streamable-http') {
      options.httpAddr = httpAddr
      options.httpPath = httpPath
      if (httpToken) options.httpToken = httpToken
      if (httpAllowedOrigins) {
        options.httpAllowedOrigins = httpAllowedOrigins.split(',').map(s => s.trim()).filter(Boolean)
      }
      if (httpJSONResponse) options.httpJSONResponse = true
      if (httpSessionTimeout > 0) options.httpSessionTimeout = httpSessionTimeout
      if (httpTLSEnabled) {
        options.httpTLSEnabled = true
        if (httpTLSCertFile) options.httpTLSCertFile = httpTLSCertFile
        if (httpTLSKeyFile) options.httpTLSKeyFile = httpTLSKeyFile
      }
      if (httpEventStore) {
        options.httpEventStore = true
        if (httpEventStoreBytes > 0) options.httpEventStoreBytes = httpEventStoreBytes
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
    httpAddr,
    httpPath,
    httpToken,
    httpAllowedOrigins,
    httpJSONResponse,
    httpSessionTimeout,
    httpTLSEnabled,
    httpTLSCertFile,
    httpTLSKeyFile,
    httpEventStore,
    httpEventStoreBytes,
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
          title: 'Claude CLI (stdio)',
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
          title: 'Codex CLI (stdio)',
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
              <ToggleGroupItem value="stdio" size="sm" variant="outline">
                <ZapIcon className="size-3" />
                stdio
              </ToggleGroupItem>
              <ToggleGroupItem value="streamable-http" size="sm" variant="outline">
                <GlobeIcon className="size-3" />
                HTTP
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          {transport === 'streamable-http' && (
            <m.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2 }}
              className="space-y-3"
            >
              <div className="space-y-2">
                <Label htmlFor="http-addr" className="text-sm">
                  HTTP Address
                </Label>
                <Input
                  id="http-addr"
                  value={httpAddr}
                  onChange={e => setHttpAddr(e.target.value)}
                  placeholder="127.0.0.1:8090"
                  className="font-mono text-xs"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="http-path" className="text-sm">
                  HTTP Path
                </Label>
                <Input
                  id="http-path"
                  value={httpPath}
                  onChange={e => setHttpPath(e.target.value)}
                  placeholder="/mcp"
                  className="font-mono text-xs"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="http-token" className="text-sm">
                  Bearer Token
                  <span className="text-xs text-muted-foreground ml-2">(required for non-localhost)</span>
                </Label>
                <Input
                  id="http-token"
                  type="password"
                  value={httpToken}
                  onChange={e => setHttpToken(e.target.value)}
                  placeholder="Optional bearer token"
                  className="font-mono text-xs"
                />
              </div>
            </m.div>
          )}

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

              {/* HTTP Advanced Settings */}
              {transport === 'streamable-http' && (
                <m.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.2 }}
                  className="space-y-3 rounded-lg border border-border/50 p-4 bg-muted/20"
                >
                  <div className="flex items-center gap-2">
                    <GlobeIcon className="size-4 text-muted-foreground" />
                    <p className="text-sm font-medium">HTTP Advanced</p>
                  </div>
                  <div className="space-y-3">
                    <div className="space-y-2">
                      <Label htmlFor="http-allowed-origins" className="text-xs">
                        Allowed Origins
                        <span className="text-muted-foreground ml-2">(comma-separated)</span>
                      </Label>
                      <Input
                        id="http-allowed-origins"
                        value={httpAllowedOrigins}
                        onChange={e => setHttpAllowedOrigins(e.target.value)}
                        placeholder="https://example.com, *"
                        className="font-mono text-xs"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="http-session-timeout" className="text-xs">
                        Session Timeout (seconds)
                      </Label>
                      <Input
                        id="http-session-timeout"
                        type="number"
                        value={httpSessionTimeout}
                        onChange={e => setHttpSessionTimeout(Number(e.target.value))}
                        placeholder="0 (disabled)"
                        className="font-mono text-xs"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <Label htmlFor="http-json-response" className="text-xs">
                        Use JSON Response (instead of SSE)
                      </Label>
                      <Switch
                        id="http-json-response"
                        checked={httpJSONResponse}
                        onCheckedChange={setHttpJSONResponse}
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <Label htmlFor="http-tls" className="text-xs">
                        Enable HTTP TLS
                      </Label>
                      <Switch
                        id="http-tls"
                        checked={httpTLSEnabled}
                        onCheckedChange={setHttpTLSEnabled}
                      />
                    </div>
                    {httpTLSEnabled && (
                      <m.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        exit={{ opacity: 0, height: 0 }}
                        transition={{ duration: 0.2 }}
                        className="space-y-3"
                      >
                        <div className="space-y-2">
                          <Label htmlFor="http-tls-cert" className="text-xs">
                            TLS Cert File
                          </Label>
                          <Input
                            id="http-tls-cert"
                            value={httpTLSCertFile}
                            onChange={e => setHttpTLSCertFile(e.target.value)}
                            placeholder="/path/to/cert.pem"
                            className="font-mono text-xs"
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="http-tls-key" className="text-xs">
                            TLS Key File
                          </Label>
                          <Input
                            id="http-tls-key"
                            value={httpTLSKeyFile}
                            onChange={e => setHttpTLSKeyFile(e.target.value)}
                            placeholder="/path/to/key.pem"
                            className="font-mono text-xs"
                          />
                        </div>
                      </m.div>
                    )}
                    <div className="flex items-center justify-between">
                      <Label htmlFor="http-event-store" className="text-xs">
                        Enable Event Store
                      </Label>
                      <Switch
                        id="http-event-store"
                        checked={httpEventStore}
                        onCheckedChange={setHttpEventStore}
                      />
                    </div>
                    {httpEventStore && (
                      <m.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        exit={{ opacity: 0, height: 0 }}
                        transition={{ duration: 0.2 }}
                        className="space-y-2"
                      >
                        <Label htmlFor="http-event-store-bytes" className="text-xs">
                          Event Store Max Bytes
                        </Label>
                        <Input
                          id="http-event-store-bytes"
                          type="number"
                          value={httpEventStoreBytes}
                          onChange={e => setHttpEventStoreBytes(Number(e.target.value))}
                          placeholder="0 (default)"
                          className="font-mono text-xs"
                        />
                      </m.div>
                    )}
                  </div>
                </m.div>
              )}
            </CollapsibleContent>
          </Collapsible>

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
