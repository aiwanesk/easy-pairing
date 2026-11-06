import { useEffect, useState } from 'react'
import { api } from '../api'
import type { AuditEntry } from '../types'
import { fmtDate } from '../util'

const actionLabels: Record<string, string> = {
  setup: 'Installation', login: 'Connexion', 'login.failed': 'Échec de connexion', logout: 'Déconnexion',
  'account.create': 'Compte créé', 'account.update': 'Compte modifié', 'account.password': 'Mot de passe', 'account.delete': 'Compte supprimé',
  'event.create': 'Événement créé', 'event.update': 'Événement modifié', 'event.delete': 'Événement supprimé', 'event.permission': 'Accès',
  'tournament.create': 'Tournoi créé', 'tournament.update': 'Réglages du tournoi', 'tournament.delete': 'Tournoi supprimé',
  'tournament.forbidden': 'Appariements interdits', 'tournament.prizes': 'Prix',
  'player.add': 'Joueur ajouté', 'player.bulk': 'Ajout en masse', 'player.update': 'Joueur modifié', 'player.delete': 'Joueur supprimé',
  'player.checkin': 'Pointage', 'player.checkin.all': 'Pointage (tous)',
  'registration.online': 'Inscription en ligne', 'registration.confirmed': 'Inscription validée', 'registration.refused': 'Inscription refusée', 'registration.pending': 'Inscription en attente',
  'round.pair': 'Appariement', 'round.delete': 'Ronde supprimée', 'round.reset': 'Calendrier supprimé', 'round.publish': 'Ronde publiée', 'round.unpublish': 'Ronde dépubliée',
  'result.set': 'Résultat', 'pairing.manual': 'Appariement manuel', 'kiosk.checkin': 'Pointage (kiosque)', 'kiosk.result': 'Résultat (kiosque)',
  'ffe.download': 'Base FFE téléchargée', 'ffe.upload': 'Base FFE importée', 'system.engine': 'Moteur réinstallé',
}

export function AuditTable({ eventId }: { eventId?: string }) {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [total, setTotal] = useState(0)
  const [q, setQ] = useState('')
  const [action, setAction] = useState('')
  const [offset, setOffset] = useState(0)
  const [error, setError] = useState('')
  const limit = 100

  useEffect(() => {
    const timer = window.setTimeout(() => {
      api.audit({ event: eventId ?? '', q, action, limit, offset })
        .then(r => { setEntries(r.entries); setTotal(r.total); setError('') })
        .catch(e => setError(e.message))
    }, 200)
    return () => window.clearTimeout(timer)
  }, [eventId, q, action, offset])

  return (
    <div>
      <div className="inline mb">
        <input className="search" placeholder="Rechercher (joueur, compte, détail…)" value={q} onChange={e => { setQ(e.target.value); setOffset(0) }} />
        <select value={action} onChange={e => { setAction(e.target.value); setOffset(0) }}>
          <option value="">Toutes les actions</option>
          {['login', 'account', 'event', 'tournament', 'player', 'registration', 'round', 'result', 'pairing', 'kiosk', 'ffe', 'system'].map(a => <option key={a} value={a}>{a}</option>)}
        </select>
        <span className="muted small">{total} entrée(s)</span>
      </div>
      {error && <p className="error">{error}</p>}
      <div className="table-wrap">
        <table className="table compact">
          <thead><tr><th>Date</th><th>Compte</th><th>Action</th><th>Objet</th><th>Détails</th><th>IP</th></tr></thead>
          <tbody>
            {entries.map(e => (
              <tr key={e.id} className={e.action === 'login.failed' ? 'warn-row' : ''}>
                <td className="nowrap muted">{fmtDate(e.at)}</td>
                <td>{e.accountName || <span className="muted">public</span>}</td>
                <td className="nowrap">{actionLabels[e.action] ?? e.action}</td>
                <td>{e.target}</td>
                <td className="small">{e.details}</td>
                <td className="mono tiny muted">{e.ip}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {total > limit && (
        <div className="inline mt">
          <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>← Plus récents</button>
          <span className="muted small">{offset + 1}–{Math.min(total, offset + limit)} sur {total}</span>
          <button disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>Plus anciens →</button>
        </div>
      )}
    </div>
  )
}

export function AuditPage() {
  return (
    <div className="page">
      <div className="page-head"><h1>Journal d'audit</h1></div>
      <section className="card">
        <p className="muted small">Toutes les actions (connexions, modifications, appariements, résultats, inscriptions, kiosques) sont enregistrées avec leur auteur et leur adresse IP.</p>
        <AuditTable />
      </section>
    </div>
  )
}
