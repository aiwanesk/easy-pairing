import { useMemo, useState } from 'react'
import { api } from '../../api'
import { fmtDate, fullName } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

export function CheckInTab({ ev, t, d, setEv, canEdit }: TProps) {
  const [filter, setFilter] = useState('')
  const [only, setOnly] = useState<'all' | 'missing' | 'present'>('all')
  const run = useAction()
  const confirmed = t.players.filter(p => p.status === 'confirmed' && !p.withdrawnFrom)
  const locked = t.rounds.length > 0
  const kioskUrl = `${window.location.origin}/e/${ev.slug}/kiosque/${t.id}`

  const rows = useMemo(() => {
    const f = filter.trim().toLowerCase()
    return confirmed
      .filter(p => !f || fullName(p).toLowerCase().includes(f) || p.club.toLowerCase().includes(f))
      .filter(p => only === 'all' || (only === 'present' ? p.checkedIn : !p.checkedIn))
      .sort((a, b) => fullName(a).localeCompare(fullName(b), 'fr'))
  }, [confirmed, filter, only])

  const toggle = async (id: number, v: boolean) => {
    const r = await run(() => api.checkIn(ev.id, t.id, id, v))
    if (r) setEv(r)
  }
  const all = async (v: boolean) => {
    if (!confirm(v ? 'Pointer tous les joueurs ?' : 'Dépointer tous les joueurs ?')) return
    const r = await run(() => api.checkInAll(ev.id, t.id, v))
    if (r) setEv(r)
  }

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <h2>Pointage <span className="muted">{d.checkedIn} / {confirmed.length}</span></h2>
          <div className="inline">
            <input className="search" placeholder="Nom ou club…" value={filter} onChange={e => setFilter(e.target.value)} autoFocus />
            <div className="segmented">
              {(['all', 'missing', 'present'] as const).map(k => (
                <button key={k} className={only === k ? 'on' : ''} onClick={() => setOnly(k)}>{k === 'all' ? 'Tous' : k === 'missing' ? 'Non pointés' : 'Pointés'}</button>
              ))}
            </div>
          </div>
        </div>
        <div className="progress-bar"><div style={{ width: `${confirmed.length ? (100 * d.checkedIn) / confirmed.length : 0}%` }} /></div>
        {locked
          ? <p className="warn small">La première ronde est appariée : le pointage est clos.</p>
          : <p className="muted small">Seuls les joueurs pointés seront appariés à la ronde 1 {t.kind === 'roundrobin' ? '(et inscrits au calendrier du toutes-rondes)' : '(les autres sont absents, 0 point)'}.</p>}
        <div className="checkin-grid">
          {rows.map(p => (
            <button key={p.id} className={`checkin ${p.checkedIn ? 'on' : ''}`} disabled={!canEdit || locked} onClick={() => toggle(p.id, !p.checkedIn)}>
              <span className="checkin-box">{p.checkedIn ? '✓' : ''}</span>
              <span>
                <b>{fullName(p)}</b> <span className="muted small">{p.rating || ''}</span>
                <br /><span className="muted tiny">{p.club}{p.checkedIn && p.checkedInAt ? ` · ${fmtDate(p.checkedInAt)}` : ''}</span>
              </span>
            </button>
          ))}
        </div>
        {canEdit && !locked && (
          <div className="actions mt">
            <button onClick={() => all(true)}>Tout pointer</button>
            <button className="ghost danger" onClick={() => all(false)}>Tout dépointer</button>
          </div>
        )}
      </section>
      <section className="card">
        <h2>Kiosque de pointage</h2>
        {t.kioskPin ? (
          <p className="small">Ouvrez <a href={kioskUrl} target="_blank" rel="noreferrer" className="mono">{kioskUrl}</a> sur une tablette à l'accueil
            (code <b className="mono">{t.kioskPin}</b>) : les joueurs se pointent eux-mêmes{t.kioskResults ? ' et saisissent leurs résultats' : ''}.
            {!ev.public && <span className="warn"> L'événement doit être public.</span>}</p>
        ) : <p className="muted small">Définissez un code de kiosque dans les Réglages pour que les joueurs se pointent eux-mêmes sur une tablette.</p>}
      </section>
    </div>
  )
}
