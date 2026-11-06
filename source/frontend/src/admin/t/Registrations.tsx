import { useEffect, useState } from 'react'
import { api } from '../../api'
import { useSession } from '../../App'
import type { NetworkInfo, Player, Tournament } from '../../types'
import { eventSettings, fmtDate, fullName, tournamentSettings } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

export function RegistrationsTab({ ev, t, setEv, canEdit }: TProps) {
  const run = useAction()
  const pending = t.players.filter(p => p.status === 'pending')
  const refused = t.players.filter(p => p.status === 'refused')
  const online = t.players.filter(p => p.source === 'online')
  const { account } = useSession()
  const path = `/e/${ev.slug}/inscription/${t.id}`
  const reg = t.registration
  const [net, setNet] = useState<NetworkInfo | null>(null)
  useEffect(() => { api.network().then(setNet).catch(() => {}) }, [])
  const links = [...(net?.publicUrl ? [`${net.publicUrl}${path}`] : []), `${window.location.origin}${path}`, ...(net?.lan && !net.server ? net.addresses.map(a => `${a}${path}`) : [])]
    .filter((u, i, a) => a.indexOf(u) === i)
  const closedReason = registrationClosedReason(t)
  const ready = ev.public && reg.open && !closedReason

  const makePublic = async () => {
    const r = await run(() => api.updateEvent(ev.id, { ...eventSettings(ev), public: true }), 'Événement public')
    if (r) setEv(r)
  }
  const toggleOpen = async (open: boolean) => {
    const s = tournamentSettings(t)
    const r = await run(() => api.updateTournament(ev.id, t.id, { ...s, registration: { ...s.registration, open } }), open ? 'Inscriptions ouvertes' : 'Inscriptions fermées')
    if (r) setEv(r)
  }
  const toggleLan = async (lan: boolean) => {
    const r = await run(() => api.setNetwork(lan), lan ? 'Accès réseau local activé' : 'Accès réseau local coupé')
    if (r) setNet(r)
    else api.network().then(setNet).catch(() => {})
  }

  const setStatus = async (p: Player, status: string) => {
    const r = await run(() => api.setStatus(ev.id, t.id, p.id, status), status === 'confirmed' ? `${fullName(p)} confirmé` : 'Statut mis à jour')
    if (r) setEv(r)
  }

  const row = (p: Player, actions: React.ReactNode) => (
    <tr key={p.id}>
      <td><span className="title">{p.title} </span>{fullName(p)}</td>
      <td className="num">{p.rating || '—'}<span className="rtype">{p.ratingType}</span></td>
      <td>{p.club}</td>
      <td className="mono small">{p.ffeId || p.fideId}</td>
      <td className="small">{p.email}{p.phone && <><br />{p.phone}</>}</td>
      <td className="muted small">{fmtDate(p.registeredAt)}</td>
      <td className="row-actions">{canEdit && actions}</td>
    </tr>
  )

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <h2>Inscriptions en ligne</h2>
          <span className={`pill ${ready ? 'ok' : 'muted'}`}>{ready ? 'ouvertes' : 'fermées'}</span>
        </div>
        <ul className="checklist">
          <li className={ev.public ? 'ok' : 'ko'}>
            {ev.public ? '✓ Événement public' : '✗ Événement non public : le lien affiche « événement introuvable ».'}
            {canEdit && !ev.public && (ev.level === 'owner' || ev.level === 'admin') && <button className="primary small" onClick={makePublic}>Rendre l'événement public</button>}
          </li>
          <li className={reg.open ? 'ok' : 'ko'}>
            {reg.open ? '✓ Inscriptions en ligne activées' : '✗ Inscriptions en ligne désactivées pour ce tournoi.'}
            {canEdit && <button className={reg.open ? 'ghost small' : 'primary small'} onClick={() => toggleOpen(!reg.open)}>{reg.open ? 'Fermer' : 'Ouvrir les inscriptions'}</button>}
          </li>
          {reg.open && closedReason && <li className="ko">✗ Formulaire fermé : {closedReason}. Modifiez la date limite ou le nombre de places dans les Réglages.</li>}
          <li className={net?.lan ? 'ok' : 'warn'}>
            {net?.server ? '✓ Plateforme en ligne' : net?.lan ? '✓ Accessible depuis les autres appareils du réseau local' : '⚠ Accessible seulement depuis cet ordinateur (adresse localhost).'}
            {account?.role === 'admin' && net && !net.forced && (
              <button className={net.lan ? 'ghost small' : 'primary small'} onClick={() => toggleLan(!net.lan)}>{net.lan ? 'Couper l\'accès réseau' : 'Ouvrir au réseau local'}</button>
            )}
            {net?.error && <div className="error small">{net.error}</div>}
          </li>
        </ul>
        <div className="links">
          {links.map(u => (
            <p className="small" key={u}><a href={u} target="_blank" rel="noreferrer" className="mono">{u}</a>
              <button className="ghost small" onClick={() => navigator.clipboard?.writeText(u)}>Copier</button></p>
          ))}
        </div>
        <p className="muted small">
          Le lien localhost ne marche que sur cet ordinateur. Les liens « réseau local » marchent pour les téléphones et tablettes connectés au même Wi-Fi.
          Pour des inscriptions depuis Internet, il faut publier le serveur (redirection de port ou tunnel).
        </p>
        <p className="muted small">
          {reg.maxPlayers ? `${reg.maxPlayers} places · ` : ''}{reg.deadline ? `clôture ${reg.deadline.replace('T', ' ')} · ` : ''}
          {reg.fee ? `droits ${reg.fee} · ` : ''}{reg.autoConfirm ? 'validation automatique' : 'validation par l\'arbitre'} · {online.length} inscription(s) en ligne au total.
          Autres paramètres dans l'onglet Réglages.
        </p>
      </section>
      <section className="card">
        <h2>À valider <span className="muted">({pending.length})</span></h2>
        {pending.length === 0 ? <p className="muted">Aucune inscription en attente.</p> : (
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Nom</th><th className="num">Elo</th><th>Club</th><th>Licence / FIDE</th><th>Contact</th><th>Reçue</th><th /></tr></thead>
              <tbody>{pending.map(p => row(p, <>
                <button className="primary small" onClick={() => setStatus(p, 'confirmed')}>Confirmer</button>
                <button className="ghost danger" onClick={() => setStatus(p, 'refused')}>Refuser</button>
              </>))}</tbody>
            </table>
          </div>
        )}
      </section>
      {refused.length > 0 && (
        <section className="card">
          <h2>Refusées <span className="muted">({refused.length})</span></h2>
          <table className="table"><tbody>{refused.map(p => row(p, <button className="ghost" onClick={() => setStatus(p, 'pending')}>Remettre en attente</button>))}</tbody></table>
        </section>
      )}
    </div>
  )
}

// Same rules as the server (registrationState), for the admin checklist.
function registrationClosedReason(t: Tournament): string {
  const reg = t.registration
  const count = t.players.filter(p => p.status !== 'refused').length
  if (t.kind === 'roundrobin' && t.rounds.length > 0) return 'tournoi commencé'
  if (reg.maxPlayers > 0 && count >= reg.maxPlayers) return 'tournoi complet'
  if (reg.deadline) {
    const d = new Date(reg.deadline.length === 10 ? `${reg.deadline}T23:59:59` : reg.deadline)
    if (!isNaN(d.getTime()) && d.getTime() < Date.now()) return 'date limite dépassée'
  }
  if (t.kind === 'swiss' && t.rounds.length >= t.totalRounds) return 'tournoi terminé'
  return ''
}
