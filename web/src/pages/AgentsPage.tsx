import { useEffect, useState } from 'react'
import { Cpu, Zap, Activity, FileText, StopCircle, RefreshCw } from 'lucide-react'

interface Provider {
  name: string
  model: string
  healthy: boolean
  cost: number
}

const roleIcons: Record<string, typeof Cpu> = {
  claude: Zap,
  openai: Activity,
  gemini: Cpu,
  ollama: FileText,
}

export default function AgentsPage() {
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/providers')
      .then((r) => r.json())
      .then((data: Provider[]) => {
        setProviders(data)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [])

  return (
    <div className="p-6 space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-display text-cyber-cyan tracking-widest">AGENT POOL</h1>
          <p className="text-xs text-cyber-dim mt-1">{providers.length} registered providers</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() =>
              fetch('/api/providers')
                .then((r) => r.json())
                .then(setProviders)
            }
            className="text-[10px] bg-cyber-dark/60 border border-cyber-border rounded px-3 py-1.5 text-cyber-cyan hover:border-cyber-cyan/50 flex items-center gap-1.5"
          >
            <RefreshCw className="w-3 h-3" /> REFRESH
          </button>
          <button className="text-[10px] bg-cyber-dark/60 border border-cyber-border rounded px-3 py-1.5 text-cyber-green hover:border-cyber-green/50 flex items-center gap-1.5">
            <Cpu className="w-3 h-3" /> SPAWN
          </button>
        </div>
      </header>

      {loading ? (
        <div className="text-cyber-dim text-xs">Loading providers...</div>
      ) : providers.length === 0 ? (
        <div className="text-cyber-dim text-xs">
          No providers configured. Add providers to dxrk.yaml.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {providers.map((p) => {
            const Icon = roleIcons[p.name] || Cpu
            const statusColor = p.healthy ? 'text-cyber-green' : 'text-cyber-red'
            const statusBg = p.healthy
              ? 'bg-cyber-green/10 border-cyber-green/30'
              : 'bg-cyber-red/10 border-cyber-red/30'
            return (
              <div key={p.name} className={`bg-cyber-dark/80 border ${statusBg} rounded p-4`}>
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <Icon className="w-4 h-4 text-cyber-cyan" />
                    <span className="text-sm font-mono text-cyber-text">{p.name}</span>
                  </div>
                  <span
                    className={`text-[10px] px-2 py-0.5 rounded-full border ${statusBg} ${statusColor}`}
                  >
                    {p.healthy ? 'ACTIVE' : 'ERROR'}
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-3 text-[10px]">
                  <div>
                    <span className="text-cyber-dim">Model</span>
                    <p className="text-cyber-text font-mono mt-0.5">{p.model}</p>
                  </div>
                  <div>
                    <span className="text-cyber-dim">Cost</span>
                    <p className="text-cyber-text font-mono mt-0.5">${p.cost.toFixed(6)}</p>
                  </div>
                </div>
                <div className="mt-3 flex gap-2">
                  <span className="text-[9px] px-1.5 py-0.5 rounded bg-cyber-card/50 text-cyber-dim border border-cyber-border">
                    {p.name === 'claude'
                      ? 'tool_use, vision'
                      : p.name === 'openai'
                        ? 'tool_use'
                        : 'completion'}
                  </span>
                  <button className="ml-auto text-[10px] text-cyber-red/60 hover:text-cyber-red flex items-center gap-1">
                    <StopCircle className="w-3 h-3" /> STOP
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
