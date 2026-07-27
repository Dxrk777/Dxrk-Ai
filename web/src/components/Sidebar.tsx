import { NavLink } from 'react-router-dom'
import { Activity, Settings, Cpu, Globe, Logs } from 'lucide-react'

const navItems = [
  { to: '/dashboard', label: 'DASHBOARD', icon: Activity },
  { to: '/logs', label: 'LOGS', icon: Logs },
  { to: '/agents', label: 'AGENTS', icon: Cpu },
  { to: '/settings', label: 'SETTINGS', icon: Settings },
]

export default function Sidebar() {
  return (
    <aside className="w-56 bg-cyber-dark border-r border-cyber-border flex flex-col">
      <div className="h-14 flex items-center gap-3 px-4 border-b border-cyber-border">
        <div className="w-7 h-7 rounded bg-cyber-accent/20 flex items-center justify-center">
          <Globe className="w-4 h-4 text-cyber-cyan" />
        </div>
        <div>
          <span className="text-cyber-cyan font-bold text-sm tracking-widest">DXRK</span>
          <span className="text-cyber-dim text-[10px] block leading-none">.ai</span>
        </div>
      </div>

      <nav className="flex-1 py-4">
        {navItems.map((item) => {
          const Icon = item.icon
          return (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `w-full flex items-center gap-3 px-4 py-2.5 text-xs font-mono tracking-wider transition-all ${
                  isActive
                    ? 'text-cyber-cyan bg-cyber-accent/10 border-r-2 border-cyber-cyan glow-cyan'
                    : 'text-cyber-dim hover:text-cyber-text hover:bg-cyber-card/50'
                }`
              }
            >
              <Icon className="w-4 h-4" />
              {item.label}
            </NavLink>
          )
        })}
      </nav>

      <div className="p-3 border-t border-cyber-border">
        <div className="text-[10px] text-cyber-dim font-mono">
          <div className="flex items-center gap-2 mb-1">
            <span className="w-1.5 h-1.5 rounded-full bg-cyber-green animate-pulse_glow" />
            <span>AGENT ACTIVE</span>
          </div>
          <div className="text-[9px] opacity-60">v4.0.0</div>
        </div>
      </div>
    </aside>
  )
}
