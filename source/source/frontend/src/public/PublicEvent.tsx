import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, NavLink, useParams } from 'react-router'
import { api } from '../api'
import type { PublicEvent, PublicPlayer, PublicTournament, Round } from '../types'
import { KO_STAGES, byeLabel, fmtPts, fullName, koRoundName, resultLabel, scoresBefore, useLiveUpdates } from '../util'
import { Crosstable, StandingsTable } from '../admin/t/Standings'
import { PublicShell } from './PublicHome'

export function usePublicEvent(slug: string) {
  const [ev, setEv] = useState<PublicEvent | null>(null)
  const [error, setError] = useState('')
  const load = useCallback(() => { api.publicEvent(slug).then(x => { setEv(x); setError('') }).catch(e => setError(e.message)) }, [slug])
  useEffect(load, [load])
  useLiveUpdates(api.publicStreamUrl(slug), load)
  return { ev, error }
}

export function PublicEventPage() {
  const { slug = '', tid, view } = useParams()
  const { ev, error } = usePublicEvent(slug)
  if (error) return <PublicShell><div className="card"><p className="error">{error}</p><Link to="/">← Accueil</Link></div></PublicShell>
  if (!ev) return <PublicShell><p className="muted">Chargement…</p></PublicShell>
  const t = ev.tournaments.find(x => String(x.id) === tid) ?? (ev.tournaments.length === 1 ? ev.tournaments[0] : undefined)

  return (
    <PublicShell>
      <div className="page-head hero">
        <div>
          <h1>{ev.name}</h1>
          <p className="muted small">{[ev.location, ev.startDate && (ev.endDate && ev.endDate !== ev.startDate ? `${ev.startDate} → ${ev.endDate}` : ev.startDate), ev.organizer].filter(Boolean).join(' · ')}</p>
        </div>
        {ev.contact && <p className="muted small">Contact : {ev.contact}</p>}
      </div>
      {ev.message && <div className="banner">{ev.message}</div>}
      {ev.tournaments.length > 1 && (
        <nav className="tabs">
          {ev.tournaments.map(x => <NavLink key={x.id} to={`/e/${slug}/t/${x.id}`} className={() => (t?.id === x.id ? 'active' : '')}>{x.name}</NavLink>)}
        </nav>
      )}
      {t ? <PublicTournamentView slug={slug} t={t} view={view} /> : (
        <div className="cards">
          {ev.tournaments.map(x => (
            <Link key={x.id} to={`/e/${slug}/t/${x.id}`} className="tcard">
              <div className="tcard-title">{x.name}</div>
              <div className="muted small">{x.kind === 'swiss' ? 'Système suisse' : 'Toutes-rondes'} · {x.timeControl}</div>
              <div className="tcard-stats"><span><b>{x.players.filter(p => p.status === 'confirmed').length}</b> joueurs</span><span>ronde <b>{x.lastPublished}</b>/{x.plannedRounds}</span>
                {x.registration.available && <span className="tag ok">inscriptions ouvertes</span>}</div>
            </Link>
          ))}
        </div>
      )}
    </PublicShell>
  )
}

