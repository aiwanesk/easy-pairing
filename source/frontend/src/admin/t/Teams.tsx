import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api } from '../../api'
import type { Player, Team, TeamMatch, TeamStanding } from '../../types'
import { fmtPts, fullName } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

const blankTeam = (): Team => ({ id: 0, name: '', club: '', fed: 'FRA', captain: '', players: [], withdrawnFrom: 0 })

/** Teams: names and board order; players come from the Joueurs tab. */
export function TeamsTab({ ev, t, setEv, canEdit }: TProps) {
  const [teams, setTeams] = useState<Team[]>(t.teams)
  useEffect(() => { setTeams(t.teams) }, [t.teams])
  const run = useAction()
  const byId = new Map(t.players.map(p => [p.id, p]))
  const assigned = new Set(teams.flatMap(x => x.players))
  const free = t.players.filter(p => p.status === 'confirmed' && !assigned.has(p.id)).sort((a, b) => b.rating - a.rating)
  const dirty = JSON.stringify(teams) !== JSON.stringify(t.teams)

  const upd = (i: number, f: (x: Team) => Team) => setTeams(ts => ts.map((x, j) => (j === i ? f(x) : x)))
  const move = (i: number, k: number, d: -1 | 1) => upd(i, x => {
    const ps = [...x.players]; const j = k + d
    if (j < 0 || j >= ps.length) return x
    ;[ps[k], ps[j]] = [ps[j], ps[k]]
    return { ...x, players: ps }
  })
  const save = async () => {
    const r = await run(() => api.setTeams(ev.id, t.id, teams), 'Équipes enregistrées')
    if (r) setEv(r)
  }
  // Groups the free players by club into new teams, board order by rating.
  const byClub = () => {
    const clubs = new Map<string, Player[]>()
    for (const p of free) {
      const c = p.club?.trim() || 'Sans club'
      clubs.set(c, [...(clubs.get(c) ?? []), p])
    }
    setTeams(ts => [...ts, ...[...clubs.entries()].map(([club, ps]) => ({ ...blankTeam(), name: club, club, players: ps.map(p => p.id) }))])
  }

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <h2>Équipes <span className="muted">({teams.length}) · {t.team.boards} échiquier(s) par match</span></h2>
          {canEdit && (
            <div className="inline">
              <button onClick={() => setTeams(ts => [...ts, { ...blankTeam(), name: `Équipe ${ts.length + 1}` }])}>Ajouter une équipe</button>
              {free.length > 0 && <button className="ghost" onClick={byClub} title="Crée une équipe par club avec les joueurs non affectés">Créer les équipes par club</button>}
              <button className="primary" disabled={!dirty} onClick={save}>{dirty ? 'Enregistrer' : 'Enregistré'}</button>
            </div>
          )}
        </div>
        <p className="muted small">Ajoutez d'abord les joueurs (onglet Joueurs), puis répartissez-les. L'ordre des joueurs est l'ordre des échiquiers ; les remplaçants viennent après. Un joueur absent à une ronde (Rondes → présents) est remplacé automatiquement par le suivant.</p>
        <div className="team-grid">
          {teams.map((tm, i) => (
            <div key={tm.id || `n${i}`} className="team-card">
              <div className="inline">
                <input value={tm.name} disabled={!canEdit} onChange={e => upd(i, x => ({ ...x, name: e.target.value }))} placeholder="Nom de l'équipe" />
                <input className="narrow" value={tm.fed} disabled={!canEdit} maxLength={3} onChange={e => upd(i, x => ({ ...x, fed: e.target.value.toUpperCase() }))} />
                {canEdit && !tm.id && <button className="ghost danger small" onClick={() => setTeams(ts => ts.filter((_, j) => j !== i))}>✕</button>}
              </div>
              <input value={tm.captain} disabled={!canEdit} onChange={e => upd(i, x => ({ ...x, captain: e.target.value }))} placeholder="Capitaine" className="small-input" />
              <ol className="team-players">
                {tm.players.map((pid, k) => {
                  const p = byId.get(pid)
                  return (
                    <li key={pid} className={k >= t.team.boards ? 'reserve' : ''}>
                      <span>{p ? fullName(p) : pid} <span className="muted small">{p?.rating || ''}</span>{k >= t.team.boards && <span className="muted small"> remplaçant</span>}</span>
                      {canEdit && <span>
                        <button className="ghost small" onClick={() => move(i, k, -1)}>↑</button>
                        <button className="ghost small" onClick={() => move(i, k, 1)}>↓</button>
                        <button className="ghost danger small" onClick={() => upd(i, x => ({ ...x, players: x.players.filter(y => y !== pid) }))}>✕</button>
                      </span>}
                    </li>
                  )
                })}
              </ol>
              {canEdit && free.length > 0 && (
                <select value="" onChange={e => e.target.value && upd(i, x => ({ ...x, players: [...x.players, +e.target.value] }))}>
                  <option value="">Ajouter un joueur…</option>
                  {free.map(p => <option key={p.id} value={p.id}>{fullName(p)} ({p.rating || '—'}) {p.club}</option>)}
                </select>
              )}
              {tm.players.length < t.team.boards && <p className="warn small">Il manque {t.team.boards - tm.players.length} joueur(s) pour remplir les échiquiers.</p>}
            </div>
          ))}
        </div>
        {free.length > 0 && <p className="muted small">{free.length} joueur(s) sans équipe.</p>}
      </section>
    </div>
  )
}

