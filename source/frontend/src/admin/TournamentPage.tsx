import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import type { ChessEvent, Derived, Tournament } from '../types'
import { Tabs } from '../components/Tabs'
import { useEvent } from './EventPage'
import { PlayersTab } from './t/Players'
import { RegistrationsTab } from './t/Registrations'
import { CheckInTab } from './t/CheckIn'
import { RoundsTab } from './t/Rounds'
import { StandingsTab, CrosstableTab } from './t/Standings'
import { PrizesTab } from './t/Prizes'
import { SettingsTab } from './t/Settings'
import { LogTab } from './t/Log'
import { NormsTab } from './t/Norms'
import { KnockoutTab } from './t/Knockout'
import { TeamRoundsTab, TeamStandingsTab, TeamsTab } from './t/Teams'
import { kindLabel } from '../util'

export interface TProps {
  ev: ChessEvent
  t: Tournament
  d: Derived
  setEv: (e: ChessEvent) => void
  canEdit: boolean
  /** May enter results (arbiters, and result-entry accounts). */
  canResults?: boolean
  /** Round count being edited in Réglages (shown in the header before saving). */
  setDraftRounds?: (n: number | null) => void
}

export function TournamentPage() {
  const { eid = '', tid = '', tab: tabParam, sub } = useParams()
  const { ev, setEv, error } = useEvent(eid)
  const tab = tabParam ?? (ev?.level === 'results' ? 'rondes' : 'joueurs')
  const [draftRounds, setDraftRounds] = useState<number | null>(null)
  useEffect(() => { if (tab !== 'reglages') setDraftRounds(null) }, [tab, tid])
  if (error) return <div className="page"><p className="error">{error}</p></div>
  if (!ev) return <div className="page muted">Chargement…</div>
  const t = ev.tournaments.find(x => String(x.id) === tid)
  if (!t) return <div className="page"><p className="error">Tournoi introuvable.</p><Link to={`/admin/e/${eid}`}>← Retour</Link></div>
  const d = ev.derived[t.id]
  const canEdit = ev.level !== 'viewer' && ev.level !== 'results'
  const canResults = ev.level !== 'viewer'
  const props: TProps = { ev, t, d, setEv, canEdit, canResults, setDraftRounds }
  const base = `/admin/e/${eid}/t/${tid}`
  const confirmed = t.players.filter(p => p.status === 'confirmed')
  const draft = t.kind === 'swiss' && draftRounds && draftRounds >= t.rounds.length ? draftRounds : null
  const planned = draft ?? d.plannedRounds

  return (
    <div className="page">
      <div className="page-head hero">
        <div>
          <Link to={`/admin/e/${eid}`} className="muted small">← {ev.name}</Link>
          <h1>{t.name}</h1>
          <p className="muted small">
            {kindLabel(t)}
            {' · '}{confirmed.length} joueur{confirmed.length > 1 ? 's' : ''}
            {t.fideMode && ' · mode FIDE'}
            {ev.public && <> · <a href={`/e/${ev.slug}/t/${t.id}`} target="_blank" rel="noreferrer">page publique ↗</a></>}
          </p>
        </div>
        <div className="progress" title={draft && draft !== d.plannedRounds ? 'nombre de rondes modifié, non enregistré' : `${t.rounds.length} ronde(s) appariée(s) sur ${planned}`}>
          {Array.from({ length: Math.max(planned, t.rounds.length) }, (_, i) => {
            const r = t.rounds[i]
            const done = r && r.boards.every(b => b.black === 0 || b.white === 0 || b.result)
            const cls = !r ? (draft && draft !== d.plannedRounds ? 'unsaved' : '') : !r.published ? 'draft' : done ? 'done' : 'live'
            return <Link key={i} to={`${base}/rondes/${i + 1}`} className={`dot ${cls}`} title={r ? (r.published ? 'publiée' : 'non publiée') : 'à venir'}>{i + 1}</Link>
          })}
        </div>
      </div>
      <Tabs tabs={[
        { to: base, label: 'Joueurs', count: confirmed.length },
        { to: `${base}/inscriptions`, label: 'Inscriptions', count: d.pendingRegistrations },
        { to: `${base}/equipes`, label: 'Équipes', count: t.teams?.length, hidden: !t.team?.boards },
        { to: `${base}/pointage`, label: 'Pointage', hidden: !t.checkIn },
        { to: `${base}/rondes`, label: 'Rondes' },
        { to: `${base}/classement`, label: 'Classement' },
        { to: `${base}/grille`, label: 'Grille', hidden: t.kind === 'knockout' || !!t.team?.boards },
        { to: `${base}/prix`, label: 'Prix', hidden: t.kind === 'knockout' || !!t.team?.boards },
        { to: `${base}/normes`, label: 'Normes', hidden: t.ratingKind !== 'standard' || t.kind === 'knockout' || !!t.team?.boards },
        { to: `${base}/reglages`, label: 'Réglages' },
        { to: `${base}/journal`, label: 'Journal', count: t.log.length },
      ]} />
      {tab === 'inscriptions' ? <RegistrationsTab {...props} />
        : tab === 'pointage' ? <CheckInTab {...props} />
        : tab === 'equipes' ? <TeamsTab {...props} />
        : tab === 'rondes' && t.team?.boards ? <TeamRoundsTab {...props} selected={sub ? +sub : undefined} />
        : tab === 'classement' && t.team?.boards ? <TeamStandingsTab {...props} />
        : tab === 'rondes' ? (t.kind === 'knockout' ? <KnockoutTab {...props} selected={sub ? +sub : undefined} /> : <RoundsTab {...props} selected={sub ? +sub : undefined} />)
        : tab === 'classement' ? <StandingsTab {...props} />
        : tab === 'grille' ? <CrosstableTab {...props} />
        : tab === 'prix' ? <PrizesTab {...props} />
        : tab === 'normes' ? <NormsTab {...props} />
        : tab === 'reglages' ? <SettingsTab {...props} />
        : tab === 'journal' ? <LogTab {...props} />
        : <PlayersTab {...props} />}
    </div>
  )
}
