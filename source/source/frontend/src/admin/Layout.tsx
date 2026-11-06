import { roleLabel } from '../util'
import { useEffect, useState } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { api } from '../api'
import { useSession } from '../App'
import type { SystemInfo } from '../types'

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const { account, version, refresh } = useSession()
  const nav = useNavigate()
  const [sys, setSys] = useState<SystemInfo | null>(null)

  useEffect(() => {
    const load = () => api.system().then(setSys).catch(() => {})
    load()
    const id = window.setInterval(load, 15000)
    return () => window.clearInterval(id)
  }, [])

  const logout = async () => {
    await api.logout().catch(() => {})
    await refresh()
    nav('/admin/connexion')
  }

  return (
    <div className="admin">
      <aside className="sidebar">
        <NavLink to="/admin" end className="brand"><img src="/logo-64.png" alt="" width={34} height={34} /><span>pairing_easy</span></NavLink>
        <nav>
          <NavLink to="/admin" end>Événements</NavLink>
          {account?.role === 'admin' && <NavLink to="/admin/comptes">Comptes</NavLink>}
          {account?.role === 'admin' && <NavLink to="/admin/journal">Journal d'audit</NavLink>}
          <NavLink to="/admin/systeme">Système</NavLink>
          <NavLink to="/admin/profil">Mon compte</NavLink>
          <a href="/" target="_blank" rel="noreferrer">Pages publiques ↗</a>
        </nav>
        <div className="sidebar-foot">
          <div className="status-lines">
            <span className={`dot-status ${sys?.engine.ready ? 'ok' : 'bad'}`} /> Moteur {sys?.engine.ready ? 'prêt' : 'absent'}
            <br />
            <span className={`dot-status ${sys?.ffe.state === 'ready' ? 'ok' : sys?.ffe.state === 'loading' ? 'warn' : 'muted'}`} />
            Base FFE {sys?.ffe.state === 'ready' ? `${sys.ffe.players.toLocaleString('fr-FR')} joueurs` : sys?.ffe.state === 'loading' ? 'chargement…' : 'absente'}
          </div>
          <div className="who">
            <strong>{account?.name}</strong>
            <span className="muted small">{roleLabel[account?.role ?? 'user']}</span>
          </div>
          <button className="ghost small" onClick={logout}>Se déconnecter</button>
          <div className="muted tiny">v{version}</div>
        </div>
      </aside>
      <main className="admin-main">{children}</main>
    </div>
  )
}