function PublicTournamentView({ slug, t, view }: { slug: string; t: PublicTournament; view?: string }) {
  const base = `/e/${slug}/t/${t.id}`
  const v = view ?? (t.rounds.length ? 'appariements' : 'joueurs')
  const players = useMemo(() => new Map(t.players.map(p => [p.id, p])), [t])
  return (
    <div className="stack">
      <div className="subtabs">
        {[['appariements', 'Appariements'], ['joueurs', 'Joueurs'], ['classement', 'Classement'], ['grille', 'Grille']].map(([k, l]) => (
          <NavLink key={k} to={`${base}/${k}`} className={() => (v === k ? 'active' : '')}>{l}</NavLink>
        ))}
        {t.registration.available && <Link to={`/e/${slug}/inscription/${t.id}`} className="button primary small">S'inscrire</Link>}
        <Link to={`/e/${slug}/ecran?t=${t.id}`} className="muted small" target="_blank">Mode écran ↗</Link>
      </div>
      {t.kind === 'knockout' && v !== 'joueurs' ? <PublicCup t={t} />
        : v === 'joueurs' ? <PublicPlayers t={t} />
        : v === 'classement' ? <section className="card"><h2>Classement {t.lastPublished ? `après la ronde ${t.lastPublished}` : ''}</h2>
            {t.standings.length ? <StandingsTable rows={t.standings} players={players} tieBreaks={t.tieBreaks} /> : <p className="muted">Pas encore de classement.</p>}</section>
        : v === 'grille' ? <section className="card"><h2>Grille</h2><Crosstable rounds={t.rounds} standings={t.standings} players={players} tieBreaks={t.tieBreaks} /></section>
        : <PublicPairings t={t} />}
    </div>
  )
}

function PublicPlayers({ t }: { t: PublicTournament }) {
  const confirmed = t.players.filter(p => p.status === 'confirmed').sort((a, b) => fullName(a).localeCompare(fullName(b), 'fr'))
  const pending = t.players.filter(p => p.status === 'pending')
  return (
    <section className="card">
      <h2>Joueurs ({confirmed.length}){t.registration.maxPlayers ? ` / ${t.registration.maxPlayers} places` : ''}</h2>
      <table className="table">
        <thead><tr><th>Nom</th><th className="num">Elo</th><th>Féd.</th><th>Cat.</th><th>Club</th>{t.started && <th className="num">Pts</th>}</tr></thead>
        <tbody>
          {confirmed.map(p => (
            <tr key={p.id} className={p.withdrawnFrom ? 'withdrawn' : ''}>
              <td><span className="title">{p.title} </span>{fullName(p)}</td>
              <td className="num">{p.rating || ''}<span className="rtype">{p.rating ? p.ratingType : ''}</span></td>
              <td>{p.fed}</td><td className="muted">{p.category}</td><td>{p.club}</td>
              {t.started && <td className="num strong">{fmtPts(p.points)}</td>}
            </tr>
          ))}
        </tbody>
      </table>
      {pending.length > 0 && <p className="muted small">En attente de validation : {pending.map(p => fullName(p)).join(', ')}</p>}
    </section>
  )
}

export function PairingsTable({ t, round, players, big }: { t: PublicTournament; round: Round; players: Map<number, PublicPlayer>; big?: boolean }) {
  const before = scoresBefore(t.rounds, round.number, t.pointsWin, t.pointsDraw, t.pointsWin, t.kind)
  const name = (id: number) => { const p = players.get(id); return p ? <><span className="title">{p.title} </span>{fullName(p)} <span className="muted small">{p.rating || ''}</span></> : null }
  return (
    <table className={`table pairings ${big ? 'big' : ''}`}>
      <thead><tr><th className="num">Éch.</th><th>Blancs</th><th className="num">Pts</th><th className="center">Résultat</th><th className="num">Pts</th><th>Noirs</th></tr></thead>
      <tbody>
        {round.boards.map(b => (
          <tr key={b.number}>
            <td className="num strong">{b.number}</td>
            <td>{name(b.white)}</td>
            <td className="num">{fmtPts(before.get(b.white))}</td>
            <td className="center strong">{b.black === 0 || b.white === 0 ? <span className="tag ok">{resultLabel[b.result]}</span> : resultLabel[b.result]}</td>
            <td className="num">{b.black ? fmtPts(before.get(b.black)) : ''}</td>
            <td>{b.black ? name(b.black) : ''}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function PublicCup({ t }: { t: PublicTournament }) {
  const players = new Map(t.players.map(p => [p.id, p]))
  const name = (id: number) => { const p = players.get(id); return p ? fullName(p) : id ? `#${id}` : 'exempt' }
  const total = t.totalRounds || t.plannedRounds
  return (
    <>
      <section className="card">
        <h2>Tableau</h2>
        {t.rounds.length === 0 ? <p className="muted">Tableau pas encore tiré.</p> : (
          <div className="bracket">
            {t.rounds.map(r => (
              <div key={r.number} className="bracket-col">
                <div className="muted small">{koRoundName(total, r.number)}</div>
                {(r.matches ?? []).map(m => (
                  <div key={m.number} className="bracket-match" title={m.games.map(g => `${name(g.white)} – ${name(g.black)} ${g.result || '…'}`).join('\n')}>
                    <div className={m.winner === m.a ? 'win' : ''}>{name(m.a)}</div>
                    <div className={m.winner === m.b && m.b ? 'win' : 'muted'}>{m.b ? name(m.b) : 'exempt'}</div>
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}
      </section>
      {t.rounds.length > 0 && (
        <section className="card">
          <h2>{koRoundName(total, t.rounds.length)} : parties</h2>
          <table className="table">
            <tbody>
              {(t.rounds[t.rounds.length - 1].matches ?? []).filter(m => m.b).flatMap(m => m.games.map(g => (
                <tr key={`${m.number}-${g.number}`}><td className="muted">Match {m.number} · {KO_STAGES[g.stage]} {g.number}</td><td>{name(g.white)}</td><td className="center"><b>{g.result || '…'}</b></td><td>{name(g.black)}</td></tr>
              )))}
            </tbody>
          </table>
        </section>
      )}
    </>
  )
}

function PublicPairings({ t }: { t: PublicTournament }) {
  const [n, setN] = useState(t.lastPublished)
  useEffect(() => { setN(t.lastPublished) }, [t.lastPublished])
  const round = t.rounds.find(r => r.number === n)
  const players = new Map(t.players.map(p => [p.id, p]))
  const [byName, setByName] = useState(false)
  if (!round) return <section className="card"><p className="muted">Aucune ronde publiée pour le moment.</p></section>
  return (
    <section className="card">
      <div className="card-head">
        <div className="round-picker">{t.rounds.map(r => <button key={r.number} className={r.number === n ? 'on' : ''} onClick={() => setN(r.number)}>R{r.number}</button>)}</div>
        <div className="segmented"><button className={!byName ? 'on' : ''} onClick={() => setByName(false)}>Par échiquier</button><button className={byName ? 'on' : ''} onClick={() => setByName(true)}>Par nom</button></div>
      </div>
      {byName ? <PairingsByName t={t} round={round} players={players} /> : <PairingsTable t={t} round={round} players={players} />}
      {Object.keys(round.absences || {}).length > 0 && (
        <p className="muted small">Non appariés : {Object.entries(round.absences).map(([id, c]) => `${players.get(+id) ? fullName(players.get(+id)!) : id} (${byeLabel[c]})`).join(', ')}</p>
      )}
    </section>
  )
}

export function PairingsByName({ round, players, big }: { t: PublicTournament; round: Round; players: Map<number, PublicPlayer>; big?: boolean }) {
  const rows: { p: PublicPlayer; board: number; color: string; opp?: PublicPlayer; res: string }[] = []
  round.boards.forEach(b => {
    const w = players.get(b.white), bl = players.get(b.black)
    if (w) rows.push({ p: w, board: b.number, color: bl ? 'Blancs' : '', opp: bl, res: b.result })
    if (bl) rows.push({ p: bl, board: b.number, color: 'Noirs', opp: w, res: b.result })
  })
  rows.sort((a, b) => fullName(a.p).localeCompare(fullName(b.p), 'fr'))
  return (
    <table className={`table ${big ? 'big' : ''}`}>
      <thead><tr><th>Joueur</th><th className="num">Éch.</th><th>Couleur</th><th>Adversaire</th></tr></thead>
      <tbody>{rows.map(r => (
        <tr key={r.p.id}><td><b>{fullName(r.p)}</b></td><td className="num strong">{r.board}</td><td>{r.color || (r.res === 'FREE' ? 'repos' : 'exempt')}</td><td>{r.opp ? fullName(r.opp) : ''}</td></tr>
      ))}</tbody>
    </table>
  )
}
