// Input: log filters, counts, server options, handlers
// Output: LogToolbar component for filtering and actions
// Position: Logs module toolbar above the log list

import {
  RefreshCwIcon,
  SearchIcon,
  TrashIcon,
  XIcon,
} from 'lucide-react'
import { memo } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'

import type { LogFilters, LogLevel, LogSource } from '../types'
import { levelLabels, sourceLabels } from '../types'

interface LogToolbarProps {
  filters: LogFilters
  onFiltersChange: (filters: LogFilters) => void
  serverOptions: string[]
  logCounts: {
    total: number
    filtered: number
    byLevel: Record<LogLevel, number>
  }
  autoScroll?: boolean
  onAutoScrollChange?: (value: boolean) => void
  connectionStatus: 'connected' | 'disconnected' | 'waiting'
  onClear: () => void
  onRefresh: () => void
}

export const LogToolbar = memo(function LogToolbar({
  filters,
  onFiltersChange,
  serverOptions,
  logCounts,
  autoScroll: _autoScroll,
  onAutoScrollChange: _onAutoScrollChange,
  connectionStatus,
  onClear,
  onRefresh,
}: LogToolbarProps) {
  const hasActiveFilters
    = filters.level !== 'all'
    || filters.source !== 'all'
    || filters.server !== 'all'
    || filters.search !== ''

  const handleClearFilters = () => {
    onFiltersChange({
      level: 'all',
      source: 'all',
      server: 'all',
      search: '',
    })
  }

  const showServerFilter
    = serverOptions.length > 0
    && (filters.source === 'all' || filters.source === 'downstream')

  return (
    <div className="flex flex-wrap items-center gap-3 border-b bg-muted/30 px-4 py-2">
      {/* Search */}
      <div className="relative flex-1 min-w-48 max-w-xs">
        <SearchIcon className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          size="sm"
          placeholder="Search logs..."
          value={filters.search}
          onChange={e =>
            onFiltersChange({ ...filters, search: e.target.value })}
          className="pl-8"
        />
        {filters.search && (
          <Button
            variant="ghost"
            size="icon-xs"
            className="absolute right-1.5 top-1/2 size-5 -translate-y-1/2"
            onClick={() => onFiltersChange({ ...filters, search: '' })}
          >
            <XIcon className="size-3" />
          </Button>
        )}
      </div>

      <Separator orientation="vertical" className="h-6" />

      {/* Level filter */}
      <Select
        value={filters.level}
        onValueChange={value =>
          onFiltersChange({ ...filters, level: value as LogLevel | 'all' })}
      >
        <SelectTrigger size="sm" className="w-32">
          <SelectValue>
            {value => (value ? levelLabels[value as LogLevel | 'all'] : 'Level')}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(Object.entries(levelLabels) as Array<[LogLevel | 'all', string]>).map(
            ([value, label]) => (
              <SelectItem key={value} value={value}>
                <span className="flex items-center gap-2">
                  {label}
                  {value !== 'all' && (
                    <Badge variant="outline" size="sm">
                      {logCounts.byLevel[value as LogLevel]}
                    </Badge>
                  )}
                </span>
              </SelectItem>
            ),
          )}
        </SelectContent>
      </Select>

      {/* Source filter */}
      <Select
        value={filters.source}
        onValueChange={value =>
          onFiltersChange({ ...filters, source: value as LogSource | 'all' })}
      >
        <SelectTrigger size="sm" className="w-36">
          <SelectValue>
            {value =>
              value ? sourceLabels[value as LogSource | 'all'] : 'Source'}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(
            Object.entries(sourceLabels) as Array<[LogSource | 'all', string]>
          ).map(([value, label]) => (
            <SelectItem key={value} value={value}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Server filter */}
      {showServerFilter && (
        <Select
          value={filters.server}
          onValueChange={value =>
            value && onFiltersChange({ ...filters, server: value })}
        >
          <SelectTrigger size="sm" className="w-40">
            <SelectValue>
              {value => (value === 'all' ? 'All servers' : value)}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All servers</SelectItem>
            {serverOptions.map(server => (
              <SelectItem key={server} value={server}>
                {server}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {/* Clear filters */}
      {hasActiveFilters && (
        <Button
          variant="ghost"
          size="sm"
          onClick={handleClearFilters}
          className="text-muted-foreground"
        >
          Clear filters
        </Button>
      )}

      <div className="flex-1" />

      {/* Status indicators */}
      <div className="flex items-center gap-2">
        <Badge
          variant={
            connectionStatus === 'connected'
              ? 'success'
              : connectionStatus === 'waiting'
                ? 'warning'
                : 'error'
          }
          size="sm"
        >
          {connectionStatus === 'connected' && 'Connected'}
          {connectionStatus === 'waiting' && 'Waiting...'}
          {connectionStatus === 'disconnected' && 'Disconnected'}
        </Badge>
        <Badge variant="outline" size="sm">
          {logCounts.filtered}
          {logCounts.filtered !== logCounts.total && ` / ${logCounts.total}`}
        </Badge>
      </div>

      <Separator orientation="vertical" className="h-6" />

      {/* Auto-scroll toggle */}
      {/* <div className="flex items-center gap-2">
        <Checkbox
          id="auto-scroll"
          checked={autoScroll}
          onCheckedChange={checked => onAutoScrollChange(checked === true)}
        />
        <Label htmlFor="auto-scroll" className="text-xs">
          Auto-scroll
        </Label>
      </div> */}

      {/* Actions */}
      <div className="flex items-center gap-1">
        {connectionStatus === 'waiting' && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onRefresh}
            title="Restart log stream"
          >
            <RefreshCwIcon className="size-4" />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onClear}
          title="Clear logs"
        >
          <TrashIcon className="size-4" />
        </Button>
      </div>
    </div>
  )
})

LogToolbar.displayName = 'LogToolbar'
