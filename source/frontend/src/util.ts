import { useEffect, useRef } from 'react'
import type { Board, EventSettings, LookupPlayer, Player, Round, Tournament, TournamentSettings } from './types'

export const fmtPts = (n: number | undefined) => {
  if (n === undefined || Number.isNaN(n)) return ''
  const whole = Math.floor(n)
  const frac = Math.round((n - whole) * 100) / 100
  if (frac === 0.5) return whole === 0 ? '½' : `${whole}½`
  return Number.isInteger(n) ? String(n) : String(Math.round(n * 100) / 100)
}

export const fmtNum = (n: number | undefined) => (n === undefined ? '' : String(Math.round(n * 100) / 100))

export const fullName = (p: { lastName: string; firstName: string }) => `${p.lastName.toUpperCase()} ${p.firstName}`.trim()

export const fmtDate = (s?: string | null) => (s ? new Date(s).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' }) : '')

export const resultLabel: Record<string, string> = {
  '1-0': '1 – 0', '0-1': '0 – 1', '1/2': '½ – ½', '+-': '+ / −', '-+': '− / +', '--': '− / −', PAB: 'Exempt', FREE: 'Repos', '': '',
}

export const byeLabel: Record<string, string> = { H: 'Bye ½ pt', Z: 'Absent', F: 'Bye 1 pt' }

export const levelLabel: Record<string, string> = { owner: 'Propriétaire', arbiter: 'Arbitre', results: 'Saisie des résultats', viewer: 'Lecture seule', admin: 'Administrateur', '': '—' }

export const TITLES = ['', 'GM', 'IM', 'WGM', 'FM', 'WIM', 'CM', 'WFM', 'WCM']

export function toPlayer(l: LookupPlayer, kind: TournamentSettings['ratingKind']): Partial<Player> {
  const rating = kind === 'rapid' ? l.rapid : kind === 'blitz' ? l.blitz : l.standard
  const ratingType = kind === 'rapid' ? l.rapidType : kind === 'blitz' ? l.blitzType : l.standardType
  return {
    lastName: l.lastName, firstName: l.firstName, title: l.title, rating: rating || 0, ratingType: ratingType || '',
    fed: l.fed, sex: l.sex, birthDate: l.birthDate, category: l.category, club: l.club, fideId: l.fideId, ffeId: l.ffeId,
    licence: l.licence, byes: {}, withdrawnFrom: 0,
  }
}

export function ratingOf(l: LookupPlayer, kind: TournamentSettings['ratingKind']) {
  const r = kind === 'rapid' ? l.rapid : kind === 'blitz' ? l.blitz : l.standard
  const t = kind === 'rapid' ? l.rapidType : kind === 'blitz' ? l.blitzType : l.standardType
  return r ? `${r}${t ? ` ${t}` : ''}` : '—'
}

/** Points a player scored on a board (or with an absence) in a round. */
export function pointsIn(r: Round, id: number, win: number, draw: number, pab: number, kind: string): number | null {
  const b = r.boards.find(x => x.white === id || x.black === id)
  if (b) {
    if (b.black === 0 || b.white === 0) return kind === 'roundrobin' || b.result === 'FREE' ? 0 : pab
    switch (b.result) {
      case '1-0': case '+-': return b.white === id ? win : 0
      case '0-1': case '-+': return b.black === id ? win : 0
      case '1/2': return draw
      case '--': return 0
      default: return null
    }
  }
  const a = r.absences?.[id]
  if (a === 'H') return draw
  if (a === 'F') return win
  if (a === 'Z') return 0
  return null
}

/** Score of every player before round `upTo` (exclusive, 1-based). */
export function scoresBefore(rounds: Round[], upTo: number, win: number, draw: number, pab: number, kind: string) {
  const m = new Map<number, number>()
  for (const r of rounds.slice(0, upTo - 1)) {
    const ids = new Set<number>()
    r.boards.forEach(b => { if (b.white) ids.add(b.white); if (b.black) ids.add(b.black) })
    Object.keys(r.absences || {}).forEach(k => ids.add(+k))
    ids.forEach(id => m.set(id, (m.get(id) ?? 0) + (pointsIn(r, id, win, draw, pab, kind) ?? 0)))
  }
  return m
}

/** Crosstable cell for player id in round r: "12w1", "+", "½" … */
/**
 * Cell of the crosstable. FFE style (as in Papi): result sign, opponent's
 * place, colour — "+12B", "=5N", "-3B"; forfeits "+F12" / "-F12"; EXE for
 * the pairing bye. FIDE style: "12w1", "5b½".
 */
export function crossCell(r: Round, id: number, rank: Map<number, number>, style: 'ffe' | 'fide' = 'ffe'): { text: string; cls: string } {
  const b: Board | undefined = r.boards.find(x => x.white === id || x.black === id)
  if (!b) {
    const a = r.absences?.[id]
    if (!a) return { text: '', cls: '' }
    if (style === 'fide') return { text: a === 'H' ? '½' : a === 'F' ? '1' : '0', cls: 'muted' }
    return { text: a === 'H' ? '=BYE' : a === 'F' ? '+BYE' : 'ABS', cls: 'muted' }
  }
  if (b.black === 0 || b.white === 0) return { text: b.result === 'FREE' ? (style === 'ffe' ? 'REPOS' : '—') : (style === 'ffe' ? 'EXE' : '+'), cls: 'muted' }
  const white = b.white === id
  const opp = rank.get(white ? b.black : b.white) ?? '?'
  const res = b.result
  const won = (res === '1-0' && white) || (res === '0-1' && !white)
  const lost = (res === '0-1' && white) || (res === '1-0' && !white)
  const fWon = (res === '+-' && white) || (res === '-+' && !white)
  const fLost = (res === '-+' && white) || (res === '+-' && !white) || res === '--'
  const cls = won || fWon ? 'win' : lost || fLost ? 'loss' : ''
  if (style === 'fide') {
    const s = res === '1/2' ? '½' : won ? '1' : lost ? '0' : fWon ? '+' : fLost ? '−' : ''
    return { text: `${opp}${white ? 'w' : 'b'}${s}`, cls }
  }
  const col = white ? 'B' : 'N'
  if (fWon) return { text: `+F${opp}`, cls }
  if (fLost) return { text: `-F${opp}`, cls }
  const sign = res === '1/2' ? '=' : won ? '+' : lost ? '-' : ''
  return { text: `${sign}${opp}${col}`, cls }
}

/** Subscribes to server-sent update notifications. */
export function useLiveUpdates(url: string | null, onUpdate: () => void) {
  const cb = useRef(onUpdate)
  cb.current = onUpdate
  useEffect(() => {
    if (!url) return
    let es: EventSource | null = new EventSource(url)
    let timer = 0
    es.addEventListener('update', () => {
      window.clearTimeout(timer)
      timer = window.setTimeout(() => cb.current(), 150)
    })
    return () => { es?.close(); es = null; window.clearTimeout(timer) }
  }, [url])
}

export const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0)

