import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import AiAdvisor from './AiAdvisor'
import {
  LayoutDashboard,
  Briefcase,
  ShoppingCart,
  Zap,
  Grid3x3,
  TrendingUp,
  BarChart2,
  Settings,
  LogOut,
  Menu,
  X,
  Activity,
  Layers,
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'

// ─── Nav items ────────────────────────────────────────────────────────────────

const NAV_ITEMS = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, exact: true },
  { to: '/portfolio', label: 'Portfolio', icon: Briefcase },
  { to: '/orders', label: 'Orders', icon: ShoppingCart },
  { to: '/smart-orders', label: 'Smart Orders', icon: Zap },
  { to: '/bots', label: 'Grid Bots', icon: Grid3x3 },
  { to: '/dca', label: 'DCA Bots', icon: TrendingUp },
  { to: '/analytics', label: 'Analytics', icon: BarChart2 },
  { to: '/futures', label: 'Futures', icon: Layers },
  { to: '/settings', label: 'Settings', icon: Settings },
]

// ─── Sidebar ──────────────────────────────────────────────────────────────────

interface SidebarProps {
  onClose?: () => void
}

function Sidebar({ onClose }: SidebarProps) {
  const { user, logout } = useAuth()

  return (
    <div className="flex flex-col h-full bg-slate-800 border-r border-slate-700">
      {/* Logo */}
      <div className="flex items-center gap-2 px-6 py-5 border-b border-slate-700">
        <Activity className="text-blue-500" size={24} />
        <span className="text-xl font-bold text-white tracking-tight">
          TradeTracker
        </span>
        {onClose && (
          <button
            onClick={onClose}
            className="ml-auto text-slate-400 hover:text-white lg:hidden"
          >
            <X size={20} />
          </button>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
        {NAV_ITEMS.map(({ to, label, icon: Icon, exact }) => (
          <NavLink
            key={to}
            to={to}
            end={exact}
            onClick={onClose}
            className={({ isActive }) =>
              `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:bg-slate-700 hover:text-white'
              }`
            }
          >
            <Icon size={18} />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* User footer */}
      <div className="px-4 py-4 border-t border-slate-700">
        <div className="flex items-center gap-3">
          <NavLink
            to="/settings"
            onClick={onClose}
            className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white text-xs font-bold flex-shrink-0 hover:bg-blue-500 transition-colors"
            title="Profile settings"
          >
            {user?.username?.[0]?.toUpperCase() ?? '?'}
          </NavLink>
          <div className="flex-1 min-w-0">
            <p className="text-sm text-white font-medium truncate">{user?.username}</p>
            <p className="text-xs text-slate-400 truncate">{user?.email}</p>
          </div>
          <button
            onClick={() => void logout()}
            title="Logout"
            className="text-slate-400 hover:text-red-400 transition-colors flex-shrink-0"
          >
            <LogOut size={17} />
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Layout ───────────────────────────────────────────────────────────────────

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="flex h-screen bg-slate-900 overflow-hidden">
      {/* Desktop sidebar */}
      <aside className="hidden lg:flex lg:flex-col w-60 flex-shrink-0">
        <Sidebar />
      </aside>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Mobile sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-60 flex flex-col transform transition-transform duration-200 lg:hidden ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <Sidebar onClose={() => setSidebarOpen(false)} />
      </aside>

      {/* Main content */}
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden">
        {/* Mobile top bar */}
        <header className="lg:hidden flex items-center gap-3 px-4 py-3 bg-slate-800 border-b border-slate-700 flex-shrink-0">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-slate-400 hover:text-white"
          >
            <Menu size={22} />
          </button>
          <Activity className="text-blue-500" size={20} />
          <span className="font-bold text-white">TradeTracker</span>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>

      {/* AI Advisor floating widget */}
      <AiAdvisor />
    </div>
  )
}
