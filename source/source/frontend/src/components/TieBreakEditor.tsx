import { useTieBreakLabels } from '../admin/t/Standings'

export const TIEBREAK_PRESETS: { id: string; label: string; kind: 'swiss' | 'roundrobin' | 'any'; tbs: string[] }[] = [
  { id: 'fide-swiss', label: 'Suisse FIDE (C.07)', kind: 'swiss', tbs: ['BH-C1', 'BH', 'SB', 'PS'] },
  { id: 'ffe-open', label: 'Open FFE (Bu tronqué, Bu, Perf)', kind: 'swiss', tbs: ['BH-C1', 'BH', 'TPR'] },
  { id: 'cumul', label: 'Cumulatif d\'abord', kind: 'swiss', tbs: ['PS', 'BH-C1', 'BH'] },
  { id: 'fide-rr', label: 'Toutes-rondes FIDE (C.07)', kind: 'roundrobin', tbs: ['DE', 'SB', 'WIN', 'KS'] },
]

/** Ordered tie-break list with presets, reordering and removal. */
export function TieBreakEditor({ value, onChange, kind, disabled }: { value: string[]; onChange: (v: string[]) => void; kind: 'swiss' | 'roundrobin' | 'knockout'; disabled?: boolean }) {
  const labels = useTieBreakLabels()
  const move = (i: number, d: -1 | 1) => {
    const j = i + d
    if (j < 0 || j >= value.length) return
    const c = [...value]; [c[i], c[j]] = [c[j], c[i]]; onChange(c)
  }
  const presets = TIEBREAK_PRESETS.filter(p => p.kind === kind || p.kind === 'any')
  return (
    <div className="tb-editor">
      <div className="chips">
        {presets.map(p => (
          <button type="button" key={p.id} disabled={disabled} className={`chip ${p.tbs.join() === value.join() ? 'on' : ''}`} onClick={() => onChange([...p.tbs])}>{p.label}</button>
        ))}
      </div>
      <ol>
        {value.map((tb, i) => (
          <li key={tb}>
            <span><b>{tb}</b> {labels[tb]}</span>
            {!disabled && (
              <span>
                <button type="button" className="ghost small" onClick={() => move(i, -1)}>↑</button>
                <button type="button" className="ghost small" onClick={() => move(i, 1)}>↓</button>
                <button type="button" className="ghost danger small" onClick={() => onChange(value.filter(y => y !== tb))}>✕</button>
              </span>
            )}
          </li>
        ))}
      </ol>
      {!disabled && (
        <select value="" onChange={e => e.target.value && onChange([...value, e.target.value])}>
          <option value="">Ajouter un départage…</option>
          {Object.entries(labels).filter(([id]) => !value.includes(id)).map(([id, l]) => <option key={id} value={id}>{id} — {l}</option>)}
        </select>
      )}
    </div>
  )
}