export function tournamentSettings(t: Tournament): TournamentSettings {
  return {
    name: t.name, kind: t.kind, system: t.system, acceleration: t.acceleration, doubleRoundRobin: t.doubleRoundRobin, reverseLastRounds: t.reverseLastRounds,
    totalRounds: t.totalRounds, initialColor: t.initialColor, ratingKind: t.ratingKind, timeControl: t.timeControl, pointsWin: t.pointsWin,
    pointsDraw: t.pointsDraw, pointsPab: t.pointsPab, tieBreaks: [...t.tieBreaks], unratedValue: t.unratedValue, fideMode: t.fideMode,
    checkIn: t.checkIn, kioskPin: t.kioskPin, kioskResults: t.kioskResults, registration: { ...t.registration },
    chiefArbiter: t.chiefArbiter, federation: t.federation, arbiterNotes: t.arbiterNotes, prizeSplit: t.prizeSplit, normsFrom: t.normsFrom, ko: { ...t.ko, tieBreaks: [...(t.ko?.tieBreaks ?? [])] }, team: { ...t.team, points: { ...t.team.points } },
  }
}

export function eventSettings(ev: EventSettings): EventSettings {
  return {
    name: ev.name, slug: ev.slug, location: ev.location, startDate: ev.startDate, endDate: ev.endDate,
    organizer: ev.organizer, contact: ev.contact, public: ev.public, message: ev.message,
  }
}

export const FFE_CATEGORIES: [string, string][] = [
  ['Ppo', 'Petits-poussins'], ['Pou', 'Poussins'], ['Pup', 'Pupilles'], ['Ben', 'Benjamins'], ['Min', 'Minimes'],
  ['Cad', 'Cadets'], ['Jun', 'Juniors'], ['Sen', 'Seniors'], ['Sep', 'Seniors plus'], ['Vet', 'Vétérans'],
]

export const PRIZE_SPLITS: { id: 'place' | 'shared' | 'hort'; label: string; help: string }[] = [
  { id: 'place', label: 'À la place (départages)', help: 'Chaque prix va au joueur classé à cette place après départages.' },
  { id: 'hort', label: 'Système Hort', help: 'Entre ex aequo : la moitié de chaque prix suit le classement aux départages, l\'autre moitié de la somme est partagée à parts égales.' },
  { id: 'shared', label: 'Partage égal', help: 'Les ex aequo se partagent à parts égales la somme des prix des places qu\'ils occupent.' },
]

export const money = (n: number) => n.toLocaleString('fr-FR', { minimumFractionDigits: Number.isInteger(n) ? 0 : 2, maximumFractionDigits: 2 })

export const roleLabel: Record<string, string> = { admin: 'Administrateur', manager: 'Gestionnaire', user: 'Utilisateur' }

export function koRoundName(total: number, n: number) {
  switch (total - n) {
    case 0: return 'Finale'
    case 1: return 'Demi-finales'
    case 2: return 'Quarts de finale'
    case 3: return 'Huitièmes de finale'
  }
  return `Tour ${n}`
}

export const KO_STAGES: Record<string, string> = { classical: 'Partie', rapid: 'Rapide', blitz: 'Blitz', armageddon: 'Armageddon' }

export const kindLabel = (t: { kind: string; system?: string; acceleration?: string; doubleRoundRobin?: boolean; team?: { boards: number } }) =>
  (t.team?.boards ? `Par équipes (${t.team.boards} éch.) · ` : '') + (t.kind === 'swiss' ? `Suisse ${t.system === 'dutch' ? 'hollandais' : 'Burstein'}${t.acceleration === 'baku' ? ' + Baku' : ''}`
    : t.kind === 'knockout' ? 'Coupe (élimination directe)' : `Toutes-rondes${t.doubleRoundRobin ? ' double' : ''}`)

export const TEAM_POINTS: { id: string; label: string; points: { win: number; draw: number; loss: number; forfeit: number } }[] = [
  { id: 'fide', label: 'FIDE (2 / 1 / 0)', points: { win: 2, draw: 1, loss: 0, forfeit: 0 } },
  { id: 'ffe', label: 'Interclubs FFE (3 / 2 / 1, forfait 0)', points: { win: 3, draw: 2, loss: 1, forfeit: 0 } },
]
