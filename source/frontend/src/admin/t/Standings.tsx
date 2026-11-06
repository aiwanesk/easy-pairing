import { useEffect, useState } from 'react'
import { api } from '../../api'
import type { PublicPlayer, Round, Standing, TieBreakDef, KOStanding } from '../../types'
import { crossCell, fmtNum, fmtPts, fullName } from '../../util'
import type { TProps } from '../TournamentPage'

export function useTieBreakLabels() {
  const [defs, setDefs] = useState<TieBreakDef[]>([])
  useEffect(() => { api.tieBreaks().then(setDefs).catch(() => {}) }, [])
  return Object.fromEntries(defs.map(d => [d.id, d.label])) as Record<string, string>
}

interface PlayerLike { id: number; title: string; lastName: string; firstName: string; rating: number; fed: string; club: string; withdrawnFrom: number }

export function StandingsTable({ rows, players, tieBreaks }: { rows: Standing[]; players: Map<number, PlayerLike | PublicPlayer>; tieBreaks: string[] }) {
  const labels = useTieBreakLabels()
  return (
    <div className="table-wrap">
      <table className="table">
        <thead>
          <tr>
            <th className="num">Pl.</th><th className="num">N°</th><th>Titre</th><th>Nom</th><th className="num">Elo</th><th>Féd.</th><th>Club</th><th className="num">Pts</th>
            {tieBreaks.map(tb => <th key={tb} className="num" title={labels[tb]}>{tb}</th>)}
          </tr>
        </thead>
        <tbody>
          {rows.map((s, i) => {
            const p = players.get(s.playerId)
            if (!p) return null
            return (
              <tr key={s.playerId} className={p.withdrawnFrom ? 'withdrawn' : ''}>
                <td className="num strong">{i > 0 && rows[i - 1].rank === s.rank ? '' : s.rank}</td>
                <td className="num muted">{s.pairingNo}</td>
                <td className="title">{p.title}</td>
                <td>{fullName(p)}</td>
                <td className="num">{p.rating || ''}</td>
                <td>{p.fed}</td>
                <td className="ellipsis" title={p.club}>{p.club}</td>
                <td className="num strong">{fmtPts(s.points)}</td>
                {tieBreaks.map(tb => <td key={tb} className="num">{fmtNum(s.tieBreaks[tb])}</td>)}
              </tr>
            )
          })}
        </tbody>
      </table>
      <p className="muted tiny">{tieBreaks.map(tb => `${tb} : ${labels[tb] ?? tb}`).join(' · ')}</p>
    </div>
  )
}

export function StandingsTab(props: TProps) {
  return props.t.kind === 'knockout' ? <KOStandingsTab {...props} /> : <SwissStandingsTab {...props} />
}