const RES: [string, string][] = [['1-0', '1-0'], ['1/2', '½'], ['0-1', '0-1']]

function gp(t: TProps['t'], m: TeamMatch): [number, number, boolean] {
  if (m.away === 0) return [0, 0, true]
  if (m.forfeit === 'home') return [0, t.team.boards, true]
  if (m.forfeit === 'away') return [t.team.boards, 0, true]
  if (m.forfeit === 'both') return [0, 0, true]
  let h = 0, a = 0, done = true
  for (const g of m.games) {
    const w = g.result === '1-0' || g.result === '+-' ? 1 : g.result === '1/2' ? 0.5 : 0
    const b = g.result === '0-1' || g.result === '-+' ? 1 : g.result === '1/2' ? 0.5 : 0
    if (!g.result) done = false
    const homeWhite = (g.board % 2 === 1) !== t.team.firstWhiteEven
    if (homeWhite) { h += w; a += b } else { h += b; a += w }
  }
  return [h, a, done]
}

/** Team rounds: matches with boards, lineups and results. */
export function TeamRoundsTab(props: TProps & { selected?: number }) {
  const { ev, t, d, setEv, canEdit, canResults, selected } = props
  const run = useAction()
  const toast = useAction()
  const [busy, setBusy] = useState(false)
  const [order, setOrder] = useState('rating')
  const paired = t.rounds.length
  const current = selected && selected >= 1 && selected <= paired ? selected : paired
  const round = t.rounds[current - 1]
  const teamName = (id: number) => t.teams.find(x => x.id === id)?.name ?? '—'
  const byId = new Map(t.players.map(p => [p.id, p]))
  const pname = (id: number) => { const p = byId.get(id); return p ? fullName(p) : <span className="muted">— (vide)</span> }

  const pair = async () => {
    setBusy(true)
    const r = await run(() => api.pair(ev.id, t.id, t.kind === 'roundrobin' ? order : undefined), t.kind === 'roundrobin' ? 'Calendrier créé' : `Ronde ${paired + 1} appariée`)
    setBusy(false)
    if (r) setEv(r)
  }
  const setRes = async (m: TeamMatch, b: number, res: string, cur: string) => {
    const r = await run(() => api.teamResult(ev.id, t.id, current, m.number, b, cur === res && canEdit ? '' : res))
    if (r) setEv(r)
  }
  const forfeit = async (m: TeamMatch, who: string) => {
    const r = await run(() => api.teamForfeit(ev.id, t.id, current, m.number, who))
    if (r) setEv(r)
  }
  const lineup = async (m: TeamMatch, side: 'home' | 'away', board: number, pid: number) => {
    const homeWhite = (b: number) => (b % 2 === 1) !== t.team.firstWhiteEven
    const cur = m.games.map(g => ((side === 'home') === homeWhite(g.board) ? g.white : g.black))
    cur[board - 1] = pid
    const r = await toast(() => api.teamLineup(ev.id, t.id, current, m.number, side, cur), 'Composition modifiée')
    if (r) setEv(r)
  }

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <div className="round-picker">
            {t.rounds.map(r => <Link key={r.number} className={`button ${r.number === current ? 'on' : ''}`} to={`/admin/e/${ev.id}/t/${t.id}/rondes/${r.number}`}>R{r.number}</Link>)}
            {paired === 0 && <span className="muted">Aucune ronde.</span>}
          </div>
          {canEdit && (t.kind === 'swiss' ? paired < t.totalRounds : paired === 0) && (
            <div className="actions">
              {t.kind === 'roundrobin' && <select value={order} onChange={e => setOrder(e.target.value)}><option value="rating">Numéros par moyenne Elo</option><option value="lots">Tirage au sort</option></select>}
              <button className="primary" disabled={!d.canPair || busy} onClick={pair}>{t.kind === 'roundrobin' ? `Créer le calendrier (${d.plannedRounds} rondes)` : `Apparier la ronde ${paired + 1}`}</button>
            </div>
          )}
        </div>
        {!d.canPair && d.canPairReason && <p className="muted small">{d.canPairReason}</p>}
      </section>
      {round && (
        <section className="card print-area">
          <h2>{t.name} — ronde {round.number}</h2>
          <div className="stack">
            {(round.teamMatches ?? []).map(m => {
              const [h, a, done] = gp(t, m)
              const editable = canEdit && (t.kind === 'roundrobin' || current === paired)
              const resultsOnly = !canEdit && !!canResults
              if (m.away === 0) return <div key={m.number} className="team-match bye"><b>{teamName(m.home)}</b> exempte</div>
              const homeTeam = t.teams.find(x => x.id === m.home)!, awayTeam = t.teams.find(x => x.id === m.away)!
              const anyResult = m.games.some(g => g.result && g.white && g.black)
              return (
                <div key={m.number} className={`team-match ${done ? 'decided' : ''}`}>
                  <div className="team-match-head">
                    <span className="muted small">Match {m.number}</span>
                    <b>{teamName(m.home)}</b><span className="team-score">{fmtPts(h)} – {fmtPts(a)}</span><b>{teamName(m.away)}</b>
                    {editable && (
                      <select value={m.forfeit ?? ''} onChange={e => forfeit(m, e.target.value)} title="Forfait d'équipe">
                        <option value="">Match joué</option><option value="home">Forfait {teamName(m.home)}</option><option value="away">Forfait {teamName(m.away)}</option><option value="both">Double forfait</option>
                      </select>
                    )}
                  </div>
                  {!m.forfeit && (
                    <table className="table compact">
                      <tbody>
                        {m.games.map(g => {
                          const homeWhite = (g.board % 2 === 1) !== t.team.firstWhiteEven
                          const hp = homeWhite ? g.white : g.black, ap = homeWhite ? g.black : g.white
                          const cell = (side: 'home' | 'away', pid: number, team: typeof homeTeam) => editable && !anyResult ? (
                            <select value={pid} onChange={e => lineup(m, side, g.board, +e.target.value)}>
                              <option value={0}>— vide (forfait) —</option>
                              {team.players.map(id => <option key={id} value={id}>{byId.get(id) ? fullName(byId.get(id)!) : id}</option>)}
                            </select>
                          ) : pname(pid)
                          return (
                            <tr key={g.board}>
                              <td className="num muted">{g.board}</td>
                              <td>{homeWhite ? '○' : '●'} {cell('home', hp, homeTeam)}</td>
                              <td className="center">
                                {(editable || (resultsOnly && !g.result)) && g.white && g.black ? (
                                  <div className={`res-group ${resultsOnly ? 'big' : ''}`}>
                                    {RES.map(([v, l]) => <button key={v} type="button" className={`res ${g.result === v ? 'on' : ''}`} onClick={() => setRes(m, g.board, v, g.result)}>{l}</button>)}
                                  </div>
                                ) : <b>{g.result === '1/2' ? '½-½' : g.result || '…'}</b>}
                              </td>
                              <td>{homeWhite ? '●' : '○'} {cell('away', ap, awayTeam)}</td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  )}
                </div>
              )
            })}
          </div>
          <p className="muted small">○ Blancs · ● Noirs. La composition se modifie tant qu'aucun résultat n'est saisi sur le match ; un échiquier vide est perdu par forfait.</p>
        </section>
      )}
    </div>
  )
}

export function TeamStandingsTab({ ev, t }: TProps) {
  const [rows, setRows] = useState<TeamStanding[]>([])
  useEffect(() => { api.standings(ev.id, t.id).then(r => setRows(r.teams ?? [])).catch(() => {}) }, [ev, t])
  return <TeamStandingsTable rows={rows} teams={t.teams} title={`${t.name} — classement par équipes`} />
}

export function TeamStandingsTable({ rows, teams, title }: { rows: TeamStanding[]; teams: Team[]; title: string }) {
  const name = (id: number) => teams.find(x => x.id === id)?.name ?? id
  return (
    <section className="card print-area">
      <div className="card-head"><h2>{title}</h2><button className="ghost no-print" onClick={() => window.print()}>Imprimer</button></div>
      <table className="table">
        <thead><tr><th className="num">Pl.</th><th>Équipe</th><th className="num">J</th><th className="num">G</th><th className="num">N</th><th className="num">P</th><th className="num">Pts match</th><th className="num">Pts parties</th><th className="num">Bu</th></tr></thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={r.teamId}>
              <td className="num strong">{i > 0 && rows[i - 1].rank === r.rank ? '' : r.rank}</td>
              <td>{name(r.teamId)}</td>
              <td className="num">{r.played}</td><td className="num">{r.won}</td><td className="num">{r.drawn}</td><td className="num">{r.lost}</td>
              <td className="num strong">{fmtPts(r.mp)}</td><td className="num">{fmtPts(r.gp)}</td><td className="num">{fmtPts(r.bh)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
