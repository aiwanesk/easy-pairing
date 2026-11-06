import { useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError, api } from '../../api'
import type { Board, Player, Round } from '../../types'
import { byeLabel, fmtPts, fullName, resultLabel, scoresBefore } from '../../util'
import { Modal } from '../../components/Modal'
import { useAction, useToast } from '../../components/Toast'
import type { TProps } from '../TournamentPage'
import { NextRoundByes } from './NextRoundByes'

export function RoundsTab(props: TProps & { selected?: number }) {
  const { ev, t, d, setEv, canEdit, canResults, selected } = props
  const resultOnly = !canEdit && !!canResults
  const run = useAction()
  const toast = useToast()
  const nav = useNavigate()
  const [busy, setBusy] = useState(false)
  const [editing, setEditing] = useState(false)
  const [rrOrder, setRrOrder] = useState('rating')
  const [manualNums, setManualNums] = useState<Record<number, number>>({})
  const paired = t.rounds.length
  const current = selected && selected >= 1 && selected <= paired ? selected : (d.lastPublished && d.lastPublished < paired && !t.rounds[d.lastPublished - 1]?.boards.every(b => b.black === 0 || b.result) ? d.lastPublished : paired)
  const round = t.rounds[current - 1]
  const byId = new Map(t.players.map(p => [p.id, p]))
  const isLast = current === paired
  const base = `/admin/e/${ev.id}/t/${t.id}/rondes`
  const goto = (n: number) => nav(`${base}/${n}`)

  const [failure, setFailure] = useState<{ title: string; details: string[] } | null>(null)
  const pair = async () => {
    setBusy(true)
    let r: Awaited<ReturnType<typeof api.pair>> | undefined
    try {
      r = await api.pair(ev.id, t.id, t.kind === 'roundrobin' ? rrOrder : undefined, rrOrder === 'manual' ? manualNums : undefined)
      toast(t.kind === 'roundrobin' ? 'Calendrier créé' : `Ronde ${paired + 1} appariée`, 'success')
    } catch (e) {
      if (e instanceof ApiError && e.details?.length) setFailure({ title: e.message, details: e.details })
      else toast(e instanceof Error ? e.message : String(e), 'error')
    }
    setBusy(false)
    if (r) { setEv(r); goto(t.kind === 'roundrobin' ? 1 : r.tournaments.find(x => x.id === t.id)!.rounds.length) }
  }
  const publish = async (v: boolean) => {
    const r = await run(() => api.publish(ev.id, t.id, current, v), v ? `Ronde ${current} publiée` : `Ronde ${current} dépubliée`)
    if (r) setEv(r)
  }
  const unpair = async () => {
    if (!confirm(`Supprimer l'appariement de la ronde ${paired} et ses éventuels résultats ?`)) return
    const r = await run(() => api.deleteLastRound(ev.id, t.id), `Ronde ${paired} supprimée`)
    if (r) { setEv(r); if (paired > 1) goto(paired - 1) }
  }
  const resetRR = async () => {
    if (!confirm('Supprimer tout le calendrier du toutes-rondes ?')) return
    const r = await run(() => api.resetRoundRobin(ev.id, t.id), 'Calendrier supprimé')
    if (r) setEv(r)
  }
  const check = async () => {
    const r = await run(() => api.checkRound(ev.id, t.id, current))
    if (r) toast(r.differences.length ? `Écarts avec le moteur : ${r.differences.join(' ; ')}` : 'Appariement conforme à bbpPairings', r.differences.length ? 'error' : 'success')
  }
  const setResult = async (b: Board, result: string) => {
    const next = b.result === result && isLast ? '' : result
    const r = await run(() => api.setResult(ev.id, t.id, current, b.number, next))
    if (r) setEv(r)
  }

  const before = scoresBefore(t.rounds, current, t.pointsWin, t.pointsDraw, t.pointsPab, t.kind)
  const games = round?.boards.filter(b => b.black !== 0 && b.white !== 0) ?? []
  const pending = games.filter(b => !b.result).length
  const hasResults = games.some(b => b.result)
  const label = (p?: Player) => p ? <span><span className="title">{p.title} </span>{fullName(p)} <span className="muted small">{p.rating || ''}</span></span> : null
  const number = (id: number) => t.kind === 'roundrobin' ? byId.get(id)?.bergerNo : d.pairingNumbers[id]

  return (
    <div className="stack">
      <section className="card">
        <div className="card-head">
          <div className="round-picker">
            {t.rounds.map(r => (
              <button key={r.number} className={`${r.number === current ? 'on' : ''} ${r.published ? '' : 'draft'}`} onClick={() => goto(r.number)}
                title={r.published ? 'publiée' : 'non publiée'}>R{r.number}</button>
            ))}
            {paired === 0 && <span className="muted">Aucune ronde.</span>}
          </div>
          {canEdit && (
            <div className="actions">
              {t.kind === 'swiss' && paired < t.totalRounds && (
                <button className="primary" disabled={!d.canPair || busy} onClick={pair} title={d.canPairReason || ''}>
                  {busy ? 'Appariement…' : `Apparier la ronde ${paired + 1}`}
                </button>
              )}
              {t.kind === 'roundrobin' && paired === 0 && (
                <>
                  <select value={rrOrder} onChange={e => setRrOrder(e.target.value)}>
                    <option value="rating">Numéros de Berger par Elo</option>
                    <option value="lots">Numéros tirés au sort</option>
                    <option value="varma">Tirage restreint FIDE (tables de Varma, 9 à 24 joueurs)</option>
                    <option value="manual">Numéros tirés en salle (saisie manuelle)</option>
                  </select>
                  <button className="primary" disabled={!d.canPair || busy} onClick={pair}>Créer le calendrier ({d.plannedRounds} rondes)</button>
                </>
              )}
            </div>
          )}
        </div>
        {!d.canPair && d.canPairReason && t.kind !== 'swiss' && <p className="muted small">{d.canPairReason}</p>}
        {t.kind === 'roundrobin' && paired === 0 && rrOrder === 'varma' && (
          <p className="muted small">Tirage FIDE C.05 annexe 2 : les fédérations les plus nombreuses tirent d'abord une enveloppe (groupe A à D) contenant assez de numéros pour tous leurs joueurs ;
            les compatriotes se rencontrent ainsi en début de tournoi. Le détail du tirage est inscrit au journal du tournoi.</p>
        )}
        {t.kind === 'roundrobin' && paired === 0 && rrOrder === 'manual' && (
          <table className="table compact mt">
            <thead><tr><th>Joueur</th><th>Féd.</th><th className="num">N° tiré</th></tr></thead>
            <tbody>
              {t.players.filter(p => p.status === 'confirmed' && !p.withdrawnFrom && (!t.checkIn || p.checkedIn)).map(p => (
                <tr key={p.id}><td>{fullName(p)}</td><td>{p.fed}</td>
                  <td className="num"><input type="number" className="narrow" min={1} value={manualNums[p.id] || ''} onChange={e => setManualNums(m => ({ ...m, [p.id]: +e.target.value }))} /></td></tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      {failure && (
        <Modal title="Appariement impossible" onClose={() => setFailure(null)} wide>
          <p className="error"><b>{failure.title}</b></p>
          <h3>Pourquoi</h3>
          <ul>{failure.details.filter(x => !x.startsWith('→')).map((x, i) => <li key={i}>{x}</li>)}</ul>
          <h3>Que faire</h3>
          <ul>{failure.details.filter(x => x.startsWith('→')).map((x, i) => <li key={i}>{x.slice(2)}</li>)}</ul>
          <div className="actions"><button className="primary" onClick={() => setFailure(null)}>Compris</button></div>
        </Modal>
      )}
      {canEdit && t.kind === 'swiss' && paired < t.totalRounds && !d.canPair && d.canPairReason && (
        <div className="banner warn"><b>Ronde {paired + 1} pas encore appariable :</b> {d.canPairReason}</div>
      )}
      {canEdit && t.kind === 'swiss' && paired < t.totalRounds && <NextRoundByes {...props} />}

      {round && (
        <section className="card print-area">
          <div className="card-head">
            <h2>{t.name} — ronde {round.number}</h2>
            <div className="actions no-print">
              <span className={`pill ${round.published ? (pending ? 'warn' : 'ok') : 'muted'}`}>
                {!round.published ? 'Non publiée' : pending ? `${pending} résultat${pending > 1 ? 's' : ''} en attente` : 'Terminée'}
              </span>
              {canEdit && (round.published
                ? !hasResults && (t.kind === 'roundrobin' || isLast) && <button className="ghost" onClick={() => publish(false)}>Dépublier</button>
                : <button className="primary" onClick={() => publish(true)}>Publier la ronde</button>)}
              {canEdit && !hasResults && (t.kind === 'roundrobin' ? isLast || true : isLast) && current === paired && <button className="ghost" onClick={() => setEditing(true)}>Modifier l'appariement</button>}
              {t.kind === 'swiss' && <button className="ghost" onClick={check}>Contrôler</button>}
              <button className="ghost" onClick={() => window.print()}>Imprimer</button>
              {canEdit && t.kind === 'swiss' && isLast && <button className="ghost danger" onClick={unpair}>Supprimer</button>}
              {canEdit && t.kind === 'roundrobin' && !t.rounds.some(r => r.boards.some(b => b.black && b.result)) && <button className="ghost danger" onClick={resetRR}>Réinitialiser</button>}
            </div>
          </div>
          {!round.published && <p className="muted small no-print">Tant que la ronde n'est pas publiée, les écrans et pages publiques restent sur la ronde précédente : vérifiez et imprimez, puis publiez.</p>}
          {round.manual && <p className={`small no-print ${round.engineCheck && round.engineCheck !== 'conforme au moteur' ? 'warn' : 'muted'}`}>Appariement modifié à la main{round.engineCheck ? ` — ${round.engineCheck}` : ''}.</p>}
          <div className="table-wrap">
            <table className="table pairings">
              <thead>
                <tr><th className="num">Éch.</th><th className="num">N°</th><th>Blancs</th><th className="num">Pts</th><th className="center">Résultat</th><th className="num">Pts</th><th>Noirs</th><th className="num">N°</th></tr>
              </thead>
              <tbody>
                {round.boards.map(b => b.black === 0 || b.white === 0 ? (
                  <tr key={b.number} className="bye">
                    <td className="num">{b.number}</td><td className="num muted">{number(b.white)}</td><td>{label(byId.get(b.white))}</td>
                    <td className="num">{fmtPts(before.get(b.white))}</td><td className="center"><span className="tag ok">{resultLabel[b.result]}</span></td><td colSpan={3} />
                  </tr>
                ) : (
                  <tr key={b.number} className={b.result ? '' : 'pending'}>
                    <td className="num">{b.number}</td>
                    <td className="num muted">{number(b.white)}</td>
                    <td>{label(byId.get(b.white))}</td>
                    <td className="num">{fmtPts(before.get(b.white))}</td>
                    <td className="center result-cell">
                      {canEdit || (resultOnly && isLast && round.published && (!b.result)) ? (
                        <div className={`res-group no-print ${resultOnly ? 'big' : ''}`}>
                          {(['1-0', '1/2', '0-1'] as const).map(r => (
                            <button key={r} type="button" className={`res ${b.result === r ? 'on' : ''}`} onClick={() => setResult(b, r)}>{r === '1/2' ? '½' : r}</button>
                          ))}
                          <select className={`res-more ${['+-', '-+', '--'].includes(b.result) ? 'on' : ''}`} value={['+-', '-+', '--'].includes(b.result) ? b.result : ''}
                            onChange={e => e.target.value && setResult(b, e.target.value)} title="Forfaits">
                            <option value="">F…</option><option value="+-">+ / − (forfait Noirs)</option><option value="-+">− / + (forfait Blancs)</option><option value="--">− / − (double forfait)</option>
                          </select>
                          {b.source === 'kiosk' && <span className="tag" title="saisi par les joueurs au kiosque">K</span>}
                        </div>
                      ) : <span>{resultLabel[b.result]}</span>}
                      <span className="print-only">{resultLabel[b.result]}</span>
                    </td>
                    <td className="num">{fmtPts(before.get(b.black))}</td>
                    <td>{label(byId.get(b.black))}</td>
                    <td className="num muted">{number(b.black)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {Object.keys(round.absences || {}).length > 0 && (
            <div className="absences">
              <h3>Non appariés</h3>
              <ul>{Object.entries(round.absences).map(([id, code]) => {
                const p = byId.get(+id)
                return <li key={id}>{p ? fullName(p) : `#${id}`} <span className="tag">{byeLabel[code]}</span></li>
              })}</ul>
            </div>
          )}
        </section>
      )}
      {editing && round && <ManualEditor {...props} round={round} onClose={() => setEditing(false)} />}
    </div>
  )
}

function ManualEditor({ ev, t, setEv, round, onClose }: TProps & { round: Round; onClose: () => void }) {
  const [boards, setBoards] = useState(round.boards.map(b => ({ white: b.white, black: b.black })))
  const [errors, setErrors] = useState<string[]>([])
  const run = useAction()
  const toast = useToast()
  const ids = round.boards.flatMap(b => [b.white, b.black]).filter(Boolean)
  const byId = new Map(t.players.map(p => [p.id, p]))
  const used = new Map<number, number>()
  boards.forEach(b => { [b.white, b.black].forEach(id => { if (id) used.set(id, (used.get(id) ?? 0) + 1) }) })
  const missing = ids.filter(id => !used.has(id))

  const set = (i: number, k: 'white' | 'black', v: number) => setBoards(bs => bs.map((b, j) => (j === i ? { ...b, [k]: v } : b)))
  const swap = (i: number) => setBoards(bs => bs.map((b, j) => (j === i ? { white: b.black, black: b.white } : b)))

  const save = async () => {
    setErrors([])
    try {
      const r = await api.manualPairing(ev.id, t.id, round.number, boards.filter(b => b.white || b.black))
      setEv(r)
      const rd = r.tournaments.find(x => x.id === t.id)!.rounds[round.number - 1]
      toast(rd.engineCheck ? `Enregistré — ${rd.engineCheck}` : 'Appariement enregistré', rd.engineCheck && rd.engineCheck !== 'conforme au moteur' ? 'info' : 'success')
      onClose()
    } catch (e) {
      if (e instanceof ApiError) setErrors([e.message, ...(e.details ?? [])])
      else run(async () => { throw e })
    }
  }

  const select = (i: number, k: 'white' | 'black', v: number) => (
    <select value={v} onChange={e => set(i, k, +e.target.value)} className={v && (used.get(v) ?? 0) > 1 ? 'invalid' : ''}>
      <option value={0}>{k === 'black' ? '— exempt —' : '—'}</option>
      {ids.map(id => <option key={id} value={id}>{fullName(byId.get(id)!)} ({byId.get(id)?.rating || '?'})</option>)}
    </select>
  )

  return (
    <Modal title={`Modifier l'appariement — ronde ${round.number}`} onClose={onClose} wide>
      <p className="muted small">
        Les joueurs absents (byes, retraits) restent hors appariement. Avertissements contrôlés : rencontres déjà jouées, appariements interdits,
        couleurs (écart &gt; 2, trois fois de suite), exempt déjà reçu.{t.kind === 'swiss' && ' Le résultat est comparé à bbpPairings'}{t.fideMode && ' ; en mode FIDE, un écart annule la modification'}.
      </p>
      <table className="table compact">
        <thead><tr><th className="num">Éch.</th><th>Blancs</th><th /><th>Noirs</th><th /></tr></thead>
        <tbody>
          {boards.map((b, i) => (
            <tr key={i}>
              <td className="num">{i + 1}</td>
              <td>{select(i, 'white', b.white)}</td>
              <td><button className="ghost small" onClick={() => swap(i)} title="Inverser les couleurs">⇄</button></td>
              <td>{select(i, 'black', b.black)}</td>
              <td><button className="ghost danger small" onClick={() => setBoards(bs => bs.filter((_, j) => j !== i))}>✕</button></td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="inline mt">
        <button onClick={() => setBoards(bs => [...bs, { white: 0, black: 0 }])}>Ajouter un échiquier</button>
        {missing.length > 0 && <span className="warn small">Non placés : {missing.map(id => fullName(byId.get(id)!)).join(', ')}</span>}
      </div>
      {errors.length > 0 && <ul className="error-list">{errors.map((e, i) => <li key={i}>{e}</li>)}</ul>}
      <div className="actions end">
        <button onClick={onClose}>Annuler</button>
        <button className="primary" onClick={save} disabled={missing.length > 0}>Enregistrer</button>
      </div>
    </Modal>
  )
}
