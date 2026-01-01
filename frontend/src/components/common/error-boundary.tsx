// Input: React error boundary API, Alert UI components
// Output: ErrorBoundary component for catching and displaying React errors
// Position: Common component - wraps route/page components for error isolation

'use client'

import { AlertCircleIcon, RefreshCwIcon } from 'lucide-react'
import { Component, type ErrorInfo, type ReactNode } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

/**
 * Props for ErrorBoundary component
 */
interface ErrorBoundaryProps {
  children: ReactNode
  /** Custom fallback UI when error occurs */
  fallback?: ReactNode
  /** Callback when error is caught */
  onError?: (error: Error, errorInfo: ErrorInfo) => void
  /** Whether to show retry button */
  showRetry?: boolean
  /** Custom error title */
  errorTitle?: string
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

/**
 * React Error Boundary component for catching and handling render errors.
 * Provides a graceful fallback UI and optional retry functionality.
 *
 * @example
 * // Basic usage - wrap around components that might throw
 * <ErrorBoundary>
 *   <SomeComponent />
 * </ErrorBoundary>
 *
 * @example
 * // With custom error handling
 * <ErrorBoundary
 *   onError={(error) => logToService(error)}
 *   errorTitle="Dashboard Error"
 * >
 *   <Dashboard />
 * </ErrorBoundary>
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    // Log error to console in development
    if (import.meta.env.DEV) {
      console.error('[ErrorBoundary] Caught error:', error)
      console.error('[ErrorBoundary] Component stack:', errorInfo.componentStack)
    }

    // Call custom error handler if provided
    this.props.onError?.(error, errorInfo)
  }

  handleRetry = (): void => {
    this.setState({ hasError: false, error: null })
  }

  render(): ReactNode {
    const { hasError, error } = this.state
    const { children, fallback, showRetry = true, errorTitle = 'Something went wrong' } = this.props

    if (hasError) {
      // Use custom fallback if provided
      if (fallback) {
        return fallback
      }

      // Default error UI
      return (
        <div className="flex min-h-[200px] items-center justify-center p-6">
          <Alert variant="error" className="max-w-md">
            <AlertCircleIcon className="size-4" />
            <AlertTitle>{errorTitle}</AlertTitle>
            <AlertDescription className="mt-2 space-y-3">
              <p className="text-sm">
                {error?.message || 'An unexpected error occurred. Please try again.'}
              </p>
              {import.meta.env.DEV && error?.stack && (
                <pre className="mt-2 max-h-32 overflow-auto rounded bg-muted p-2 text-xs">
                  {error.stack}
                </pre>
              )}
              {showRetry && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={this.handleRetry}
                  className="mt-2"
                >
                  <RefreshCwIcon className="mr-2 size-3" />
                  Try again
                </Button>
              )}
            </AlertDescription>
          </Alert>
        </div>
      )
    }

    return children
  }
}

/**
 * Props for DataErrorFallback component
 */
interface DataErrorFallbackProps {
  /** Error object or message */
  error: Error | string | null | undefined
  /** Callback to retry the failed operation */
  onRetry?: () => void
  /** Custom title for the error */
  title?: string
  /** Whether to show the retry button */
  showRetry?: boolean
  /** Additional CSS class */
  className?: string
}

/**
 * Fallback component for data loading errors.
 * Use this for SWR/async data errors, not for React render errors.
 *
 * @example
 * // In a component with SWR
 * const { data, error, mutate } = useSWR('key', fetcher)
 *
 * if (error) {
 *   return <DataErrorFallback error={error} onRetry={mutate} />
 * }
 */
export function DataErrorFallback({
  error,
  onRetry,
  title = 'Failed to load data',
  showRetry = true,
  className,
}: DataErrorFallbackProps) {
  const errorMessage = error instanceof Error
    ? error.message
    : typeof error === 'string'
      ? error
      : 'An unexpected error occurred'

  return (
    <Alert variant="error" className={className}>
      <AlertCircleIcon className="size-4" />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription className="mt-1 space-y-2">
        <p className="text-sm">{errorMessage}</p>
        {showRetry && onRetry && (
          <Button variant="outline" size="sm" onClick={() => onRetry()}>
            <RefreshCwIcon className="mr-2 size-3" />
            Retry
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}

/**
 * Props for withErrorBoundary HOC
 */
interface WithErrorBoundaryOptions {
  fallback?: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
  errorTitle?: string
}

/**
 * Higher-order component to wrap a component with ErrorBoundary.
 *
 * @example
 * const SafeDashboard = withErrorBoundary(Dashboard, {
 *   errorTitle: 'Dashboard Error',
 * })
 */
export function withErrorBoundary<P extends object>(
  WrappedComponent: React.ComponentType<P>,
  options: WithErrorBoundaryOptions = {},
) {
  const displayName = WrappedComponent.displayName || WrappedComponent.name || 'Component'

  const ComponentWithErrorBoundary = (props: P) => (
    <ErrorBoundary {...options}>
      <WrappedComponent {...props} />
    </ErrorBoundary>
  )

  ComponentWithErrorBoundary.displayName = `withErrorBoundary(${displayName})`

  return ComponentWithErrorBoundary
}
