import { useEffect, useMemo, useRef, useState } from 'react'
import { ApiError, api, type BulkResult } from '../../api'
import type { ByeCode, LookupPlayer, Player } from '../../types'
import { TITLES, fmtPts, fullName, ratingOf, toPlayer } from '../../util'
import { Modal } from '../../components/Modal'
import { useAction, useToast } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

type SortKey = 'pno' | 'name' | 'rating' | 'club' | 'points'

export function PlayersTab(props: TProps) {
  const { ev, t, d, setEv, canEdit } = props
  const [editing, setEditing] = useState<Player | null>(null)
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<SortKey>('pno')
  const run = useAction()

  const rows = useMemo(() => {
    const f = filter.trim().toLowerCase()
    const list = t.players.filter(p => p.status === 'confirmed' && (!f || [p.lastName, p.firstName, p.club, p.ffeId, p.fideId, p.fed, p.category]
      .some(x => x?.toLowerCase().includes(f))))
    const pno = (p: Player) => d.pairingNumbers[p.id] ?? 999
    const cmp: Record<SortKey, (a: Player, b: Player) => number> = {
      pno: (a, b) => pno(a) - pno(b),
      name: (a, b) => fullName(a).localeCompare(fullName(b), 'fr'),
      rating: (a, b) => b.rating - a.rating,
      club: (a, b) => (a.club || '').localeCompare(b.club || '', 'fr'),
      points: (a, b) => (d.scores[b.id] ?? 0) - (d.scores[a.id] ?? 0) || pno(a) - pno(b),
    }
    return [...list].sort(cmp[sort])
  }, [t, d, filter, sort])

  const th = (key: SortKey, label: string, cls = '') => (
    <th className={`sortable ${cls} ${sort === key ? 'sorted' : ''}`} onClick={() => setSort(key)}>{label}</th>
  )

  const refreshRatings = async () => {
    const r = await run(() => api.refreshRatings(ev.id, t.id), 'Elo mis à jour')
    if (r) setEv(r)
  }

  const remove = async (p: Player) => {
    if (!confirm(`Supprimer ${fullName(p)} du tournoi ?`)) return
    const r = await run(() => api.deletePlayer(ev.id, t.id, p.id), 'Joueur supprimé')
    if (r) setEv(r)
  }

  return (
    <div className="stack">
      {canEdit && <AddPlayer {...props} />}
      <section className="card">
        <div className="card-head">
          <h2>Joueurs <span className="muted">({rows.length})</span></h2>
          <div className="inline">
            <input className="search" placeholder="Filtrer…" value={filter} onChange={e => setFilter(e.target.value)} />
            {canEdit && t.rounds.length === 0 && <button onClick={refreshRatings} title="Recharge l'Elo de la cadence du tournoi depuis la base FFE puis la FIDE">Mettre à jour les Elo</button>}
            <a className="button" href={api.exportUrl(ev.id, t.id, 'players.csv')}>Export CSV</a>
          </div>
        </div>
        {rows.length === 0 ? <p className="muted">Aucun joueur.</p> : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  {th('pno', 'N°', 'num')}<th>Titre</th>{th('name', 'Nom')}{th('rating', 'Elo', 'num')}<th>Féd.</th><th>Cat.</th>
                  {th('club', 'Club')}<th>FIDE</th><th>FFE</th>{t.checkIn && <th>Pointé</th>}{th('points', 'Pts', 'num')}<th>Statut</th><th />
                </tr>
              </thead>
              <tbody>
                {rows.map(p => {
                  const byes = Object.entries(p.byes || {}).filter(([r]) => +r > t.rounds.length)
                  return (
                    <tr key={p.id} className={p.withdrawnFrom ? 'withdrawn' : ''}>
                      <td className="num muted">{t.kind === 'roundrobin' && p.bergerNo ? p.bergerNo : d.pairingNumbers[p.id]}</td>
                      <td className="title">{p.title}</td>
                      <td><button className="link" onClick={() => setEditing(p)}>{fullName(p)}</button></td>
                      <td className="num">{p.rating || '—'}<span className="rtype">{p.rating ? p.ratingType : ''}</span></td>
                      <td>{p.fed}</td>
                      <td className="muted">{p.category}</td>
                      <td className="ellipsis" title={p.club}>{p.club}</td>
                      <td className="mono small">{p.fideId && <a href={`https://ratings.fide.com/profile/${p.fideId}`} target="_blank" rel="noreferrer">{p.fideId}</a>}</td>
                      <td className="mono small">{p.ffeId}</td>
                      {t.checkIn && <td>{p.checkedIn ? <span className="tag ok">oui</span> : <span className="tag">non</span>}</td>}
                      <td className="num strong">{fmtPts(d.scores[p.id])}</td>
                      <td className="small">
                        {p.withdrawnFrom ? <span className="tag bad">Retiré R{p.withdrawnFrom}</span> : null}
                        {byes.map(([r, c]) => <span key={r} className="tag">{c === 'H' ? '½' : c === 'F' ? '1' : '0'} R{r}</span>)}
                        {p.source === 'online' && <span className="tag">en ligne</span>}
                      </td>
                      <td className="row-actions">
                        {canEdit && <><button className="ghost" onClick={() => setEditing(p)}>Modifier</button>
                          <button className="ghost danger" onClick={() => remove(p)} title="Supprimer">✕</button></>}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
      {editing && <PlayerEditor {...props} player={editing} onClose={() => setEditing(null)} />}
    </div>
  )
}

type Mode = 'ffe' | 'fide' | 'manual' | 'bulk'

function AddPlayer(props: TProps) {
  const [mode, setMode] = useState<Mode>('ffe')
  useEffect(() => { api.ffeStatus().then(s => { if (s.state !== 'ready') setMode('fide') }).catch(() => {}) }, [])
  return (
    <section className="card">
      <div className="card-head">
        <h2>Inscrire des joueurs</h2>
        <div className="segmented">
          {([['ffe', 'Base FFE'], ['fide', 'Code FIDE'], ['manual', 'Manuel'], ['bulk', 'Ajout en masse']] as const).map(([m, l]) => (
            <button key={m} className={mode === m ? 'on' : ''} onClick={() => setMode(m)}>{l}</button>
          ))}
        </div>
      </div>
      {mode === 'ffe' ? <FFESearch {...props} /> : mode === 'fide' ? <FIDELookup {...props} /> : mode === 'manual' ? <ManualForm {...props} /> : <BulkAdd {...props} />}
    </section>
  )
}

function useRegister({ ev, t, setEv }: TProps) {
  const run = useAction()
  return async (l: LookupPlayer) => {
    const r = await run(() => api.addPlayer(ev.id, t.id, toPlayer(l, t.ratingKind)), `${l.lastName.toUpperCase()} ${l.firstName} inscrit`)
    if (r) setEv(r)
    return !!r
  }
}

const isRegistered = (t: TProps['t'], l: LookupPlayer) =>
  t.players.some(p => p.status !== 'refused' && ((l.ffeId && p.ffeId === l.ffeId) || (l.fideId && p.fideId === l.fideId)))

function FFESearch(props: TProps) {
  const [q, setQ] = useState('')
  const [res, setRes] = useState<LookupPlayer[]>([])
  const [msg, setMsg] = useState('')
  const register = useRegister(props)
  const inputRef = useRef<HTMLInputElement>(null)
  const seq = useRef(0)

  useEffect(() => {
    const query = q.trim()
    if (query.length < 2) { setRes([]); setMsg(''); return }
    const my = ++seq.current
    const timer = window.setTimeout(async () => {
      try {
        const r = await api.searchFfe(query)
        if (my !== seq.current) return
        setRes(r)
        setMsg(r.length ? '' : 'Aucun joueur trouvé.')
      } catch (e) {
        if (my === seq.current) { setRes([]); setMsg(e instanceof ApiError ? e.message : String(e)) }
      }
    }, 200)
    return () => window.clearTimeout(timer)
  }, [q])

  const add = async (l: LookupPlayer) => { if (await register(l)) { setQ(''); inputRef.current?.focus() } }

  return (
    <div>
      <input ref={inputRef} className="big-input" value={q} onChange={e => setQ(e.target.value)} autoFocus
        onKeyDown={e => { if (e.key === 'Enter' && res.length === 1 && !isRegistered(props.t, res[0])) add(res[0]) }}
        placeholder="N° de licence (K59144), nom (DUPONT Jean) ou code FIDE" />
      {msg && <p className="muted small">{msg}</p>}
      {res.length > 0 && (
        <div className="table-wrap">
          <table className="table compact">
            <thead><tr><th>Licence</th><th>Nom</th><th className="num">Elo</th><th>Cat.</th><th>Féd.</th><th>Club</th><th>Lic.</th><th /></tr></thead>
            <tbody>
              {res.map(l => {
                const done = isRegistered(props.t, l)
                return (
                  <tr key={l.ffeId || l.fideId || l.lastName + l.firstName}>
                    <td className="mono small">{l.ffeId}</td>
                    <td><span className="title">{l.title} </span>{l.lastName.toUpperCase()} {l.firstName}</td>
                    <td className="num">{ratingOf(l, props.t.ratingKind)}</td>
                    <td className="muted">{l.category}</td>
                    <td>{l.fed}</td>
                    <td className="ellipsis" title={l.club}>{l.club}</td>
                    <td><span className={`tag ${l.licence === 'A' ? 'ok' : l.licence === 'N' ? 'bad' : ''}`}>{l.licence || '?'}</span></td>
                    <td className="row-actions"><button className="primary small" disabled={done} onClick={() => add(l)}>{done ? 'Inscrit' : 'Inscrire'}</button></td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function FIDELookup(props: TProps) {
  const [id, setId] = useState('')
  const [found, setFound] = useState<LookupPlayer | null>(null)
  const [loading, setLoading] = useState(false)
  const register = useRegister(props)
  const toast = useToast()

  const search = async (online = false) => {
    if (!/^\d{3,10}$/.test(id.trim())) { toast('Un code FIDE est numérique (ex. 1503014)', 'error'); return }
    setLoading(true)
    setFound(null)
    try { setFound(await api.lookupFide(id.trim(), online)) } catch (e) { toast(e instanceof Error ? e.message : String(e), 'error') } finally { setLoading(false) }
  }

  return (
    <div>
      <form className="inline" onSubmit={e => { e.preventDefault(); search() }}>
        <input className="big-input" value={id} onChange={e => setId(e.target.value)} placeholder="Code FIDE (ex. 1503014)" inputMode="numeric" autoFocus />
        <button className="primary" disabled={loading}>{loading ? 'Recherche…' : 'Rechercher'}</button>
      </form>
      {found && (
        <div className="found">
          <div>
            <div className="found-name"><span className="title">{found.title} </span>{found.lastName.toUpperCase()} {found.firstName}</div>
            <div className="muted small">{[found.fed, found.birthDate && `né(e) ${found.birthDate}`, found.club, found.ffeId && `FFE ${found.ffeId}`].filter(Boolean).join(' · ')}</div>
            <div className="ratings">
              <span>Lent <b>{found.standard || '—'}</b></span><span>Rapide <b>{found.rapid || '—'}</b></span><span>Blitz <b>{found.blitz || '—'}</b></span>
              <span className="muted small">source : {found.source === 'ffe' ? 'base FFE' : 'ratings.fide.com'}</span>
            </div>
          </div>
          <div className="actions">
            {found.source === 'ffe' && <button className="ghost" onClick={() => search(true)}>Vérifier sur fide.com</button>}
            <button className="primary" disabled={isRegistered(props.t, found)} onClick={async () => { if (await register(found)) { setFound(null); setId('') } }}>
              {isRegistered(props.t, found) ? 'Déjà inscrit' : 'Inscrire'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const emptyPlayer: Partial<Player> = { lastName: '', firstName: '', title: '', rating: 0, ratingType: 'E', fed: 'FRA', sex: '', birthDate: '', club: '', fideId: '', ffeId: '', category: '' }

function PlayerFields({ p, set }: { p: Partial<Player>; set: (k: keyof Player) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void }) {
  return (
    <>
      <label>Nom<input value={p.lastName} onChange={set('lastName')} required /></label>
      <label>Prénom<input value={p.firstName} onChange={set('firstName')} /></label>
      <label>Elo<input type="number" min={0} max={3500} value={p.rating || ''} onChange={set('rating')} /></label>
      <label>Type Elo
        <select value={p.ratingType} onChange={set('ratingType')}><option value="">—</option><option value="F">F — FIDE</option><option value="N">N — national</option><option value="E">E — estimé</option></select>
      </label>
      <label>Titre<select value={p.title} onChange={set('title')}>{TITLES.map(x => <option key={x} value={x}>{x || '—'}</option>)}</select></label>
      <label>Fédération<input value={p.fed} onChange={set('fed')} maxLength={3} /></label>
      <label>Club<input value={p.club} onChange={set('club')} /></label>
      <label>Catégorie<input value={p.category} onChange={set('category')} placeholder="PouM, SenF…" /></label>
      <label>Sexe<select value={p.sex} onChange={set('sex')}><option value="">—</option><option value="m">M</option><option value="f">F</option></select></label>
      <label>Naissance<input value={p.birthDate} onChange={set('birthDate')} placeholder="AAAA/MM/JJ" /></label>
      <label>Code FIDE<input value={p.fideId} onChange={set('fideId')} inputMode="numeric" /></label>
      <label>Licence FFE<input value={p.ffeId} onChange={set('ffeId')} /></label>
    </>
  )
}

function useFieldSetter<T>(setP: React.Dispatch<React.SetStateAction<T>>) {
  return (k: keyof Player) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const target = e.target as HTMLInputElement
    const v = target.type === 'checkbox' ? target.checked : ['rating', 'fee', 'paid'].includes(k) ? +target.value : target.value
    setP(x => ({ ...x, [k]: v }))
  }
}

function ManualForm({ ev, t, setEv }: TProps) {
  const [p, setP] = useState<Partial<Player>>(emptyPlayer)
  const run = useAction()
  const set = useFieldSetter(setP)
  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const r = await run(() => api.addPlayer(ev.id, t.id, { ...p, byes: {}, withdrawnFrom: 0 }), 'Joueur inscrit')
    if (r) { setEv(r); setP(emptyPlayer) }
  }
  return (
    <form className="form-grid" onSubmit={submit}>
      <PlayerFields p={p} set={set} />
      <div className="actions span-4"><button className="primary">Inscrire</button></div>
    </form>
  )
}

function BulkAdd({ ev, t, setEv }: TProps) {
  const [text, setText] = useState('')
  const [result, setResult] = useState<BulkResult | null>(null)
  const [busy, setBusy] = useState(false)
  const run = useAction()
  const submit = async () => {
    setBusy(true)
    const r = await run(() => api.bulkAdd(ev.id, t.id, text))
    setBusy(false)
    if (r) {
      setResult(r)
      setEv(r.event)
      setText(r.lines.filter(l => l.error).map(l => l.input).join('\n'))
    }
  }
  return (
    <div>
      <p className="muted small">
        Une ligne par joueur : n° de licence FFE (<code>K59144</code>), code FIDE (<code>1503014</code>) ou
        <code> Nom;Prénom;Elo;Club;Féd.;FIDE;FFE</code> (copier-coller depuis un tableur accepté, séparateur ; , ou tabulation).
        Les lignes en erreur restent dans la zone pour correction.
      </p>
      <textarea className="mono" rows={8} value={text} onChange={e => setText(e.target.value)} placeholder={'K59144\n1503014\nDUPONT;Jean;1850;Échiquier Lancéen'} />
      <div className="actions mt"><button className="primary" disabled={busy || !text.trim()} onClick={submit}>{busy ? 'Recherche des joueurs…' : 'Ajouter'}</button></div>
      {result && (
        <div className="mt">
          <p><b>{result.added}</b> joueur(s) ajouté(s).</p>
          {result.lines.some(l => l.error) && (
            <ul className="error-list">{result.lines.filter(l => l.error).map(l => <li key={l.line}>Ligne {l.line} « {l.input} » : {l.error}</li>)}</ul>
          )}
        </div>
      )}
    </div>
  )
}

export function PlayerEditor({ ev, t, setEv, canEdit, player, onClose }: TProps & { player: Player; onClose: () => void }) {
  const [p, setP] = useState<Player>({ ...player, byes: { ...(player.byes || {}) } })
  const run = useAction()
  const set = useFieldSetter(setP)
  const paired = t.rounds.length

  const setBye = (round: number, code: ByeCode | '') => setP(x => {
    const byes = { ...x.byes }
    if (code) byes[round] = code
    else delete byes[round]
    return { ...x, byes }
  })

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const r = await run(() => api.updatePlayer(ev.id, t.id, p), 'Joueur mis à jour')
    if (r) { setEv(r); onClose() }
  }

  return (
    <Modal title={fullName(player)} onClose={onClose} wide>
      <form onSubmit={save}>
        <fieldset disabled={!canEdit}>
          <div className="form-grid">
            <PlayerFields p={p} set={set} />
            <label>E-mail<input type="email" value={p.email} onChange={set('email')} /></label>
            <label>Téléphone<input value={p.phone} onChange={set('phone')} /></label>
            <label>Droits d'inscription<input type="number" step="0.5" value={p.fee || ''} onChange={set('fee')} /></label>
            <label>Payé<input type="number" step="0.5" value={p.paid || ''} onChange={set('paid')} /></label>
            <label className="span-4">Notes<input value={p.notes} onChange={set('notes')} /></label>
            <label className="check span-4"><input type="checkbox" checked={p.byeIneligible} onChange={set('byeIneligible')} /> Pas de droit aux byes (FIDE C.05:6.7.4)</label>
          </div>
          {paired > 0 && p.rating !== player.rating && <p className="warn small">Modifier l'Elo après la ronde 1 peut changer l'ordre initial ; la modification est journalisée.</p>}

          {t.kind === 'swiss' && (
            <>
              <h3>Absences prévues</h3>
              <div className="bye-grid">
                {Array.from({ length: t.totalRounds }, (_, i) => i + 1).map(r => {
                  const locked = r <= paired
                  const code = locked ? (t.rounds[r - 1]?.absences?.[p.id] ?? '') : (p.byes[r] ?? '')
                  return (
                    <label key={r} className={locked ? 'locked' : ''}>R{r}
                      <select value={code} disabled={locked || (!!p.withdrawnFrom && r >= p.withdrawnFrom)} onChange={e => setBye(r, e.target.value as ByeCode | '')}>
                        <option value="">{locked ? '—' : 'Présent'}</option>
                        <option value="H" disabled={p.byeIneligible}>Bye ½</option><option value="Z">Absent 0</option><option value="F">Bye 1</option>
                      </select>
                    </label>
                  )
                })}
              </div>
            </>
          )}

          <h3>Retrait / forfait général</h3>
          <div className="inline">
            {p.withdrawnFrom ? (
              <><span>Retiré à partir de la ronde <b>{p.withdrawnFrom}</b>.</span>
                <button type="button" className="ghost" onClick={() => setP(x => ({ ...x, withdrawnFrom: 0 }))}>Réintégrer</button></>
            ) : (
              <button type="button" className="ghost danger" disabled={t.kind === 'swiss' && paired >= t.totalRounds}
                onClick={() => setP(x => ({ ...x, withdrawnFrom: paired + 1 }))}>Retirer à partir de la ronde {paired + 1}</button>
            )}
          </div>
        </fieldset>
        <div className="actions end">
          <button type="button" onClick={onClose}>Fermer</button>
          {canEdit && <button className="primary">Enregistrer</button>}
        </div>
      </form>
    </Modal>
  )
}
