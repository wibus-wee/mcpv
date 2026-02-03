// Input: Label, FieldDescription/FieldError, FieldHelp component
// Output: ServerFormField component for consistent field layout
// Position: Shared field layout in servers module forms

import type { ReactNode } from 'react'

import type { FieldHelpContent } from '@/components/common/field-help'
import { FieldHelp } from '@/components/common/field-help'
import { FieldDescription, FieldError } from '@/components/ui/field'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

interface ServerFormFieldProps {
  id: string
  label: string
  description?: string
  required?: boolean
  help?: FieldHelpContent
  error?: string
  children: ReactNode
  className?: string
}

export function ServerFormField({
  id,
  label,
  description,
  required,
  help,
  error,
  children,
  className,
}: ServerFormFieldProps) {
  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex items-center gap-2">
        <Label htmlFor={id}>
          {label}
          {required ? (
            <span className="ml-1 text-destructive" aria-hidden="true">
              *
            </span>
          ) : null}
        </Label>
        {help ? <FieldHelp content={help} /> : null}
      </div>
      {description ? (
        <FieldDescription>
          {description}
        </FieldDescription>
      ) : null}
      {children}
      {error ? (
        <FieldError>
          {error}
        </FieldError>
      ) : null}
    </div>
  )
}