function KOStandingsTab({ ev, t }: TProps) {
  const [rows, setRows] = useState<KOStanding[]>([])
  useEffect(() => { api.standings(ev.id, t.id).then(r => setRows(r.ko ?? [])).catch(() => {}) }, [ev, t])
  const players = new Map(t.players.map(p => [p.id, p]))
  return (
    <section className="card print-area">
      <div className="card-head"><h2>{t.name} — classement de la coupe</h2><button className="ghost no-print" onClick={() => window.print()}>Imprimer</button></div>
      <table className="table">
        <thead><tr><th className="num">Pl.</th><th className="num">Tête de série</th><th>Joueur</th><th className="num">Elo</th><th>Club</th><th>Parcours</th></tr></thead>
        <tbody>
          {rows.map((r, i) => {
            const p = players.get(r.playerId)
            return (
              <tr key={r.playerId}>
                <td className="num strong">{i > 0 && rows[i - 1].rank === r.rank ? '' : r.rank}</td>
                <td className="num muted">{p?.bergerNo}</td>
                <td><span className="title">{p?.title} </span>{p ? fullName(p) : r.playerId}</td>
                <td className="num">{p?.rating || ''}</td>
                <td>{p?.club}</td>
                <td>{r.label}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </section>
  )
}

function SwissStandingsTab({ ev, t }: TProps) {
  const paired = t.rounds.length
  const [round, setRound] = useState(paired)
  const [rows, setRows] = useState<Standing[] | null>(null)
  useEffect(() => { setRound(t.rounds.length) }, [t.rounds.length])
  useEffect(() => { api.standings(ev.id, t.id, round).then(r => setRows(r.standings)).catch(() => setRows([])) }, [ev, t, round])
  const players = new Map(t.players.map(p => [p.id, p]))
  return (
    <section className="card print-area">
      <div className="card-head">
        <h2>{t.name} — classement {round > 0 ? `après la ronde ${round}` : 'initial'}</h2>
        <div className="actions no-print">
          <select value={round} onChange={e => setRound(+e.target.value)}>
            {Array.from({ length: paired + 1 }, (_, i) => <option key={i} value={i}>{i === 0 ? 'Initial' : `Après R${i}`}</option>)}
          </select>
          <a className="button" href={api.exportUrl(ev.id, t.id, 'standings.csv')}>CSV</a>
          <button className="ghost" onClick={() => window.print()}>Imprimer</button>
        </div>
      </div>
      {rows === null ? <p className="muted">Chargement…</p> : <StandingsTable rows={rows} players={players} tieBreaks={t.tieBreaks} />}
      <p className="muted small no-print">
        Conventions FIDE C.07 (2023) : rondes non jouées volontairement par un adversaire comptées nulles ; rondes non jouées du joueur contre un adversaire
        virtuel ayant son propre score ; confrontation directe appliquée seulement si tous les ex aequo se sont rencontrés.
      </p>
    </section>
  )
}

export function Crosstable({ rounds, standings, players, tieBreaks = [], style = 'ffe' }: {
  rounds: Round[]; standings: Standing[]; players: Map<number, PlayerLike | PublicPlayer>; tieBreaks?: string[]; style?: 'ffe' | 'fide'
}) {
  const labels = useTieBreakLabels()
  const place = new Map(standings.map((s, i) => [s.playerId, i + 1]))
  return (
    <div className="table-wrap">
      <table className="table crosstable">
        <thead>
          <tr>
            <th className="num">Pl.</th><th>Nom</th><th className="num">Elo</th><th>Féd.</th><th>Club</th>
            {rounds.map(r => <th key={r.number} className="center">R{r.number}</th>)}
            <th className="num">Pts</th>
            {tieBreaks.map(tb => <th key={tb} className="num" title={labels[tb]}>{tb}</th>)}
          </tr>
        </thead>
        <tbody>
          {standings.map((s, i) => {
            const p = players.get(s.playerId)
            if (!p) return null
            return (
              <tr key={s.playerId} className={p.withdrawnFrom ? 'withdrawn' : ''}>
                <td className="num strong">{i > 0 && standings[i - 1].rank === s.rank ? '' : s.rank}</td>
                <td className="nowrap"><span className="title">{p.title} </span>{fullName(p)}</td>
                <td className="num muted">{p.rating || ''}</td>
                <td>{p.fed}</td>
                <td className="ellipsis" title={p.club}>{p.club}</td>
                {rounds.map(r => { const c = crossCell(r, s.playerId, place, style); return <td key={r.number} className={`center cell ${c.cls}`}>{c.text}</td> })}
                <td className="num strong">{fmtPts(s.points)}</td>
                {tieBreaks.map(tb => <td key={tb} className="num">{fmtNum(s.tieBreaks[tb])}</td>)}
              </tr>
            )
          })}
        </tbody>
      </table>
      <p className="muted tiny">
        {style === 'ffe'
          ? 'Chaque case : résultat (+ gain, = nulle, - perte), place de l\'adversaire au classement, couleur (B Blancs, N Noirs) ; +F / -F forfait, EXE exempt, =BYE / +BYE bye demandé, ABS absent.'
          : 'Each cell: opponent\'s place, colour (w/b) and result.'}
      </p>
    </div>
  )
}

export function CrosstableTab({ ev, t }: TProps) {
  const [rows, setRows] = useState<Standing[]>([])
  const [style, setStyle] = useState<'ffe' | 'fide'>('ffe')
  const [round, setRound] = useState(t.rounds.length)
  useEffect(() => { setRound(t.rounds.length) }, [t.rounds.length])
  useEffect(() => { api.standings(ev.id, t.id, round).then(r => setRows(r.standings)).catch(() => {}) }, [ev, t, round])
  return (
    <section className="card print-area">
      <div className="card-head">
        <h2>{t.name} — grille {t.kind === 'roundrobin' ? '' : 'américaine'} {round < t.rounds.length ? `après la ronde ${round}` : ''}</h2>
        <div className="actions no-print">
          <select value={round} onChange={e => setRound(+e.target.value)}>
            {t.rounds.map(r => <option key={r.number} value={r.number}>Après la ronde {r.number}</option>)}
          </select>
          <div className="segmented"><button className={style === 'ffe' ? 'on' : ''} onClick={() => setStyle('ffe')}>FFE (+12B)</button><button className={style === 'fide' ? 'on' : ''} onClick={() => setStyle('fide')}>FIDE (12w1)</button></div>
          <button className="ghost" onClick={() => window.print()}>Imprimer</button>
        </div>
      </div>
      <Crosstable rounds={t.rounds.slice(0, round)} standings={rows} players={new Map(t.players.map(p => [p.id, p]))} tieBreaks={t.tieBreaks} style={style} />
    </section>
  )
}
