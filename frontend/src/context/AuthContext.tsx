import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from 'react'
import { login as apiLogin, logout as apiLogout, me } from '../api'
import type { User } from '../api'

// ─── Context types ────────────────────────────────────────────────────────────

interface AuthContextValue {
  user: User | null
  token: string | null
  loading: boolean
  login: (login: string, password: string) => Promise<void>
  logout: () => Promise<void>
  updateUser: (user: User) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

// ─── Provider ─────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem('tt_token'),
  )
  const [loading, setLoading] = useState(true)

  // On mount or token change: verify the token via /auth/me.
  // The cancellation flag handles React StrictMode double-invoke in dev
  // (the first effect run is cancelled before the second one fires).
  useEffect(() => {
    if (!token) {
      setLoading(false)
      return
    }
    let cancelled = false
    me()
      .then((u) => { if (!cancelled) setUser(u) })
      .catch(() => {
        if (cancelled) return
        localStorage.removeItem('tt_token')
        setToken(null)
        setUser(null)
      })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [token])

  const login = useCallback(async (loginId: string, password: string) => {
    const data = await apiLogin(loginId, password)
    localStorage.setItem('tt_token', data.token)
    setUser(data.user)
    setToken(data.token)
  }, [])

  const logout = useCallback(async () => {
    try {
      await apiLogout()
    } catch {
      // ignore server errors on logout
    }
    localStorage.removeItem('tt_token')
    setToken(null)
    setUser(null)
  }, [])

  const updateUser = useCallback((u: User) => {
    setUser(u)
  }, [])

  return (
    <AuthContext.Provider value={{ user, token, loading, login, logout, updateUser }}>
      {children}
    </AuthContext.Provider>
  )
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
