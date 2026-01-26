// Input: Context, layout, and field components
// Output: SettingsCard compound component
// Position: Main entry point for SettingsCard compound component

import type { ReactNode } from 'react'
import type { FieldValues, UseFormReturn } from 'react-hook-form'

import { Card } from '@/components/ui/card'

import { SettingsCardProvider, useSettingsCardContext } from './context'
import {
  Field,
  FieldRow,
  NumberField,
  SelectField,
  SwitchField,
  TextareaField,
  TextField,
} from './fields'
import {
  Content,
  ErrorAlert,
  Footer,
  Header,
  ReadOnlyAlert,
  Section,
} from './layout'

interface SettingsCardRootProps<T extends FieldValues> {
  form: UseFormReturn<T>
  canEdit: boolean
  onSubmit: (event?: React.BaseSyntheticEvent) => void
  children: ReactNode
}

function SettingsCardRoot<T extends FieldValues>({
  form,
  canEdit,
  onSubmit,
  children,
}: SettingsCardRootProps<T>) {
  return (
    <SettingsCardProvider form={form} canEdit={canEdit}>
      <form className="flex w-full flex-col gap-0" onSubmit={onSubmit}>
        <Card className="p-1">{children}</Card>
      </form>
    </SettingsCardProvider>
  )
}

export const SettingsCard = Object.assign(SettingsCardRoot, {
  Header,
  Content,
  Section,
  Footer,
  Field,
  FieldRow,
  NumberField,
  SelectField,
  SwitchField,
  TextField,
  TextareaField,
  ReadOnlyAlert,
  ErrorAlert,
})

export { useSettingsCardContext }
