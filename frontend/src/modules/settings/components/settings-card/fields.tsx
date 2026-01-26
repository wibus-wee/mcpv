// Input: SettingsCard context, form field props
// Output: Compound field components (Field, Number, Select, Switch, Text, Textarea)
// Position: Field components for SettingsCard

import type { ReactNode } from 'react'
import type { FieldPath, FieldValues } from 'react-hook-form'
import { Controller } from 'react-hook-form'

import { Badge } from '@/components/ui/badge'
import { InputGroup, InputGroupInput, InputGroupText, InputGroupTextarea } from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import { useSettingsCardContext } from './context'

interface FieldRowProps {
  label: string
  description?: string
  htmlFor: string
  children: ReactNode
  className?: string
}

export const FieldRow = ({
  label,
  description,
  htmlFor,
  children,
  className,
}: FieldRowProps) => (
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

interface FieldProps {
  label: string
  description?: string
  htmlFor: string
  children: ReactNode
  className?: string
}

export const Field = ({ label, description, htmlFor, children, className }: FieldProps) => (
  <FieldRow label={label} description={description} htmlFor={htmlFor} className={className}>
    {children}
  </FieldRow>
)

interface NumberFieldProps<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  description?: string
  unit?: string
  min?: number
  step?: number
}

export function NumberField<T extends FieldValues>({
  name,
  label,
  description,
  unit,
  min = 0,
  step = 1,
}: NumberFieldProps<T>) {
  const { form, canEdit, isSaving } = useSettingsCardContext<T>()
  const id = `settings-${name}`

  return (
    <FieldRow label={label} description={description} htmlFor={id}>
      <InputGroup className="w-full">
        <InputGroupInput
          id={id}
          type="number"
          min={min}
          step={step}
          disabled={!canEdit || isSaving}
          inputMode="numeric"
          {...form.register(name, {
            valueAsNumber: true,
            setValueAs: (v: unknown) => {
              if (v === '' || v === null || v === undefined) return 0
              const n = Number(v)
              return Number.isNaN(n) ? 0 : n
            },
          })}
        />
        {unit && (
          <InputGroupText className="pr-4 text-xs text-muted-foreground">
            {unit}
          </InputGroupText>
        )}
      </InputGroup>
    </FieldRow>
  )
}

interface SelectFieldProps<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  description?: string
  options: readonly { value: string, label: string }[]
  labels?: Record<string, string>
}

export function SelectField<T extends FieldValues>({
  name,
  label,
  description,
  options,
  labels,
}: SelectFieldProps<T>) {
  const { form, canEdit, isSaving } = useSettingsCardContext<T>()
  const id = `settings-${name}`

  return (
    <FieldRow label={label} description={description} htmlFor={id}>
      <Controller
        control={form.control}
        name={name}
        render={({ field }) => (
          <Select
            value={field.value as string}
            onValueChange={field.onChange}
            disabled={!canEdit || isSaving}
          >
            <SelectTrigger id={id}>
              <SelectValue>
                {(value) => {
                  if (!value) return 'Select...'
                  return labels?.[String(value)] ?? String(value)
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {options.map(option => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
    </FieldRow>
  )
}

interface SwitchFieldProps<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  description?: string
  enabledLabel?: string
  disabledLabel?: string
}

export function SwitchField<T extends FieldValues>({
  name,
  label,
  description,
  enabledLabel = 'Enabled',
  disabledLabel = 'Disabled',
}: SwitchFieldProps<T>) {
  const { form, canEdit, isSaving } = useSettingsCardContext<T>()
  const id = `settings-${name}`

  return (
    <FieldRow label={label} description={description} htmlFor={id}>
      <Controller
        control={form.control}
        name={name}
        render={({ field }) => (
          <div className="flex items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
            <Switch
              id={id}
              checked={field.value as boolean}
              onCheckedChange={checked => field.onChange(checked === true)}
              disabled={!canEdit || isSaving}
            />
            <Badge
              variant={field.value ? 'success' : 'secondary'}
              size="sm"
            >
              {field.value ? enabledLabel : disabledLabel}
            </Badge>
          </div>
        )}
      />
    </FieldRow>
  )
}

interface TextFieldProps<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  description?: string
  placeholder?: string
  type?: 'text' | 'password'
  autoComplete?: string
}

export function TextField<T extends FieldValues>({
  name,
  label,
  description,
  placeholder,
  type = 'text',
  autoComplete,
}: TextFieldProps<T>) {
  const { form, canEdit, isSaving } = useSettingsCardContext<T>()
  const id = `settings-${name}`

  return (
    <FieldRow label={label} description={description} htmlFor={id}>
      <InputGroup className="w-full">
        <InputGroupInput
          id={id}
          type={type}
          placeholder={placeholder}
          autoComplete={autoComplete}
          disabled={!canEdit || isSaving}
          {...form.register(name)}
        />
      </InputGroup>
    </FieldRow>
  )
}

interface TextareaFieldProps<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  description?: string
  placeholder?: string
  rows?: number
}

export function TextareaField<T extends FieldValues>({
  name,
  label,
  description,
  placeholder,
  rows = 4,
}: TextareaFieldProps<T>) {
  const { form, canEdit, isSaving } = useSettingsCardContext<T>()
  const id = `settings-${name}`

  return (
    <FieldRow
      label={label}
      description={description}
      htmlFor={id}
      className="sm:grid-cols-[minmax(0,1fr)_minmax(0,360px)]"
    >
      <InputGroup className="w-full" data-align="block-start">
        <InputGroupTextarea
          id={id}
          placeholder={placeholder}
          rows={rows}
          disabled={!canEdit || isSaving}
          {...form.register(name)}
        />
      </InputGroup>
    </FieldRow>
  )
}
