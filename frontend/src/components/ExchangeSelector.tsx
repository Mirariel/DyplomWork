import { useQuery } from '@tanstack/react-query'
import { getCredentials } from '../api'

interface ExchangeSelectorProps {
  value: string
  onChange: (exchange: string) => void
  className?: string
}

/**
 * Dropdown що дозволяє вибрати біржу з активних credentials користувача.
 * Дефолтна опція — "All exchanges" (порожній рядок).
 */
export default function ExchangeSelector({ value, onChange, className = '' }: ExchangeSelectorProps) {
  const { data: credentials = [] } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
    staleTime: 30_000,
  })

  const activeExchanges = credentials
    .filter((c) => c.is_active)
    .map((c) => c.exchange)

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={`bg-slate-700 border border-slate-600 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-1 focus:ring-blue-500 ${className}`}
    >
      <option value="">All exchanges</option>
      {activeExchanges.map((ex) => (
        <option key={ex} value={ex}>
          {ex.charAt(0).toUpperCase() + ex.slice(1)}
        </option>
      ))}
    </select>
  )
}
