import { useEffect, useRef, useState } from 'react'
import { Trash2, Pause, Play } from 'lucide-react'
import { useWebSocket } from '../hooks/useWebSocket'

interface LogEntry {
  timestamp: string
  level: string
  source: string
  message: string
}

const LOG_LEVELS = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG']

export default function LogsPage() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [filter, setFilter] = useState('ALL')
  const [paused, setPaused] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const { connected, send, on } = useWebSocket(`ws://${window.location.host}/ws`)

  useEffect(() => {
    if (connected) {
      send({ method: 'subscribe_logs' })
    }
  }, [connected, send])

  useEffect(() => {
    on('logEntry', (msg: any) => {
      if (paused) return
      if (msg.timestamp) {
        setLogs((prev) => [...prev.slice(-999), msg])
      }
    })
    on('message', (msg: any) => {
      if (paused) return
      if (msg.timestamp) {
        setLogs((prev) => [...prev.slice(-999), msg])
      }
    })
  }, [on, paused])

  useEffect(() => {
    if (!paused) bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs, paused])

  const filteredLogs = filter === 'ALL' ? logs : logs.filter((l) => l.level === filter)

  return (
    <div className="p-6 space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-display text-cyber-cyan tracking-widest">LIVE LOGS</h1>
          <p className="text-xs text-cyber-dim mt-1">
            {filteredLogs.length} entries · {logs.length} total ·{' '}
            {connected ? 'CONNECTED' : 'DISCONNECTED'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {LOG_LEVELS.map((l) => (
            <button
              key={l}
              onClick={() => setFilter(l)}
              className={`text-[10px] px-2 py-1 rounded border ${
                filter === l
                  ? 'bg-cyber-cyan/20 border-cyber-cyan/40 text-cyber-cyan'
                  : 'bg-cyber-dark/60 border-cyber-border text-cyber-dim hover:border-cyber-dim'
              }`}
            >
              {l}
            </button>
          ))}
          <button
            onClick={() => setPaused(!paused)}
            className={`text-[10px] px-2 py-1 rounded border ${
              paused
                ? 'bg-cyber-amber/20 border-cyber-amber/40 text-cyber-amber'
                : 'bg-cyber-dark/60 border-cyber-border text-cyber-dim'
            }`}
          >
            {paused ? <Play className="w-3 h-3 inline" /> : <Pause className="w-3 h-3 inline" />}
          </button>
          <button
            onClick={() => setLogs([])}
            className="text-[10px] px-2 py-1 rounded border bg-cyber-dark/60 border-cyber-border text-cyber-dim hover:text-cyber-red hover:border-cyber-red/40"
          >
            <Trash2 className="w-3 h-3 inline" /> CLEAR
          </button>
        </div>
      </header>

      <div className="bg-cyber-dark/80 border border-cyber-border rounded p-4 h-[60vh] overflow-y-auto font-mono text-[11px]">
        {filteredLogs.length === 0 ? (
          <div className="text-cyber-dim text-center mt-20">
            {logs.length === 0 ? 'Waiting for log data...' : 'No matching entries'}
          </div>
        ) : (
          <div className="space-y-0.5">
            {filteredLogs.map((entry, i) => {
              const levelColor =
                entry.level === 'ERROR'
                  ? 'text-cyber-red'
                  : entry.level === 'WARN'
                    ? 'text-cyber-amber'
                    : 'text-cyber-cyan'
              return (
                <div key={i} className="flex gap-2 hover:bg-cyber-card/20 rounded px-1 py-0.5">
                  <span className="text-cyber-dim w-20 shrink-0">
                    {entry.timestamp?.split('T')[1]?.split('.')[0] || entry.timestamp}
                  </span>
                  <span className={`w-10 shrink-0 ${levelColor}`}>{entry.level}</span>
                  <span className="text-cyber-dim w-14 shrink-0">[{entry.source}]</span>
                  <span className="text-cyber-text/80">{entry.message}</span>
                </div>
              )
            })}
            <div ref={bottomRef} />
          </div>
        )}
      </div>
    </div>
  )
}
