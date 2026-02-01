// Input: React state/effects, DOM mouse events, optional localStorage
// Output: useResizable hook with size state and resize handle props
// Position: Shared hook for resizable panels across the app

import { useCallback, useEffect, useRef, useState } from 'react'

type ResizeDirection = 'horizontal' | 'vertical'
type ResizeHandle = 'left' | 'right' | 'top' | 'bottom'

interface UseResizableConfig {
  defaultSize: number
  minSize?: number
  maxSize?: number
  storageKey?: string
  direction?: ResizeDirection
  handle?: ResizeHandle
}

interface UseResizableReturn {
  size: number
  setSize: (size: number) => void
  resizeHandleProps: {
    onMouseDown: (e: React.MouseEvent) => void
    onDoubleClick: (e: React.MouseEvent) => void
    className: string
  }
  isDragging: boolean
}

export function useResizable({
  defaultSize,
  minSize = 100,
  maxSize = Infinity,
  storageKey,
  direction = 'horizontal',
  handle = 'right',
}: UseResizableConfig): UseResizableReturn {
  // Initialize size from localStorage if available
  const [size, setSizeState] = useState(() => {
    if (typeof window === 'undefined') return defaultSize

    if (storageKey) {
      const stored = localStorage.getItem(storageKey)
      if (stored) {
        const parsedSize = Number.parseInt(stored, 10)
        if (!Number.isNaN(parsedSize)) {
          return Math.min(Math.max(parsedSize, minSize), maxSize)
        }
      }
    }
    return defaultSize
  })

  const [isDragging, setIsDragging] = useState(false)
  const [startPosition, setStartPosition] = useState(0)
  const [startSize, setStartSize] = useState(0)
  const dragStateRef = useRef({ isDragging: false })

  const setSize = useCallback(
    (newSize: number) => {
      const constrainedSize = Math.min(Math.max(newSize, minSize), maxSize)
      setSizeState(constrainedSize)

      // Persist to localStorage if storageKey is provided
      if (storageKey) {
        localStorage.setItem(storageKey, constrainedSize.toString())
      }
    },
    [minSize, maxSize, storageKey],
  )

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      e.stopPropagation()

      setIsDragging(true)
      dragStateRef.current.isDragging = true

      if (direction === 'horizontal') {
        setStartPosition(e.clientX)
      }
      else {
        setStartPosition(e.clientY)
      }

      setStartSize(size)
    },
    [direction, size],
  )

  const handleDoubleClick = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      e.stopPropagation()

      // Reset to default size
      setSize(defaultSize)
    },
    [defaultSize, setSize],
  )

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!dragStateRef.current.isDragging) return

      const currentPosition
        = direction === 'horizontal' ? e.clientX : e.clientY
      const diff = currentPosition - startPosition

      // Calculate new size based on handle position
      let newSize: number
      if (handle === 'right' || handle === 'bottom') {
        newSize = startSize + diff
      }
      else {
        newSize = startSize - diff
      }

      setSize(newSize)
    },
    [direction, startPosition, startSize, handle, setSize],
  )

  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
    dragStateRef.current.isDragging = false
  }, [])

  useEffect(() => {
    if (isDragging) {
      document.addEventListener('mousemove', handleMouseMove)
      document.addEventListener('mouseup', handleMouseUp)

      // Set cursor and disable text selection during drag
      const cursor = direction === 'horizontal' ? 'col-resize' : 'row-resize'
      document.body.style.cursor = cursor
      document.body.style.userSelect = 'none'

      return () => {
        document.removeEventListener('mousemove', handleMouseMove)
        document.removeEventListener('mouseup', handleMouseUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
      }
    }
  }, [isDragging, handleMouseMove, handleMouseUp, direction])

  // Generate className for resize handle based on direction and handle position
  const getHandleClassName = useCallback(() => {
    const baseClasses = direction === 'horizontal'
      ? 'absolute cursor-col-resize group transition-colors'
      : 'absolute cursor-row-resize group transition-colors'
    const hoverClasses = 'hover:bg-border'
    const activeClasses = isDragging ? 'bg-border' : ''

    let positionClasses = ''
    let sizeClasses = ''

    if (direction === 'horizontal') {
      sizeClasses = 'w-1 h-full'
      if (handle === 'right') {
        positionClasses = 'top-0 right-0'
      }
      else {
        positionClasses = 'top-0 left-0'
      }
    }
    else {
      sizeClasses = 'h-1 w-full'
      if (handle === 'bottom') {
        positionClasses = 'bottom-0 left-0'
      }
      else {
        positionClasses = 'top-0 left-0'
      }
    }

    return `${baseClasses} ${hoverClasses} ${activeClasses} ${positionClasses} ${sizeClasses}`.trim()
  }, [direction, handle, isDragging])

  const resizeHandleProps = {
    onMouseDown: handleMouseDown,
    onDoubleClick: handleDoubleClick,
    className: getHandleClassName(),
  }

  return {
    size,
    setSize,
    resizeHandleProps,
    isDragging,
  }
}
