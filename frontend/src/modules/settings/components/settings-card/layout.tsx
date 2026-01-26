// Input: SettingsCard context, UI components
// Output: Layout components (Header, Section, Footer, ReadOnlyAlert, ErrorAlert)
// Position: Layout components for SettingsCard

import { AlertCircleIcon, SaveIcon, ShieldAlertIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'

import { useSettingsCardContext } from './context'

interface HeaderProps {
  title: string
  description?: string
  badge?: string
}

export const Header = ({ title, description, badge }: HeaderProps) => {
  const { canEdit } = useSettingsCardContext()

  return (
    <CardHeader className="pt-3">
      <CardTitle className="flex items-center gap-2">
        {title}
        {badge && (
          <Badge variant="secondary" size="sm">
            {badge}
          </Badge>
        )}
        {!canEdit && (
          <Badge variant="warning" size="sm">
            Read-only
          </Badge>
        )}
      </CardTitle>
      {description && <CardDescription>{description}</CardDescription>}
    </CardHeader>
  )
}

interface SectionProps {
  title: string
  children: ReactNode
}

export const Section = ({ title, children }: SectionProps) => (
  <div className="space-y-3">
    <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      {title}
    </div>
    <div className="divide-y divide-border">{children}</div>
  </div>
)

interface ContentProps {
  children: ReactNode
}

export const Content = ({ children }: ContentProps) => (
  <CardContent className="space-y-6">{children}</CardContent>
)

interface FooterProps {
  statusLabel: string
  saveDisabledReason?: string
  isDirty?: boolean
  customDisabled?: boolean
}

export const Footer = ({
  statusLabel,
  saveDisabledReason,
  isDirty,
  customDisabled,
}: FooterProps) => {
  const { form, canEdit, isSaving } = useSettingsCardContext()
  const formIsDirty = isDirty ?? form.formState.isDirty
  const isDisabled = customDisabled ?? (!canEdit || !formIsDirty || isSaving)

  return (
    <CardFooter className="border-t">
      <div className="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-xs text-muted-foreground">{statusLabel}</div>
        <Button
          type="submit"
          size="sm"
          disabled={isDisabled}
          title={saveDisabledReason}
        >
          {isSaving ? (
            <Spinner className="size-4" />
          ) : (
            <SaveIcon className="size-4" />
          )}
          {isSaving ? 'Saving...' : 'Save changes'}
        </Button>
      </div>
    </CardFooter>
  )
}

export const ReadOnlyAlert = () => {
  const { canEdit } = useSettingsCardContext()

  if (canEdit) return null

  return (
    <Alert variant="warning">
      <ShieldAlertIcon />
      <AlertTitle>Configuration is read-only</AlertTitle>
      <AlertDescription>
        Update permissions to enable edits.
      </AlertDescription>
    </Alert>
  )
}

interface ErrorAlertProps {
  error: unknown
  title?: string
  fallbackMessage?: string
}

export const ErrorAlert = ({
  error,
  title = 'Failed to load settings',
  fallbackMessage = 'Unable to load configuration.',
}: ErrorAlertProps) => {
  if (!error) return null

  return (
    <Alert variant="error">
      <AlertCircleIcon />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        {error instanceof Error ? error.message : fallbackMessage}
      </AlertDescription>
    </Alert>
  )
}
