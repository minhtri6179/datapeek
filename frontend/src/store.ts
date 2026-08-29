import { create } from 'zustand'
import { config, query } from '../wailsjs/go/models'
import { Connect, Disconnect, ListConnections } from '../wailsjs/go/main/App'

export type Connection = config.Connection
export type QueryResult = query.QueryResult
export type SortSpec = query.SortSpec
export type TableInfo = { name: string; type: string }

// A tab showing one table's data.
export interface TableTab {
  connId: string
  schema: string
  table: string
}

interface AppState {
  connections: Connection[]
  connectedIds: Set<string>
  activeConnId: string
  tabs: TableTab[]
  activeTab: number
  status: string

  refreshConnections: () => Promise<void>
  connect: (id: string) => Promise<void>
  disconnect: (id: string) => Promise<void>
  setActiveConn: (id: string) => void
  openTable: (connId: string, schema: string, table: string) => void
  closeTab: (index: number) => void
  setActiveTab: (index: number) => void
  setStatus: (s: string) => void
}

export const useStore = create<AppState>((set, get) => ({
  connections: [],
  connectedIds: new Set<string>(),
  activeConnId: '',
  tabs: [],
  activeTab: -1,
  status: '',

  refreshConnections: async () => {
    try {
      const conns = await ListConnections()
      set({ connections: conns ?? [] })
    } catch (err) {
      get().setStatus(`Failed to list connections: ${err}`)
    }
  },

  connect: async (id) => {
    get().setStatus('Connecting…')
    try {
      await Connect(id)
      const connected = new Set(get().connectedIds)
      connected.add(id)
      set({ connectedIds: connected, activeConnId: id, status: '' })
    } catch (err) {
      get().setStatus(`Connection failed: ${err}`)
      throw err
    }
  },

  disconnect: async (id) => {
    await Disconnect(id)
    const connected = new Set(get().connectedIds)
    connected.delete(id)
    const tabs = get().tabs.filter((t) => t.connId !== id)
    const activeConn = get().activeConnId === id ? '' : get().activeConnId
    set({ connectedIds: connected, tabs, activeConnId: activeConn, activeTab: tabs.length - 1 })
  },

  setActiveConn: (id) => set({ activeConnId: id }),

  openTable: (connId, schema, table) => {
    const tabs = get().tabs
    const existing = tabs.findIndex(
      (t) => t.connId === connId && t.schema === schema && t.table === table
    )
    if (existing >= 0) {
      set({ activeTab: existing })
      return
    }
    set({ tabs: [...tabs, { connId, schema, table }], activeTab: tabs.length })
  },

  closeTab: (index) => {
    const tabs = get().tabs.filter((_, i) => i !== index)
    let active = get().activeTab
    if (active >= tabs.length) active = tabs.length - 1
    if (active === index) active = Math.min(index, tabs.length - 1)
    set({ tabs, activeTab: active })
  },

  setActiveTab: (index) => set({ activeTab: index }),

  setStatus: (status) => set({ status }),
}))
