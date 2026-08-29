import { useState } from 'react'
import { useStore } from '../store'
import { ConnectionForm } from './ConnectionForm'
import { SchemaTree } from './SchemaTree'

export function Sidebar() {
  const { connections, connectedIds, activeConnId, connect, disconnect, refreshConnections, setActiveConn } = useStore()
  const [editing, setEditing] = useState<string | 'new' | null>(null)

  const editingConn = editing && editing !== 'new'
    ? connections.find((c) => c.id === editing) ?? null
    : null

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <span className="brand">datapeek</span>
        <button className="icon-btn" title="New connection"
          onClick={() => setEditing('new')}>+</button>
      </div>

      {editing !== null ? (
        <ConnectionForm
          editing={editingConn}
          onSaved={async () => { setEditing(null); await refreshConnections() }}
          onCancel={() => setEditing(null)}
        />
      ) : (
        <div className="conn-list">
          {connections.length === 0 && (
            <div className="hint">No connections yet.<br />Click + to add one.</div>
          )}
          {connections.map((c) => {
            const connected = connectedIds.has(c.id)
            const active = c.id === activeConnId
            return (
              <div key={c.id}
                className={`conn-item ${active ? 'active' : ''}`}
                onClick={() => setActiveConn(c.id)}
                onDoubleClick={() => (connected ? disconnect(c.id) : connect(c.id).catch(() => {}))}
              >
                <span className={`dot ${connected ? 'on' : ''}`} title={connected ? 'Connected' : 'Disconnected'} />
                <span className="conn-name">{c.name}</span>
                <span className="conn-type">{c.type}</span>
                <button className="icon-btn" title="Edit"
                  onClick={(e) => { e.stopPropagation(); setEditing(c.id) }}>✎</button>
                <button className="icon-btn" title={connected ? 'Disconnect' : 'Connect'}
                  onClick={(e) => {
                    e.stopPropagation()
                    connected ? disconnect(c.id) : connect(c.id).catch(() => {})
                  }}>{connected ? '⏻' : '▶'}</button>
              </div>
            )
          })}
        </div>
      )}

      {activeConnId && connectedIds.has(activeConnId) && <SchemaTree connId={activeConnId} />}
    </div>
  )
}
