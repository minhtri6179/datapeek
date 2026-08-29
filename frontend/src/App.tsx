import React, { useEffect } from 'react'
import { useStore } from './store'
import { Sidebar } from './components/Sidebar'
import { TableDataGrid } from './components/DataGrid'
import { LogFrontendEvent } from '../wailsjs/go/main/App'

// ErrorBoundary funnels React render errors into the backend log pipeline.
class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    LogFrontendEvent('error', `${error.message} @ ${info.componentStack?.split('\n')[1]?.trim() ?? 'unknown'}`, '')
  }

  render() {
    if (this.state.error) {
      return <div className="crash">UI error: {this.state.error.message}</div>
    }
    return this.props.children
  }
}

export default function App() {
  const { tabs, activeTab, closeTab, setActiveTab, status, refreshConnections } = useStore()

  useEffect(() => {
    refreshConnections()
  }, [refreshConnections])

  const active = tabs[activeTab]

  return (
    <ErrorBoundary>
      <div className="app">
        <Sidebar />
        <div className="main">
          <div className="tab-bar">
            {tabs.map((t, i) => (
              <div key={`${t.connId}:${t.schema}.${t.table}`}
                className={`tab ${i === activeTab ? 'active' : ''}`}
                onClick={() => setActiveTab(i)}>
                <span className="tab-label">{t.schema}.{t.table}</span>
                <button className="icon-btn" title="Close"
                  onClick={(e) => { e.stopPropagation(); closeTab(i) }}>✕</button>
              </div>
            ))}
          </div>
          <div className="tab-content">
            {active ? <TableDataGrid key={`${active.connId}:${active.schema}.${active.table}`} tab={active} /> : (
              <div className="empty-state">
                <div className="empty-title">datapeek</div>
                <div>Select a connection, then browse a table from the schema tree.</div>
                <div className="empty-hint">Double-click a connection to connect.</div>
              </div>
            )}
          </div>
          <div className="status-bar">{status}</div>
        </div>
      </div>
    </ErrorBoundary>
  )
}
