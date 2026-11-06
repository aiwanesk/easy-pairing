import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { api } from '../api'
import type { EventSummary } from '../types'
import { fmtDate, levelLabel } from '../util'
import { useAction } from '../components/Toast'
import { useSession } from '../App'

export function EventsPage() {
  const { account } = useSession()
  const [list, setList] = useState<EventSummary[] | null>(null)
  const [name, setName] = useState('')
  const [location, setLocation] = useState('')
  const [startDate, setStartDate] = useState('')
  const run = useAction()
  const nav = useNavigate()

  useEffect(() => { api.events().then(setList).catch(() => setList([])) }, [])

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    const ev = await run(() => api.createEvent({ name, location, startDate, endDate: startDate }))
    if (ev) nav(`/admin/e/${ev.id}`)
  }

  const importFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    e.target.value = ''
    if (!f) return
    const ev = await run(() => api.importEvent(f), 'Événement importé')
    if (ev) nav(`/admin/e/${ev.id}`)
  }

  return (
    <div className="page">
      <div className="page-head"><h1>Événements</h1></div>
      {(account?.role === 'admin' || account?.role === 'manager') && <section className="card">
        <h2>Nouvel événement</h2>
        <p className="muted small">Un événement regroupe un ou plusieurs tournois (open A, open B, rapide…) joués au même endroit.</p>
        <form className="form-grid" onSubmit={create}>
          <label className="span-2">Nom<input value={name} onChange={e => setName(e.target.value)} placeholder="Festival d'échecs de Lancy 2026" required /></label>
          <label>Lieu<input value={location} onChange={e => setLocation(e.target.value)} /></label>
          <label>Date de début<input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} /></label>
          <div className="actions span-4">
            <button className="primary" disabled={!name.trim()}>Créer l'événement</button>
            <label className="button ghost" title="Fichier exporté depuis une autre installation (en ligne ou hors ligne)">
              Importer un fichier…<input type="file" accept=".json,application/json" hidden onChange={importFile} />
            </label>
          </div>
        </form>
      </section>}
      <section className="card">
        <h2>Mes événements</h2>
        {list === null ? <p className="muted">Chargement…</p> : list.length === 0 ? <p className="muted">Aucun événement.</p> : (
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Nom</th><th>Lieu</th><th>Dates</th><th className="num">Tournois</th><th className="num">Joueurs</th><th>Public</th><th>Mes droits</th><th>Modifié</th></tr></thead>
              <tbody>
                {list.map(e => (
                  <tr key={e.id} className="clickable" onClick={() => nav(`/admin/e/${e.id}`)}>
                    <td><Link to={`/admin/e/${e.id}`}>{e.name}</Link></td>
                    <td>{e.location}</td>
                    <td>{e.startDate}{e.endDate && e.endDate !== e.startDate ? ` → ${e.endDate}` : ''}</td>
                    <td className="num">{e.tournaments}</td>
                    <td className="num">{e.players}</td>
                    <td>{e.public ? <span className="tag ok">public</span> : <span className="tag">privé</span>}</td>
                    <td>{levelLabel[e.level]}</td>
                    <td className="muted">{fmtDate(e.updatedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
