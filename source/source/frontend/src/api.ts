import type {
  Account, AuditEntry, ChessEvent, EventSettings, EventSummary, FFEStatus, LookupPlayer, Permission, Player,
  Prize, PrizeResult, PrizeSplit, PublicEvent, Standing, SystemInfo, NetworkInfo, NormReport, KOStanding, Team, TeamStanding, TieBreakDef, TournamentSettings,
} from './types'

export class ApiError extends Error {
  status: number
  details?: string[]
  constructor(status: number, message: string, details?: string[]) {
    super(message)
    this.status = status
    this.details = details
  }
}

/** Called on 401 for admin calls, so the app can go back to the login. */
let onUnauthorized: (() => void) | null = null
export const setUnauthorizedHandler = (fn: () => void) => { onUnauthorized = fn }

async function request<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<T> {
  const init: RequestInit = { method, headers: { 'X-Requested-With': 'pairing-easy', ...headers }, credentials: 'same-origin' }
  if (body instanceof FormData) {
    init.body = body
  } else if (body !== undefined) {
    init.body = JSON.stringify(body)
    ;(init.headers as Record<string, string>)['Content-Type'] = 'application/json'
  }
  const res = await fetch(`/api${path}`, init)
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let data: unknown
  try { data = text ? JSON.parse(text) : undefined } catch { data = undefined }
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/public') && path !== '/login' && onUnauthorized) onUnauthorized()
    const e = data as { error?: string; details?: string[] } | undefined
    throw new ApiError(res.status, e?.error ?? `Erreur HTTP ${res.status}`, e?.details)
  }
  return data as T
}

const ev = (id: string) => `/events/${encodeURIComponent(id)}`
const tr = (eid: string, tid: number) => `${ev(eid)}/tournaments/${tid}`

export interface SessionInfo { account: Account | null; setupRequired: boolean; version: string }
export interface BulkResult { added: number; lines: { line: number; input: string; player?: string; error?: string }[]; event: ChessEvent }

