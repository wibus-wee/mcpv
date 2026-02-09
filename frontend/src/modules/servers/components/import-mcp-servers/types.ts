import type { ImportServerSpec, McpTransferPreview } from '@bindings/mcpv/internal/ui/types'

type TransferStatus = 'idle' | 'loading' | 'ready' | 'missing' | 'error' | 'empty'

export type TransferSource = 'claude' | 'codex' | 'gemini'

export type TransferState = {
  status: TransferStatus
  preview?: McpTransferPreview
  error?: string
}

export type MergedServer = ImportServerSpec & {
  source: TransferSource
  selected: boolean
}

export type TransferSummary = {
  imported: number
  skipped: number
}
