import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router'
import { Crosstable, StandingsTable } from '../admin/t/Standings'
import { PairingsByName, PairingsTable, usePublicEvent } from './PublicEvent'

const VIEWS: Record<string, string> = { boards: 'Appariements', players: 'Appariements par nom', standings: 'Classement', crosstable: 'Grille' }

/**
 * Projector / TV screen: ?t=<tournament ids, comma-separated>&v=<views>&rotate=<seconds>
 * Views rotate (boards, players, standings, crosstable) and long tables scroll by themselves.
 */
export function ScreenPage() {
  const { slug = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const { ev, error } = usePublicEvent(slug)
  const views = (params.get('v') || 'boards,standings').split(',').filter(v => VIEWS[v])
  const tids = (params.get('t') || '').split(',').filter(Boolean)
  const rotate = Math.max(5, +(params.get('rotate') || 20))
  const [step, setStep] = useState(0)
  const scroller = useRef<HTMLDivElement>(null)

  const pages = useMemo(() => {
    if (!ev) return []
    const ts = tids.length ? ev.tournaments.filter(t => tids.includes(String(t.id))) : ev.tournaments
    return ts.flatMap(t => views.map(v => ({ t, v })))
  }, [ev, tids.join(), views.join()])

  useEffect(() => {
    if (pages.length <= 1) return
    const id = window.setInterval(() => setStep(s => (s + 1) % pages.length), rotate * 1000)
    return () => window.clearInterval(id)
  }, [pages.length, rotate])

  // slow auto-scroll for long lists
  useEffect(() => {
    const el = scroller.current
    if (!el) return
    el.scrollTop = 0
    let raf = 0, last = performance.now(), pause = 2500
    const tick = (now: number) => {
      const dt = now - last
      last = now
      if (pause > 0) pause -= dt
      else if (el.scrollTop + el.clientHeight < el.scrollHeight - 1) el.scrollTop += dt * 0.03
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [step, pages.length])

  if (error) return <div className="screen center-page"><p className="error">{error}</p></div>
  if (!ev) return <div className="screen center-page">Chargement…</div>
  if (!pages.length) return <div className="screen center-page">Rien à afficher</div>
  const { t, v } = pages[step % pages.length]
  const round = t.rounds.find(r => r.number === t.lastPublished)
  const players = new Map(t.players.map(p => [p.id, p]))

  return (
    <div className="screen">
      <header className="screen-head">
        <div><b>{ev.name}</b> · {t.name}</div>
        <div>{VIEWS[v]}{round && (v === 'boards' || v === 'players') ? ` — ronde ${round.number}` : t.lastPublished ? ` — après la ronde ${t.lastPublished}` : ''}</div>
        <div className="screen-clock">{new Date().toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}</div>
      </header>
      {ev.message && <div className="screen-banner">{ev.message}</div>}
      <div className="screen-body" ref={scroller}>
        {(v === 'boards' || v === 'players') && !round ? <p className="center muted">Appariements à venir</p>
          : v === 'boards' ? <PairingsTable t={t} round={round!} players={players} big />
          : v === 'players' ? <PairingsByName t={t} round={round!} players={players} big />
          : v === 'standings' ? <StandingsTable rows={t.standings} players={players} tieBreaks={t.tieBreaks} />
          : <Crosstable rounds={t.rounds} standings={t.standings} players={players} />}
      </div>
      <footer className="screen-foot no-print">
        <span>{pages.length > 1 ? `${(step % pages.length) + 1}/${pages.length}` : ''}</span>
        <details>
          <summary>Réglages de l'écran</summary>
          <div className="screen-settings">
            {Object.entries(VIEWS).map(([k, l]) => (
              <label key={k} className="check"><input type="checkbox" checked={views.includes(k)} onChange={e => {
                const next = e.target.checked ? [...views, k] : views.filter(x => x !== k)
                params.set('v', next.join(',') || 'boards'); setParams(params)
              }} />{l}</label>
            ))}
            <label>Rotation (s) <input type="number" min={5} value={rotate} onChange={e => { params.set('rotate', e.target.value); setParams(params) }} /></label>
            {ev.tournaments.length > 1 && ev.tournaments.map(x => (
              <label key={x.id} className="check"><input type="checkbox" checked={!tids.length || tids.includes(String(x.id))} onChange={e => {
                const cur = tids.length ? tids : ev.tournaments.map(y => String(y.id))
                const next = e.target.checked ? [...cur, String(x.id)] : cur.filter(y => y !== String(x.id))
                params.set('t', next.join(',')); setParams(params)
              }} />{x.name}</label>
            ))}
            <span className="muted tiny">F11 : plein écran</span>
          </div>
        </details>
      </footer>
    </div>
  )
}
