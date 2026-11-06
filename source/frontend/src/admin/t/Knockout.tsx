import { useState } from 'react'
import { Link } from 'react-router'
import { api } from '../../api'
import type { Match, Player } from '../../types'
import { KO_STAGES, fullName, koRoundName } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

const RESULTS: [string, string][] = [['1-0', '1-0'], ['1/2', '½'], ['0-1', '0-1']]

/** Cup system: bracket by round, match cards with games and tie-breaks. */
export function KnockoutTab(props: TProps & { selected?: number }) {
  const { ev, t, d, setEv, canEdit, canResults, selected } = props
  const run = useAction()
  const [busy, setBusy] = useState(false)
  const byId = new Map(t.players.map(p => [p.id, p]))
  const rounds = t.rounds
  const total = t.totalRounds || d.plannedRounds
  const current = selected && selected >= 1 && selected <= rounds.length ? selected : rounds.length
  const round = rounds[current - 1]
  const isLast = current === rounds.length

  const next = async () => {
    setBusy(true)
    const r = await run(() => api.pair(ev.id, t.id), rounds.length ? `${koRoundName(total, rounds.length + 1)} créé(e)s` : 'Tableau créé')
    setBusy(false)
    if (r) setEv(r)
  }

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <div className="round-picker">
            {rounds.map(r => (
              <Link key={r.number} className={`button ${r.number === current ? 'on' : ''}`} to={`/admin/e/${ev.id}/t/${t.id}/rondes/${r.number}`}>{koRoundName(total, r.number)}</Link>
            ))}
            {rounds.length === 0 && <span className="muted">Tableau pas encore créé ({d.plannedRounds} tour(s) prévus).</span>}
          </div>
          {canEdit && (rounds.length === 0 || rounds[rounds.length - 1].matches!.length > 1) && (
            <button className="primary" disabled={!d.canPair || busy} onClick={next} title={d.canPairReason || ''}>
              {rounds.length === 0 ? 'Créer le tableau' : `Créer : ${koRoundName(total, rounds.length + 1)}`}
            </button>
          )}
        </div>
        {!d.canPair && d.canPairReason && <p className="muted small">{d.canPairReason}</p>}
        <p className="muted small">
          {t.ko.games} partie(s) par match{t.ko.final ? ` (${t.ko.final} en finale)` : ''} · départages : {t.ko.tieBreaks.map(s => KO_STAGES[s.split(':')[0]] + (s.includes(':') ? ` ×${s.split(':')[1]}` : '')).join(', ') || 'aucun'}
          · têtes de série {t.ko.seeding === 'lots' ? 'tirées au sort' : 'par Elo'}
        </p>
      </section>

      {round && (
        <section className="card print-area">
          <h2>{t.name} — {koRoundName(total, round.number)}</h2>
          <div className="ko-grid">
            {round.matches!.map(m => (
              <MatchCard key={m.number} m={m} n={round.number} byId={byId} {...props}
                editable={canEdit && isLast} resultsOnly={!canEdit && !!canResults && isLast} />
            ))}
          </div>
        </section>
      )}

      {rounds.length > 0 && <Bracket {...props} total={total} byId={byId} />}
    </div>
  )
}

