import * as React from 'react'
import { Calendar, ChevronLeft, ChevronRight } from 'lucide-react'
import { DayPicker, type DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useClickOutside } from '@/hooks/use-click-outside'
import { buttonVariants } from '@/components/ui/button'
import type { DateTimeRangeValue } from '@/utils/date-range'
import {
  DEFAULT_END_TIME,
  DEFAULT_START_TIME,
  defaultDateTimeRangeValue,
  isSameTime,
  normalizeDateTimeRangeValue,
} from '@/utils/date-range'
import { dayPickerClassNames, dayPickerComponents, dayPickerFormatters } from './day-picker-config'
import { TimeField } from './time-field'
import { addMonthsSafe, formatRange } from './utils'

export interface DateTimeRangePickerProps {
  value?: DateTimeRangeValue
  onChange?: (next: DateTimeRangeValue | undefined) => void
  onCancel?: () => void
  onConfirm?: (next: DateTimeRangeValue) => void
  className?: string
}

export function DateTimeRangePicker(props: DateTimeRangePickerProps) {
  const { value, onChange, onCancel, onConfirm, className } = props
  const { t } = useTranslation()
  const isControlled = Object.prototype.hasOwnProperty.call(props, 'value')
  const normalizedValue = React.useMemo(() => normalizeDateTimeRangeValue(value), [value])
  const [internal, setInternal] = React.useState<DateTimeRangeValue>(() => normalizedValue)

  React.useEffect(() => {
    if (!isControlled) return
    setInternal(normalizedValue)
  }, [isControlled, normalizedValue])

  const emit = React.useCallback(
    (next: DateTimeRangeValue) => {
      if (!isControlled) setInternal(next)
      onChange?.(next)
    },
    [isControlled, onChange]
  )

  const handleReset = React.useCallback(() => {
    const next = defaultDateTimeRangeValue()
    if (!isControlled) setInternal(next)
    onChange?.(undefined)
  }, [isControlled, onChange])

  const range: DateRange | undefined =
    internal.from || internal.to ? { from: internal.from, to: internal.to } : undefined

  const [month, setMonth] = React.useState<Date>(() => internal.from ?? new Date())
  React.useEffect(() => {
    if (internal.from) setMonth(internal.from)
  }, [internal.from])

  const [openPanel, setOpenPanel] = React.useState<'start' | 'end' | null>(null)
  const startOpen = openPanel === 'start'
  const endOpen = openPanel === 'end'
  const startWrapRef = React.useRef<HTMLDivElement>(null)
  const endWrapRef = React.useRef<HTMLDivElement>(null)
  const closePanel = React.useCallback(() => setOpenPanel(null), [])
  useClickOutside(startWrapRef, closePanel, startOpen)
  useClickOutside(endWrapRef, closePanel, endOpen)

  const toggleStart = React.useCallback(() => {
    setOpenPanel((current) => (current === 'start' ? null : 'start'))
  }, [])

  const toggleEnd = React.useCallback(() => {
    setOpenPanel((current) => (current === 'end' ? null : 'end'))
  }, [])

  const headerText = React.useMemo(
    () => formatRange(internal.from, internal.to, t('common.filters.dateRange')),
    [internal.from, internal.to, t]
  )

  const startActive = startOpen || !isSameTime(internal.startTime, DEFAULT_START_TIME)
  const endActive = endOpen || !isSameTime(internal.endTime, DEFAULT_END_TIME)

  return (
    <div
      className={cn(
        'bg-popover text-popover-foreground w-full max-w-[720px] overflow-visible rounded-xl shadow-md ring-1 ring-foreground/10',
        className
      )}
    >
      <div className='flex items-center justify-between border-b p-4'>
        <div
          className={cn(
            buttonVariants({ variant: 'outline', size: 'sm' }),
            'h-8 cursor-default border-solid'
          )}
        >
          <Calendar className='h-4 w-4' />
          <span>{headerText}</span>
        </div>

        <div className='flex gap-1 text-muted-foreground'>
          <button
            type='button'
            className='hover:bg-foreground/5 rounded-full p-2 transition-colors'
            onClick={() => setMonth((m) => addMonthsSafe(m, -1))}
          >
            <ChevronLeft className='h-5 w-5' />
          </button>
          <button
            type='button'
            className='hover:bg-foreground/5 rounded-full p-2 transition-colors'
            onClick={() => setMonth((m) => addMonthsSafe(m, 1))}
          >
            <ChevronRight className='h-5 w-5' />
          </button>
        </div>
      </div>

      <div className='flex flex-col gap-8 p-6 md:flex-row'>
        <div className='flex-1'>
          <DayPicker
            mode='range'
            selected={range}
            onSelect={(next) => {
              emit({
                ...internal,
                from: next?.from,
                to: next?.to,
              })
            }}
            month={month}
            onMonthChange={setMonth}
            numberOfMonths={2}
            showOutsideDays
            fixedWeeks
            weekStartsOn={0}
            classNames={dayPickerClassNames}
            formatters={dayPickerFormatters}
            components={dayPickerComponents}
          />
        </div>
      </div>

      <div className='bg-muted border-t px-6 py-6'>
        <div className='flex flex-col gap-6 md:flex-row'>
          <TimeField
            label={t('common.filters.startTime')}
            value={internal.startTime}
            active={startActive}
            open={startOpen}
            onToggle={toggleStart}
            onChange={(next) => emit({ ...internal, startTime: next })}
            onClose={closePanel}
            closeLabel={t('common.close')}
            wrapperRef={startWrapRef}
          />

          <TimeField
            label={t('common.filters.endTime')}
            value={internal.endTime}
            active={endActive}
            open={endOpen}
            onToggle={toggleEnd}
            onChange={(next) => emit({ ...internal, endTime: next })}
            onClose={closePanel}
            closeLabel={t('common.close')}
            wrapperRef={endWrapRef}
          />
        </div>
      </div>

      <div className='flex items-center justify-between border-t px-6 py-4'>
        <button
          type='button'
          className='text-muted-foreground hover:text-foreground rounded-md text-xs font-medium transition-colors'
          onClick={handleReset}
        >
          {t('common.filters.reset')}
        </button>

        <div className='flex gap-4'>
          <button
            type='button'
            className='text-muted-foreground hover:bg-muted hover:text-foreground h-9 min-w-24 rounded-lg px-6 text-sm font-medium transition-colors'
            onClick={onCancel}
          >
            {t('common.buttons.cancel')}
          </button>
          <button
            type='button'
            className='bg-primary text-primary-foreground h-9 min-w-24 rounded-lg px-6 text-sm font-medium transition-colors'
            onClick={() => onConfirm?.(internal)}
          >
            {t('common.buttons.confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}
