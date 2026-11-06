import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api } from '../api'
import type { EventSummary } from '../types'

export function PublicShell({ children, title }: { children: React.ReactNode; title?: React.ReactNode }) {
  return (
    <div className="public">
      <header className="public-top">
        <Link to="/" className="brand"><img src="/logo-64.png" alt="" width={32} height={32} /><span>pairing_easy</span></Link>
        {title}
        <Link to="/admin" className="muted small">Espace arbitre</Link>
      </header>
      <main className="public-main">{children}</main>
    </div>
  )
}

export function PublicHome() {
  const [list, setList] = useState<EventSummary[] | null>(null)
  useEffect(() => { api.publicEvents().then(setList).catch(() => setList([])) }, [])
  return (
    <PublicShell>
      <h1>Tournois</h1>
      {list === null ? <p className="muted">Chargement…</p> : list.length === 0 ? (
        <div className="card"><p className="muted">Aucun événement public pour le moment.</p><p><Link to="/admin">Espace arbitre →</Link></p></div>
      ) : (
        <div className="cards">
          {list.map(e => (
            <Link key={e.id} to={`/e/${e.slug}`} className="tcard">
              <div className="tcard-title">{e.name}</div>
              <div className="muted small">{[e.location, e.startDate].filter(Boolean).join(' · ')}</div>
              <div className="tcard-stats"><span><b>{e.tournaments}</b> tournoi(s)</span><span><b>{e.players}</b> joueurs</span></div>
            </Link>
          ))}
        </div>
      )}
    </PublicShell>
  )
}
