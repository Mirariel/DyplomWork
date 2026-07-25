import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { User, KeyRound, CheckCircle, AlertCircle, FolderPlus, Trash2, X } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import {
  updateProfile, changePassword,
  getCredentials, listCredentialGroups, createCredentialGroup,
  deleteCredentialGroup, setGroupMembers,
  type Credential, type CredentialGroup,
} from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

interface AlertProps {
  type: 'success' | 'error'
  message: string
}

function Alert({ type, message }: AlertProps) {
  return (
    <div
      className={`flex items-center gap-2 rounded-lg px-4 py-3 text-sm ${
        type === 'success'
          ? 'bg-green-500/10 text-green-400 border border-green-500/20'
          : 'bg-red-500/10 text-red-400 border border-red-500/20'
      }`}
    >
      {type === 'success' ? <CheckCircle size={16} /> : <AlertCircle size={16} />}
      {message}
    </div>
  )
}

// ─── Profile section ──────────────────────────────────────────────────────────

function ProfileSection() {
  const { user, updateUser } = useAuth()
  const [username, setUsername] = useState(user?.username ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [loading, setLoading] = useState(false)
  const [alert, setAlert] = useState<AlertProps | null>(null)

  useEffect(() => {
    setUsername(user?.username ?? '')
    setEmail(user?.email ?? '')
  }, [user])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setAlert(null)
    setLoading(true)
    try {
      const updated = await updateProfile(username.trim(), email.trim())
      updateUser(updated)
      setAlert({ type: 'success', message: 'Profile updated successfully.' })
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Failed to update profile.'
      setAlert({ type: 'error', message: msg })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-6 space-y-5">
      <div className="flex items-center gap-3 border-b border-slate-700 pb-4">
        <div className="w-9 h-9 rounded-full bg-blue-600 flex items-center justify-center text-white font-bold">
          {user?.username?.[0]?.toUpperCase() ?? '?'}
        </div>
        <div>
          <h2 className="text-base font-semibold text-white">Profile</h2>
          <p className="text-xs text-slate-400">Update your username and email address</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-400 uppercase tracking-wider">
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              minLength={3}
              maxLength={50}
              className="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-400 uppercase tracking-wider">
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>
        </div>

        {alert && <Alert {...alert} />}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={loading}
            className="px-5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
          >
            {loading ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── Password section ─────────────────────────────────────────────────────────

function PasswordSection() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [alert, setAlert] = useState<AlertProps | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setAlert(null)
    if (next !== confirm) {
      setAlert({ type: 'error', message: 'New passwords do not match.' })
      return
    }
    if (next.length < 6) {
      setAlert({ type: 'error', message: 'New password must be at least 6 characters.' })
      return
    }
    setLoading(true)
    try {
      await changePassword(current, next)
      setAlert({ type: 'success', message: 'Password changed successfully.' })
      setCurrent('')
      setNext('')
      setConfirm('')
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Failed to change password.'
      setAlert({ type: 'error', message: msg })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-6 space-y-5">
      <div className="flex items-center gap-3 border-b border-slate-700 pb-4">
        <div className="w-9 h-9 rounded-full bg-slate-700 flex items-center justify-center">
          <KeyRound size={18} className="text-slate-300" />
        </div>
        <div>
          <h2 className="text-base font-semibold text-white">Change Password</h2>
          <p className="text-xs text-slate-400">Minimum 6 characters</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-slate-400 uppercase tracking-wider">
            Current password
          </label>
          <input
            type="password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            required
            autoComplete="current-password"
            className="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-400 uppercase tracking-wider">
              New password
            </label>
            <input
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              required
              minLength={6}
              autoComplete="new-password"
              className="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-400 uppercase tracking-wider">
              Confirm new password
            </label>
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>
        </div>

        {alert && <Alert {...alert} />}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={loading}
            className="px-5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
          >
            {loading ? 'Saving…' : 'Change password'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── Account info section ─────────────────────────────────────────────────────

function AccountInfo() {
  const { user } = useAuth()
  if (!user) return null

  const rows = [
    { label: 'User ID', value: `#${user.id}` },
    { label: 'Username', value: user.username },
    { label: 'Email', value: user.email },
    { label: 'Member since', value: fmtDate(user.created_at) },
  ]

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-6 space-y-5">
      <div className="flex items-center gap-3 border-b border-slate-700 pb-4">
        <div className="w-9 h-9 rounded-full bg-slate-700 flex items-center justify-center">
          <User size={18} className="text-slate-300" />
        </div>
        <div>
          <h2 className="text-base font-semibold text-white">Account Info</h2>
          <p className="text-xs text-slate-400">Read-only overview</p>
        </div>
      </div>

      <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {rows.map(({ label, value }) => (
          <div key={label} className="space-y-0.5">
            <dt className="text-xs font-medium text-slate-500 uppercase tracking-wider">{label}</dt>
            <dd className="text-sm text-white">{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}

// ─── Credential Groups section ───────────────────────────────────────────────

function credLabel(c: Credential) {
  return c.label || `${c.exchange.toUpperCase()} (${c.api_key_hint})`
}

function CredentialGroupsSection() {
  const qc = useQueryClient()
  const [newName, setNewName] = useState('')
  const [selectedCreds, setSelectedCreds] = useState<number[]>([])
  const [editingGroupId, setEditingGroupId] = useState<number | null>(null)
  const [editCreds, setEditCreds] = useState<number[]>([])

  const { data: credentials = [] } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
    staleTime: 30_000,
  })

  const { data: groups = [] } = useQuery({
    queryKey: ['credential-groups'],
    queryFn: listCredentialGroups,
  })

  const activeCreds = credentials.filter((c: Credential) => c.is_active)

  const createMut = useMutation({
    mutationFn: createCredentialGroup,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['credential-groups'] })
      setNewName('')
      setSelectedCreds([])
    },
  })

  const deleteMut = useMutation({
    mutationFn: deleteCredentialGroup,
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['credential-groups'] }),
  })

  const setMembersMut = useMutation({
    mutationFn: ({ id, members }: { id: number; members: number[] }) =>
      setGroupMembers(id, members),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['credential-groups'] })
      setEditingGroupId(null)
    },
  })

  const toggleCred = (id: number, list: number[], setList: (v: number[]) => void) => {
    setList(list.includes(id) ? list.filter((x) => x !== id) : [...list, id])
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    createMut.mutate({ name: newName.trim(), member_ids: selectedCreds })
  }

  const startEdit = (g: CredentialGroup) => {
    setEditingGroupId(g.id)
    setEditCreds([...g.member_ids])
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-6 space-y-5">
      <div className="flex items-center gap-3 border-b border-slate-700 pb-4">
        <div className="w-9 h-9 rounded-full bg-slate-700 flex items-center justify-center">
          <FolderPlus size={18} className="text-slate-300" />
        </div>
        <div>
          <h2 className="text-base font-semibold text-white">API Key Groups</h2>
          <p className="text-xs text-slate-400">Group API keys for batch order placement</p>
        </div>
      </div>

      {/* Create new group */}
      <form onSubmit={handleCreate} className="space-y-3">
        <div className="flex gap-2">
          <input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Group name"
            required
            className="flex-1 bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
          <button
            type="submit"
            disabled={createMut.isPending || !newName.trim()}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            Create
          </button>
        </div>

        {activeCreds.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {activeCreds.map((c: Credential) => (
              <button
                key={c.id}
                type="button"
                onClick={() => toggleCred(c.id, selectedCreds, setSelectedCreds)}
                className={`text-xs px-2.5 py-1 rounded-full border transition-colors ${
                  selectedCreds.includes(c.id)
                    ? 'bg-blue-600/30 border-blue-500 text-blue-300'
                    : 'bg-slate-700 border-slate-600 text-slate-400 hover:border-slate-500'
                }`}
              >
                {credLabel(c)}
              </button>
            ))}
          </div>
        )}
      </form>

      {/* Existing groups */}
      {groups.length > 0 && (
        <div className="space-y-3 pt-2">
          {groups.map((g: CredentialGroup) => (
            <div key={g.id} className="bg-slate-900 rounded-lg border border-slate-700 p-3 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-white">{g.name}</span>
                <div className="flex items-center gap-2">
                  {editingGroupId === g.id ? (
                    <>
                      <button
                        onClick={() => setMembersMut.mutate({ id: g.id, members: editCreds })}
                        disabled={setMembersMut.isPending}
                        className="text-xs px-2 py-1 bg-green-600 text-white rounded hover:bg-green-500"
                      >
                        Save
                      </button>
                      <button
                        onClick={() => setEditingGroupId(null)}
                        className="text-slate-400 hover:text-white"
                      >
                        <X size={14} />
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        onClick={() => startEdit(g)}
                        className="text-xs px-2 py-1 bg-slate-700 text-slate-300 rounded hover:bg-slate-600"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => deleteMut.mutate(g.id)}
                        disabled={deleteMut.isPending}
                        className="text-slate-400 hover:text-red-400"
                      >
                        <Trash2 size={14} />
                      </button>
                    </>
                  )}
                </div>
              </div>

              {editingGroupId === g.id ? (
                <div className="flex flex-wrap gap-1.5">
                  {activeCreds.map((c: Credential) => (
                    <button
                      key={c.id}
                      type="button"
                      onClick={() => toggleCred(c.id, editCreds, setEditCreds)}
                      className={`text-xs px-2 py-0.5 rounded-full border transition-colors ${
                        editCreds.includes(c.id)
                          ? 'bg-blue-600/30 border-blue-500 text-blue-300'
                          : 'bg-slate-800 border-slate-600 text-slate-500 hover:border-slate-500'
                      }`}
                    >
                      {credLabel(c)}
                    </button>
                  ))}
                </div>
              ) : (
                <div className="flex flex-wrap gap-1.5">
                  {g.member_ids.length === 0 ? (
                    <span className="text-xs text-slate-500">No keys assigned</span>
                  ) : (
                    g.member_ids.map((mid: number) => {
                      const c = activeCreds.find((cr: Credential) => cr.id === mid)
                      return (
                        <span
                          key={mid}
                          className="text-xs px-2 py-0.5 rounded-full bg-slate-700 text-slate-300"
                        >
                          {c ? credLabel(c) : `#${mid}`}
                        </span>
                      )
                    })
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function SettingsPage() {
  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <p className="text-slate-400 text-sm mt-1">Manage your account and security preferences</p>
      </div>

      <AccountInfo />
      <ProfileSection />
      <PasswordSection />
      <CredentialGroupsSection />
    </div>
  )
}
