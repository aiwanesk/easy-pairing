import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api'
import { TieBreakEditor, TIEBREAK_PRESETS } from '../components/TieBreakEditor'
import type { ChessEvent, EventSettings, Permission, PrizeSplit, TournamentSettings } from '../types'
import { PRIZE_SPLITS, TEAM_POINTS, kindLabel, levelLabel, useLiveUpdates } from '../util'
import { Tabs } from '../components/Tabs'
import { useAction } from '../components/Toast'
import { AuditTable } from './Audit'

export function useEvent(eid: string) {
  const [ev, setEv] = useState<ChessEvent | null>(null)
  const [error, setError] = useState('')
  const load = useCallback(() => {
    api.event(eid).then(x => { setEv(x); setError('') }).catch(e => setError(e.message))
  }, [eid])
  useEffect(load, [load])
  useLiveUpdates(api.streamUrl(eid), load)
  return { ev, setEv, error, reload: load }
}

export function EventPage() {
  const { eid = '', tab = 'tournois' } = useParams()
  const { ev, setEv, error } = useEvent(eid)
  if (error) return <div className="page"><div className="card"><p className="error">{error}</p><Link to="/admin">← Événements</Link></div></div>
  if (!ev) return <div className="page muted">Chargement…</div>
  const owner = ev.level === 'owner' || ev.level === 'admin'
  return (
    <div className="page">
      <div className="page-head hero">
        <div>
          <Link to="/admin" className="muted small">← Événements</Link>
          <h1>{ev.name}</h1>
          <p className="muted small">
            {[ev.location, ev.startDate, levelLabel[ev.level]].filter(Boolean).join(' · ')}
            {ev.public && <> · <a href={`/e/${ev.slug}`} target="_blank" rel="noreferrer">page publique ↗</a></>}
          </p>
        </div>
      </div>
      <Tabs tabs={[
        { to: `/admin/e/${eid}`, label: 'Tournois', count: ev.tournaments.length },
        { to: `/admin/e/${eid}/reglages`, label: 'Réglages' },
        { to: `/admin/e/${eid}/acces`, label: 'Accès' },
        { to: `/admin/e/${eid}/journal`, label: 'Journal', hidden: !owner },
      ]} />
      {tab === 'reglages' ? <EventSettingsTab ev={ev} setEv={setEv} />
        : tab === 'acces' ? <AccessTab ev={ev} />
        : tab === 'journal' && owner ? <section className="card"><AuditTable eventId={ev.id} /></section>
        : <TournamentsTab ev={ev} setEv={setEv} />}
    </div>
  )
}

