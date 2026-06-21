import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useRef,
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

  // Запобігає повторному виклику me() коли токен щойно встановлено через login()
  const skipNextMeCall = useRef(false)

  // On mount (or token change from logout): verify stored token via /auth/me
  useEffect(() => {
    if (!token) {
      setLoading(false)
      return
    }
    if (skipNextMeCall.current) {
      skipNextMeCall.current = false
      setLoading(false)
      return
    }
    me()
      .then((u) => setUser(u))
      .catch(() => {
        localStorage.removeItem('tt_token')
        setToken(null)
        setUser(null)
      })
      .finally(() => setLoading(false))
  }, [token])

  const login = useCallback(async (loginId: string, password: string) => {
    const data = await apiLogin(loginId, password)
    localStorage.setItem('tt_token', data.token)
    skipNextMeCall.current = true
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
