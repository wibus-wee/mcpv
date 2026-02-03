// Input: Alert UI components, lucide icons
// Output: ExperimentalBanner component for marking beta features
// Position: Reusable banner for experimental/beta features

import { InfoIcon } from 'lucide-react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

interface ExperimentalBannerProps {
  feature?: string
  description?: string
  inspirationName?: string
  inspirationUrl?: string
}

export function ExperimentalBanner({
  feature = 'Feature',
  description = 'This feature is currently under active development and the implementation may change.',
  inspirationName,
  inspirationUrl,
}: ExperimentalBannerProps) {
  return (
    <Alert variant="info" className="border-info/50 bg-linear-to-br from-info/8 to-info/4">
      <InfoIcon />
      <div className="flex flex-col gap-1.5">
        <AlertTitle className="flex items-center gap-2">
          Experimental
          {' '}
          {feature}
          <span className="rounded-full bg-info/20 px-2 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wide">
            Beta
          </span>
        </AlertTitle>
        <AlertDescription>
          <p>{description}</p>
          {inspirationName && inspirationUrl ? (
            <p className="text-xs">
              Inspired by
              {' '}
              <a
                className="font-medium underline decoration-info/40 underline-offset-2 hover:text-foreground hover:decoration-info"
                href={inspirationUrl}
                rel="noopener noreferrer"
                target="_blank"
              >
                {inspirationName}
              </a>
              .
            </p>
          ) : null}
        </AlertDescription>
      </div>
    </Alert>
  )
}
