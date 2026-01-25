// Input: TanStack Router error props, UI components, Motion
// Output: RouterErrorComponent for global route error handling
// Position: Common component - used as defaultErrorComponent in router

import type { ErrorComponentProps } from '@tanstack/react-router'
import { useRouter } from '@tanstack/react-router'
import {
  ChevronDownIcon,
  ClipboardCopyIcon,
  HomeIcon,
  RefreshCwIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Kbd } from '@/components/ui/kbd'
import { Separator } from '@/components/ui/separator'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

function IsometricErrorBox({ className }: { className?: string }) {
  return (
    <svg
      className={cn('size-24', className)}
      fill="none"
      viewBox="0 0 120 120"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <linearGradient gradientUnits="userSpaceOnUse" id="topFace" x1="60" x2="60" y1="20" y2="50">
          <stop offset="0%" stopColor="hsl(var(--destructive))" stopOpacity="0.9" />
          <stop offset="100%" stopColor="hsl(var(--destructive))" stopOpacity="0.7" />
        </linearGradient>
        <linearGradient gradientUnits="userSpaceOnUse" id="leftFace" x1="25" x2="60" y1="70" y2="70">
          <stop offset="0%" stopColor="hsl(var(--destructive))" stopOpacity="0.5" />
          <stop offset="100%" stopColor="hsl(var(--destructive))" stopOpacity="0.3" />
        </linearGradient>
        <linearGradient gradientUnits="userSpaceOnUse" id="rightFace" x1="60" x2="95" y1="70" y2="70">
          <stop offset="0%" stopColor="hsl(var(--destructive))" stopOpacity="0.4" />
          <stop offset="100%" stopColor="hsl(var(--destructive))" stopOpacity="0.2" />
        </linearGradient>
      </defs>

      <m.ellipse
        animate={{ opacity: [0.15, 0.25, 0.15], scale: [1, 1.05, 1] }}
        cx="60"
        cy="100"
        fill="hsl(var(--foreground))"
        rx="35"
        ry="8"
        transition={{ duration: 3, repeat: Infinity, ease: 'easeInOut' }}
      />

      <m.g
        animate={{ y: [0, -3, 0] }}
        transition={{ duration: 2, repeat: Infinity, ease: 'easeInOut' }}
      >
        <polygon
          fill="url(#topFace)"
          points="60,20 95,40 60,60 25,40"
          stroke="hsl(var(--destructive))"
          strokeLinejoin="round"
          strokeWidth="1.5"
        />
        <polygon
          fill="url(#leftFace)"
          points="25,40 60,60 60,95 25,75"
          stroke="hsl(var(--destructive))"
          strokeLinejoin="round"
          strokeWidth="1.5"
        />
        <polygon
          fill="url(#rightFace)"
          points="95,40 60,60 60,95 95,75"
          stroke="hsl(var(--destructive))"
          strokeLinejoin="round"
          strokeWidth="1.5"
        />

        <m.g
          animate={{ opacity: [0.8, 1, 0.8] }}
          transition={{ duration: 1.5, repeat: Infinity, ease: 'easeInOut' }}
        >
          <text
            dominantBaseline="middle"
            fill="hsl(var(--destructive-foreground))"
            fontSize="24"
            fontWeight="bold"
            textAnchor="middle"
            x="60"
            y="45"
          >
            !
          </text>
        </m.g>

        <m.line
          animate={{ pathLength: [0, 1] }}
          stroke="hsl(var(--destructive-foreground))"
          strokeDasharray="4 2"
          strokeLinecap="round"
          strokeWidth="1"
          transition={{ duration: 0.5, delay: 0.3 }}
          x1="45"
          x2="75"
          y1="72"
          y2="72"
        />
        <m.line
          animate={{ pathLength: [0, 1] }}
          stroke="hsl(var(--destructive-foreground))"
          strokeDasharray="4 2"
          strokeLinecap="round"
          strokeWidth="1"
          transition={{ duration: 0.5, delay: 0.5 }}
          x1="48"
          x2="72"
          y1="80"
          y2="80"
        />
      </m.g>

      <m.circle
        animate={{ opacity: [0, 0.6, 0], scale: [0.8, 1.2, 0.8] }}
        cx="85"
        cy="25"
        fill="hsl(var(--destructive))"
        r="3"
        transition={{ duration: 2, repeat: Infinity, delay: 0.5 }}
      />
      <m.circle
        animate={{ opacity: [0, 0.4, 0], scale: [0.8, 1.1, 0.8] }}
        cx="30"
        cy="30"
        fill="hsl(var(--destructive))"
        r="2"
        transition={{ duration: 2, repeat: Infinity, delay: 1 }}
      />
      <m.circle
        animate={{ opacity: [0, 0.5, 0], scale: [0.8, 1.15, 0.8] }}
        cx="90"
        cy="55"
        fill="hsl(var(--destructive))"
        r="2.5"
        transition={{ duration: 2, repeat: Infinity, delay: 1.5 }}
      />
    </svg>
  )
}

