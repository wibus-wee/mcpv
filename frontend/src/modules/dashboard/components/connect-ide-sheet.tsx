import {
  CheckIcon,
  ClipboardIcon,
  Code2Icon,
  CogIcon,
  MessageCircleIcon,
  MousePointerClickIcon,
  RocketIcon,
} from 'lucide-react'
import type React from 'react'
import { useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { buildClientConfig } from '@/lib/mcpdmcp'
import { useMcpdmcpPath } from '@/hooks/use-mcpdmcp-path'
import { useRpcAddress } from '@/hooks/use-rpc-address'
import { useCallers } from '@/modules/config/hooks'

type ClientTab = 'cursor' | 'claude' | 'vscode' | 'codex'

const clientMeta: Record<ClientTab, { title: string; Icon: React.ComponentType<any> }> = {
  cursor: { title: 'Cursor', Icon: MousePointerClickIcon },
  claude: { title: 'Claude Desktop', Icon: MessageCircleIcon },
  vscode: { title: 'VS Code', Icon: Code2Icon },
  codex: { title: 'Codex', Icon: CogIcon },
}

type PresetBlock = { title: string; value: string }

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

export function ConnectIdeSheet() {
  const [open, setOpen] = useState(false)
  const [caller, setCaller] = useState<string>('cursor')
  const { path, isFallback } = useMcpdmcpPath()
  const { rpcAddress } = useRpcAddress()
  const { data: callers } = useCallers()

  const callerOptions = useMemo(
    () => (callers ? Object.keys(callers) : []),
    [callers],
  )

  useEffect(() => {
    if (callerOptions.length === 0) {
      setCaller('cursor')
      return
    }
    if (!callerOptions.includes(caller)) {
      setCaller(callerOptions[0])
    }
  }, [caller, callerOptions])

  const configByClient = useMemo<Record<ClientTab, PresetBlock[]>>(
    () => ({
      cursor: [
        {
          title: 'Cursor config (json)',
          value: buildClientConfig('cursor', path, caller, rpcAddress),
        },
      ],
      vscode: [
        {
          title: 'VS Code config (json)',
          value: buildClientConfig('vscode', path, caller, rpcAddress),
        },
      ],
      claude: [
        {
          title: 'Claude CLI (stdio)',
          value: `claude mcp add --transport stdio mcpd -- ${path} ${caller} --rpc ${rpcAddress}`,
        },
        {
          title: 'Claude config (json)',
          value: buildClientConfig('claude', path, caller, rpcAddress),
        },
      ],
      codex: [
        {
          title: 'Codex CLI (stdio)',
          value: `codex mcp add mcpd -- ${path} ${caller} --rpc ${rpcAddress}`,
        },
        {
          title: 'Codex config (config.toml)',
          value: [
            `[mcp_servers.mcpd]`,
            `command = "${path}"`,
            `args = ["${caller}", "--rpc", "${rpcAddress}"]`,
          ].join('\n'),
        },
      ],
    }),
    [caller, path, rpcAddress],
  )

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger>
        <Button variant="secondary" size="sm">
          <RocketIcon className="size-4" />
          Connect IDE
        </Button>
      </SheetTrigger>
      <SheetContent side="right" showCloseButton>
        <SheetHeader>
          <SheetTitle>Connect your IDE</SheetTitle>
          <SheetDescription>Copy ready-to-use commands or config snippets for common clients.</SheetDescription>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Badge variant={isFallback ? 'outline' : 'secondary'} size="sm">
              Path {isFallback ? 'Fallback' : 'Detected'}
            </Badge>
          </div>
        </SheetHeader>
        <SheetPanel className="space-y-6">
          <div className="space-y-3">
            <p className="text-sm font-medium">Client presets</p>
            <div className="flex flex-col gap-1">
              <p className="text-xs text-muted-foreground">Caller</p>
              <Select value={caller} onValueChange={setCaller}>
                <SelectTrigger className="w-48">
                  <SelectValue placeholder="Select caller" />
                </SelectTrigger>
                <SelectContent>
                  {(callerOptions.length ? callerOptions : ['cursor', 'claude', 'vscode', 'codex']).map(option => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
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

              {(Object.keys(clientMeta) as ClientTab[]).map(key => {
                const Icon = clientMeta[key].Icon
                return (
                  <TabsContent key={key} value={key} className="mt-4 space-y-3">
                    {configByClient[key].map(block => (
                      <Card key={`${key}-${block.title}`}>
                        <CardHeader className="flex flex-row items-center justify-between space-y-0">
                          <div className="flex items-center gap-2">
                            <Icon className="size-4 text-muted-foreground" />
                            <CardTitle className="text-sm">{block.title}</CardTitle>
                          </div>
                          <div className="flex items-center justify-end">
                            <CopyButton text={block.value} />
                          </div>
                        </CardHeader>
                        <CardContent className="space-y-2">
                          <Textarea readOnly value={block.value} className="font-mono text-xs min-h-42" />
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
            Need a different caller or RPC address? Adjust the command after copying.
          </p>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
