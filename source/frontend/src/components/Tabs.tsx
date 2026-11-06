import { NavLink } from 'react-router'

export interface TabDef { to: string; label: string; count?: number; hidden?: boolean }

export function Tabs({ tabs }: { tabs: TabDef[] }) {
  return (
    <nav className="tabs">
      {tabs.filter(t => !t.hidden).map(t => (
        <NavLink key={t.to} to={t.to} end className={({ isActive }) => (isActive ? 'active' : '')}>
          {t.label}
          {t.count !== undefined && t.count > 0 && <span className="count">{t.count}</span>}
        </NavLink>
      ))}
    </nav>
  )
}
