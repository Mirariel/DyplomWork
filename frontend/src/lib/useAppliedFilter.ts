import { useState, useCallback } from 'react'

export interface DateFilter {
  from: string
  to: string
  preset: string
}

const EMPTY: DateFilter = { from: '', to: '', preset: '' }

export function useAppliedFilter() {
  const [draft, setDraft] = useState<DateFilter>(EMPTY)
  const [applied, setApplied] = useState<DateFilter>(EMPTY)

  const draftChanged = draft.from !== applied.from || draft.to !== applied.to

  const applyPreset = useCallback((label: string, days: number) => {
    const to = new Date()
    const from = new Date()
    from.setDate(from.getDate() - (days - 1))
    const fmt = (d: Date) => d.toISOString().slice(0, 10)
    setDraft({ from: fmt(from), to: fmt(to), preset: label })
  }, [])

  const applyFilters = useCallback(() => {
    setApplied({ ...draft })
  }, [draft])

  const clearFilters = useCallback(() => {
    setDraft(EMPTY)
    setApplied(EMPTY)
  }, [])

  const setFrom = useCallback((v: string) => {
    setDraft((d) => ({ ...d, from: v, preset: '' }))
  }, [])

  const setTo = useCallback((v: string) => {
    setDraft((d) => ({ ...d, to: v, preset: '' }))
  }, [])

  return {
    draft,
    applied,
    draftChanged,
    applyPreset,
    applyFilters,
    clearFilters,
    setFrom,
    setTo,
  }
}
