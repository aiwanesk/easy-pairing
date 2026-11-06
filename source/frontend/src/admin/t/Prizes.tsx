import { useEffect, useState } from 'react'
import { api } from '../../api'
import type { Prize, PrizeResult, PrizeSplit } from '../../types'
import { FFE_CATEGORIES, PRIZE_SPLITS, fmtPts, fullName, money } from '../../util'
import { Modal } from '../../components/Modal'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

const blank = (): Prize => ({ id: 0, label: '', amount: 0, category: '', minRating: 0, maxRating: 0, sex: '', fed: '', club: '', unrated: false, minPoints: 0 })

/** Conditions of a prize in French (same wording as the server). */
export function prizeConditions(p: Prize): string {
  const c: string[] = []
  if (p.category) c.push(p.category.split(',').map(x => FFE_CATEGORIES.find(([k]) => k.toLowerCase() === x.trim().toLowerCase())?.[1] ?? x.trim()).join(', '))
  if (p.minRating && p.maxRating) c.push(`Elo ${p.minRating} à ${p.maxRating}`)
  else if (p.minRating) c.push(`Elo ≥ ${p.minRating}`)
  else if (p.maxRating) c.push(`Elo ≤ ${p.maxRating}`)
  if (p.sex === 'f') c.push('féminines')
  if (p.sex === 'm') c.push('masculins')
  if (p.fed) c.push(`fédération ${p.fed}`)
  if (p.club) c.push(`club ${p.club}`)
  if (p.unrated) c.push('non classés')
  if (p.minPoints) c.push(`au moins ${fmtPts(p.minPoints)} pt`)
  return c.length ? c.join(' · ') : 'Classement général'
}

