import { useEffect, useState } from 'react'
import { api } from '../api'
import { useSession } from '../App'
import type { Account } from '../types'
import { fmtDate, roleLabel } from '../util'
import { Modal } from '../components/Modal'
import { useAction } from '../components/Toast'

export function AccountsPage() {
  const [list, setList] = useState<Account[]>([])
  const [editing, setEditing] = useState<Account | null>(null)
  const [creating, setCreating] = useState(false)
  const run = useAction()
  const { account } = useSession()
  const load = () => api.accounts().then(setList).catch(() => {})
  useEffect(() => { load() }, [])

  const remove = async (a: Account) => {
    if (!confirm(`Supprimer le compte ${a.name} ?`)) return
    if (await run(() => api.deleteAccount(a.id).then(() => true), 'Compte supprimé')) load()
  }

  return (
    <div className="page">
      <div className="page-head"><h1>Comptes</h1><button className="primary" onClick={() => setCreating(true)}>Nouveau compte</button></div>
      <section className="card">
        <table className="table">
          <thead><tr><th>Nom</th><th>Identifiant</th><th>E-mail</th><th>Rôle</th><th>État</th><th>Dernière connexion</th><th /></tr></thead>
          <tbody>
            {list.map(a => (
              <tr key={a.id} className={a.active ? '' : 'withdrawn'}>
                <td><button className="link" onClick={() => setEditing(a)}>{a.name}</button></td>
                <td className="mono small">{a.login}</td>
                <td>{a.email}</td>
                <td>{a.role === 'admin' ? <span className="tag ok">Administrateur</span> : roleLabel[a.role]}</td>
                <td>{a.active ? 'actif' : <span className="tag bad">désactivé</span>}</td>
                <td className="muted">{fmtDate(a.lastLogin)}</td>
                <td className="row-actions">
                  <button className="ghost" onClick={() => setEditing(a)}>Modifier</button>
                  {a.id !== account?.id && <button className="ghost danger" onClick={() => remove(a)}>✕</button>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="muted small">Les droits sur chaque événement se donnent dans l'onglet « Accès » de l'événement.</p>
      </section>
      {creating && <AccountForm onClose={() => setCreating(false)} onSaved={() => { setCreating(false); load() }} />}
      {editing && <AccountForm account={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); load() }} />}
    </div>
  )
}

function AccountForm({ account, onClose, onSaved }: { account?: Account; onClose: () => void; onSaved: () => void }) {
  const [f, setF] = useState({ login: account?.login ?? '', name: account?.name ?? '', email: account?.email ?? '', role: account?.role ?? 'user', active: account?.active ?? true, password: '' })
  const run = useAction()
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setF(x => ({ ...x, [k]: e.target.type === 'checkbox' ? (e.target as HTMLInputElement).checked : e.target.value }))

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    let ok
    if (account) {
      ok = await run(async () => {
        await api.updateAccount({ ...account, name: f.name, email: f.email, role: f.role as Account['role'], active: f.active })
        if (f.password) await api.resetPassword(account.id, f.password)
        return true
      }, 'Compte mis à jour')
    } else {
      ok = await run(() => api.createAccount(f), 'Compte créé')
    }
    if (ok) onSaved()
  }

  return (
    <Modal title={account ? `Compte ${account.login}` : 'Nouveau compte'} onClose={onClose}>
      <form className="form-grid two" onSubmit={save}>
        <label>Identifiant<input value={f.login} onChange={set('login')} required disabled={!!account} /></label>
        <label>Nom<input value={f.name} onChange={set('name')} required /></label>
        <label>E-mail<input type="email" value={f.email} onChange={set('email')} /></label>
        <label>Rôle
          <select value={f.role} onChange={set('role')}>
            <option value="user">Utilisateur (accès aux seuls événements partagés avec lui : arbitre, saisie des résultats, lecture)</option>
            <option value="manager">Gestionnaire (crée et gère ses événements)</option>
            <option value="admin">Administrateur (tout, comptes et système)</option>
          </select>
        </label>
        <label className="span-2">{account ? 'Nouveau mot de passe (laisser vide pour ne pas changer)' : 'Mot de passe (10 caractères minimum)'}
          <input type="password" value={f.password} onChange={set('password')} required={!account} minLength={account && !f.password ? 0 : 10} autoComplete="new-password" />
        </label>
        {account && <label className="check span-2"><input type="checkbox" checked={f.active} onChange={set('active')} /> Compte actif (désactiver déconnecte immédiatement)</label>}
        <div className="actions end span-2"><button type="button" onClick={onClose}>Annuler</button><button className="primary">Enregistrer</button></div>
      </form>
    </Modal>
  )
}
