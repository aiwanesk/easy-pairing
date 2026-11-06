import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { useSession } from '../App'
import type { FFEStatus, SystemInfo } from '../types'
import { fmtDate } from '../util'
import { useAction, useToast } from '../components/Toast'

export function SystemPage() {
  const [sys, setSys] = useState<SystemInfo | null>(null)
  const [busy, setBusy] = useState(false)
  const run = useAction()
  const { account } = useSession()
  const load = () => api.system().then(setSys).catch(() => {})
  useEffect(() => { load() }, [])

  const reinstall = async () => {
    setBusy(true)
    await run(() => api.reinstallEngine(), 'Moteur réinstallé')
    setBusy(false)
    load()
  }

  return (
    <div className="page">
      <div className="page-head"><h1>Système</h1></div>
      <section className="card">
        <h2>Moteur d'appariement</h2>
        {sys && (
          <dl className="kv">
            <dt>État</dt><dd>{sys.engine.ready ? <span className="tag ok">prêt</span> : <span className="tag bad">absent</span>}</dd>
            <dt>Origine</dt><dd>{sys.engine.source || '—'}</dd>
            <dt>Fichier</dt><dd className="mono small">{sys.engine.path || '—'}</dd>
          </dl>
        )}
        <p className="muted small">
          bbpPairings (système hollandais FIDE et Burstein). Au démarrage, la dernière version officielle est téléchargée depuis GitHub ;
          sans connexion, la copie intégrée à l'exécutable Windows est utilisée.
        </p>
        {account?.role === 'admin' && <button onClick={reinstall} disabled={busy}>{busy ? 'Installation…' : 'Retélécharger la dernière version'}</button>}
      </section>
      <FFEPanel onChange={load} />
      <section className="card">
        <h2>Fichiers</h2>
        {sys && (
          <dl className="kv">
            <dt>Version</dt><dd>{sys.version}</dd>
            <dt>Données</dt><dd className="mono small">{sys.dataDir}</dd>
            <dt>Journaux techniques</dt><dd className="mono small">{sys.logDir}</dd>
          </dl>
        )}
        <p className="muted small">La base de données (pairing_easy.db) contient tous les événements, comptes et le journal d'audit : sauvegardez ce dossier.</p>
      </section>
    </div>
  )
}

export function FFEPanel({ onChange }: { onChange?: () => void }) {
  const [status, setStatus] = useState<FFEStatus | null>(null)
  const [busy, setBusy] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const run = useAction()
  const toast = useToast()

  useEffect(() => { api.ffeStatus().then(setStatus).catch(() => {}) }, [])
  useEffect(() => {
    if (status?.state !== 'loading') return
    const id = window.setInterval(async () => {
      const s = await api.ffeStatus().catch(() => null)
      if (!s) return
      setStatus(s)
      if (s.state !== 'loading') {
        onChange?.()
        if (s.state === 'ready' && !s.error) toast(`Base FFE chargée : ${s.players.toLocaleString('fr-FR')} joueurs`, 'success')
        if (s.error) toast(s.error, 'error')
      }
    }, 1500)
    return () => window.clearInterval(id)
  }, [status?.state, onChange, toast])

  const upload = async (f: File) => {
    setBusy(true)
    const s = await run(() => api.ffeUpload(f), 'Base FFE importée')
    setBusy(false)
    if (s) { setStatus(s); onChange?.() }
  }

  return (
    <section className="card">
      <h2>Base joueurs FFE (PAPI)</h2>
      <p className="muted small">
        La base des licenciés publiée par la FFE (Data.mdb, celle de Papi) permet d'inscrire par n° de licence, nom ou code FIDE, hors ligne,
        et de remplir automatiquement les inscriptions en ligne.
      </p>
      <div className="inline mb">
        {status && <span className={`pill ${status.state === 'ready' ? 'ok' : status.state === 'loading' ? 'warn' : status.state === 'error' ? 'bad' : 'muted'}`}>
          {status.state === 'ready' ? `${status.players.toLocaleString('fr-FR')} joueurs` : status.state === 'loading' ? 'Chargement…' : status.state === 'error' ? 'Erreur' : 'Non chargée'}
        </span>}
        {status?.state === 'ready' && status.fileDate && <span className="muted small">fichier du {fmtDate(status.fileDate)}</span>}
        {status?.error && <span className="error small">{status.error}</span>}
      </div>
      <div className="actions">
        <button disabled={status?.state === 'loading' || busy} onClick={async () => { const s = await run(() => api.ffeDownload()); if (s) setStatus(s) }}>Télécharger depuis echecs.asso.fr</button>
        <button disabled={status?.state === 'loading' || busy} onClick={() => fileRef.current?.click()}>{busy ? 'Import…' : 'Importer PapiData.zip / Data.mdb'}</button>
        <input ref={fileRef} type="file" accept=".zip,.mdb" hidden onChange={e => { const f = e.target.files?.[0]; if (f) upload(f); e.target.value = '' }} />
      </div>
    </section>
  )
}
