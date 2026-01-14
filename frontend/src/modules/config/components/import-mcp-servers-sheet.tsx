// Input: MCP JSON payload, profile list/details, config mode
// Output: ImportMcpServersSheet component - JSON import flow for profiles
// Position: Config header action entry

import { AlertCircleIcon, CheckCircleIcon, FileUpIcon } from 'lucide-react'
import { useEffect, useState } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { CheckboxGroup } from '@/components/ui/checkbox-group'
import { Input } from '@/components/ui/input'
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
import { Textarea } from '@/components/ui/textarea'

import {
  parseMcpServersJson,
  type ImportMcpServersRequest,
  type ImportServerDraft,
} from '../lib/mcp-import'
import { useConfigMode, useProfileDetails, useProfiles } from '../hooks'
import { ConfigService } from '@bindings/mcpd/internal/ui'
import { reloadConfig } from '../lib/reload-config'

export const ImportMcpServersSheet = () => {
  const { data: configMode } = useConfigMode()
  const { data: profiles, mutate: mutateProfiles } = useProfiles()
  const {
    data: profileDetails,
    isLoading: profileDetailsLoading,
    mutate: mutateProfileDetails,
  } = useProfileDetails(profiles)

  const [open, setOpen] = useState(false)
  const [rawInput, setRawInput] = useState('')
  const [servers, setServers] = useState<ImportServerDraft[]>([])
  const [parseErrors, setParseErrors] = useState<string[]>([])
  const [selectedProfiles, setSelectedProfiles] = useState<string[]>([])
  const [applyError, setApplyError] = useState<string | null>(null)
  const [isApplying, setIsApplying] = useState(false)
  const [isSaved, setIsSaved] = useState(false)

  useEffect(() => {
    if (open) {
      return
    }
    setRawInput('')
    setServers([])
    setParseErrors([])
    setSelectedProfiles([])
    setApplyError(null)
    setIsApplying(false)
    setIsSaved(false)
  }, [open])

  useEffect(() => {
    if (selectedProfiles.length === 0 && profiles?.length === 1) {
      setSelectedProfiles([profiles[0].name])
    }
  }, [profiles, selectedProfiles.length])

  const existingServerNames = new Set<string>()
  if (profileDetails && selectedProfiles.length > 0) {
    profileDetails.forEach(profile => {
      if (!selectedProfiles.includes(profile.name)) {
        return
      }
      profile.servers.forEach(server => {
        existingServerNames.add(server.name)
      })
    })
  }

  const normalizedNames = servers.map(server => server.name.trim())
  const missingNames = normalizedNames.filter(name => !name)
  const duplicateNames = normalizedNames.filter(
    (name, index) => name && normalizedNames.indexOf(name) !== index,
  )
  const conflicts = normalizedNames.filter(
    name => name && existingServerNames.has(name),
  )

  const issues: string[] = []
  if (selectedProfiles.length === 0) {
    issues.push('Select at least one profile.')
  }
  if (profileDetailsLoading && selectedProfiles.length > 0) {
    issues.push('Loading profile details for conflict checks.')
  }
  if (missingNames.length > 0) {
    issues.push('Server names cannot be empty.')
  }
  if (duplicateNames.length > 0) {
    issues.push(`Duplicate server names: ${Array.from(new Set(duplicateNames)).join(', ')}`)
  }
  if (conflicts.length > 0) {
    issues.push(`Name conflicts in selected profiles: ${Array.from(new Set(conflicts)).join(', ')}`)
  }

  const isWritable = configMode?.isWritable ?? false
  const canApply
    = isWritable
      && parseErrors.length === 0
      && servers.length > 0
      && issues.length === 0
      && !isApplying

  const handleParse = () => {
    const result = parseMcpServersJson(rawInput)
    setServers(result.servers)
    setParseErrors(result.errors)
    setApplyError(null)
    setIsSaved(false)
  }

  const handleNameChange = (id: string, value: string) => {
    setServers(current =>
      current.map(server => (server.id === id ? { ...server, name: value } : server)),
    )
  }

  const handleApply = async () => {
    if (!canApply) {
      return
    }

    setIsApplying(true)
    setApplyError(null)

    const payload: ImportMcpServersRequest = {
      profiles: selectedProfiles,
      servers: servers.map(server => ({
        name: server.name.trim(),
        cmd: server.cmd,
        env: server.env,
        cwd: server.cwd,
      })),
    }

    try {
      await ConfigService.ImportMcpServers(payload)
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setIsSaved(false)
        setApplyError(`Reload failed: ${reloadResult.message}`)
        return
      }
      await mutateProfiles()
      await mutateProfileDetails()
      setIsSaved(true)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Import failed.'
      setApplyError(message)
    } finally {
      setIsApplying(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger>
        <Button
          variant="secondary"
          size="sm"
          disabled={!isWritable}
          title={isWritable ? 'Import MCP servers' : 'Configuration is not writable'}
        >
          <FileUpIcon className="size-4" />
          Import MCP Server
        </Button>
      </SheetTrigger>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>Import MCP servers</SheetTitle>
          <SheetDescription>
            Paste your mcpServers JSON, review the servers, and apply them to profiles.
          </SheetDescription>
        </SheetHeader>
        <SheetPanel className="space-y-6">
          {parseErrors.length > 0 && (
            <Alert variant="error">
              <AlertCircleIcon />
              <AlertTitle>Parsing failed</AlertTitle>
              <AlertDescription>
                {parseErrors.map(error => (
                  <span key={error}>{error}</span>
                ))}
              </AlertDescription>
            </Alert>
          )}

          {applyError && (
            <Alert variant="error">
              <AlertCircleIcon />
              <AlertTitle>Import failed</AlertTitle>
              <AlertDescription>{applyError}</AlertDescription>
            </Alert>
          )}

          {isSaved && (
            <Alert variant="success">
              <CheckCircleIcon />
              <AlertTitle>Saved to profiles</AlertTitle>
              <AlertDescription>Changes applied.</AlertDescription>
            </Alert>
          )}

          <section className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium">Source JSON</h3>
              {servers.length > 0 && (
                <Badge variant="secondary" size="sm">
                  {servers.length} servers
                </Badge>
              )}
            </div>
            <Textarea
              value={rawInput}
              onChange={event => setRawInput(event.target.value)}
              placeholder="Paste mcpServers JSON from Claude or Cursor"
              className="min-h-36 font-mono text-xs"
            />
            <div className="flex items-center gap-2">
              <Button variant="default" size="sm" onClick={handleParse}>
                Parse JSON
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setRawInput('')}
                disabled={!rawInput}
              >
                Clear
              </Button>
            </div>
          </section>

          <Separator />

          <section className="space-y-3">
            <h3 className="text-sm font-medium">Target profiles</h3>
            <CheckboxGroup
              value={selectedProfiles}
              onValueChange={setSelectedProfiles}
            >
              {(profiles ?? []).map(profile => (
                <label
                  key={profile.name}
                  className="flex items-center gap-2 text-sm"
                >
                  <Checkbox value={profile.name} />
                  <span className="font-mono">{profile.name}</span>
                </label>
              ))}
            </CheckboxGroup>
          </section>

          <Separator />

          <section className="space-y-3">
            <h3 className="text-sm font-medium">Servers preview</h3>
            {servers.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                Parse JSON to review servers before importing.
              </p>
            ) : (
              <div className="space-y-3">
                {servers.map(server => (
                  <div
                    key={server.id}
                    className="rounded-lg border bg-muted/20 p-3 space-y-2"
                  >
                    <div className="space-y-1">
                      <span className="text-xs text-muted-foreground">
                        Server name
                      </span>
                      <Input
                        value={server.name}
                        onChange={event => handleNameChange(server.id, event.target.value)}
                        className="font-mono text-xs"
                      />
                    </div>
                    <div className="text-xs text-muted-foreground wrap-break-word">
                      {server.cmd.join(' ')}
                    </div>
                    {(server.cwd || Object.keys(server.env).length > 0) && (
                      <div className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
                        {server.cwd && (
                          <span className="rounded bg-muted/40 px-2 py-0.5 font-mono">
                            cwd: {server.cwd}
                          </span>
                        )}
                        {Object.keys(server.env).length > 0 && (
                          <span className="rounded bg-muted/40 px-2 py-0.5 font-mono">
                            env: {Object.keys(server.env).length}
                          </span>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>

          {issues.length > 0 && (
            <Alert variant="warning">
              <AlertCircleIcon />
              <AlertTitle>Resolve before importing</AlertTitle>
              <AlertDescription>
                {issues.map(issue => (
                  <span key={issue}>{issue}</span>
                ))}
              </AlertDescription>
            </Alert>
          )}
        </SheetPanel>
        <SheetFooter>
          <Button variant="ghost" onClick={() => setOpen(false)}>
            Close
          </Button>
          <Button variant="default" onClick={handleApply} disabled={!canApply}>
            {isApplying ? 'Saving...' : 'Apply to profiles'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
