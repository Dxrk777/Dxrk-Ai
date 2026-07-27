import { useEffect, useState } from 'react'
import { Save, RotateCcw } from 'lucide-react'

interface SettingField {
  key: string
  label: string
  value: string | boolean | number
  type: 'text' | 'number' | 'toggle'
  section: string
}

export default function SettingsPage() {
  const [fields, setFields] = useState<SettingField[]>([])
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    fetch('/api/status')
      .then((r) => r.json())
      .then((data) => {
        const f: SettingField[] = []
        f.push({
          key: 'version',
          label: 'Version',
          value: data.version || '4.0.0',
          type: 'text',
          section: 'System',
        })
        f.push({
          key: 'status',
          label: 'Status',
          value: data.status || 'ok',
          type: 'text',
          section: 'System',
        })
        f.push({
          key: 'uptime',
          label: 'Uptime',
          value: data.uptime || '0s',
          type: 'text',
          section: 'System',
        })
        f.push({
          key: 'requests',
          label: 'API Requests',
          value: data.requests || 0,
          type: 'number',
          section: 'System',
        })

        if (data.autonomy) {
          f.push({
            key: 'autonomy_enabled',
            label: 'Autonomy',
            value: data.autonomy.enabled,
            type: 'toggle',
            section: 'Autonomy',
          })
          f.push({
            key: 'autonomy_iq',
            label: 'IQ Score',
            value: data.autonomy.iq.toFixed(1),
            type: 'text',
            section: 'Autonomy',
          })
          f.push({
            key: 'autonomy_permissions',
            label: 'Permissions',
            value: data.autonomy.permissions,
            type: 'number',
            section: 'Autonomy',
          })
          f.push({
            key: 'autonomy_evolving',
            label: 'Evolution',
            value: data.autonomy.evolving,
            type: 'toggle',
            section: 'Autonomy',
          })
        }

        if (data.cache) {
          f.push({
            key: 'cache_size',
            label: 'Cache Entries',
            value: `${data.cache.size}/${data.cache.maxSize}`,
            type: 'text',
            section: 'Cache',
          })
          f.push({
            key: 'cache_hits',
            label: 'Cache Hits',
            value: data.cache.hits,
            type: 'number',
            section: 'Cache',
          })
          f.push({
            key: 'cache_ttl',
            label: 'Cache TTL',
            value: data.cache.ttl || '5m0s',
            type: 'text',
            section: 'Cache',
          })
        }

        if (data.vault) {
          f.push({
            key: 'vault_enabled',
            label: 'Vault',
            value: data.vault.enabled,
            type: 'toggle',
            section: 'Vault',
          })
          f.push({
            key: 'vault_secrets',
            label: 'Stored Secrets',
            value: data.vault.secrets,
            type: 'number',
            section: 'Vault',
          })
        }

        f.push({
          key: 'providers',
          label: 'Active Providers',
          value: (data.providers || []).length,
          type: 'number',
          section: 'Providers',
        })

        setFields(f)
      })
      .catch(() => {
        setFields([
          { key: 'status', label: 'Status', value: 'offline', type: 'text', section: 'System' },
          {
            key: 'note',
            label: 'Note',
            value: 'Start the Dxrk server to see live settings',
            type: 'text',
            section: 'System',
          },
        ])
      })
  }, [])

  const updateField = (key: string, newValue: string | boolean | number) => {
    setFields((prev) => prev.map((f) => (f.key === key ? { ...f, value: newValue } : f)))
    setDirty(true)
    setMessage('')
  }

  const handleSave = async () => {
    setSaving(true)
    setMessage('')
    try {
      const payload: Record<string, unknown> = {}
      for (const f of fields) {
        payload[f.key] = f.value
      }
      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (res.ok) {
        setMessage('Settings saved')
        setDirty(false)
      } else {
        setMessage('Failed to save')
      }
    } catch {
      setMessage('Server unreachable')
    } finally {
      setSaving(false)
    }
  }

  const sections = [...new Set(fields.map((f) => f.section))]

  return (
    <div className="p-6 space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-display text-cyber-cyan tracking-widest">CONFIGURATION</h1>
          <p className="text-xs text-cyber-dim mt-1">System settings from dxrk.yaml</p>
        </div>
        <div className="flex items-center gap-3">
          {message && <span className="text-[10px] text-cyber-green">{message}</span>}
          <button
            onClick={() => window.location.reload()}
            className="text-[10px] bg-cyber-dark/60 border border-cyber-border rounded px-3 py-1.5 text-cyber-dim hover:text-cyber-cyan flex items-center gap-1.5"
          >
            <RotateCcw className="w-3 h-3" /> RESET
          </button>
          <button
            disabled={saving || !dirty}
            onClick={handleSave}
            className="text-[10px] bg-cyber-cyan/20 border border-cyber-cyan/30 rounded px-3 py-1.5 text-cyber-cyan hover:bg-cyber-cyan/30 flex items-center gap-1.5 disabled:opacity-40"
          >
            <Save className="w-3 h-3" /> {saving ? 'SAVING...' : 'SAVE'}
          </button>
        </div>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {sections.map((section) => (
          <div key={section} className="bg-cyber-dark/80 border border-cyber-border rounded p-4">
            <h2 className="text-xs text-cyber-cyan tracking-wider mb-4 pb-2 border-b border-cyber-border">
              {section}
            </h2>
            <div className="space-y-4">
              {fields
                .filter((f) => f.section === section)
                .map((field) => (
                  <div key={field.key}>
                    <label className="text-[10px] text-cyber-dim block mb-1">{field.label}</label>
                    {field.type === 'toggle' ? (
                      <button
                        onClick={() => updateField(field.key, !field.value)}
                        className={`text-xs px-3 py-1.5 rounded bg-cyber-deeper/60 border font-mono ${field.value ? 'text-cyber-green border-cyber-green/30' : 'text-cyber-red border-cyber-red/30'}`}
                      >
                        {field.value ? 'ENABLED' : 'DISABLED'}
                      </button>
                    ) : field.type === 'number' ? (
                      <input
                        type="number"
                        value={field.value as number}
                        onChange={(e) => updateField(field.key, Number(e.target.value))}
                        className="w-full text-xs bg-cyber-deeper/60 border border-cyber-border rounded px-3 py-1.5 text-cyber-text font-mono focus:border-cyber-cyan/50 outline-none"
                      />
                    ) : (
                      <input
                        type="text"
                        value={field.value as string}
                        onChange={(e) => updateField(field.key, e.target.value)}
                        className="w-full text-xs bg-cyber-deeper/60 border border-cyber-border rounded px-3 py-1.5 text-cyber-text font-mono focus:border-cyber-cyan/50 outline-none"
                      />
                    )}
                  </div>
                ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
