import type { ReactNode } from 'react'
import type { FieldError } from 'react-hook-form'

// ─── FieldWrap ────────────────────────────────────────────────────────────────

interface FieldWrapProps {
  label: ReactNode
  error?: FieldError | { message?: string }
  tooltip?: string
  children: ReactNode
}

export function FieldWrap({ label, error, tooltip, children }: FieldWrapProps) {
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1">
        <label className="text-xs font-medium text-slate-400">{label}</label>
        {tooltip && (
          <span title={tooltip} className="text-slate-600 cursor-help text-xs leading-none">ⓘ</span>
        )}
      </div>
      {children}
      {error && <p className="text-xs text-red-400">{error.message}</p>}
    </div>
  )
}

// ─── Input ────────────────────────────────────────────────────────────────────

export function Field({ className = '', ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={`w-full bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
    />
  )
}

// ─── Select ───────────────────────────────────────────────────────────────────

export function SelectField({ className = '', children, ...props }: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      {...props}
      className={`w-full bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-1 focus:ring-blue-500 ${className}`}
    >
      {children}
    </select>
  )
}

// ─── Toggle ───────────────────────────────────────────────────────────────────

interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  tooltip?: string
}

export function Toggle({ checked, onChange, label, tooltip }: ToggleProps) {
  return (
    <label className="flex items-center gap-2 cursor-pointer select-none">
      <div
        onClick={() => onChange(!checked)}
        className={`relative w-9 h-5 rounded-full transition-colors ${checked ? 'bg-blue-600' : 'bg-slate-600'}`}
      >
        <div className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${checked ? 'translate-x-4' : ''}`} />
      </div>
      <span className="text-xs text-slate-400">{label}</span>
      {tooltip && <span title={tooltip} className="text-slate-600 cursor-help text-xs">ⓘ</span>}
    </label>
  )
}

// ─── RadioGroup ───────────────────────────────────────────────────────────────

interface RadioOption { value: string; label: string; tooltip?: string }

interface RadioGroupProps {
  value: string
  onChange: (v: string) => void
  options: RadioOption[]
}

export function RadioGroup({ value, onChange, options }: RadioGroupProps) {
  return (
    <div className="flex rounded-lg overflow-hidden border border-slate-600">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          title={o.tooltip}
          onClick={() => onChange(o.value)}
          className={`flex-1 py-1.5 text-xs font-medium transition-colors ${
            value === o.value
              ? 'bg-blue-600 text-white'
              : 'bg-slate-700 text-slate-400 hover:bg-slate-600'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
