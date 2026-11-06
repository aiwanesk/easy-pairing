import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'
import { api } from '../api'
import { PublicShell } from './PublicHome'
import { usePublicEvent } from './PublicEvent'

type Found = Awaited<ReturnType<typeof api.publicLookup>>[number]

export function RegisterPage() {
  const { slug = '', tid = '' } = useParams()
  const { ev, error } = usePublicEvent(slug)
  const t = ev?.tournaments.find(x => String(x.id) === tid)
  const [f, setF] = useState({ lastName: '', firstName: '', email: '', phone: '', birthDate: '', sex: '', club: '', fed: 'FRA', ffeId: '', fideId: '', rating: 0, website: '' })
  const [q, setQ] = useState('')
  const [found, setFound] = useState<Found[]>([])
  const [chosen, setChosen] = useState<Found | null>(null)
  const [done, setDone] = useState<{ status: string; player: string } | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const seq = useRef(0)

  useEffect(() => {
    if (q.trim().length < 3 || chosen) { setFound([]); return }
    const my = ++seq.current
    const timer = window.setTimeout(() => api.publicLookup(q).then(r => { if (my === seq.current) setFound(r) }).catch(() => {}), 300)
    return () => window.clearTimeout(timer)
  }, [q, chosen])

  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setF(x => ({ ...x, [k]: k === 'rating' ? +e.target.value : e.target.value }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      setDone(await api.register(slug, +tid, chosen ? { ...f, ffeId: chosen.ffeId } : f))
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2))
    } finally {
      setBusy(false)
    }
  }

  if (error) return (
    <PublicShell>
      <div className="card">
        <h2>Inscription indisponible</h2>
        <p className="error">{error}</p>
        <p className="muted small">Organisateur : rendez l'événement public et ouvrez les inscriptions du tournoi (onglet Inscriptions du tournoi).</p>
      </div>
    </PublicShell>
  )
  if (ev && !t) return <PublicShell><p className="error">Tournoi introuvable dans cet événement.</p></PublicShell>
  if (!ev || !t) return <PublicShell><p className="muted">Chargement…</p></PublicShell>
  const reg = t.registration

  return (
    <PublicShell>
      <Link to={`/e/${slug}/t/${t.id}`} className="muted small">← {ev.name} — {t.name}</Link>
      <h1>Inscription — {t.name}</h1>
      <p className="muted">{[t.timeControl, `${t.plannedRounds} rondes`, reg.fee ? `droits ${reg.fee}` : '', reg.maxPlayers ? `${reg.count}/${reg.maxPlayers} inscrits` : `${reg.count} inscrits`,
        reg.deadline ? `clôture ${reg.deadline.replace('T', ' ')}` : ''].filter(Boolean).join(' · ')}</p>
      {reg.message && <div className="banner">{reg.message}</div>}
      {done ? (
        <div className="card success-card">
          <h2>{done.status === 'confirmed' ? 'Inscription confirmée' : 'Inscription enregistrée'}</h2>
          <p><b>{done.player}</b> {done.status === 'confirmed' ? 'est inscrit(e).' : 'est en attente de validation par l\'organisateur.'}</p>
          <Link to={`/e/${slug}/t/${t.id}/joueurs`}>Voir la liste des joueurs →</Link>
        </div>
      ) : !reg.available ? (
        <div className="card"><p className="warn">Inscription impossible : {reg.closedReason}.</p></div>
      ) : (
        <form className="card stack-form wide" onSubmit={submit}>
          <h2>Licencié FFE ?</h2>
          {chosen ? (
            <div className="found">
              <div><div className="found-name">{chosen.lastName} {chosen.firstName}</div><div className="muted small">{chosen.ffeId} · {chosen.club} · {chosen.category} · Elo {chosen.standard || '—'}</div></div>
              <button type="button" className="ghost" onClick={() => { setChosen(null); setQ('') }}>Changer</button>
            </div>
          ) : (
            <>
              <input value={q} onChange={e => setQ(e.target.value)} placeholder="Votre nom ou n° de licence (ex. K59144)" />
              {found.length > 0 && (
                <ul className="pick-list">
                  {found.map(p => <li key={p.ffeId}><button type="button" onClick={() => setChosen(p)}><b>{p.lastName} {p.firstName}</b> <span className="muted small">{p.ffeId} · {p.club} · {p.category}</span></button></li>)}
                </ul>
              )}
              <p className="muted small">Sinon, remplissez vos informations :</p>
              <div className="form-grid two">
                <label>Nom<input value={f.lastName} onChange={set('lastName')} required={!chosen} /></label>
                <label>Prénom<input value={f.firstName} onChange={set('firstName')} required={!chosen} /></label>
                <label>Club<input value={f.club} onChange={set('club')} /></label>
                <label>Fédération<input value={f.fed} onChange={set('fed')} maxLength={3} /></label>
                <label>Code FIDE (facultatif)<input value={f.fideId} onChange={set('fideId')} inputMode="numeric" /></label>
                <label>Elo estimé<input type="number" min={0} max={3000} value={f.rating || ''} onChange={set('rating')} /></label>
                <label>Date de naissance<input value={f.birthDate} onChange={set('birthDate')} placeholder="AAAA/MM/JJ" /></label>
                <label>Sexe<select value={f.sex} onChange={set('sex')}><option value="">—</option><option value="m">M</option><option value="f">F</option></select></label>
              </div>
            </>
          )}
          <h2>Contact</h2>
          <div className="form-grid two">
            <label>E-mail<input type="email" value={f.email} onChange={set('email')} required /></label>
            <label>Téléphone<input value={f.phone} onChange={set('phone')} /></label>
          </div>
          <input className="hp" tabIndex={-1} autoComplete="off" value={f.website} onChange={set('website')} aria-hidden="true" />
          <p className="muted tiny">Vos coordonnées ne sont visibles que des organisateurs et arbitres du tournoi.</p>
          {err && <p className="error">{err}</p>}
          <button className="primary" disabled={busy}>{busy ? 'Envoi…' : 'Envoyer mon inscription'}</button>
        </form>
      )}
    </PublicShell>
  )
}
