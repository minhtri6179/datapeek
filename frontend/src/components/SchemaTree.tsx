import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { GetSchemas, GetTables } from '../../wailsjs/go/main/App'

// Lazy-loaded tree: schemas expand to load tables; clicking a table
// opens a datagrid tab.
export function SchemaTree({ connId }: { connId: string }) {
  const [schemas, setSchemas] = useState<string[]>([])
  const [expanded, setExpanded] = useState<string | null>(null)
  const [tables, setTables] = useState<Record<string, { name: string; type: string }[]>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const openTable = useStore((s) => s.openTable)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    GetSchemas(connId)
      .then((s) => { if (!cancelled) setSchemas(s ?? []) })
      .catch((e) => { if (!cancelled) setError(String(e)) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [connId])

  const toggle = async (schema: string) => {
    if (expanded === schema) {
      setExpanded(null)
      return
    }
    setExpanded(schema)
    if (!tables[schema]) {
      try {
        const t = await GetTables(connId, schema)
        setTables((prev) => ({ ...prev, [schema]: t ?? [] }))
      } catch (e) {
        setError(String(e))
      }
    }
  }

  if (loading) return <div className="tree-status">Loading schemas…</div>
  if (error) return <div className="tree-status error">{error}</div>
  if (schemas.length === 0) return <div className="tree-status">No schemas found.</div>

  return (
    <div className="schema-tree">
      {schemas.map((s) => (
        <div key={s}>
          <div className="tree-node schema" onClick={() => toggle(s)}>
            <span className="caret">{expanded === s ? '▾' : '▸'}</span>
            <span className="tree-label">{s}</span>
          </div>
          {expanded === s && (
            <div className="tree-children">
              {!tables[s] && <div className="tree-status">Loading…</div>}
              {tables[s]?.length === 0 && <div className="tree-status">No tables.</div>}
              {tables[s]?.map((t) => (
                <div key={t.name} className="tree-node table"
                  title={`${s}.${t.name}`}
                  onClick={() => openTable(connId, s, t.name)}>
                  <span className={`tree-icon ${t.type}`} title={t.type}>
                    {t.type === 'view' ? 'v' : 't'}
                  </span>
                  <span className="tree-label">{t.name}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
