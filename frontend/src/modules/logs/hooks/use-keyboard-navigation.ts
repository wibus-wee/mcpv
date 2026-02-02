// Input: keyboard events, navigation callbacks
// Output: useKeyboardNavigation hook for arrow key and escape handling
// Position: Logs module keyboard interaction hook

import { useEffect } from 'react'

export interface UseKeyboardNavigationOptions {
  onUp?: () => void
  onDown?: () => void
  onEscape?: () => void
  enabled?: boolean
}

export function useKeyboardNavigation({
  onUp,
  onDown,
  onEscape,
  enabled = true,
}: UseKeyboardNavigationOptions) {
  useEffect(() => {
    if (!enabled) return

    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      switch (e.key) {
        case 'ArrowUp':
          e.preventDefault()
          onUp?.()
          break
        case 'ArrowDown':
          e.preventDefault()
          onDown?.()
          break
        case 'Escape':
          onEscape?.()
          break
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [enabled, onUp, onDown, onEscape])
}
