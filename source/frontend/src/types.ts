export type ByeCode = 'H' | 'Z' | 'F'
export type Kind = 'swiss' | 'roundrobin' | 'knockout'
export type Level = 'owner' | 'arbiter' | 'results' | 'viewer' | 'admin' | ''

export interface Account {
  id: number
  login: string
  name: string
  email: string
  role: 'admin' | 'manager' | 'user'
  active: boolean
  createdAt: string
  lastLogin: string | null
}

export interface Player {
  id: number
  lastName: string
  firstName: string
  title: string
  rating: number
  ratingType: string
  fed: string
  sex: string
  birthDate: string
  category: string
  club: string
  fideId: string
  ffeId: string
  licence: string
  email: string
  phone: string
  status: 'confirmed' | 'pending' | 'refused'
  source: string
  registeredAt: string
  fee: number
  paid: number
  checkedIn: boolean
  checkedInAt?: string
  byeIneligible: boolean
  bergerNo: number
  withdrawnFrom: number
  byes: Record<string, ByeCode>
  notes: string
}

export interface Board {
  number: number
  white: number
  black: number
  result: string
  source?: string
}

export interface Round {
  number: number
  boards: Board[]
  absences: Record<string, ByeCode>
  pairedAt: string
  published: boolean
  manual: boolean
  engineCheck: string
  matches?: Match[]
  teamMatches?: TeamMatch[]
}

export interface Team { id: number; name: string; club: string; fed: string; captain: string; players: number[]; withdrawnFrom: number }
export interface TeamPoints { win: number; draw: number; loss: number; forfeit: number }
export interface TeamSettings { boards: number; points: TeamPoints; gamePointsFirst: boolean; firstWhiteEven: boolean }
export interface TeamGame { board: number; white: number; black: number; result: string }
export interface TeamMatch { number: number; home: number; away: number; games: TeamGame[]; forfeit?: '' | 'home' | 'away' | 'both' }
export interface TeamStanding { rank: number; teamId: number; mp: number; gp: number; played: number; won: number; drawn: number; lost: number; bh: number }

export interface KOGame { number: number; stage: 'classical' | 'rapid' | 'blitz' | 'armageddon'; white: number; black: number; result: string }
export interface Match { number: number; a: number; b: number; games: KOGame[]; winner: number; decided: string; note?: string }
export interface KOSettings { games: number; tieBreaks: string[]; seeding: 'rating' | 'lots'; final: number }
export interface KOStanding { rank: number; playerId: number; eliminated: number; label: string }

export interface Registration {
  open: boolean
  deadline: string
  maxPlayers: number
  minRating: number
  maxRating: number
  fee: number
  autoConfirm: boolean
  message: string
  showList: boolean
}

export type PrizeSplit = 'place' | 'shared' | 'hort'

export interface Prize {
  id: number
  label: string
  amount: number
  category: string
  minRating: number
  maxRating: number
  sex: '' | 'f' | 'm'
  fed: string
  club: string
  unrated: boolean
  minPoints: number
}

export interface LogEntry {
  at: string
  round: number
  kind: string
  message: string
  by: string
}

export interface TournamentSettings {
  name: string
  kind: Kind
  system: 'dutch' | 'burstein'
  acceleration: 'none' | 'baku'
  doubleRoundRobin: boolean
  reverseLastRounds: boolean
  totalRounds: number
  initialColor: 'white' | 'black'
  ratingKind: 'standard' | 'rapid' | 'blitz'
  timeControl: string
  pointsWin: number
  pointsDraw: number
  pointsPab: number
  tieBreaks: string[]
  unratedValue: number
  fideMode: boolean
  checkIn: boolean
  kioskPin: string
  kioskResults: boolean
  registration: Registration
  chiefArbiter: string
  federation: string
  arbiterNotes: string
  prizeSplit: PrizeSplit
  normsFrom: number
  ko: KOSettings
  team: TeamSettings
}

export interface Tournament extends TournamentSettings {
  id: number
  teams: Team[]
  players: Player[]
  rounds: Round[]
  prizes: Prize[]
  forbiddenGroups: number[][]
  log: LogEntry[]
}

