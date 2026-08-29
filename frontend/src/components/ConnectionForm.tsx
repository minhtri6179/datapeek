import { useEffect, useState } from 'react'
import { Connection } from '../store'
import { DeleteConnection, SaveConnection, TestConnection } from '../../wailsjs/go/main/App'
import { config } from '../../wailsjs/go/models'

interface Props {
  editing: Connection | null
  onSaved: () => void
  onCancel: () => void
}

const empty: config.Connection = new config.Connection({
  name: '',
  type: 'mysql',
  host: '127.0.0.1',
  port: 3306,
  user: 'root',
  database: '',
  ssl: false,
})

export function ConnectionForm({ editing, onSaved, onCancel }: Props) {
  const [cfg, setCfg] = useState<config.Connection>(empty)
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (editing) {
      setCfg(new config.Connection({ ...editing }))
      setPassword('')
    } else {
      setCfg(new config.Connection({ ...empty }))
      setPassword('')
    }
    setError('')
  }, [editing])

  // Default port follows the selected type.
  const set = (patch: Partial<config.Connection>) => {
    setCfg((c) => {
      const next = { ...c, ...patch }
      if (patch.type && patch.type !== c.type) {
        next.port = patch.type === 'mysql' ? 3306 : 5432
      }
      return next
    })
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await SaveConnection(cfg, password, false)
      onSaved()
    } catch (err) {
      setError(String(err))
    } finally {
      setBusy(false)
    }
  }

  const test = async () => {
    setError('')
    setBusy(true)
    try {
      await TestConnection(cfg, password)
      setError('')
      alert('Connection OK')
    } catch (err) {
      setError(String(err))
    } finally {
      setBusy(false)
    }
  }

  const remove = async () => {
    if (!editing) return
    if (!confirm(`Delete connection "${editing.name}"?`)) return
    await DeleteConnection(editing.id)
    onSaved()
  }

  const field = 'field'
  return (
    <form className="conn-form" onSubmit={submit}>
      <h2>{editing ? 'Edit Connection' : 'New Connection'}</h2>
      {error && <div className="error-banner">{error}</div>}
      <label>Name
        <input className={field} value={cfg.name} onChange={(e) => set({ name: e.target.value })} required />
      </label>
      <label>Type
        <select className={field} value={cfg.type} onChange={(e) => set({ type: e.target.value })}>
          <option value="mysql">MySQL</option>
          <option value="postgres">PostgreSQL</option>
        </select>
      </label>
      <label>Host
        <input className={field} value={cfg.host} onChange={(e) => set({ host: e.target.value })} required />
      </label>
      <label>Port
        <input className={field} type="number" value={cfg.port}
          onChange={(e) => set({ port: Number(e.target.value) })} required />
      </label>
      <label>User
        <input className={field} value={cfg.user} onChange={(e) => set({ user: e.target.value })} required />
      </label>
      <label>Password {editing?.has_password && '(stored in keychain)'}
        <input className={field} type="password" value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={editing?.has_password ? 'leave blank to keep' : ''} />
      </label>
      <label>Database
        <input className={field} value={cfg.database} onChange={(e) => set({ database: e.target.value })} required />
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={cfg.ssl} onChange={(e) => set({ ssl: e.target.checked })} />
        Use SSL
      </label>
      <div className="form-actions">
        <button type="button" onClick={test} disabled={busy}>Test</button>
        {editing && <button type="button" className="danger" onClick={remove} disabled={busy}>Delete</button>}
        <span className="spacer" />
        <button type="button" onClick={onCancel} disabled={busy}>Cancel</button>
        <button type="submit" disabled={busy}>{busy ? '…' : 'Save'}</button>
      </div>
    </form>
  )
}
