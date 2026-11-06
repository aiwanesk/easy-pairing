import { useEffect, useState } from 'react'
import { api } from '../../api'
import type { NormReport, NormStatus } from '../../types'
import { fmtPts, fullName, tournamentSettings } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

export function NormsTab({ ev, t, setEv, canEdit }: TProps) {
  const [rep, setRep] = useState<NormReport | null>(null)
  const [showAll, setShowAll] = useState(false)
  const run = useAction()
  const byId = new Map(t.players.map(p => [p.id, p]))
  const played = t.rounds.filter(r => r.boards.every(b => b.black === 0 || b.white === 0 || b.result)).length

  useEffect(() => { api.norms(ev.id, t.id).then(setRep).catch(() => {}) }, [ev, t])

  const setFrom = async (n: number) => {
    const r = await run(() => api.updateTournament(ev.id, t.id, { ...tournamentSettings(t), normsFrom: n }))
    if (r) setEv(r)
  }

  if (!rep) return <div className="card muted">Calcul…</div>
  if (rep.disabled) return <section className="card"><h2>Normes</h2><p className="muted">Pas de calcul de norme : {rep.disabled}.</p></section>

  const achieved = rep.players.filter(s => s.achieved)
  const watching = played >= t.normsFrom || t.kind === 'roundrobin'
  const close = rep.players.filter(s => !s.achieved && s.possible)
  const row = (s: NormStatus) => {
    const p = byId.get(s.playerId)
    return (
      <tr key={`${s.playerId}-${s.norm}`}>
        <td><span className="title">{p?.title} </span>{p ? fullName(p) : s.playerId} <span className="muted small">{p?.fed} {p?.rating}</span></td>
        <td><span className={`tag ${s.achieved ? 'ok' : s.close ? 'warn' : ''}`}>{s.label}</span></td>
        <td className="num">{fmtPts(s.score)}/{s.games}</td>
        <td className="num">{s.ra || '—'}</td>
        <td className="num">{s.games ? s.rp : '—'}</td>
        <td className="small">
          {s.achieved ? <b>Norme obtenue</b> : s.need}
          <div className="norm-conds">
            {s.conditions.map(c => <span key={c.label} className={`cond ${c.ok ? 'ok' : 'ko'}`} title={c.detail}>{c.ok ? '✓' : '✗'} {c.label}</span>)}
          </div>
        </td>
      </tr>
    )
  }

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <h2>Normes de titre {rep.round ? `après la ronde ${rep.round}` : ''}</h2>
          <div className="inline">
            {t.kind === 'swiss' && (
              <label className="inline small">Suivre les joueurs proches à partir de la ronde
                <input type="number" className="narrow" min={1} max={t.totalRounds} value={t.normsFrom} disabled={!canEdit}
                  onChange={e => +e.target.value > 0 && setFrom(+e.target.value)} />
              </label>
            )}
          </div>
        </div>
        {achieved.length > 0 && (
          <div className="banner ok-banner">Norme obtenue : {achieved.map(s => `${fullName(byId.get(s.playerId)!)} (${s.label})`).join(', ')}</div>
        )}
        {!watching ? (
          <p className="muted">Le suivi des joueurs proches d'une norme s'affiche à partir de la ronde {t.normsFrom} ({played} ronde(s) terminée(s)).</p>
        ) : close.length === 0 && achieved.length === 0 ? (
          <p className="muted">Aucun joueur ne peut encore obtenir de norme.</p>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Joueur</th><th>Norme</th><th className="num">Score</th><th className="num">Moy. adv.</th><th className="num">Perf.</th><th>Ce qu'il faut</th></tr></thead>
              <tbody>{[...achieved, ...close].map(row)}</tbody>
            </table>
          </div>
        )}
        {watching && rep.players.length > achieved.length + close.length && (
          <button className="ghost small mt" onClick={() => setShowAll(v => !v)}>{showAll ? 'Masquer' : 'Voir'} les joueurs qui ont manqué de peu</button>
        )}
        {showAll && (
          <table className="table"><tbody>{rep.players.filter(s => !s.achieved && !s.possible).map(row)}</tbody></table>
        )}
      </section>
      <section className="card">
        <h3>Règles appliquées</h3>
        <ul className="small muted">{rep.notes.map((n, i) => <li key={i}>{n}</li>)}</ul>
      </section>
    </div>
  )
}
