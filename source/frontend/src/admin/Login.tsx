import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { api } from '../api'
import { useSession } from '../App'

function Shell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="brand big"><img src="/logo.png" alt="" width={150} height={150} /><span>pairing_easy</span></div>
        <h1>{title}</h1>
        {children}
      </div>
    </div>
  )
}

export function Login() {
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const { refresh } = useSession()
  const nav = useNavigate()
  const [params] = useSearchParams()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.login(login, password)
      await refresh()
      const next = params.get('next')
      nav(next && next.startsWith('/admin') ? next : '/admin', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell title="Espace arbitre">
      <form className="stack-form" onSubmit={submit}>
        <label>Identifiant<input value={login} onChange={e => setLogin(e.target.value)} autoComplete="username" autoFocus required /></label>
        <label>Mot de passe<input type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="current-password" required /></label>
        {error && <p className="error">{error}</p>}
        <button className="primary" disabled={busy}>{busy ? 'Connexion…' : 'Se connecter'}</button>
      </form>
      <p className="muted small"><a href="/">← Pages publiques</a></p>
    </Shell>
  )
}

export function Setup() {
  const [f, setF] = useState({ login: 'admin', name: '', email: '', password: '', confirm: '' })
  const [error, setError] = useState('')
  const { refresh } = useSession()
  const nav = useNavigate()
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement>) => setF(x => ({ ...x, [k]: e.target.value }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (f.password !== f.confirm) {
      setError('Les deux mots de passe diffèrent.')
      return
    }
    try {
      await api.setup({ login: f.login, name: f.name, email: f.email, password: f.password })
      await refresh()
      nav('/admin', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Shell title="Première installation">
      <p className="muted">Créez le compte administrateur. Il pourra ensuite créer les comptes des arbitres.</p>
      <form className="stack-form" onSubmit={submit}>
        <label>Identifiant<input value={f.login} onChange={set('login')} required autoComplete="username" /></label>
        <label>Nom affiché<input value={f.name} onChange={set('name')} required autoFocus /></label>
        <label>E-mail (facultatif)<input type="email" value={f.email} onChange={set('email')} /></label>
        <label>Mot de passe (10 caractères minimum)<input type="password" value={f.password} onChange={set('password')} required minLength={10} autoComplete="new-password" /></label>
        <label>Confirmation<input type="password" value={f.confirm} onChange={set('confirm')} required autoComplete="new-password" /></label>
        {error && <p className="error">{error}</p>}
        <button className="primary">Créer le compte administrateur</button>
      </form>
    </Shell>
  )
}
