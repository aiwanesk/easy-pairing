import { useState } from 'react'
import { api } from '../../api'
import type { Player } from '../../types'
import { fullName } from '../../util'
import { useAction } from '../../components/Toast'
import type { TProps } from '../TournamentPage'

const CHOICES: { code: '' | 'H' | 'F' | 'Z'; label: string; title: string }[] = [
  { code: '', label: 'Présent', title: 'Apparié normalement' },
  { code: 'H', label: 'Bye ½', title: 'Absence annoncée, ½ point (bye demandé)' },
  { code: 'F', label: 'Bye 1', title: 'Exempté avec 1 point (décision de l\'arbitre)' },
  { code: 'Z', label: 'Absent 0', title: 'Non apparié, 0 point' },
]

/** Quick presence / bye editor for the next round to pair (Swiss). */
export function NextRoundByes({ ev, t, setEv }: TProps) {
  const run = useAction()
  const next = t.rounds.length + 1
  const [filter, setFilter] = useState('')
  const [open, setOpen] = useState(false)
  const players = t.players.filter(p => p.status === 'confirmed').sort((a, b) => fullName(a).localeCompare(fullName(b), 'fr'))
  const withdrawn = players.filter(p => p.withdrawnFrom > 0 && p.withdrawnFrom <= next)
  const active = players.filter(p => !(p.withdrawnFrom > 0 && p.withdrawnFrom <= next))
  const absent = active.filter(p => p.byes[next])
  const f = filter.trim().toLowerCase()
  const shown = active.filter(p => !f || fullName(p).toLowerCase().includes(f) || p.club?.toLowerCase().includes(f))

  const set = async (p: Player, code: string) => {
    const r = await run(() => api.setBye(ev.id, t.id, p.id, { round: next, code }))
    if (r) setEv(r)
  }
  const withdraw = async (p: Player) => {
    if (!confirm(`Retirer ${fullName(p)} du tournoi à partir de la ronde ${next} ? Il ne sera plus apparié.`)) return
    const r = await run(() => api.setBye(ev.id, t.id, p.id, { round: next, withdraw: true }), `${fullName(p)} retiré`)
    if (r) setEv(r)
  }
  const back = async (p: Player) => {
    const r = await run(() => api.setBye(ev.id, t.id, p.id, { round: next, return: true }), `${fullName(p)} réintégré`)
    if (r) setEv(r)
  }

  return (
    <section className="card">
      <div className="card-head">
        <h2>Ronde {next} : présents et exempts</h2>
        <div className="inline">
          <span className="muted small">{active.length - absent.length} apparié(s) · {absent.length} exempt(s)/absent(s){withdrawn.length ? ` · ${withdrawn.length} retiré(s)` : ''}</span>
          <button className="ghost" onClick={() => setOpen(o => !o)}>{open ? 'Réduire' : 'Modifier'}</button>
        </div>
      </div>
      {!open && absent.length > 0 && (
        <p className="small">{absent.map(p => `${fullName(p)} (${CHOICES.find(c => c.code === p.byes[next])?.label})`).join(' · ')}</p>
      )}
      {open && (
        <>
          <input className="search" placeholder="Chercher un joueur…" value={filter} onChange={e => setFilter(e.target.value)} />
          <table className="table compact">
            <tbody>
              {shown.map(p => (
                <tr key={p.id}>
                  <td>{fullName(p)} <span className="muted small">{p.rating || ''} {p.club}</span></td>
                  <td>
                    <div className="seg">
                      {CHOICES.map(c => (
                        <button key={c.code} type="button" title={c.title} className={(p.byes[next] ?? '') === c.code ? 'on' : ''}
                          disabled={c.code === 'H' && p.byeIneligible} onClick={() => set(p, c.code)}>{c.label}</button>
                      ))}
                    </div>
                  </td>
                  <td className="row-actions"><button className="ghost danger small" onClick={() => withdraw(p)}>Retirer</button></td>
                </tr>
              ))}
              {withdrawn.map(p => (
                <tr key={p.id} className="muted">
                  <td>{fullName(p)}</td><td>retiré depuis la ronde {p.withdrawnFrom}</td>
                  <td className="row-actions"><button className="ghost small" onClick={() => back(p)}>Réintégrer</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </section>
  )
}
