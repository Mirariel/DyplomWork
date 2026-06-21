import { lazy, Suspense } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import Layout from './components/Layout'
import type { ReactNode } from 'react'

// ─── Lazy pages ───────────────────────────────────────────────────────────────

const Login      = lazy(() => import('./pages/Login'))
const Register   = lazy(() => import('./pages/Register'))
const Dashboard  = lazy(() => import('./pages/Dashboard'))
const Portfolio  = lazy(() => import('./pages/Portfolio'))
const Orders     = lazy(() => import('./pages/Orders'))
const SmartOrders = lazy(() => import('./pages/SmartOrders'))
const GridBots   = lazy(() => import('./pages/GridBots'))
const DCABots    = lazy(() => import('./pages/DCABots'))
const Analytics  = lazy(() => import('./pages/Analytics'))

// ─── Shared spinner used by Suspense fallback ─────────────────────────────────

function PageSpinner() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-slate-900">
      <div className="w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

// ─── Protected route wrapper ──────────────────────────────────────────────────

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()

  if (loading) return <PageSpinner />
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

// ─── App routes ───────────────────────────────────────────────────────────────

function AppRoutes() {
  const { user, loading } = useAuth()

  if (loading) return <PageSpinner />

  return (
    <Suspense fallback={<PageSpinner />}>
      <Routes>
        {/* Public */}
        <Route
          path="/login"
          element={user ? <Navigate to="/" replace /> : <Login />}
        />
        <Route
          path="/register"
          element={user ? <Navigate to="/" replace /> : <Register />}
        />

        {/* Protected */}
        <Route
          element={
            <RequireAuth>
              <Layout />
            </RequireAuth>
          }
        >
          <Route path="/" element={<Dashboard />} />
          <Route path="/portfolio" element={<Portfolio />} />
          <Route path="/orders" element={<Orders />} />
          <Route path="/smart-orders" element={<SmartOrders />} />
          <Route path="/bots" element={<GridBots />} />
          <Route path="/dca" element={<DCABots />} />
          <Route path="/analytics" element={<Analytics />} />
        </Route>

        {/* Fallback */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}
