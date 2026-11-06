import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { ApiError, api } from '../api'
import type { PublicTournament } from '../types'
import { fullName, resultLabel, useLiveUpdates } from '../util'

const storeKey = (slug: string, tid: string) => `pe-kiosk-${slug}-${tid}`

export function KioskPage() {
  const { slug = '', tid = '' } = useParams()
  const [token, setToken] = useState(() => { try { return sessionStorage.getItem(storeKey(slug, tid)) ?? '' } catch { return '' } })
  const [pin, setPin] = useState('')
  const [data, setData] = useState<{ event: string; tournament: PublicTournament } | null>(null)
  const [msg, setMsg] = useState<{ text: string; ok: boolean } | null>(null)
  const [q, setQ] = useState('')
  const [mode, setMode] = useState<'checkin' | 'results'>('checkin')

  const load = useCallback(async () => {
    if (!token) return
    try {
      const d = await api.kiosk(slug, +tid, token)
      setData(d)
      if (d.tournament.started && mode === 'checkin') setMode('results')
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) { setToken(''); try { sessionStorage.removeItem(storeKey(slug, tid)) } catch { /* empty */ } }
    }
  }, [slug, tid, token, mode])
  useEffect(() => { load() }, [load])
  useLiveUpdates(token ? api.publicStreamUrl(slug) : null, load)

  const flash = (text: string, ok: boolean) => { setMsg({ text, ok }); window.setTimeout(() => setMsg(null), 4000) }

  const unlock = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const r = await api.kioskAuth(slug, +tid, pin)
      setToken(r.token)
      try { sessionStorage.setItem(storeKey(slug, tid), r.token) } catch { /* empty */ }
      setPin('')
    } catch (err) {
      flash(err instanceof Error ? err.message : String(err), false)
    }
  }

  if (!token) {
    return (
      <div className="kiosk center-page">
        <form className="auth-card" onSubmit={unlock}>
          <h1>Kiosque</h1>
          <p className="muted">Code fourni par l'arbitre</p>
          <input className="pin" value={pin} onChange={e => setPin(e.target.value)} inputMode="numeric" autoFocus />
          <button className="primary big-btn">Déverrouiller</button>
          {msg && <p className={msg.ok ? 'ok' : 'error'}>{msg.text}</p>}
        </form>
      </div>
    )
  }
  if (!data) return <div className="kiosk center-page muted">Chargement…</div>
  const t = data.tournament
  const round = t.rounds.find(r => r.number === t.lastPublished)
  const players = new Map(t.players.map(p => [p.id, p]))
  const f = q.trim().toLowerCase()

  const checkIn = async (id: number) => {
    try { const r = await api.kioskCheckIn(slug, t.id, token, id); flash(`${r.player} : pointé(e) ✓`, true); setQ(''); load() } catch (e) { flash(e instanceof Error ? e.message : String(e), false) }
  }
  const result = async (board: number, res: string) => {
    const b = round?.boards.find(x => x.number === board)
    if (!b || !round) return
    if (!confirm(`Échiquier ${board} : ${fullName(players.get(b.white)!)} ${resultLabel[res]} ${fullName(players.get(b.black)!)} ?`)) return
    try { await api.kioskResult(slug, t.id, token, round.number, board, res); flash(`Échiquier ${board} : ${resultLabel[res]} enregistré ✓`, true); load() } catch (e) { flash(e instanceof Error ? e.message : String(e), false) }
  }

  return (
    <div className="kiosk">
      <header className="kiosk-head">
        <div><b>{data.event}</b> — {t.name}</div>
        <div className="segmented">
          {t.checkIn && !t.started && <button className={mode === 'checkin' ? 'on' : ''} onClick={() => setMode('checkin')}>Pointage</button>}
          {t.kioskResults && <button className={mode === 'results' ? 'on' : ''} onClick={() => setMode('results')}>Résultats</button>}
        </div>
      </header>
      {msg && <div className={`kiosk-msg ${msg.ok ? 'ok' : 'bad'}`}>{msg.text}</div>}
      {mode === 'checkin' ? (
        !t.checkIn ? <p className="muted center">Le pointage n'est pas activé.</p> : t.started ? <p className="muted center">Le pointage est clos.</p> : (
          <div className="kiosk-body">
            <input className="kiosk-search" value={q} onChange={e => setQ(e.target.value)} placeholder="Tapez votre nom…" autoFocus />
            <div className="kiosk-list">
              {t.players.filter(p => p.status === 'confirmed' && !p.withdrawnFrom && (!f || fullName(p).toLowerCase().includes(f)))
                .sort((a, b) => fullName(a).localeCompare(fullName(b), 'fr')).slice(0, f ? 30 : 200).map(p => (
                  <button key={p.id} className={`kiosk-item ${p.checkedIn ? 'on' : ''}`} disabled={p.checkedIn} onClick={() => checkIn(p.id)}>
                    <span>{fullName(p)}<br /><span className="muted small">{p.club}</span></span>
                    <span>{p.checkedIn ? 'Pointé ✓' : 'Je suis présent'}</span>
                  </button>
                ))}
            </div>
          </div>
        )
      ) : !round ? <p className="muted center">Aucune ronde en cours.</p> : (
        <div className="kiosk-body">
          <h2>Ronde {round.number} — saisissez le résultat de votre partie</h2>
          <div className="kiosk-list">
            {round.boards.filter(b => b.black && b.white).map(b => (
              <div key={b.number} className={`kiosk-board ${b.result ? 'done' : ''}`}>
                <div className="kb-num">{b.number}</div>
                <div className="kb-names">{fullName(players.get(b.white)!)}<br /><span className="muted">{fullName(players.get(b.black)!)}</span></div>
                {b.result ? <div className="kb-res">{resultLabel[b.result]}</div> : (
                  <div className="kb-btns">
                    <button onClick={() => result(b.number, '1-0')}>1-0</button>
                    <button onClick={() => result(b.number, '1/2')}>½</button>
                    <button onClick={() => result(b.number, '0-1')}>0-1</button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
