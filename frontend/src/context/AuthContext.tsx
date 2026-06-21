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
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

// ─── Provider ─────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem('tt_token'),
  )
  const [loading, setLoading] = useState(true)

  // On mount: if we have a stored token, verify it by calling /auth/me
  useEffect(() => {
    if (!token) {
      setLoading(false)
      return
    }
    me()
      .then((u) => setUser(u))
      .catch(() => {
        localStorage.removeItem('tt_token')
        setToken(null)
      })
      .finally(() => setLoading(false))
  }, [token])

  const login = useCallback(async (email: string, password: string) => {
    const data = await apiLogin(email, password)
    localStorage.setItem('tt_token', data.token)
    setToken(data.token)
    setUser(data.user)
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

  return (
    <AuthContext.Provider value={{ user, token, loading, login, logout }}>
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
