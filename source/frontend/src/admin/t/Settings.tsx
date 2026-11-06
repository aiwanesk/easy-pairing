import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../../api'
import type { TournamentSettings } from '../../types'
import { fullName, tournamentSettings } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'
import { TieBreakEditor } from '../../components/TieBreakEditor'
import { PRIZE_SPLITS, TEAM_POINTS } from '../../util'

export function SettingsTab(props: TProps) {
  const { ev, t, setEv, canEdit, setDraftRounds } = props
  const pick = (): TournamentSettings => tournamentSettings(t)
  const [s, setS] = useState<TournamentSettings>(pick)
  const run = useAction()
  const nav = useNavigate()
  const started = t.rounds.length > 0
  const tooFew = s.kind === 'swiss' && s.totalRounds < t.rounds.length
  useEffect(() => { setDraftRounds?.(s.kind === 'swiss' ? s.totalRounds : null) }, [s.totalRounds, s.kind, setDraftRounds])
  useEffect(() => () => setDraftRounds?.(null), [setDraftRounds])

  const set = (k: keyof TournamentSettings) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
    const target = e.target as HTMLInputElement
    const num = ['totalRounds', 'pointsWin', 'pointsDraw', 'pointsPab', 'unratedValue'].includes(k)
    setS(x => ({ ...x, [k]: target.type === 'checkbox' ? target.checked : num ? +target.value : target.value }))
  }
  const setReg = (k: keyof TournamentSettings['registration']) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const target = e.target as HTMLInputElement
    const num = ['maxPlayers', 'minRating', 'maxRating', 'fee'].includes(k)
    setS(x => ({ ...x, registration: { ...x.registration, [k]: target.type === 'checkbox' ? target.checked : num ? +target.value : target.value } }))
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const r = await run(() => api.updateTournament(ev.id, t.id, s), 'Réglages enregistrés')
    if (r) setEv(r)
  }
  const remove = async () => {
    if (!confirm(`Supprimer définitivement le tournoi « ${t.name} » ?`)) return
    const r = await run(() => api.deleteTournament(ev.id, t.id), 'Tournoi supprimé')
    if (r) { setEv(r); nav(`/admin/e/${ev.id}`) }
  }

  return (
    <div className="stack">
      <form onSubmit={save} className="stack">
        <fieldset disabled={!canEdit} className="stack">
          <section className="card">
            <h2>Système</h2>
            <div className="form-grid">
              <label className="span-2">Nom<input value={s.name} onChange={set('name')} required /></label>
              <label>Type
                <select value={s.kind} onChange={set('kind')} disabled={started}><option value="swiss">Suisse</option><option value="roundrobin">Toutes-rondes</option><option value="knockout">Coupe</option></select>
              </label>
              {s.kind === 'swiss' ? (
                <label>Rondes<input type="number" min={1} max={30} value={s.totalRounds || ''} onChange={set('totalRounds')} className={tooFew ? 'invalid' : ''} />
                  {tooFew && <span className="error small">Impossible : {t.rounds.length} ronde(s) déjà appariée(s). Minimum {t.rounds.length}.</span>}</label>
              ) : <span />}
              {s.kind === 'swiss' ? (
                <>
                  <label>Système d'appariement
                    <select value={s.system} onChange={set('system')}><option value="dutch">Hollandais (FIDE C.04.3)</option><option value="burstein">Burstein</option></select>
                  </label>
                  <label>Accélération
                    <select value={s.acceleration} onChange={set('acceleration')}><option value="none">Aucune</option><option value="baku">Baku (FIDE C.04.5)</option></select>
                  </label>
                  <label>Couleur du n°1 à la ronde 1
                    <select value={s.initialColor} onChange={set('initialColor')} disabled={started}><option value="white">Blancs</option><option value="black">Noirs</option></select>
                  </label>
                </>
              ) : s.kind === 'knockout' ? (
                <>
                  <label>Parties par match<input type="number" min={1} max={8} value={s.ko.games} disabled={started}
                    onChange={e => setS(x => ({ ...x, ko: { ...x.ko, games: +e.target.value } }))} /></label>
                  <label>Parties en finale (0 = idem)<input type="number" min={0} max={8} value={s.ko.final || ''}
                    onChange={e => setS(x => ({ ...x, ko: { ...x.ko, final: +e.target.value } }))} /></label>
                  <label>Têtes de série
                    <select value={s.ko.seeding} disabled={started} onChange={e => setS(x => ({ ...x, ko: { ...x.ko, seeding: e.target.value as 'rating' | 'lots' } }))}>
                      <option value="rating">Par Elo (1 contre le dernier)</option><option value="lots">Tirage au sort</option>
                    </select>
                  </label>
                  <label className="span-4">Départages en cas d'égalité, dans l'ordre (ex. rapid:2, blitz:2, armageddon)
                    <input value={s.ko.tieBreaks.join(', ')} onChange={e => setS(x => ({ ...x, ko: { ...x.ko, tieBreaks: e.target.value.split(',').map(v => v.trim()).filter(Boolean) } }))} />
                  </label>
                </>
              ) : (
                <>
                  <label className="check"><input type="checkbox" checked={s.doubleRoundRobin} onChange={set('doubleRoundRobin')} disabled={started} /> Double (aller-retour)</label>
                  <label className="check span-2"><input type="checkbox" checked={s.reverseLastRounds} onChange={set('reverseLastRounds')} disabled={started || !s.doubleRoundRobin} /> Inverser les deux dernières rondes du premier cycle (recommandation FIDE)</label>
                </>
              )}
              <label>Elo utilisé (cadence)
                <select value={s.ratingKind} onChange={set('ratingKind')}><option value="standard">Lent</option><option value="rapid">Rapide</option><option value="blitz">Blitz</option></select>
                {!started && s.ratingKind !== t.ratingKind && <span className="warn small">Les Elo {s.ratingKind === 'standard' ? 'lents' : s.ratingKind === 'rapid' ? 'rapides' : 'blitz'} des joueurs seront rechargés (base FFE, puis FIDE).</span>}
              </label>
              <label>Cadence<input value={s.timeControl} onChange={set('timeControl')} placeholder="90 min + 30 s/coup" /></label>
              <label>Victoire<input type="number" step={0.5} min={0.5} value={s.pointsWin} onChange={set('pointsWin')} disabled={started} /></label>
              <label>Nulle<input type="number" step={0.5} min={0} value={s.pointsDraw} onChange={set('pointsDraw')} disabled={started} /></label>
              {s.kind === 'swiss' && <label>Exempt (PAB)<input type="number" step={0.5} min={0} value={s.pointsPab} onChange={set('pointsPab')} disabled={started} /></label>}
              <label>Arbitre principal<input value={s.chiefArbiter} onChange={set('chiefArbiter')} /></label>
              <label>Fédération<input value={s.federation} onChange={set('federation')} maxLength={3} placeholder="FRA" /></label>
              <label className="check span-4"><input type="checkbox" checked={s.fideMode} onChange={set('fideMode')} disabled={started && t.fideMode} />
                Mode FIDE : refuse les actions interdites par les règlements (appariement manuel différent du moteur, correction des rondes anciennes…). Définitif une fois commencé.</label>
            </div>
          </section>

          {s.kind !== 'knockout' && (
            <section className="card">
              <h2>Par équipes</h2>
              <div className="form-grid">
                <label>Échiquiers par match (0 = tournoi individuel)<input type="number" min={0} max={20} value={s.team.boards} disabled={started}
                  onChange={e => setS(x => ({ ...x, team: { ...x.team, boards: +e.target.value } }))} /></label>
                {s.team.boards > 0 && <>
                  <label>Points de match
                    <select value={TEAM_POINTS.find(p => JSON.stringify(p.points) === JSON.stringify(s.team.points))?.id ?? ''}
                      onChange={e => { const p = TEAM_POINTS.find(x => x.id === e.target.value); if (p) setS(x => ({ ...x, team: { ...x.team, points: { ...p.points } } })) }}>
                      {TEAM_POINTS.map(p => <option key={p.id} value={p.id}>{p.label}</option>)}
                      {!TEAM_POINTS.some(p => JSON.stringify(p.points) === JSON.stringify(s.team.points)) && <option value="">Personnalisé</option>}
                    </select>
                  </label>
                  <label className="check"><input type="checkbox" checked={s.team.gamePointsFirst} onChange={e => setS(x => ({ ...x, team: { ...x.team, gamePointsFirst: e.target.checked } }))} /> Classer d'abord aux points de parties</label>
                  <label className="check"><input type="checkbox" checked={s.team.firstWhiteEven} disabled={started} onChange={e => setS(x => ({ ...x, team: { ...x.team, firstWhiteEven: e.target.checked } }))} /> L'équipe citée en premier a les Blancs aux échiquiers pairs</label>
                </>}
              </div>
            </section>
          )}

          <section className="card">
            <h2>Départages</h2>
            <p className="muted small">Ordre d'application (glisser avec les flèches). {started && 'Les changements après le début sont journalisés.'}</p>
            <TieBreakEditor value={s.tieBreaks} onChange={tieBreaks => setS(x => ({ ...x, tieBreaks }))} kind={s.kind} />
            <div className="tb-editor">
              <label>Elo des non classés pour ARO/TPR (0 = ignorés)<input type="number" value={s.unratedValue || ''} onChange={set('unratedValue')} placeholder="1400" /></label>
            </div>
          </section>

          <section className="card">
            <h2>Prix : répartition entre ex aequo</h2>
            <div className="split-choice">
              {PRIZE_SPLITS.map(o => (
                <label key={o.id} className={`choice ${s.prizeSplit === o.id ? 'on' : ''}`}>
                  <input type="radio" name="prizeSplit" checked={s.prizeSplit === o.id} onChange={() => setS(x => ({ ...x, prizeSplit: o.id }))} />
                  <b>{o.label}</b><span className="muted small">{o.help}</span>
                </label>
              ))}
            </div>
            <p className="muted small">La liste des prix et leurs conditions se règlent dans l'onglet Prix.</p>
          </section>

          <section className="card">
            <h2>Pointage et kiosque</h2>
            <div className="form-grid">
              <label className="check span-4"><input type="checkbox" checked={s.checkIn} onChange={set('checkIn')} disabled={started} /> Pointage obligatoire : seuls les joueurs pointés sont appariés à la ronde 1</label>
              <label>Code du kiosque (4 à 8 chiffres)<input value={s.kioskPin} onChange={set('kioskPin')} inputMode="numeric" placeholder="désactivé" /></label>
              <label className="check span-3"><input type="checkbox" checked={s.kioskResults} onChange={set('kioskResults')} /> Les joueurs saisissent leurs résultats au kiosque (sans écraser un résultat existant)</label>
            </div>
          </section>

          <section className="card">
            <h2>Inscriptions en ligne</h2>
            <div className="form-grid">
              <label className="check span-4"><input type="checkbox" checked={s.registration.open} onChange={setReg('open')} /> Formulaire d'inscription ouvert (événement public requis)</label>
              <label>Date limite<input type="datetime-local" value={s.registration.deadline} onChange={setReg('deadline')} /></label>
              <label>Places (0 = illimité)<input type="number" min={0} value={s.registration.maxPlayers || ''} onChange={setReg('maxPlayers')} /></label>
              <label>Elo minimum<input type="number" min={0} value={s.registration.minRating || ''} onChange={setReg('minRating')} /></label>
              <label>Elo maximum<input type="number" min={0} value={s.registration.maxRating || ''} onChange={setReg('maxRating')} /></label>
              <label>Droits d'inscription<input type="number" step={0.5} min={0} value={s.registration.fee || ''} onChange={setReg('fee')} /></label>
              <label className="check"><input type="checkbox" checked={s.registration.autoConfirm} onChange={setReg('autoConfirm')} /> Validation automatique</label>
              <label className="check span-2"><input type="checkbox" checked={s.registration.showList} onChange={setReg('showList')} /> Publier la liste des inscrits (y compris en attente)</label>
              <label className="span-4">Message affiché sur le formulaire<textarea rows={2} value={s.registration.message} onChange={setReg('message')} /></label>
            </div>
          </section>

          <section className="card">
            <h2>Notes de l'arbitre</h2>
            <textarea rows={3} value={s.arbiterNotes} onChange={set('arbiterNotes')} />
          </section>
          {canEdit && <div className="actions sticky-actions"><button className="primary" disabled={tooFew}>Enregistrer les réglages</button></div>}
        </fieldset>
      </form>

      {t.kind === 'swiss' && <ForbiddenPairs {...props} />}

      <section className="card">
        <h2>Exports</h2>
        <div className="actions">
          <a className="button" href={api.exportUrl(ev.id, t.id, 'trf')}>TRF (FIDE)</a>
          <a className="button" href={api.exportUrl(ev.id, t.id, 'pgn')}>PGN (toutes rondes)</a>
          <a className="button" href={api.exportUrl(ev.id, t.id, 'players.csv')}>Joueurs CSV</a>
          <a className="button" href={api.exportUrl(ev.id, t.id, 'standings.csv')}>Classement CSV</a>
        </div>
        <p className="muted small">Le TRF contient le journal du tournoi en commentaires ; il se recharge dans bbpPairings, JaVaFo ou Swiss-Manager.</p>
      </section>

      {(ev.level === 'owner' || ev.level === 'admin') && (
        <section className="card danger-zone"><h2>Zone dangereuse</h2><button className="danger" onClick={remove}>Supprimer le tournoi</button></section>
      )}
    </div>
  )
}

