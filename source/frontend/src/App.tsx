import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router'
import { api, setUnauthorizedHandler, type SessionInfo } from './api'
import type { Account } from './types'
import { ToastProvider } from './components/Toast'
import { AdminLayout } from './admin/Layout'
import { Login, Setup } from './admin/Login'
import { EventsPage } from './admin/Events'
import { EventPage } from './admin/EventPage'
import { TournamentPage } from './admin/TournamentPage'
import { AccountsPage } from './admin/Accounts'
import { AuditPage } from './admin/Audit'
import { SystemPage } from './admin/System'
import { ProfilePage } from './admin/Profile'
import { PublicHome } from './public/PublicHome'
import { PublicEventPage } from './public/PublicEvent'
import { RegisterPage } from './public/Register'
import { KioskPage } from './public/Kiosk'
import { ScreenPage } from './public/Screen'

interface SessionCtx {
  account: Account | null
  version: string
  refresh: () => Promise<void>
}
const Ctx = createContext<SessionCtx>({ account: null, version: '', refresh: async () => {} })
export const useSession = () => useContext(Ctx)

function RequireAuth({ session, children }: { session: SessionInfo; children: React.ReactNode }) {
  const loc = useLocation()
  if (session.setupRequired) return <Navigate to="/admin/installation" replace />
  if (!session.account) return <Navigate to={`/admin/connexion?next=${encodeURIComponent(loc.pathname)}`} replace />
  return <>{children}</>
}

export default function App() {
  const [session, setSession] = useState<SessionInfo | null>(null)
  const navigate = useNavigate()

  const refresh = useCallback(async () => {
    try {
      setSession(await api.session())
    } catch {
      setSession({ account: null, setupRequired: false, version: '' })
    }
  }, [])
  useEffect(() => { refresh() }, [refresh])
  useEffect(() => {
    setUnauthorizedHandler(() => {
      setSession(s => (s ? { ...s, account: null } : s))
      navigate(`/admin/connexion?next=${encodeURIComponent(window.location.pathname)}`)
    })
  }, [navigate])

  if (!session) return <div className="center-page muted">Chargement…</div>

  const admin = (el: React.ReactNode) => (
    <RequireAuth session={session}><AdminLayout>{el}</AdminLayout></RequireAuth>
  )

  return (
    <Ctx.Provider value={{ account: session.account, version: session.version, refresh }}>
      <ToastProvider>
        <Routes>
          <Route path="/" element={<PublicHome />} />
          <Route path="/e/:slug" element={<PublicEventPage />} />
          <Route path="/e/:slug/t/:tid" element={<PublicEventPage />} />
          <Route path="/e/:slug/t/:tid/:view" element={<PublicEventPage />} />
          <Route path="/e/:slug/inscription/:tid" element={<RegisterPage />} />
          <Route path="/e/:slug/kiosque/:tid" element={<KioskPage />} />
          <Route path="/e/:slug/ecran" element={<ScreenPage />} />

          <Route path="/admin/installation" element={session.setupRequired ? <Setup /> : <Navigate to="/admin" replace />} />
          <Route path="/admin/connexion" element={<Login />} />
          <Route path="/admin" element={admin(<EventsPage />)} />
          <Route path="/admin/e/:eid" element={admin(<EventPage />)} />
          <Route path="/admin/e/:eid/:tab" element={admin(<EventPage />)} />
          <Route path="/admin/e/:eid/t/:tid" element={admin(<TournamentPage />)} />
          <Route path="/admin/e/:eid/t/:tid/:tab" element={admin(<TournamentPage />)} />
          <Route path="/admin/e/:eid/t/:tid/:tab/:sub" element={admin(<TournamentPage />)} />
          <Route path="/admin/comptes" element={admin(<AccountsPage />)} />
          <Route path="/admin/journal" element={admin(<AuditPage />)} />
          <Route path="/admin/systeme" element={admin(<SystemPage />)} />
          <Route path="/admin/profil" element={admin(<ProfilePage />)} />
          <Route path="*" element={<div className="center-page"><h2>Page introuvable</h2><a href="/">Accueil</a></div>} />
        </Routes>
      </ToastProvider>
    </Ctx.Provider>
  )
}
