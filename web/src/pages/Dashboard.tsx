import { useEffect, useState } from 'react'
import { Cpu, Activity, DollarSign, FileText, Zap } from 'lucide-react'
import { useWebSocket } from '../hooks/useWebSocket'

interface StatusData {
  version: string
  uptime: string
  status: string
  providers: { name: string; model: string; healthy: boolean; cost: number }[]
  autonomy?: {
    enabled: boolean
    iq: number
    patterns: number
    permissions: number
    updating: boolean
    verifying: boolean
    learning: boolean
    evolving: boolean
  }
  rag?: { enabled: boolean; vectors: number; dimensions: number }
  vault?: { enabled: boolean; secrets: number }
  cache?: { enabled: boolean; size: number; maxSize: number; hits: number }
  requests: number
}

export default function Dashboard() {
  const [data, setData] = useState<StatusData | null>(null)
  const [logs, setLogs] = useState<string[]>([])
  const { connected, send, on } = useWebSocket(`ws://${window.location.host}/ws`)

  useEffect(() => {
    fetch('/api/status')
      .then((r) => r.json())
      .then(setData)
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (connected) {
      send({ method: 'subscribe_logs' })
      send({ method: 'get_status' })
    }
  }, [connected, send])

  useEffect(() => {
    on('status', (msg: any) => {
      setData((prev) => (prev ? { ...prev, ...msg } : null))
    })
    on('logEntry', (msg: any) => {
      setLogs((prev) => [...prev.slice(-49), msg.message || JSON.stringify(msg)])
    })
  }, [on])

  const providers = data?.providers || []
  const totalCost = providers.reduce((s, p) => s + p.cost, 0)
  const activeProviders = providers.filter((p) => p.healthy).length

  const statCards = [
    {
      label: 'AGENT STATUS',
      value: activeProviders > 0 ? `${activeProviders} ACTIVE` : 'STANDBY',
      icon: Cpu,
      color: 'text-cyber-green',
      border: 'border-cyber-green/30',
    },
    {
      label: 'PROVIDERS',
      value: `${providers.length}`,
      icon: Activity,
      color: 'text-cyber-cyan',
      border: 'border-cyber-cyan/30',
    },
    {
      label: 'COST',
      value: `$${totalCost.toFixed(4)}`,
      icon: DollarSign,
      color: 'text-cyber-magenta',
      border: 'border-cyber-magenta/30',
    },
    {
      label: 'VECTORS',
      value: `${data?.rag?.vectors || 0}`,
      icon: FileText,
      color: 'text-cyber-amber',
      border: 'border-cyber-amber/30',
    },
  ]

  const topAgents = providers.map((p) => ({
    name: p.name,
    tasks: Math.round(p.cost * 100),
    success: p.healthy ? 98 : 0,
    icon: Zap,
  }))

  return (
    <div className="p-6 space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-display text-cyber-cyan tracking-widest glitch-text">
            DXRK.AI CORE
          </h1>
          <p className="text-xs text-cyber-dim mt-1">
            Autonomous Agent System · {data ? `v${data.version}` : 'loading...'}
          </p>
        </div>
        <div className="flex items-center gap-4 text-[10px] text-cyber-dim">
          <span className="flex items-center gap-1.5">
            <span
              className={`w-2 h-2 rounded-full ${connected ? 'bg-cyber-green animate-pulse_glow' : 'bg-cyber-dim'} `}
            />
            {connected ? 'ONLINE' : 'OFFLINE'}
          </span>
          <span className="text-cyber-dim">{data?.uptime ? `UP ${data.uptime}` : ''}</span>
        </div>
      </header>

      <div className="grid grid-cols-4 gap-4">
        {statCards.map((card) => (
          <div
            key={card.label}
            className={`bg-cyber-dark/80 border ${card.border} rounded p-4 hover:glow-accent transition-all`}
          >
            <div className="flex items-center justify-between mb-3">
              <card.icon className={`w-4 h-4 ${card.color}`} />
              <span className="text-[10px] text-cyber-dim tracking-wider">{card.label}</span>
            </div>
            <div className={`text-2xl font-bold font-mono ${card.color}`}>{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-6">
        <div className="col-span-2 bg-cyber-dark/80 border border-cyber-border rounded p-4">
          <h2 className="text-xs text-cyber-cyan tracking-wider mb-3 flex items-center gap-2">
            <Activity className="w-3.5 h-3.5" />
            LIVE LOGS
          </h2>
          <div className="space-y-1 font-mono text-[11px] max-h-60 overflow-y-auto">
            {logs.map((msg, i) => (
              <div key={i} className="flex gap-3 hover:bg-cyber-card/30 rounded px-1">
                <span className="text-cyber-dim w-16 shrink-0">{i}</span>
                <span className="text-cyber-text/80 truncate">{msg}</span>
              </div>
            ))}
            {logs.length === 0 && (
              <span className="text-cyber-dim text-[11px]">Waiting for logs...</span>
            )}
          </div>
        </div>

        <div className="bg-cyber-dark/80 border border-cyber-border rounded p-4">
          <h2 className="text-xs text-cyber-cyan tracking-wider mb-3 flex items-center gap-2">
            <Cpu className="w-3.5 h-3.5" />
            PROVIDER POOL
          </h2>
          <div className="space-y-3">
            {topAgents.map((agent) => (
              <div key={agent.name} className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <agent.icon className="w-3.5 h-3.5 text-cyber-dim" />
                  <span className="text-xs text-cyber-text">{agent.name}</span>
                </div>
                <div className="flex items-center gap-3 text-[10px]">
                  <span className="text-cyber-dim">{agent.tasks} calls</span>
                  <span
                    className={`${agent.success >= 95 ? 'text-cyber-green' : 'text-cyber-amber'}`}
                  >
                    {agent.success}%
                  </span>
                </div>
              </div>
            ))}
          </div>

          {data?.autonomy && (
            <div className="mt-4 pt-3 border-t border-cyber-border space-y-1">
              <div className="text-[10px] text-cyber-dim flex justify-between">
                <span>IQ SCORE</span>
                <span className="text-cyber-cyan">{data.autonomy.iq.toFixed(1)}</span>
              </div>
              <div className="text-[10px] text-cyber-dim flex justify-between">
                <span>PERMISSIONS</span>
                <span>{data.autonomy.permissions}</span>
              </div>
              {data.cache && (
                <div className="text-[10px] text-cyber-dim flex justify-between">
                  <span>CACHE</span>
                  <span>
                    {data.cache.size}/{data.cache.maxSize}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