export const api = {
  session: () => request<SessionInfo>('GET', '/session'),
  setup: (b: { login: string; name: string; email: string; password: string }) => request<{ account: Account }>('POST', '/setup', b),
  login: (login: string, password: string) => request<{ account: Account }>('POST', '/login', { login, password }),
  logout: () => request<void>('POST', '/logout'),
  changePassword: (current: string, next: string) => request<void>('POST', '/me/password', { current, new: next }),

  accounts: () => request<Account[]>('GET', '/accounts'),
  createAccount: (b: { login: string; name: string; email: string; password: string; role: string }) => request<Account>('POST', '/accounts', b),
  updateAccount: (a: Account) => request<Account>('PUT', `/accounts/${a.id}`, a),
  resetPassword: (id: number, password: string) => request<void>('POST', `/accounts/${id}/password`, { password }),
  deleteAccount: (id: number) => request<void>('DELETE', `/accounts/${id}`),
  directory: () => request<{ id: number; name: string; login: string }[]>('GET', '/directory'),
  audit: (q: Record<string, string | number>) => {
    const p = new URLSearchParams(Object.entries(q).filter(([, v]) => v !== '' && v !== 0).map(([k, v]) => [k, String(v)]))
    return request<{ entries: AuditEntry[]; total: number }>('GET', `/audit?${p}`)
  },
  system: () => request<SystemInfo>('GET', '/system'),
  network: () => request<NetworkInfo>('GET', '/network'),
  setNetwork: (lan: boolean) => request<NetworkInfo>('PUT', '/network', { lan }),
  reinstallEngine: () => request<SystemInfo['engine']>('POST', '/system/engine'),
  ffeStatus: () => request<FFEStatus>('GET', '/ffe/status'),
  ffeDownload: () => request<FFEStatus>('POST', '/ffe/download'),
  ffeUpload: (file: File) => { const fd = new FormData(); fd.append('file', file); return request<FFEStatus>('POST', '/ffe/upload', fd) },
  searchFfe: (q: string) => request<LookupPlayer[]>('GET', `/lookup/ffe?q=${encodeURIComponent(q)}`),
  lookupFide: (id: string, online = false) => request<LookupPlayer>('GET', `/lookup/fide/${encodeURIComponent(id)}${online ? '?online=1' : ''}`),
  tieBreaks: () => request<TieBreakDef[]>('GET', '/tiebreaks'),

  events: () => request<EventSummary[]>('GET', '/events'),
  createEvent: (s: Partial<EventSettings>) => request<ChessEvent>('POST', '/events', s),
  event: (id: string) => request<ChessEvent>('GET', ev(id)),
  updateEvent: (id: string, s: EventSettings) => request<ChessEvent>('PUT', ev(id), s),
  exportEventUrl: (id: string) => `/api${ev(id)}/export`,
  importEvent: async (file: File, replace = false) => request<ChessEvent>('POST', `/events/import${replace ? '?replace=1' : ''}`, JSON.parse(await file.text())),
  deleteEvent: (id: string) => request<void>('DELETE', ev(id)),
  permissions: (id: string) => request<Permission[]>('GET', `${ev(id)}/permissions`),
  setPermission: (id: string, accountId: number, level: string) => request<Permission[]>('PUT', `${ev(id)}/permissions`, { accountId, level }),
  streamUrl: (id: string) => `/api${ev(id)}/stream`,

  createTournament: (eid: string, s: Partial<TournamentSettings>) => request<ChessEvent>('POST', `${ev(eid)}/tournaments`, s),
  updateTournament: (eid: string, tid: number, s: TournamentSettings) => request<ChessEvent>('PUT', tr(eid, tid), s),
  deleteTournament: (eid: string, tid: number) => request<ChessEvent>('DELETE', tr(eid, tid)),
  addPlayer: (eid: string, tid: number, p: Partial<Player>) => request<ChessEvent>('POST', `${tr(eid, tid)}/players`, p),
  bulkAdd: (eid: string, tid: number, text: string) => request<BulkResult>('POST', `${tr(eid, tid)}/players/bulk`, { text }),
  updatePlayer: (eid: string, tid: number, p: Player) => request<ChessEvent>('PUT', `${tr(eid, tid)}/players/${p.id}`, p),
  deletePlayer: (eid: string, tid: number, pid: number) => request<ChessEvent>('DELETE', `${tr(eid, tid)}/players/${pid}`),
  setBye: (eid: string, tid: number, pid: number, b: { round: number; code?: string; withdraw?: boolean; return?: boolean }) =>
    request<ChessEvent>('POST', `${tr(eid, tid)}/players/${pid}/bye`, b),
  checkIn: (eid: string, tid: number, pid: number, checkedIn: boolean) => request<ChessEvent>('POST', `${tr(eid, tid)}/players/${pid}/checkin`, { checkedIn }),
  checkInAll: (eid: string, tid: number, checkedIn: boolean) => request<ChessEvent>('POST', `${tr(eid, tid)}/checkin`, { checkedIn }),
  setStatus: (eid: string, tid: number, pid: number, status: string) => request<ChessEvent>('POST', `${tr(eid, tid)}/players/${pid}/status`, { status }),
  setForbidden: (eid: string, tid: number, groups: number[][]) => request<ChessEvent>('PUT', `${tr(eid, tid)}/forbidden`, groups),
  setPrizes: (eid: string, tid: number, prizes: Prize[], split: PrizeSplit) => request<ChessEvent>('PUT', `${tr(eid, tid)}/prizes`, { prizes, split }),
  norms: (eid: string, tid: number, round?: number) => request<NormReport>('GET', `${tr(eid, tid)}/norms${round !== undefined ? `?round=${round}` : ''}`),
  refreshRatings: (eid: string, tid: number) => request<ChessEvent>('POST', `${tr(eid, tid)}/ratings/refresh`),
  prizes: (eid: string, tid: number) => request<PrizeResult>('GET', `${tr(eid, tid)}/prizes`),
  standings: (eid: string, tid: number, round?: number) =>
    request<{ round: number; standings: Standing[]; ko?: KOStanding[]; teams?: TeamStanding[] }>('GET', `${tr(eid, tid)}/standings${round !== undefined ? `?round=${round}` : ''}`),
  pair: (eid: string, tid: number, order?: string, numbers?: Record<number, number>) =>
    request<ChessEvent>('POST', `${tr(eid, tid)}/rounds`, order ? { order, numbers } : undefined),
  deleteLastRound: (eid: string, tid: number) => request<ChessEvent>('DELETE', `${tr(eid, tid)}/rounds/last`),
  resetRoundRobin: (eid: string, tid: number) => request<ChessEvent>('POST', `${tr(eid, tid)}/rounds/reset`),
  setResult: (eid: string, tid: number, n: number, b: number, result: string) => request<ChessEvent>('PUT', `${tr(eid, tid)}/rounds/${n}/boards/${b}`, { result }),
  publish: (eid: string, tid: number, n: number, published: boolean) => request<ChessEvent>('POST', `${tr(eid, tid)}/rounds/${n}/publish`, { published }),
  manualPairing: (eid: string, tid: number, n: number, boards: { white: number; black: number }[]) =>
    request<ChessEvent>('PUT', `${tr(eid, tid)}/rounds/${n}/pairing`, { boards }),
  setTeams: (eid: string, tid: number, teams: Team[]) => request<ChessEvent>('PUT', `${tr(eid, tid)}/teams`, teams),
  teamResult: (eid: string, tid: number, n: number, m: number, b: number, result: string) =>
    request<ChessEvent>('PUT', `${tr(eid, tid)}/rounds/${n}/teammatches/${m}/boards/${b}`, { result }),
  teamLineup: (eid: string, tid: number, n: number, m: number, side: 'home' | 'away', players: number[]) =>
    request<ChessEvent>('PUT', `${tr(eid, tid)}/rounds/${n}/teammatches/${m}/lineup`, { side, players }),
  teamForfeit: (eid: string, tid: number, n: number, m: number, who: string) =>
    request<ChessEvent>('POST', `${tr(eid, tid)}/rounds/${n}/teammatches/${m}/forfeit`, { who }),
  koResult: (eid: string, tid: number, n: number, m: number, g: number, result: string) =>
    request<ChessEvent>('PUT', `${tr(eid, tid)}/rounds/${n}/matches/${m}/games/${g}`, { result }),
  koTieBreak: (eid: string, tid: number, n: number, m: number) => request<ChessEvent>('POST', `${tr(eid, tid)}/rounds/${n}/matches/${m}/tiebreak`),
  koWinner: (eid: string, tid: number, n: number, m: number, winner: number, note: string) =>
    request<ChessEvent>('POST', `${tr(eid, tid)}/rounds/${n}/matches/${m}/winner`, { winner, note }),
  checkRound: (eid: string, tid: number, n: number) => request<{ differences: string[] }>('POST', `${tr(eid, tid)}/rounds/${n}/check`),
  exportUrl: (eid: string, tid: number, format: string) => `/api${tr(eid, tid)}/export/${format}`,

  // public
  publicEvents: () => request<EventSummary[]>('GET', '/public/events'),
  publicEvent: (slug: string) => request<PublicEvent>('GET', `/public/e/${encodeURIComponent(slug)}`),
  publicStreamUrl: (slug: string) => `/api/public/e/${encodeURIComponent(slug)}/stream`,
  publicLookup: (q: string) => request<{ ffeId: string; lastName: string; firstName: string; club: string; category: string; standard: number; rapid: number; blitz: number }[]>('GET', `/public/lookup/ffe?q=${encodeURIComponent(q)}`),
  register: (slug: string, tid: number, b: Record<string, unknown>) =>
    request<{ status: string; player: string }>('POST', `/public/e/${encodeURIComponent(slug)}/t/${tid}/register`, b),
  kioskAuth: (slug: string, tid: number, pin: string) => request<{ token: string }>('POST', `/public/e/${encodeURIComponent(slug)}/t/${tid}/kiosk/auth`, { pin }),
  kiosk: (slug: string, tid: number, token: string) =>
    request<{ event: string; tournament: PublicEvent['tournaments'][number] }>('GET', `/public/e/${encodeURIComponent(slug)}/t/${tid}/kiosk`, undefined, { 'X-Kiosk-Token': token }),
  kioskCheckIn: (slug: string, tid: number, token: string, playerId: number) =>
    request<{ player: string }>('POST', `/public/e/${encodeURIComponent(slug)}/t/${tid}/kiosk/checkin`, { playerId }, { 'X-Kiosk-Token': token }),
  kioskResult: (slug: string, tid: number, token: string, round: number, board: number, result: string) =>
    request<void>('POST', `/public/e/${encodeURIComponent(slug)}/t/${tid}/kiosk/result`, { round, board, result }, { 'X-Kiosk-Token': token }),
}
