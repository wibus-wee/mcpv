// Input: Tooltip, Popover, Button UI components, lucide icons
// Output: FieldHelp component and FieldHelpContent type
// Position: Shared help affordance for form labels

import { InfoIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

export type FieldHelpContent = {
  id: string
  title: string
  summary: string
  details?: string
  tips?: string[]
  variant?: 'tooltip' | 'popover'
}

interface FieldHelpProps {
  content: FieldHelpContent
  className?: string
}

export function FieldHelp({ content, className }: FieldHelpProps) {
  const hasPopoverContent = Boolean(content.details) || Boolean(content.tips?.length)
  const variant = content.variant ?? (hasPopoverContent ? 'popover' : 'tooltip')
  const trigger = (
    <Button
      aria-label={content.title}
      className={cn('h-5 w-5 p-0 text-muted-foreground hover:text-foreground', className)}
      size="icon-xs"
      variant="ghost"
    >
      <InfoIcon className="size-3.5" />
    </Button>
  )

  if (variant === 'popover') {
    return (
      <Popover>
        <PopoverTrigger render={trigger} />
        <PopoverContent className="w-72" align="start" side="right">
          <div className="space-y-2">
            <PopoverTitle className="text-sm">{content.title}</PopoverTitle>
            <PopoverDescription className="text-xs">
              {content.summary}
            </PopoverDescription>
            {content.details ? (
              <p className="text-xs text-muted-foreground">{content.details}</p>
            ) : null}
            {content.tips && content.tips.length > 0 ? (
              <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
                {content.tips.map(tip => (
                  <li key={tip}>{tip}</li>
                ))}
              </ul>
            ) : null}
          </div>
        </PopoverContent>
      </Popover>
    )
  }

  return (
    <Tooltip>
      <TooltipTrigger render={trigger} />
      <TooltipContent side="right" className="max-w-64">
        {content.summary}
      </TooltipContent>
    </Tooltip>
  )
}
