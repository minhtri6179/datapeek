import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ReadTable } from '../../wailsjs/go/main/App'
import { query } from '../../wailsjs/go/models'
import type { TableTab } from '../store'

const PAGE_SIZES = [50, 100, 200, 500]

interface Props {
  tab: TableTab
}

// TableDataGrid renders one table tab: server-side pagination + sorting,
// virtualized rows, click-to-open cell detail.
export function TableDataGrid({ tab }: Props) {
  const [result, setResult] = useState<query.QueryResult | null>(null)
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState(100)
  const [sort, setSort] = useState<query.SortSpec | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [detail, setDetail] = useState<{ col: string; value: any } | null>(null)
  const loadToken = useRef(0)

  const load = useCallback(async () => {
    const token = ++loadToken.current
    setLoading(true)
    setError('')
    try {
      // Wails bindings type the pointer arg as required; an empty column
      // means "no sort" on the Go side.
      const sortArg = sort ?? new query.SortSpec({ column: '', desc: false })
      const res = await ReadTable(tab.connId, tab.schema, tab.table, page, pageSize, sortArg)
      if (token === loadToken.current) setResult(res)
    } catch (e) {
      if (token === loadToken.current) setError(String(e))
    } finally {
      if (token === loadToken.current) setLoading(false)
    }
  }, [tab.connId, tab.schema, tab.table, page, pageSize, sort])

  useEffect(() => {
    // Reset paging when the tab switches to a different table.
    setPage(0)
    setSort(null)
  }, [tab.connId, tab.schema, tab.table])

  useEffect(() => {
    load()
  }, [load])

  const total = result?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(total / pageSize))

  const rows = result?.rows ?? []

  const headerClick = (col: query.ColumnMeta) => {
    if (sort?.column === col.name) {
      if (sort.desc) setSort(null)
      else setSort(new query.SortSpec({ column: col.name, desc: true }))
    } else {
      setSort(new query.SortSpec({ column: col.name, desc: false }))
    }
    setPage(0)
  }

  return (
    <div className="grid-wrap">
      {error && <div className="error-banner">{error}</div>}
      {loading && !result && <div className="grid-placeholder">Loading…</div>}
      {!loading && !error && rows.length === 0 && (
        <div className="grid-placeholder">Table is empty.</div>
      )}

      {rows.length > 0 && (
        <TableBody
          columns={result!.columns}
          rows={rows}
          sort={sort}
          onHeaderClick={headerClick}
          onCellClick={setDetail}
        />
      )}

      <div className="grid-footer">
        <span>{total.toLocaleString()} rows</span>
        <span className="spacer" />
        <label>
          Rows/page
          <select value={pageSize} onChange={(e) => { setPageSize(Number(e.target.value)); setPage(0) }}>
            {PAGE_SIZES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </label>
        <button disabled={page === 0} onClick={() => setPage(0)}>«</button>
        <button disabled={page === 0} onClick={() => setPage(page - 1)}>‹</button>
        <span className="page-indicator">Page {page + 1} / {pageCount}</span>
        <button disabled={page >= pageCount - 1} onClick={() => setPage(page + 1)}>›</button>
        <button disabled={page >= pageCount - 1} onClick={() => setPage(pageCount - 1)}>»</button>
      </div>

      {detail && <CellDetail detail={detail} onClose={() => setDetail(null)} />}
    </div>
  )
}

function TableBody({
  columns, rows, sort, onHeaderClick, onCellClick,
}: {
  columns: query.ColumnMeta[]
  rows: any[][]
  sort: query.SortSpec | null
  onHeaderClick: (c: query.ColumnMeta) => void
  onCellClick: (d: { col: string; value: any }) => void
}) {
  const parentRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 28,
    overscan: 20,
  })

  const renderCell = useCallback((v: any) => {
    if (v === null || v === undefined) return <span className="null">NULL</span>
    if (v === '') return <span className="null">''</span>
    const s = String(v)
    return s.length > 64 ? s.slice(0, 64) + '…' : s
  }, [])

  return (
    <div className="grid-scroll" ref={parentRef}>
      <table className="datagrid">
        <thead>
          <tr>
            <th className="rownum">#</th>
            {columns.map((c) => (
              <th key={c.name} onClick={() => onHeaderClick(c)} className="sortable">
                <span className="col-name">{c.name}</span>
                <span className="col-type">{c.dataType}{c.nullable ? '?' : ''}</span>
                {sort?.column === c.name && <span className="sort-dir">{sort.desc ? '↓' : '↑'}</span>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody style={{ height: virtualizer.getTotalSize(), position: 'relative', display: 'block' }}>
          {virtualizer.getVirtualItems().map((vi) => {
            const row = rows[vi.index]
            return (
              <tr key={vi.key}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${vi.start}px)`,
                }}>
                <td className="rownum">{page0Offset(vi.index)}</td>
                {columns.map((c, ci) => (
                  <td key={c.name} onClick={() => onCellClick({ col: c.name, value: row[ci] })}>
                    {renderCell(row[ci])}
                  </td>
                ))}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// row numbers are per-page; the parent passes 0-based page rows,
// so vi.index + 1 is the in-page row number.
function page0Offset(i: number) {
  return i + 1
}

// CellDetail shows the full value of a clicked cell: raw text for long
// strings, pretty JSON, hex for binary markers.
export function CellDetail({ detail, onClose }: { detail: { col: string; value: any }; onClose: () => void }) {
  const { text, kind } = useMemo(() => {
    const v = detail.value
    if (v === null || v === undefined) return { text: 'NULL', kind: 'null' }
    const s = String(v)
    if (s.startsWith('0x') && /^0x[0-9a-f]*$/.test(s)) {
      return { text: s, kind: 'binary' }
    }
    const trimmed = s.trim()
    if ((trimmed.startsWith('{') && trimmed.endsWith('}')) ||
        (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
      try {
        return { text: JSON.stringify(JSON.parse(s), null, 2), kind: 'json' }
      } catch { /* not JSON */ }
    }
    return { text: s, kind: 'text' }
  }, [detail])

  return (
    <div className="cell-detail" onClick={onClose}>
      <div className="cell-detail-panel" onClick={(e) => e.stopPropagation()}>
        <div className="cell-detail-header">
          <strong>{detail.col}</strong>
          <span className={`kind-badge ${kind}`}>{kind}</span>
          <span className="spacer" />
          <button className="icon-btn" onClick={() => navigator.clipboard?.writeText(text)}>Copy</button>
          <button className="icon-btn" onClick={onClose}>✕</button>
        </div>
        <pre>{text}</pre>
      </div>
    </div>
  )
}
