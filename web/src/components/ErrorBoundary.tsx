import { Component, type ReactNode } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex flex-col items-center justify-center h-full p-8 text-center">
          <AlertTriangle className="w-12 h-12 text-cyber-amber mb-4" />
          <h2 className="text-lg font-mono text-cyber-red mb-2">SYSTEM ERROR</h2>
          <p className="text-xs text-cyber-dim mb-4 max-w-md">
            {this.state.error?.message || 'An unexpected error occurred'}
          </p>
          <button
            onClick={() => {
              this.setState({ hasError: false, error: null })
              window.location.reload()
            }}
            className="flex items-center gap-2 text-xs bg-cyber-dark/60 border border-cyber-border rounded px-4 py-2 text-cyber-cyan hover:border-cyber-cyan/50"
          >
            <RefreshCw className="w-3 h-3" /> RESTART
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
