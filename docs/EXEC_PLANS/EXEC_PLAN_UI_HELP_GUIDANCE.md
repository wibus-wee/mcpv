# UI Help Tooltips and Advanced Guidance

This ExecPlan is a living document. The sections "Progress", "Surprises & Discoveries", "Decision Log", and "Outcomes & Retrospective" must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` in the repository root.

## Purpose / Big Picture

完成后，用户在配置 MCP Server 与 Runtime/SubAgent 设置时可以获得字段级解释、上下文提示和最佳实践建议，并通过 "Advanced" 折叠区收敛复杂选项。新增的帮助系统统一在 UI 内实现，不引入新的路由或强制向导步骤，默认视图即为 "Guided" 的基础路径，Advanced 则承载低频字段。可通过打开 Add Server Sheet、Runtime Settings、SubAgent Settings 来观察帮助图标、提示内容和建议信息。

## Progress

- [x] (2026-02-03 09:10Z) Draft ExecPlan for UI help, advanced view, and advice system.
- [x] (2026-02-03 09:25Z) Implemented help content registry and FieldHelp component.
- [x] (2026-02-03 09:40Z) Refactored ServerEditSheet with Advanced sections and advice banners.
- [x] (2026-02-03 09:55Z) Extended SettingsCard fields to support help icons and applied Runtime/SubAgent help.
- [x] (2026-02-03 10:05Z) Updated docs/EXEC_PLANS/README.md inventory.

## Surprises & Discoveries

- Observation: `ServerEditSheet` uses a local `FormField` with no standard place to attach help affordances or validation messages.
  Evidence: `frontend/src/modules/servers/components/server-edit-sheet.tsx`.
- Observation: Settings pages already use `SettingsCard` with Core/Advanced layout, but fields do not support help icons.
  Evidence: `frontend/src/modules/settings/components/settings-card/fields.tsx`.

## Decision Log

- Decision: 不新增 Guided/Advanced 双视图切换，仅保留 Advanced 折叠区；默认视图即为“Guided”主路径。
  Rationale: 降低 UI 复杂度，满足“Advanced 视图即可”的要求。
  Date/Author: 2026-02-03 / Codex.
- Decision: 所有新文案集中在模块级常量文件，禁止在组件内散落硬编码字符串。
  Rationale: 避免 magic strings，提升可维护性和后续国际化可行性。
  Date/Author: 2026-02-03 / Codex.
- Decision: 新增 `FieldHelp` 放在 `components/common`，并在 SettingsCard 与 ServerEditSheet 共享。
  Rationale: App 级复用但不是通用 UI primitive，符合组件摆放要求。
  Date/Author: 2026-02-03 / Codex.
- Decision: 将 Server 表单的文案、帮助与建议规则集中到 `server-form-content.ts`。
  Rationale: 统一管理文案与规则，避免 magic strings 分散在组件内。
  Date/Author: 2026-02-03 / Codex.

## Outcomes & Retrospective

- Delivered FieldHelp component with centralized help content for Servers/Runtime/SubAgent.
- Added Advanced sections and advice banners in ServerEditSheet.
- Updated SettingsCard fields to render help icons and refreshed directory READMEs.

## Context and Orientation

前端位于 `frontend/`，服务器配置表单在 `frontend/src/modules/servers/components/server-edit-sheet.tsx`，运行时配置在 `frontend/src/modules/settings/components/runtime-settings-card.tsx`，SubAgent 配置在 `frontend/src/modules/settings/components/subagent-settings-card.tsx`。Tooltip/Popover 基础组件在 `frontend/src/components/ui/tooltip.tsx` 和 `frontend/src/components/ui/popover.tsx`。Advanced 折叠可使用 `frontend/src/components/ui/collapsible.tsx`。

“Tooltip”指鼠标悬停即可出现的短文本解释；“Popover”指点击后出现的可承载更长内容的解释面板；“Advanced”折叠区指默认收起的低频字段区域。

## Plan of Work

首先，建立帮助文案的集中式注册表，避免在组件内新增零散字符串。建议在 `frontend/src/modules/servers/lib/server-form-content.ts` 和 `frontend/src/modules/settings/lib/runtime-help.ts` 分别定义字段帮助与建议规则。每条帮助包含短说明与可选扩展说明，Advanced 建议与警告使用结构化常量列表。

其次，在 `frontend/src/components/common/field-help.tsx` 新建 `FieldHelp` 组件，统一渲染帮助图标，支持 Tooltip/Popover 两种展示。`FieldHelp` 仅接收结构化内容，禁止接收自由字符串。该组件用在 Server 与 Settings 的字段标签上。

第三，重构 `ServerEditSheet`：保留现有主字段顺序，把低频字段归入 Advanced 折叠区（例如 HTTP headers、drain timeout、session TTL 等）。Advanced 标题与说明从常量文件读取。删除或替换现有局部 `FormField` 结构，必要时抽离为 `ServerFormField` 组件并放置在 `modules/servers/components/`，以便统一标签/说明/校验/帮助图标渲染。

第四，在 ServerEditSheet 中加入规则化建议与校验提示：基于表单值计算建议（例如 activationMode=always-on 且 minReady=0 的提醒，idleSeconds=0 的常驻提示等），提示内容来自常量列表。校验错误应显示在字段下方。计算建议时使用 `useMemo` 与 `useWatch`，避免不必要的重渲染，遵循 Vercel React Best Practices 的 re-render 规则。

第五，扩展 SettingsCard 的字段渲染以支持可选帮助图标。更新 `frontend/src/modules/settings/components/settings-card/fields.tsx` 的 `FieldRow`/`Field` 接口，增加 `help` 参数，并在 Runtime/SubAgent 页面注入帮助内容。帮助内容从 `runtime-help.ts` 与 `subagent-help.ts` 等常量文件读取。

最后，更新 `docs/EXEC_PLANS/README.md`，确保目录有完整文件清单，并加入本 ExecPlan。

## Concrete Steps

在仓库根目录运行以下命令确认位置与依赖：

  rg -n "server-edit-sheet" frontend/src/modules/servers/components/server-edit-sheet.tsx
  rg -n "SettingsCard" frontend/src/modules/settings/components/settings-card/fields.tsx
  rg -n "Tooltip" frontend/src/components/ui/tooltip.tsx
  rg -n "Popover" frontend/src/components/ui/popover.tsx
  rg -n "Collapsible" frontend/src/components/ui/collapsible.tsx

然后按顺序修改：

  1. 新增 `frontend/src/components/common/field-help.tsx`，提供 `FieldHelp` 与结构化类型定义。
  2. 新增 `frontend/src/modules/servers/lib/server-form-content.ts`，集中定义 Server 字段帮助与建议规则。
  3. 新增 `frontend/src/modules/settings/lib/runtime-help.ts` 与 `frontend/src/modules/settings/lib/subagent-help.ts`，集中定义 Settings 帮助内容。
  4. 重构 `frontend/src/modules/servers/components/server-edit-sheet.tsx`，引入 Advanced 折叠区、帮助图标、校验错误与建议提示。
  5. 修改 `frontend/src/modules/settings/components/settings-card/fields.tsx` 支持 help 参数，并在 Runtime/SubAgent 卡片里注入帮助内容。
  6. 更新 `docs/EXEC_PLANS/README.md`。

## Validation and Acceptance

手动验证即可：

1. 打开 Add Server Sheet，确认每个关键字段旁出现帮助图标，点击或悬停展示说明。
2. Advanced 折叠区默认收起，展开后显示低频字段与对应帮助说明。
3. 修改表单值时，相关建议提示出现且文案来自常量映射，不在组件内硬编码。
4. Runtime 与 SubAgent 设置字段显示帮助图标，行为与 Server 一致。
5. 未引入动态 Tailwind 类名，Motion 组件使用 `m.` 前缀。

可选运行命令（不要求必须执行）：

  cd frontend
  pnpm lint
  pnpm test

## Idempotence and Recovery

所有改动为 UI 层与常量新增，可重复执行。若需回退，删除新增文件并恢复修改的组件文件即可，不影响后端配置或数据结构。

## Artifacts and Notes

避免散落硬编码字符串。新文案必须来自 `server-form-content.ts`、`runtime-help.ts`、`subagent-help.ts` 或类似集中式常量文件。必要时为文案键建立枚举或常量 ID，方便统一管理与后续国际化。

## Interfaces and Dependencies

在 `frontend/src/components/common/field-help.tsx` 中定义：

  export type FieldHelpContent = {
    id: string
    title: string
    summary: string
    details?: string
    tips?: string[]
    variant?: 'tooltip' | 'popover'
  }

  export type FieldHelpProps = {
    content: FieldHelpContent
    className?: string
  }

在 `frontend/src/modules/servers/lib/server-form-content.ts` 中导出：

  export const SERVER_FIELD_IDS: Record<string, string>
  export const SERVER_FORM_TEXT: Record<string, unknown>
  export const SERVER_FORM_VALIDATION: Record<string, string>
  export const SERVER_FIELD_HELP: Record<string, FieldHelpContent>
  export const SERVER_ADVICE_RULES: Array<{
    id: string
    when: (values: ServerFormValues) => boolean
    severity: 'info' | 'warning'
    message: string
  }>

在 `frontend/src/modules/settings/lib/runtime-help.ts` 与 `subagent-help.ts` 中导出：

  export const RUNTIME_FIELD_HELP: Record<string, FieldHelpContent>
  export const SUBAGENT_FIELD_HELP: Record<string, FieldHelpContent>

`SettingsCard` 字段扩展接口：

  interface FieldRowProps {
    label: string
    description?: string
    htmlFor: string
    children: ReactNode
    help?: FieldHelpContent
  }

Plan update note: 2026-02-03 Updated file references to server-form-content.ts and recorded completed progress items after implementation work.
