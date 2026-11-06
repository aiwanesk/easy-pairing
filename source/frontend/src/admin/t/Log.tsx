import { fmtDate } from '../../util'
import type { TProps } from '../TournamentPage'

const kinds: Record<string, string> = { pairing: 'Appariement', manual: 'Manuel', correction: 'Correction', settings: 'Réglages', publish: 'Publication' }

export function LogTab({ t }: TProps) {
  const entries = [...t.log].reverse()
  return (
    <section className="card">
      <h2>Journal du tournoi</h2>
      <p className="muted small">Appariements, modifications manuelles, corrections de résultats et changements de réglages. Il figure en commentaires dans l'export TRF. Le détail de chaque action (auteur, IP) est dans le journal d'audit.</p>
      {entries.length === 0 ? <p className="muted">Vide.</p> : (
        <table className="table compact">
          <thead><tr><th>Date</th><th>Ronde</th><th>Type</th><th>Message</th><th>Par</th></tr></thead>
          <tbody>
            {entries.map((l, i) => (
              <tr key={i} className={l.kind === 'manual' || l.kind === 'correction' ? 'warn-row' : ''}>
                <td className="nowrap muted">{fmtDate(l.at)}</td>
                <td className="num">{l.round || ''}</td>
                <td>{kinds[l.kind] ?? l.kind}</td>
                <td>{l.message}</td>
                <td>{l.by}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
