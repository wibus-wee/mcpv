<!-- Once this directory changes, update this README.md -->

# SettingsCard Compound Component

SettingsCard 采用复合组件模式构建设置表单。
使用 React Context 避免在字段间传递 form、canEdit 与 isSaving。
支持字段级帮助图标与统一的布局约束。

## Files

- **index.tsx**: 入口导出 SettingsCard 复合组件
- **context.tsx**: 表单状态上下文与 Provider
- **fields.tsx**: 字段组件（Number, Select, Switch, Text, Textarea）
- **layout.tsx**: 布局组件（Header, Content, Section, Footer, ReadOnlyAlert, ErrorAlert）

## Usage

```tsx
import { SettingsCard } from './settings-card'

<SettingsCard form={form} canEdit={canEdit} onSubmit={handleSave}>
  <SettingsCard.Header title="Runtime" description="Configure runtime settings" />
  <SettingsCard.Content>
    <SettingsCard.ReadOnlyAlert />
    <SettingsCard.ErrorAlert error={error} />
    <SettingsCard.Section title="Core">
      <SettingsCard.SelectField
        name="bootstrapMode"
        label="Bootstrap Mode"
        options={BOOTSTRAP_MODE_OPTIONS}
      />
      <SettingsCard.NumberField
        name="routeTimeoutSeconds"
        label="Route Timeout"
        unit="seconds"
      />
    </SettingsCard.Section>
  </SettingsCard.Content>
  <SettingsCard.Footer statusLabel={statusLabel} />
</SettingsCard>
```

## Custom Fields

如需自定义渲染，可使用 `SettingsCard.Field` 或直接消费 context：

```tsx
import { useSettingsCardContext } from './settings-card'

function CustomModelField() {
  const { form, canEdit, isSaving } = useSettingsCardContext()
  // Custom implementation
}
```
