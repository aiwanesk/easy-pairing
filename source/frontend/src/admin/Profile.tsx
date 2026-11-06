import { roleLabel } from '../util'
import { useState } from 'react'
import { api } from '../api'
import { useSession } from '../App'
import { useAction } from '../components/Toast'

export function ProfilePage() {
  const { account } = useSession()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const run = useAction()

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (next !== confirm) { alert('Les deux mots de passe diffèrent.'); return }
    const ok = await run(() => api.changePassword(current, next).then(() => true), 'Mot de passe changé')
    if (ok) { setCurrent(''); setNext(''); setConfirm('') }
  }

  return (
    <div className="page">
      <div className="page-head"><h1>Mon compte</h1></div>
      <section className="card">
        <dl className="kv">
          <dt>Nom</dt><dd>{account?.name}</dd>
          <dt>Identifiant</dt><dd className="mono">{account?.login}</dd>
          <dt>Rôle</dt><dd>{roleLabel[account?.role ?? 'user']}</dd>
        </dl>
      </section>
      <section className="card">
        <h2>Changer de mot de passe</h2>
        <form className="form-grid" onSubmit={save}>
          <label>Mot de passe actuel<input type="password" value={current} onChange={e => setCurrent(e.target.value)} required autoComplete="current-password" /></label>
          <label>Nouveau (10 caractères min.)<input type="password" value={next} onChange={e => setNext(e.target.value)} required minLength={10} autoComplete="new-password" /></label>
          <label>Confirmation<input type="password" value={confirm} onChange={e => setConfirm(e.target.value)} required autoComplete="new-password" /></label>
          <div className="actions"><button className="primary">Changer</button></div>
        </form>
        <p className="muted small">Changer de mot de passe déconnecte vos autres sessions.</p>
      </section>
    </div>
  )
}