function TournamentsTab({ ev, setEv }: { ev: ChessEvent; setEv: (e: ChessEvent) => void }) {
  const [name, setName] = useState('')
  const [kind, setKind] = useState<TournamentSettings['kind']>('swiss')
  const [rounds, setRounds] = useState(7)
  const [ratingKind, setRatingKind] = useState<TournamentSettings['ratingKind']>('standard')
  const [tieBreaks, setTieBreaks] = useState<string[]>(TIEBREAK_PRESETS[0].tbs)
  const [prizeSplit, setPrizeSplit] = useState<PrizeSplit>('place')
  const [timeControl, setTimeControl] = useState('')
  const [koGames, setKoGames] = useState(2)
  const [teamBoards, setTeamBoards] = useState(0)
  const [teamPoints, setTeamPoints] = useState('fide')
  const run = useAction()
  const changeKind = (k: TournamentSettings['kind']) => {
    setKind(k)
    setTieBreaks([...(TIEBREAK_PRESETS.find(p => p.kind === k)?.tbs ?? [])])
  }
  const nav = useNavigate()
  const canEdit = ev.level !== 'viewer' && ev.level !== 'results'

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    const r = await run(() => api.createTournament(ev.id, { name, kind, totalRounds: rounds, ratingKind, system: 'dutch', initialColor: 'white', tieBreaks, prizeSplit, timeControl, ...(kind === 'knockout' ? { ko: { games: koGames, tieBreaks: ['rapid:2', 'blitz:2', 'armageddon'], seeding: 'rating' as const, final: 0 } } : {}), ...(teamBoards > 0 && kind !== 'knockout' ? { team: { boards: teamBoards, points: TEAM_POINTS.find(x => x.id === teamPoints)!.points, gamePointsFirst: false, firstWhiteEven: false } } : {}) }))
    if (r) {
      setEv(r)
      const t = r.tournaments[r.tournaments.length - 1]
      nav(`/admin/e/${ev.id}/t/${t.id}`)
    }
  }

  return (
    <div className="stack">
      <section className="card">
        <h2>Tournois</h2>
        {ev.tournaments.length === 0 ? <p className="muted">Aucun tournoi.</p> : (
          <div className="cards">
            {ev.tournaments.map(t => {
              const d = ev.derived[t.id]
              const active = t.players.filter(p => p.status === 'confirmed').length
              return (
                <Link key={t.id} to={`/admin/e/${ev.id}/t/${t.id}`} className="tcard">
                  <div className="tcard-title">{t.name}</div>
                  <div className="muted small">{kindLabel(t)} · {({ standard: 'lente', rapid: 'rapide', blitz: 'blitz' } as Record<string, string>)[t.ratingKind]}</div>
                  <div className="tcard-stats">
                    <span><b>{active}</b> joueurs</span>
                    <span><b>{t.rounds.length}</b>/{d?.plannedRounds ?? '?'} rondes</span>
                    {d?.pendingRegistrations ? <span className="tag warn">{d.pendingRegistrations} inscription(s) à valider</span> : null}
                    {t.fideMode && <span className="tag">mode FIDE</span>}
                  </div>
                </Link>
              )
            })}
          </div>
        )}
      </section>
      {canEdit && (
        <section className="card">
          <h2>Nouveau tournoi</h2>
          <form className="stack" onSubmit={create}>
            <div className="form-grid">
              <label className="span-2">Nom<input value={name} onChange={e => setName(e.target.value)} placeholder="Open A" required /></label>
              <label>Type
                <select value={kind} onChange={e => changeKind(e.target.value as TournamentSettings['kind'])}>
                  <option value="swiss">Système suisse</option>
                  <option value="roundrobin">Toutes-rondes / fermé (Berger)</option>
                  <option value="knockout">Coupe (élimination directe)</option>
                </select>
              </label>
              {kind === 'swiss'
                ? <label>Rondes<input type="number" min={1} max={30} value={rounds} onChange={e => setRounds(+e.target.value)} /></label>
                : kind === 'knockout'
                  ? <label>Parties par match<input type="number" min={1} max={8} value={koGames} onChange={e => setKoGames(+e.target.value)} /></label>
                  : <span />}
              <label>Elo utilisé
                <select value={ratingKind} onChange={e => setRatingKind(e.target.value as TournamentSettings['ratingKind'])}>
                  <option value="standard">Lent (Elo standard)</option><option value="rapid">Rapide</option><option value="blitz">Blitz</option>
                </select>
              </label>
              <label className="span-2">Cadence<input value={timeControl} onChange={e => setTimeControl(e.target.value)} placeholder="90 min + 30 s/coup" /></label>
              {kind !== 'knockout' && (
                <label>Par équipes : échiquiers par match (0 = individuel)<input type="number" min={0} max={20} value={teamBoards} onChange={e => setTeamBoards(+e.target.value)} /></label>
              )}
              {kind !== 'knockout' && teamBoards > 0 && (
                <label>Points de match
                  <select value={teamPoints} onChange={e => setTeamPoints(e.target.value)}>{TEAM_POINTS.map(x => <option key={x.id} value={x.id}>{x.label}</option>)}</select>
                </label>
              )}
            </div>
            <div className="two-cols">
              {kind === 'knockout' ? (
                <div>
                  <h3>Départage des matchs nuls</h3>
                  <p className="muted small">Par défaut : 2 parties rapides, puis 2 blitz, puis Armageddon (nulle = qualification des Noirs). Réglable ensuite dans Réglages.</p>
                </div>
              ) : (
                <div>
                  <h3>Départages</h3>
                  <TieBreakEditor value={tieBreaks} onChange={setTieBreaks} kind={kind} />
                </div>
              )}
              <div>
                <h3>Prix : répartition entre ex aequo</h3>
                <div className="split-choice vertical">
                  {PRIZE_SPLITS.map(o => (
                    <label key={o.id} className={`choice ${prizeSplit === o.id ? 'on' : ''}`}>
                      <input type="radio" name="newSplit" checked={prizeSplit === o.id} onChange={() => setPrizeSplit(o.id)} />
                      <b>{o.label}</b><span className="muted small">{o.help}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
            <div className="actions"><button className="primary" disabled={!name.trim()}>Créer le tournoi</button></div>
          </form>
        </section>
      )}
    </div>
  )
}

function EventSettingsTab({ ev, setEv }: { ev: ChessEvent; setEv: (e: ChessEvent) => void }) {
  const [s, setS] = useState<EventSettings>({
    name: ev.name, slug: ev.slug, location: ev.location, startDate: ev.startDate, endDate: ev.endDate,
    organizer: ev.organizer, contact: ev.contact, public: ev.public, message: ev.message,
  })
  const run = useAction()
  const nav = useNavigate()
  const owner = ev.level === 'owner' || ev.level === 'admin'
  const set = (k: keyof EventSettings) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setS(x => ({ ...x, [k]: e.target.type === 'checkbox' ? (e.target as HTMLInputElement).checked : e.target.value }))

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const r = await run(() => api.updateEvent(ev.id, s), 'Événement enregistré')
    if (r) setEv(r)
  }
  const remove = async () => {
    if (!confirm(`Supprimer définitivement « ${ev.name} » et tous ses tournois ?`)) return
    const ok = await run(() => api.deleteEvent(ev.id).then(() => true), 'Événement supprimé')
    if (ok) nav('/admin')
  }

  return (
    <div className="stack">
      <section className="card">
        <h2>Événement</h2>
        <form className="form-grid" onSubmit={save}>
          <label className="span-2">Nom<input value={s.name} onChange={set('name')} required disabled={!owner} /></label>
          <label className="span-2">Adresse publique<div className="prefix-input"><span>/e/</span><input value={s.slug} onChange={set('slug')} disabled={!owner} /></div></label>
          <label>Lieu<input value={s.location} onChange={set('location')} disabled={!owner} /></label>
          <label>Début<input type="date" value={s.startDate} onChange={set('startDate')} disabled={!owner} /></label>
          <label>Fin<input type="date" value={s.endDate} onChange={set('endDate')} disabled={!owner} /></label>
          <label>Organisateur<input value={s.organizer} onChange={set('organizer')} disabled={!owner} /></label>
          <label className="span-2">Contact (affiché publiquement)<input value={s.contact} onChange={set('contact')} disabled={!owner} /></label>
          <label className="span-2">Message d'information (bandeau public)<input value={s.message} onChange={set('message')} disabled={!owner} /></label>
          <label className="check span-4"><input type="checkbox" checked={s.public} onChange={set('public')} disabled={!owner} />
            Événement public : pages d'appariements et de classement, inscriptions en ligne, écrans et kiosques accessibles sans compte</label>
          {owner && <div className="actions span-4"><button className="primary">Enregistrer</button></div>}
        </form>
      </section>
      {owner && (<>
        <section className="card">
          <h2>Travailler hors ligne / en ligne</h2>
          <p className="muted small">
            Exportez l'événement dans un fichier pour le reprendre sur un autre ordinateur (tournoi hors ligne en salle, sur le réseau local),
            puis réimportez-le ici pour remettre la plateforme à jour. Le remplacement écrase l'événement actuel par le contenu du fichier.
          </p>
          <div className="actions">
            <a className="button" href={api.exportEventUrl(ev.id)}>Exporter (fichier)</a>
            {owner && (
              <label className="button ghost">Remplacer depuis un fichier…
                <input type="file" accept=".json,application/json" hidden onChange={async e => {
                  const f = e.target.files?.[0]
                  e.target.value = ''
                  if (!f || !confirm(`Remplacer « ${ev.name} » par le contenu de ${f.name} ?`)) return
                  const r = await run(() => api.importEvent(f, true), 'Événement mis à jour depuis le fichier')
                  if (r) setEv(r)
                }} />
              </label>
            )}
          </div>
        </section>
        <section className="card danger-zone">
          <h2>Zone dangereuse</h2>
          <button className="danger" onClick={remove}>Supprimer l'événement</button>
        </section>
      </>)}
    </div>
  )
}

function AccessTab({ ev }: { ev: ChessEvent }) {
  const [perms, setPerms] = useState<Permission[]>([])
  const [dir, setDir] = useState<{ id: number; name: string; login: string }[]>([])
  const [who, setWho] = useState(0)
  const [level, setLevel] = useState('arbiter')
  const run = useAction()
  const owner = ev.level === 'owner' || ev.level === 'admin'

  useEffect(() => {
    api.permissions(ev.id).then(setPerms).catch(() => {})
    if (owner) api.directory().then(setDir).catch(() => {})
  }, [ev.id, owner])

  const set = async (accountId: number, lvl: string) => {
    const r = await run(() => api.setPermission(ev.id, accountId, lvl), 'Accès mis à jour')
    if (r) setPerms(r)
  }

  return (
    <section className="card">
      <h2>Qui peut gérer cet événement</h2>
      <p className="muted small">
        Propriétaire : tout, y compris les accès et la suppression. Arbitre : joueurs, appariements, résultats, réglages des tournois.
        Lecture seule : consultation. Les administrateurs ont accès à tous les événements.
      </p>
      <table className="table">
        <thead><tr><th>Nom</th><th>Identifiant</th><th>Niveau</th><th /></tr></thead>
        <tbody>
          {perms.map(p => (
            <tr key={p.accountId}>
              <td>{p.name}</td>
              <td className="mono small">{p.login}</td>
              <td>
                {owner ? (
                  <select value={p.level} onChange={e => set(p.accountId, e.target.value)}>
                    <option value="owner">Propriétaire</option><option value="arbiter">Arbitre</option><option value="results">Saisie des résultats</option><option value="viewer">Lecture seule</option>
                  </select>
                ) : levelLabel[p.level]}
              </td>
              <td className="row-actions">{owner && <button className="ghost danger" onClick={() => set(p.accountId, '')}>Retirer</button>}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {owner && (
        <div className="inline mt">
          <select value={who} onChange={e => setWho(+e.target.value)}>
            <option value={0}>Ajouter un compte…</option>
            {dir.filter(d => !perms.some(p => p.accountId === d.id)).map(d => <option key={d.id} value={d.id}>{d.name} ({d.login})</option>)}
          </select>
          <select value={level} onChange={e => setLevel(e.target.value)}>
            <option value="arbiter">Arbitre</option><option value="results">Saisie des résultats</option><option value="viewer">Lecture seule</option><option value="owner">Propriétaire</option>
          </select>
          <button className="primary" disabled={!who} onClick={() => { set(who, level); setWho(0) }}>Donner l'accès</button>
        </div>
      )}
    </section>
  )
}
