import * as React from 'react'
import type { RefObject } from 'react'
import { Clock } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { TimeValue } from '@/utils/date-range'
import { TimeDropdown } from './time-dropdown'
import { parseTimeString, timeToString } from './utils'

interface TimeFieldProps {
  label: string
  value: TimeValue
  active: boolean
  open: boolean
  onToggle: () => void
  onChange: (next: TimeValue) => void
  onClose: () => void
  closeLabel?: string
  wrapperRef?: RefObject<HTMLDivElement | null>
}

export function TimeField({
  label,
  value,
  active,
  open,
  onToggle,
  onChange,
  onClose,
  closeLabel,
  wrapperRef,
}: TimeFieldProps) {
  const inputClass = cn(
    'w-full border-none bg-transparent p-0 text-sm transition-colors focus:outline-none focus:ring-0',
    active ? 'font-semibold text-muted-foreground dark:text-muted-foreground' : 'font-medium text-muted-foreground dark:text-muted-foreground'
  )

  const iconClass = cn(
    'ml-2 h-5 w-5 transition-colors',
    active ? 'text-muted-foreground dark:text-muted-foreground' : 'text-muted-foreground dark:text-muted-foreground'
  )

  const [inputValue, setInputValue] = React.useState(() => timeToString(value))
  const lastValidValueRef = React.useRef<TimeValue>(value)

  React.useEffect(() => {
    lastValidValueRef.current = value
    setInputValue(timeToString(value))
  }, [value])

  const commitInput = React.useCallback(() => {
    const next = parseTimeString(inputValue, lastValidValueRef.current)
    if (!next) {
      setInputValue(timeToString(lastValidValueRef.current))
      return
    }
    lastValidValueRef.current = next
    setInputValue(timeToString(next))
    onChange(next)
  }, [inputValue, onChange])

  const handleInputKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLInputElement>) => {
      if (event.key === 'Enter') {
        event.preventDefault()
        commitInput()
        if (open) onClose()
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        setInputValue(timeToString(lastValidValueRef.current))
        if (open) onClose()
      }
    },
    [commitInput, onClose, open]
  )

  const handleWrapperClick = React.useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      const target = event.target as HTMLElement
      if (target.closest('input')) return
      onToggle()
    },
    [onToggle]
  )

  const handleWrapperKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      if (event.target !== event.currentTarget) return
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        onToggle()
      }
    },
    [onToggle]
  )

  return (
    <div className='flex-1 space-y-3'>
      <label className='text-muted-foreground block text-sm font-medium'>{label}</label>

      <div className='relative' ref={wrapperRef}>
        <div
          className={cn(
            'border-input bg-background dark:bg-input/30 flex w-full items-center rounded-lg border px-4 py-3.5 transition-[color,box-shadow]',
            open && 'border-ring ring-(--focus-ring) ring-3 ring-inset'
          )}
          role='button'
          tabIndex={0}
          onClick={handleWrapperClick}
          onKeyDown={handleWrapperKeyDown}
        >
          <input
            className={inputClass}
            value={inputValue}
            placeholder='HH:MM:SS'
            inputMode='numeric'
            onChange={(event) => setInputValue(event.target.value)}
            onBlur={commitInput}
            onKeyDown={handleInputKeyDown}
            onFocus={() => {
              if (!open) onToggle()
            }}
          />
          <Clock className={iconClass} />
        </div>

        {open && <TimeDropdown value={value} onChange={onChange} onClose={onClose} closeLabel={closeLabel} />}
      </div>
    </div>
  )
}
