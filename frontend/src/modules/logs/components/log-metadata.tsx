// Log metadata section for detail drawer
// Displays key-value pairs: Request ID, Path, Host, etc.

import { CheckIcon, CopyIcon } from 'lucide-react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

import type { LogEntry } from '../types'
import { formatDateTime, formatFieldValue, getVisibleFields } from '../utils'

interface LogMetadataProps {
  log: LogEntry
}

export function LogMetadata({ log }: LogMetadataProps) {
  const [copiedField, setCopiedField] = useState<string | null>(null)

  const handleCopy = async (key: string, value: string) => {
    await navigator.clipboard.writeText(value)
    setCopiedField(key)
    setTimeout(() => setCopiedField(null), 2000)
  }

  const visibleFields = getVisibleFields(log.fields)

  // Extract commonly used metadata
  const metadata = [
    { key: 'ID', value: log.id },
    { key: 'Timestamp', value: formatDateTime(log.timestamp) },
    { key: 'Level', value: log.level.toUpperCase() },
    { key: 'Source', value: log.source },
    ...(log.logger ? [{ key: 'Logger', value: log.logger }] : []),
    ...(log.serverType ? [{ key: 'Server', value: log.serverType }] : []),
    ...(log.stream ? [{ key: 'Stream', value: log.stream }] : []),
  ]

  return (
    <div className="space-y-4">
      {/* General Metadata */}
      <div>
        <h4 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          General
        </h4>
        <div className="space-y-1">
          {metadata.map(({ key, value }) => (
            <MetadataRow
              key={key}
              label={key}
              value={value}
              isCopied={copiedField === key}
              onCopy={() => handleCopy(key, value)}
            />
          ))}
        </div>
      </div>

      {/* Fields as tags */}
      {visibleFields.length > 0 && (
        <>
          <Separator />
          <div>
            <h4 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Fields
            </h4>
            <div className="flex flex-wrap gap-1.5">
              {visibleFields.map(([key, value]) => (
                <Badge
                  key={key}
                  variant="outline"
                  size="sm"
                  className="cursor-pointer font-mono"
                  onClick={() => handleCopy(key, formatFieldValue(value))}
                >
                  {key}: {truncateValue(formatFieldValue(value))}
                </Badge>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

interface MetadataRowProps {
  label: string
  value: string
  isCopied?: boolean
  onCopy?: () => void
}

function MetadataRow({ label, value, isCopied, onCopy }: MetadataRowProps) {
  return (
    <div className="group flex items-start justify-between gap-2 py-1">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-center gap-1">
        <span className="truncate font-mono text-xs">{value}</span>
        {onCopy && (
          <Button
            variant="ghost"
            size="icon-xs"
            className="size-5 shrink-0 opacity-0 group-hover:opacity-100"
            onClick={onCopy}
          >
            {isCopied ? (
              <CheckIcon className="size-3" />
            ) : (
              <CopyIcon className="size-3" />
            )}
          </Button>
        )}
      </div>
    </div>
  )
}

function truncateValue(value: string, maxLength = 32): string {
  if (value.length <= maxLength) return value
  return `${value.slice(0, maxLength)}...`
}
