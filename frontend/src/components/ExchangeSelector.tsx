import { useQuery } from '@tanstack/react-query'
import { getCredentials } from '../api'

interface Props {
  value: string       // exchange name, '' = all
  onChange: (exchange: string) => void
  className?: string
}

/**
 * Дропдаун для вибору ключа/біржі. Показує label ключа (не технічну назву біржі).
 * Внутрішньо фільтрує за exchange — значення передається у value/onChange.
 */
export default function ExchangeSelector({ value, onChange, className = '' }: Props) {
  const { data: credentials = [] } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
    staleTime: 30_000,
  })

  const active = credentials.filter((c) => c.is_active)

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={`bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-1 focus:ring-blue-500 ${className}`}
    >
      <option value="">All accounts</option>
      {active.map((c) => (
        <option key={c.id} value={c.exchange}>
          {c.label || c.exchange}
        </option>
      ))}
    </select>
  )
}