export interface Derived {
  pairingNumbers: Record<string, number>
  scores: Record<string, number>
  plannedRounds: number
  lastPublished: number
  canPair: boolean
  canPairReason?: string
  pendingRegistrations: number
  checkedIn: number
}

export interface EventSettings {
  name: string
  slug: string
  location: string
  startDate: string
  endDate: string
  organizer: string
  contact: string
  public: boolean
  message: string
}

export interface ChessEvent extends EventSettings {
  id: string
  tournaments: Tournament[]
  level: Level
  derived: Record<string, Derived>
  updatedAt: string
}

export interface EventSummary {
  id: string
  slug: string
  name: string
  location: string
  startDate: string
  endDate: string
  public: boolean
  tournaments: number
  players: number
  level: Level
  updatedAt: string
}

export interface Standing {
  rank: number
  playerId: number
  pairingNo: number
  points: number
  games: number
  tieBreaks: Record<string, number>
}

export interface PrizeAward {
  prize: Prize
  playerId: number
  tied: number[]
}

export interface Payout {
  playerId: number
  rank: number
  points: number
  prizeId: number
  nominal: number
  amount: number
  sharedWith: number[]
  detail: string
}

export interface PrizeResult {
  split: PrizeSplit
  round: number
  awards: PrizeAward[]
  payouts: Payout[]
  total: number
}

export interface LookupPlayer {
  source: 'fide' | 'ffe'
  lastName: string
  firstName: string
  title: string
  fed: string
  sex: string
  birthDate: string
  category: string
  club: string
  fideId: string
  ffeId: string
  licence: string
  standard: number
  standardType: string
  rapid: number
  rapidType: string
  blitz: number
  blitzType: string
}

export interface FFEStatus {
  state: 'empty' | 'loading' | 'ready' | 'error'
  error?: string
  players: number
  loadedAt: string
  fileDate: string
  url: string
}

export interface EngineStatus {
  ready: boolean
  path: string
  source: string
  version: string
}

export interface SystemInfo {
  version: string
  engine: EngineStatus
  ffe: FFEStatus
  dataDir: string
  logDir: string
}

export interface Permission {
  accountId: number
  name: string
  login: string
  level: Level
}

export interface AuditEntry {
  id: number
  at: string
  accountId: number
  accountName: string
  ip: string
  eventId: string
  tournamentId: number
  action: string
  target: string
  details: string
}

export interface TieBreakDef {
  id: string
  label: string
}

// ---- public

export interface PublicPlayer {
  id: number
  title: string
  lastName: string
  firstName: string
  rating: number
  ratingType: string
  fed: string
  club: string
  category: string
  status: string
  withdrawnFrom: number
  checkedIn: boolean
  pairingNo: number
  bergerNo: number
  points: number
}

export interface PublicTournament {
  id: number
  name: string
  kind: Kind
  system: string
  plannedRounds: number
  timeControl: string
  pointsWin: number
  pointsDraw: number
  tieBreaks: string[]
  checkIn: boolean
  started: boolean
  lastPublished: number
  kiosk: boolean
  kioskResults: boolean
  registration: Registration & { count: number; available: boolean; closedReason?: string }
  players: PublicPlayer[]
  rounds: Round[]
  standings: Standing[]
  ko?: KOStanding[]
  totalRounds: number
  teams?: Team[]
  team?: TeamSettings
  teamStandings?: TeamStanding[]
}

export interface PublicEvent {
  slug: string
  name: string
  location: string
  startDate: string
  endDate: string
  organizer: string
  contact: string
  message: string
  tournaments: PublicTournament[]
}

export interface NetworkInfo {
  lan: boolean
  forced: boolean
  local: string
  addresses: string[]
  error?: string
  publicUrl?: string
  server: boolean
}

export interface NormCondition { label: string; ok: boolean; detail: string }
export interface NormStatus {
  playerId: number
  norm: 'GM' | 'IM' | 'WGM' | 'WIM'
  label: string
  games: number
  score: number
  ra: number
  rp: number
  achieved: boolean
  possible: boolean
  close: boolean
  missing: number
  need: string
  conditions: NormCondition[]
}
export interface NormReport { round: number; remaining: number; disabled?: string; notes: string[]; players: NormStatus[] }
