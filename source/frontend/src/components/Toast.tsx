import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'

type Kind = 'info' | 'error' | 'success'
interface Toast { id: number; kind: Kind; text: string }

const Ctx = createContext<(text: string, kind?: Kind) => void>(() => {})

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const push = useCallback((text: string, kind: Kind = 'info') => {
    const id = Date.now() + Math.random()
    setToasts(t => [...t.slice(-2), { id, kind, text }])
    window.setTimeout(() => setToasts(t => t.filter(x => x.id !== id)), kind === 'error' ? 7000 : 3500)
  }, [])
  return (
    <Ctx.Provider value={push}>
      {children}
      <div className="toasts" aria-live="polite">
        {toasts.map(t => (
          <div key={t.id} className={`toast ${t.kind}`}
            onClick={() => setToasts(x => x.filter(y => y.id !== t.id))}>{t.text}</div>
        ))}
      </div>
    </Ctx.Provider>
  )
}

export const useToast = () => useContext(Ctx)

/** Wraps an async action: shows the error message as a toast. */
export function useAction() {
  const toast = useToast()
  return useCallback(async <T,>(fn: () => Promise<T>, success?: string): Promise<T | undefined> => {
    try {
      const r = await fn()
      if (success) toast(success, 'success')
      return r
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
      return undefined
    }
  }, [toast])
}
