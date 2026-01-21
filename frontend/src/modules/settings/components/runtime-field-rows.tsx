// Input: form register props, UI input components
// Output: Runtime field rows and skeleton
// Position: Shared runtime form rows

import type * as React from 'react'
import type { UseFormRegisterReturn } from 'react-hook-form'

import { InputGroup, InputGroupInput, InputGroupText } from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface RuntimeFieldRowProps {
  label: string
  description?: string
  htmlFor: string
  children: React.ReactNode
  className?: string
}

export const RuntimeFieldRow = ({
  label,
  description,
  htmlFor,
  children,
  className,
}: RuntimeFieldRowProps) => (
  <div
    className={cn(
      'grid gap-3 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,280px)] sm:items-center',
      className,
    )}
  >
    <div className="space-y-1">
      <Label htmlFor={htmlFor}>{label}</Label>
      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}
    </div>
    <div className="w-full sm:max-w-64 sm:justify-self-end">
      {children}
    </div>
  </div>
)

interface RuntimeNumberRowProps {
  id: string
  label: string
  description: string
  unit?: string
  disabled?: boolean
  inputProps: UseFormRegisterReturn
}

export const RuntimeNumberRow = ({
  id,
  label,
  description,
  unit,
  disabled,
  inputProps,
}: RuntimeNumberRowProps) => (
  <RuntimeFieldRow label={label} description={description} htmlFor={id}>
    <InputGroup className="w-full">
      <InputGroupInput
        id={id}
        type="number"
        min={0}
        step={1}
        disabled={disabled}
        inputMode="numeric"
        {...inputProps}
      />
      {unit && (
        <InputGroupText className="text-xs text-muted-foreground pr-4">
          {unit}
        </InputGroupText>
      )}
    </InputGroup>
  </RuntimeFieldRow>
)

export const RuntimeSkeleton = () => (
  <div className="space-y-4">
    <div className="space-y-2">
      <Skeleton className="h-4 w-24" />
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, index) => (
          <div
            key={index}
            className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,280px)]"
          >
            <div className="space-y-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-9 w-full sm:max-w-64 sm:justify-self-end" />
          </div>
        ))}
      </div>
    </div>
  </div>
)