export function PrizesTab({ ev, t, setEv, canEdit }: TProps) {
  const [prizes, setPrizes] = useState<Prize[]>(t.prizes)
  const [split, setSplit] = useState<PrizeSplit>(t.prizeSplit || 'place')
  const [result, setResult] = useState<PrizeResult | null>(null)
  const [editing, setEditing] = useState<number | null>(null)
  const run = useAction()
  const byId = new Map(t.players.map(p => [p.id, p]))
  const name = (id: number) => { const p = byId.get(id); return p ? fullName(p) : `#${id}` }
  const dirty = JSON.stringify(prizes) !== JSON.stringify(t.prizes) || split !== (t.prizeSplit || 'place')

  useEffect(() => { api.prizes(ev.id, t.id).then(setResult).catch(() => {}) }, [ev, t])

  const save = async () => {
    const r = await run(() => api.setPrizes(ev.id, t.id, prizes, split), 'Prix enregistrés')
    if (r) setEv(r)
  }
  const update = (i: number, p: Prize) => setPrizes(ps => ps.map((x, j) => (j === i ? p : x)))
  const move = (i: number, d: -1 | 1) => setPrizes(ps => {
    const j = i + d
    if (j < 0 || j >= ps.length) return ps
    const c = [...ps]; [c[i], c[j]] = [c[j], c[i]]; return c
  })
  const addSeries = (cond: Partial<Prize>, title: string, amounts: number[]) =>
    setPrizes(ps => [...ps, ...amounts.map((a, k) => ({ ...blank(), ...cond, label: `${k + 1}${k ? 'e' : 'er'} ${title}`.trim(), amount: a }))])

  // Prizes grouped by identical conditions, in list order.
  const groups: { cond: string; idx: number[] }[] = []
  prizes.forEach((p, i) => {
    const cond = prizeConditions(p)
    const g = groups.find(x => x.cond === cond)
    if (g) g.idx.push(i)
    else groups.push({ cond, idx: [i] })
  })
  const total = prizes.reduce((a, p) => a + (p.amount || 0), 0)

  return (
    <div className="stack">
      <section className="card">
        <h2>Répartition entre ex aequo</h2>
        <div className="split-choice">
          {PRIZE_SPLITS.map(o => (
            <label key={o.id} className={`choice ${split === o.id ? 'on' : ''}`}>
              <input type="radio" name="split" checked={split === o.id} onChange={() => setSplit(o.id)} disabled={!canEdit} />
              <b>{o.label}</b><span className="muted small">{o.help}</span>
            </label>
          ))}
        </div>
        <p className="muted small">
          Les prix ne se cumulent pas : chaque joueur reçoit le prix le plus élevé auquel il a droit. Le partage (Hort ou égal) se fait entre ex aequo
          à l'intérieur de chaque groupe de prix aux mêmes conditions (général, féminines, tranche Elo…).
        </p>
      </section>

      <section className="card">
        <div className="card-head">
          <h2>Liste des prix <span className="muted small">({prizes.length} · {money(total)})</span></h2>
          {canEdit && (
            <div className="inline">
              <button onClick={() => setPrizes(ps => [...ps, { ...blank(), label: `${ps.length + 1}${ps.length ? 'e' : 'er'} prix` }])}>Ajouter un prix</button>
              <button className="ghost" onClick={() => addSeries({}, '', [300, 200, 100])}>+ 3 prix généraux</button>
              <button className="ghost" onClick={() => addSeries({ sex: 'f' }, 'féminine', [50])}>+ féminine</button>
              <button className="ghost" onClick={() => addSeries({ category: 'Ppo,Pou,Pup,Ben,Min,Cad,Jun' }, 'jeune', [50])}>+ jeune</button>
              <button className="ghost" onClick={() => addSeries({ category: 'Sep,Vet' }, 'vétéran', [50])}>+ vétéran</button>
              <button className="ghost" onClick={() => addSeries({ maxRating: 1600 }, '−1600', [50])}>+ tranche Elo</button>
            </div>
          )}
        </div>
        {prizes.length === 0 ? <p className="muted">Aucun prix. Ajoutez-en avec les boutons ci-dessus.</p> : (
          <table className="table compact">
            <thead><tr><th>Libellé</th><th className="num">Montant</th><th>Conditions</th><th /></tr></thead>
            {groups.map(g => (
              <tbody key={g.cond} className="prize-group">
                <tr className="group-row"><td colSpan={4}>{g.cond}</td></tr>
                {g.idx.map(i => {
                  const p = prizes[i]
                  return (
                    <tr key={i}>
                      <td><input value={p.label} onChange={e => update(i, { ...p, label: e.target.value })} disabled={!canEdit} placeholder="1er prix" /></td>
                      <td><input type="number" min={0} step={1} className="narrow" value={p.amount || ''} onChange={e => update(i, { ...p, amount: +e.target.value })} disabled={!canEdit} /></td>
                      <td>{canEdit ? <button className="ghost small" onClick={() => setEditing(i)}>{prizeConditions(p)} ✎</button> : prizeConditions(p)}</td>
                      <td className="row-actions">{canEdit && <>
                        <button className="ghost small" onClick={() => move(i, -1)}>↑</button>
                        <button className="ghost small" onClick={() => move(i, 1)}>↓</button>
                        <button className="ghost small" onClick={() => setPrizes(ps => [...ps.slice(0, i + 1), { ...p, id: 0, label: '' }, ...ps.slice(i + 1)])} title="Dupliquer avec les mêmes conditions">⧉</button>
                        <button className="ghost danger small" onClick={() => setPrizes(ps => ps.filter((_, j) => j !== i))}>✕</button>
                      </>}</td>
                    </tr>
                  )
                })}
              </tbody>
            ))}
          </table>
        )}
        {canEdit && <div className="actions mt"><button className="primary" onClick={save} disabled={!dirty}>{dirty ? 'Enregistrer' : 'Enregistré'}</button></div>}
      </section>

      {editing !== null && prizes[editing] && (
        <ConditionsModal prize={prizes[editing]} onClose={() => setEditing(null)} onSave={p => { update(editing, p); setEditing(null) }} />
      )}

      <section className="card print-area">
        <div className="card-head">
          <h2>Montants versés {result?.round ? `après la ronde ${result.round}` : ''} <span className="muted small">{PRIZE_SPLITS.find(o => o.id === result?.split)?.label}</span></h2>
          <button className="ghost no-print" onClick={() => window.print()}>Imprimer</button>
        </div>
        {dirty && <p className="warn small no-print">Modifications non enregistrées : le calcul ci-dessous correspond à la liste enregistrée.</p>}
        {!result || result.payouts.length === 0 ? <p className="muted">Aucun prix attribué.</p> : (
          <table className="table">
            <thead><tr><th className="num">Pl.</th><th>Joueur</th><th className="num">Pts</th><th>Prix aux départages</th><th className="num">Nominal</th><th className="num">Versé</th><th>Détail</th></tr></thead>
            <tbody>
              {result.payouts.map(p => {
                const prize = result.awards.find(a => a.prize.id === p.prizeId)?.prize
                return (
                  <tr key={p.playerId}>
                    <td className="num">{p.rank}</td>
                    <td>{name(p.playerId)}</td>
                    <td className="num">{fmtPts(p.points)}</td>
                    <td>{prize ? prize.label : <span className="muted">— (part d'ex aequo)</span>}</td>
                    <td className="num">{p.nominal ? money(p.nominal) : ''}</td>
                    <td className="num"><b>{money(p.amount)}</b></td>
                    <td className="small">{p.detail}{p.sharedWith.length > 0 && <> avec {p.sharedWith.map(name).join(', ')}</>}</td>
                  </tr>
                )
              })}
              <tr className="total-row"><td colSpan={5}>Total</td><td className="num"><b>{money(result.total)}</b></td><td /></tr>
            </tbody>
          </table>
        )}
        {result && result.awards.some(a => !a.playerId || a.tied.length > 0) && (
          <>
            <h3>Points d'attention</h3>
            <ul className="small">
              {result.awards.filter(a => !a.playerId).map(a => <li key={a.prize.id}>« {a.prize.label} » non attribué : aucun joueur éligible ({prizeConditions(a.prize)}).</li>)}
              {result.awards.filter(a => a.playerId && a.tied.length).map(a => (
                <li key={`t${a.prize.id}`}>« {a.prize.label} » : {name(a.playerId)} devant {a.tied.map(name).join(', ')} aux départages (même nombre de points, sans prix).</li>
              ))}
            </ul>
          </>
        )}
      </section>
    </div>
  )
}

function ConditionsModal({ prize, onClose, onSave }: { prize: Prize; onClose: () => void; onSave: (p: Prize) => void }) {
  const [p, setP] = useState<Prize>({ ...prize })
  const cats = p.category.split(',').map(x => x.trim()).filter(Boolean)
  const toggleCat = (c: string) => {
    const has = cats.some(x => x.toLowerCase() === c.toLowerCase())
    setP({ ...p, category: (has ? cats.filter(x => x.toLowerCase() !== c.toLowerCase()) : [...cats, c]).join(',') })
  }
  const invalid = p.minRating > 0 && p.maxRating > 0 && p.minRating > p.maxRating
  return (
    <Modal title={`Conditions — ${p.label || 'prix'}`} onClose={onClose}>
      <div className="stack-form">
        <fieldset className="plain">
          <legend>Catégories d'âge FFE (aucune = toutes)</legend>
          <div className="chips">
            {FFE_CATEGORIES.map(([k, l]) => (
              <label key={k} className={`chip ${cats.some(x => x.toLowerCase() === k.toLowerCase()) ? 'on' : ''}`}>
                <input type="checkbox" checked={cats.some(x => x.toLowerCase() === k.toLowerCase())} onChange={() => toggleCat(k)} />{l}
              </label>
            ))}
          </div>
        </fieldset>
        <div className="form-grid two">
          <label>Elo minimum<input type="number" min={0} value={p.minRating || ''} onChange={e => setP({ ...p, minRating: +e.target.value })} /></label>
          <label>Elo maximum<input type="number" min={0} value={p.maxRating || ''} onChange={e => setP({ ...p, maxRating: +e.target.value })} className={invalid ? 'invalid' : ''} /></label>
          <label>Sexe
            <select value={p.sex} onChange={e => setP({ ...p, sex: e.target.value as Prize['sex'] })}>
              <option value="">Tous</option><option value="f">Féminines</option><option value="m">Masculins</option>
            </select>
          </label>
          <label>Score minimum (points)<input type="number" step={0.5} min={0} value={p.minPoints || ''} onChange={e => setP({ ...p, minPoints: +e.target.value })} /></label>
          <label>Fédérations (ex. FRA,MON)<input value={p.fed} onChange={e => setP({ ...p, fed: e.target.value.toUpperCase() })} /></label>
          <label>Club (nom exact)<input value={p.club} onChange={e => setP({ ...p, club: e.target.value })} /></label>
          <label className="check span-2"><input type="checkbox" checked={p.unrated} onChange={e => setP({ ...p, unrated: e.target.checked })} /> Réservé aux non classés (Elo estimé ou absent)</label>
        </div>
        {invalid && <p className="error small">L'Elo minimum dépasse l'Elo maximum.</p>}
        <p className="muted small">Résumé : {prizeConditions(p)}</p>
        <div className="actions">
          <button className="ghost" onClick={onClose}>Annuler</button>
          <button className="primary" disabled={invalid} onClick={() => onSave(p)}>Valider</button>
        </div>
      </div>
    </Modal>
  )
}