function ForbiddenPairs({ ev, t, setEv, canEdit }: TProps) {
  const [groups, setGroups] = useState<number[][]>(t.forbiddenGroups.map(g => [...g]))
  const [pick, setPick] = useState<number[]>([])
  const run = useAction()
  const byId = new Map(t.players.map(p => [p.id, p]))
  const players = t.players.filter(p => p.status === 'confirmed').sort((a, b) => fullName(a).localeCompare(fullName(b), 'fr'))
  const locked = t.fideMode && t.rounds.length >= 1

  const save = async (gs: number[][]) => {
    const r = await run(() => api.setForbidden(ev.id, t.id, gs), 'Appariements interdits enregistrés')
    if (r) { setEv(r); setGroups(gs) }
  }

  return (
    <section className="card">
      <h2>Appariements interdits</h2>
      <p className="muted small">Groupes de joueurs qui ne doivent jamais se rencontrer (famille, même club en début de tournoi…), transmis à bbpPairings.</p>
      {groups.length === 0 ? <p className="muted">Aucun.</p> : (
        <ul>{groups.map((g, i) => (
          <li key={i}>{g.map(id => fullName(byId.get(id)!)).join(' · ')}
            {canEdit && !locked && <button className="ghost danger small" onClick={() => save(groups.filter((_, j) => j !== i))}>✕</button>}</li>
        ))}</ul>
      )}
      {canEdit && !locked && (
        <div className="inline mt">
          <select multiple size={6} value={pick.map(String)} onChange={e => setPick(Array.from(e.target.selectedOptions).map(o => +o.value))} className="multi">
            {players.map(p => <option key={p.id} value={p.id}>{fullName(p)}</option>)}
          </select>
          <button disabled={pick.length < 2} onClick={() => { save([...groups, pick]); setPick([]) }}>Ajouter le groupe ({pick.length})</button>
        </div>
      )}
      {locked && <p className="muted small">Mode FIDE : fixés après la première ronde.</p>}
    </section>
  )
}