export function RouterErrorComponent({ error, reset }: ErrorComponentProps) {
  const router = useRouter()
  const isDev = import.meta.env.DEV
  const [copied, setCopied] = useState(false)
  const [stackOpen, setStackOpen] = useState(false)

  const message = error instanceof Error ? error.message : 'An unexpected error occurred'
  const stack = error instanceof Error ? error.stack : undefined
  const errorName = error instanceof Error ? error.name : 'Error'

  const handleCopyError = useCallback(async () => {
    const errorText = `${errorName}: ${message}\n\n${stack || 'No stack trace available'}`
    await navigator.clipboard.writeText(errorText)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [errorName, message, stack])

  const handleGoHome = useCallback(() => {
    router.navigate({ to: '/' })
  }, [router])

  return (
    <m.div
      animate={{ opacity: 1, y: 0 }}
      className="flex size-full items-center justify-center p-6"
      initial={{ opacity: 0, y: 20 }}
      transition={Spring.smooth(0.5)}
    >
      <Empty className="max-w-md border border-border bg-card/50 backdrop-blur-sm">
        <EmptyHeader>
          <EmptyMedia className="mb-4" variant="default">
            <IsometricErrorBox />
          </EmptyMedia>

          <m.div
            animate={{ opacity: 1, scale: 1 }}
            className="mb-2"
            initial={{ opacity: 0, scale: 0.9 }}
            transition={Spring.snappy(0.3)}
          >
            <Badge size="sm" variant="error">
              {errorName}
            </Badge>
          </m.div>

          <EmptyTitle className="text-destructive-foreground">
            Something went wrong
          </EmptyTitle>
          <EmptyDescription className="mt-2">
            {message}
          </EmptyDescription>
        </EmptyHeader>

        <EmptyContent className="w-full gap-3">
          {isDev && stack && (
            <Collapsible onOpenChange={setStackOpen} open={stackOpen} className={"w-90"}>
              <CollapsibleTrigger
                className="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-muted-foreground text-xs hover:bg-muted/50"
              >
                <span>Stack Trace</span>
                <m.span
                  animate={{ rotate: stackOpen ? 180 : 0 }}
                  transition={Spring.snappy(0.2)}
                >
                  <ChevronDownIcon className="size-3.5" />
                </m.span>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <m.div
                  animate={{ opacity: 1 }}
                  initial={{ opacity: 0 }}
                  transition={Spring.smooth(0.2)}
                >
                  <pre className="p-3 font-mono text-xs leading-relaxed text-muted-foreground bg-muted/50 rounded-md mt-2 overflow-x-auto max-h-48 overflow-scroll">
                    {stack}
                  </pre>
                </m.div>
              </CollapsibleContent>
            </Collapsible>
          )}

          <Separator className="my-1" />

          <div className="flex w-full flex-col gap-2">
            <div className="flex gap-2">
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      className="flex-1"
                      onClick={reset}
                      size="sm"
                      variant="default"
                    >
                      <RefreshCwIcon />
                      Try again
                    </Button>
                  }
                />
                <TooltipContent>Retry the failed operation</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      onClick={handleGoHome}
                      size="sm"
                      variant="outline"
                    >
                      <HomeIcon />
                      Home
                    </Button>
                  }
                />
                <TooltipContent>Return to dashboard</TooltipContent>
              </Tooltip>

              {isDev && (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        onClick={handleCopyError}
                        size="icon-sm"
                        variant="ghost"
                      >
                        <ClipboardCopyIcon />
                      </Button>
                    }
                  />
                  <TooltipContent>{copied ? 'Copied!' : 'Copy error details'}</TooltipContent>
                </Tooltip>
              )}
            </div>

            <m.p
              animate={{ opacity: 1 }}
              className="flex items-center justify-center gap-1.5 text-center text-muted-foreground text-xs"
              initial={{ opacity: 0 }}
              transition={{ delay: 0.3 }}
            >
              Press
              <Kbd>⌘</Kbd>
              <Kbd>R</Kbd>
              to refresh the page
            </m.p>
          </div>
        </EmptyContent>
      </Empty>
    </m.div>
  )
}