function MatchCard({ ev, t, setEv, m, n, byId, editable, resultsOnly }: TProps & { m: Match; n: number; byId: Map<number, Player>; editable: boolean; resultsOnly: boolean }) {
  const run = useAction()
  const name = (id: number) => { const p = byId.get(id); return p ? fullName(p) : '—' }
  const setRes = async (g: number, res: string, cur: string) => {
    const r = await run(() => api.koResult(ev.id, t.id, n, m.number, g, cur === res && editable ? '' : res))
    if (r) setEv(r)
  }
  const tieBreak = async () => {
    const r = await run(() => api.koTieBreak(ev.id, t.id, n, m.number), 'Départage ajouté')
    if (r) setEv(r)
  }
  const designate = async (id: number) => {
    const note = id ? prompt(`Qualifier ${name(id)} par décision de l'arbitre. Motif (tirage au sort, forfait…) :`, 'tirage au sort') : ''
    if (id && note === null) return
    const r = await run(() => api.koWinner(ev.id, t.id, n, m.number, id, note ?? ''), id ? `${name(id)} qualifié` : 'Décision annulée')
    if (r) setEv(r)
  }
  const score = (id: number, stage: string) => m.games.filter(g => g.stage === stage && g.result).reduce((s, g) => {
    const white = g.white === id
    return s + (g.result === '1/2' ? 0.5 : (g.result === '1-0' || g.result === '+-') === white ? 1 : 0)
  }, 0)
  const stages = [...new Set(m.games.map(g => g.stage))]
  const allDone = m.games.length > 0 && m.games.every(g => g.result)
  const seed = (id: number) => byId.get(id)?.bergerNo

  if (m.b === 0) {
    return (
      <div className="match bye">
        <div className="match-head">Match {m.number}</div>
        <div className="match-player win"><span className="seed">{seed(m.a)}</span>{name(m.a)}</div>
        <div className="muted small">qualifié d'office (exempt)</div>
      </div>
    )
  }
  return (
    <div className={`match ${m.winner ? 'decided' : ''}`}>
      <div className="match-head">Match {m.number}{m.decided === 'arbiter' && <span className="tag warn">décision arbitre{m.note ? ` : ${m.note}` : ''}</span>}</div>
      {[m.a, m.b].map(id => (
        <div key={id} className={`match-player ${m.winner === id ? 'win' : m.winner ? 'lose' : ''}`}>
          <span className="seed">{seed(id)}</span>{name(id)} <span className="muted small">{byId.get(id)?.rating || ''}</span>
          <span className="match-score">{stages.map(s => score(id, s)).join(' · ')}</span>
          {editable && !m.winner && allDone && <button className="ghost small" onClick={() => designate(id)} title="Qualifier par décision de l'arbitre">qualifier</button>}
        </div>
      ))}
      <table className="table compact games">
        <tbody>
          {m.games.map(g => (
            <tr key={g.number}>
              <td className="muted small">{KO_STAGES[g.stage]} {g.number}</td>
              <td className="small">{name(g.white)} – {name(g.black)}</td>
              <td>
                {editable || (resultsOnly && !g.result) ? (
                  <div className={`res-group ${resultsOnly ? 'big' : ''}`}>
                    {RESULTS.map(([v, l]) => <button key={v} type="button" className={`res ${g.result === v ? 'on' : ''}`} onClick={() => setRes(g.number, v, g.result)}>{l}</button>)}
                    {editable && (
                      <select className={`res-more ${['+-', '-+', '--'].includes(g.result) ? 'on' : ''}`} value={['+-', '-+', '--'].includes(g.result) ? g.result : ''}
                        onChange={e => e.target.value && setRes(g.number, e.target.value, g.result)}>
                        <option value="">F…</option><option value="+-">+ / −</option><option value="-+">− / +</option><option value="--">− / −</option>
                      </select>
                    )}
                  </div>
                ) : <b>{g.result === '1/2' ? '½-½' : g.result}</b>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {editable && !m.winner && allDone && (
        <div className="actions">
          <button className="small" onClick={tieBreak}>Égalité : ajouter le départage suivant</button>
        </div>
      )}
      {editable && m.decided === 'arbiter' && <button className="ghost small" onClick={() => designate(0)}>Annuler la décision</button>}
    </div>
  )
}

function Bracket({ t, total, byId }: TProps & { total: number; byId: Map<number, Player> }) {
  const name = (id: number) => { const p = byId.get(id); return p ? `${p.bergerNo}. ${fullName(p)}` : id ? `#${id}` : '—' }
  return (
    <section className="card">
      <h2>Tableau</h2>
      <div className="bracket">
        {t.rounds.map(r => (
          <div key={r.number} className="bracket-col">
            <div className="muted small">{koRoundName(total, r.number)}</div>
            {r.matches!.map(m => (
              <div key={m.number} className="bracket-match">
                <div className={m.winner === m.a ? 'win' : ''}>{name(m.a)}</div>
                <div className={m.winner === m.b && m.b ? 'win' : 'muted'}>{m.b ? name(m.b) : 'exempt'}</div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </section>
  )
}
