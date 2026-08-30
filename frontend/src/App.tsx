import React, { useEffect, useState } from 'react'
import { useStore } from './store'
import { Sidebar } from './components/Sidebar'
import { TableDataGrid } from './components/DataGrid'
import { config, query } from '../wailsjs/go/models'
import { RunQuery } from '../wailsjs/go/main/App'
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
  const [activeConsoleQuery, setActiveConsoleQuery] = useState<string | null>(null)
  const [consoleResult, setConsoleResult] = useState<any | null>(null)
  const [consoleLoading, setConsoleLoading] = useState(false)

  useEffect(() => {
    refreshConnections()
  }, [refreshConnections])

  const active = tabs[activeTab]

  const handleRunQuery = async (sql: string) => {
    setConsoleLoading(true)
    setConsoleResult(null)
    try {
      const res = await RunQuery(active?.connId || '', sql)
      setConsoleResult(res)
    } catch (e) {
      setConsoleResult({ columns: [], rows: [], total: 0, truncated: false })
    } finally {
      setConsoleLoading(false)
    }
  }

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
            {active ? (
              <TableDataGrid key={`${active.connId}:${active.schema}.${active.table}`} tab={active} />
            ) : (
              <div className="empty-state">
                <div className="empty-title">datapeek</div>
                <div>Select a connection, then browse a table from the schema tree.</div>
                <div className="empty-hint">Double-click a connection to connect.</div>
              </div>
            )}
            {activeConsoleQuery && (
              <div className="console-panel">
                <div className="console-header">
                  <span className="console-query-label">Console</span>
                  <button className="icon-btn" onClick={() => setActiveConsoleQuery(null)}>&times;</button>
                </div>
                <textarea
                  className="console-input"
                  rows={4}
                  value={activeConsoleQuery}
                  onChange={(e) => setActiveConsoleQuery(e.target.value)}
                  placeholder="Enter SQL query..."
                />
                <div className="console-actions">
                  <button onClick={() => handleRunQuery(activeConsoleQuery)} disabled={consoleLoading} className="console-btn">
                    {consoleLoading ? 'Running…' : 'Run'}
                  </button>
                  <button onClick={() => setActiveConsoleQuery(null)} className="console-btn-secondary">Clear</button>
                </div>
              </div>
            )}
          </div>
          {consoleResult && !consoleLoading && (
            <div className="console-results">
              {consoleResult.truncated && <div className="truncated-notice">Results truncated at 1000 rows.</div>}
              <pre className="console-output">{JSON.stringify(consoleResult, null, 2)}</pre>
            </div>
          )}
          <div className="status-bar">{status}</div>
        </div>
      </div>
    </ErrorBoundary>
  )
}