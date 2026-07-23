import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { TestNode, TestOptions, WebSocketMessage } from '@/types'
import { tt } from '@/lib/i18n'
import { getSpeed, formatSeconds } from '@/lib/utils'

interface TestState {
  // 测试状态
  loading: boolean
  result: TestNode[]
  testCount: number
  testOkCount: number
  totalTraffic: number
  totalTime: number
  picdata: string

  // 公网出口(测速机自身,来自后端 ipinfo 消息)
  ipv4: string
  ipv6: string
  ipv4geo: string
  ipv6geo: string

  // 实时速率(测速进行中,驱动实时表盘)
  currentTestingId: number | null
  currentDirection: 'down' | 'up' | null
  liveDownloadBps: number
  liveUploadBps: number

  // 选择状态
  selectedNodes: TestNode[]
  
  // 本次运行是否启用了上传(测试开始时快照,用于让上传列一开始就出现)
  runUploadEnabled: boolean

  // 结果表格的用户偏好(持久化):列顺序、列宽
  columnOrder: string[]
  columnSizing: Record<string, number>

  // WebSocket
  ws: WebSocket | null

  // 配置
  options: TestOptions

  // Actions
  setLoading: (loading: boolean) => void
  setResult: (result: TestNode[]) => void
  updateNode: (id: number, data: Partial<TestNode>) => void
  addNodes: (nodes: TestNode[]) => void
  setSelectedNodes: (nodes: TestNode[]) => void
  setOptions: (options: Partial<TestOptions>) => void
  incrementTestCount: () => void
  incrementTestOkCount: () => void
  addTraffic: (traffic: number) => void
  incrementTime: () => void
  setPicdata: (data: string) => void
  regenerateImage: () => Promise<boolean>
  setRunUploadEnabled: (v: boolean) => void
  setColumnOrder: (order: string[]) => void
  setColumnSizing: (sizing: Record<string, number>) => void
  resetTableLayout: () => void
  reset: () => void
  
  // WebSocket Actions
  connect: (url: string) => void
  disconnect: () => void
  send: (message: string) => void
  handleMessage: (message: WebSocketMessage) => void
}

const defaultOptions: TestOptions = {
  subscription: '',
  concurrency: 2,
  threads: 1,
  timeout: 15,
  unique: true,
  groupname: '',
  speedtestMode: 'all',
  sortMethod: 'rspeed',
  language: 'cn',
  fontSize: 24,
  theme: 'rainbow',
  appearance: 'dark',
  testMode: 2,
  downloadSize: 'cloudflare',
  downloadUrl: '',
  uploadEnable: false,
  uploadSize: 'cloudflare',
  uploadUrl: '',
  workerKey: '',
}

export const useTestStore = create<TestState>()(persist((set, get) => ({
  loading: false,
  result: [],
  testCount: 0,
  testOkCount: 0,
  totalTraffic: 0,
  totalTime: 0,
  picdata: '',
  ipv4: '',
  ipv6: '',
  ipv4geo: '',
  ipv6geo: '',
  runUploadEnabled: false,
  columnOrder: [],
  columnSizing: {},
  currentTestingId: null,
  currentDirection: null,
  liveDownloadBps: 0,
  liveUploadBps: 0,
  selectedNodes: [],
  ws: null,
  options: defaultOptions,

  setLoading: (loading) => set({ loading }),
  setResult: (result) => set({ result }),
  
  updateNode: (id, data) => set((state) => ({
    result: state.result.map((node) =>
      node.id === id ? { ...node, ...data } : node
    ),
  })),
  
  addNodes: (nodes) => set((state) => ({
    result: [...state.result, ...nodes],
  })),
  
  setSelectedNodes: (nodes) => set({ selectedNodes: nodes }),
  
  setOptions: (options) => set((state) => ({
    options: { ...state.options, ...options },
  })),
  
  incrementTestCount: () => set((state) => ({ testCount: state.testCount + 1 })),
  incrementTestOkCount: () => set((state) => ({ testOkCount: state.testOkCount + 1 })),
  addTraffic: (traffic) => set((state) => ({ totalTraffic: state.totalTraffic + traffic })),
  incrementTime: () => set((state) => ({ totalTime: state.totalTime + 1 })),
  setPicdata: (data) => set({ picdata: data }),

  // 用当前语言/主题/深浅色重新生成结果图片(不重测)。节点按当前排序方式排序,
  // 与后端自动出图保持一致;成功则更新 picdata。
  regenerateImage: async () => {
    const s = get()
    if (s.result.length === 0) return false
    const spd = (n: TestNode) => { const v = getSpeed(n.speed); return isNaN(v) ? -1 : v }
    const png = (n: TestNode) => (typeof n.ping === 'number' ? n.ping : 0)
    const nodes = [...s.result]
    switch (s.options.sortMethod) {
      case 'speed': nodes.sort((a, b) => spd(a) - spd(b)); break
      case 'rspeed': nodes.sort((a, b) => spd(b) - spd(a)); break
      case 'ping': nodes.sort((a, b) => (png(a) || 1e9) - (png(b) || 1e9)); break
      case 'rping': nodes.sort((a, b) => png(b) - png(a)); break
    }
    const payload = {
      language: s.options.language,
      appearance: s.options.appearance,
      theme: s.options.theme,
      fontSize: s.options.fontSize,
      traffic: s.totalTraffic,
      duration: formatSeconds(s.totalTime),
      successCount: s.testOkCount,
      linksCount: s.result.length,
      ipv4: s.ipv4, ipv6: s.ipv6, ipv4geo: s.ipv4geo, ipv6geo: s.ipv6geo,
      nodes: nodes.map((n) => ({
        group: n.group,
        remarks: n.remark,
        protocol: n.protocol,
        ping: String(n.ping),
        avgSpeed: Math.max(0, Math.floor(getSpeed(n.speed)) || 0),
        uploadSpeed: Math.max(0, Math.floor(getSpeed(n.uploadspeed ?? '')) || 0),
      })),
    }
    try {
      const resp = await fetch('/renderImage', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (!resp.ok) return false
      const json = await resp.json()
      if (json?.data) {
        set({ picdata: json.data })
        return true
      }
      return false
    } catch {
      return false
    }
  },

  setRunUploadEnabled: (v) => set({ runUploadEnabled: v }),
  setColumnOrder: (order) => set({ columnOrder: order }),
  setColumnSizing: (sizing) => set({ columnSizing: sizing }),
  resetTableLayout: () => set({ columnOrder: [], columnSizing: {} }),

  reset: () => set({
    result: [],
    testCount: 0,
    testOkCount: 0,
    totalTraffic: 0,
    totalTime: 0,
    picdata: '',
    ipv4: '',
    ipv6: '',
    ipv4geo: '',
    ipv6geo: '',
    runUploadEnabled: false,
    currentTestingId: null,
    currentDirection: null,
    liveDownloadBps: 0,
    liveUploadBps: 0,
    selectedNodes: [],
  }),

  connect: (url) => {
    const ws = new WebSocket(url)
    set({ ws })
    
    ws.onmessage = (event) => {
      const message = JSON.parse(event.data) as WebSocketMessage
      get().handleMessage(message)
    }
    
    ws.onclose = () => {
      set({ ws: null })
    }
    
    ws.onerror = () => {
      set({ ws: null, loading: false })
    }
  },
  
  disconnect: () => {
    const { ws } = get()
    if (ws) {
      ws.close()
      set({ ws: null, loading: false })
    }
  },
  
  send: (message) => {
    const { ws } = get()
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(message)
    }
  },

  handleMessage: (json) => {
    const state = get()
    const id = json.id
    const testing = tt(state.options.language, 'status.testing')

    switch (json.info) {
      case 'started':
        break
      case 'fetchingsub':
        break
      case 'begintest':
        break
      case 'gotserver':
        state.updateNode(id, {
          id,
          group: state.options.groupname || json.group || '',
          remark: json.remarks || '',
          server: json.server || '',
          protocol: json.protocol || '',
          link: json.link || '',
          loss: '0.00%',
          ping: 0,
          speed: '0.00B',
          maxspeed: '0.00B',
        })
        break
      case 'gotservers':
        if (json.servers) {
          const nodes: TestNode[] = json.servers.map((s) => ({
            id: s.id,
            group: state.options.groupname || s.group || '',
            remark: s.remarks || '',
            server: s.server || '',
            protocol: s.protocol || '',
            link: s.link || '',
            loss: '0.00%',
            ping: 0,
            speed: '0.00B',
            maxspeed: '0.00B',
          }))
          state.addNodes(nodes)
        }
        break
      case 'startping':
        set({ currentTestingId: id })
        state.updateNode(id, { ping: testing, testing: true })
        break
      case 'ipinfo':
        set({
          ipv4: json.ipv4 || '',
          ipv6: json.ipv6 || '',
          ipv4geo: json.ipv4geo || '',
          ipv6geo: json.ipv6geo || '',
        })
        break
      case 'gotping':
        state.incrementTestCount()
        if (json.ping && json.ping > 0) {
          state.incrementTestOkCount()
        }
        // 不置 testing:false:测速/上传阶段仍在进行,保持高亮直到 endone(修复每秒闪烁)
        state.updateNode(id, { ping: json.ping || 0 })
        break
      case 'startspeed':
        set({ currentTestingId: id, currentDirection: 'down', liveDownloadBps: 0 })
        state.updateNode(id, { speed: testing, maxspeed: testing, testing: true })
        break
      case 'gotspeed':
        state.addTraffic(json.traffic || 0)
        set({ currentTestingId: id, currentDirection: 'down', liveDownloadBps: json.traffic || 0 })
        state.updateNode(id, {
          speed: json.speed || 'N/A',
          maxspeed: json.maxspeed || 'N/A',
        })
        break
      case 'startupload':
        set({ currentTestingId: id, currentDirection: 'up', liveUploadBps: 0 })
        state.updateNode(id, { uploadspeed: testing, maxuploadspeed: testing, testing: true })
        break
      case 'gotupload':
        state.addTraffic(json.traffic || 0)
        set({ currentTestingId: id, currentDirection: 'up', liveUploadBps: json.traffic || 0 })
        state.updateNode(id, {
          uploadspeed: json.uploadspeed || 'N/A',
          maxuploadspeed: json.maxuploadspeed || 'N/A',
        })
        break
      case 'endone':
        state.updateNode(id, { testing: false })
        break
      case 'picdata':
        state.setPicdata(json.data || '')
        break
      case 'eof':
        set((s) => ({
          loading: false,
          currentTestingId: null,
          currentDirection: null,
          liveDownloadBps: 0,
          liveUploadBps: 0,
          // 兜底:清掉任何残留 testing:true 的节点(若某 id 的 endone 未送达),
          // 否则该行的选择复选框会永久禁用
          result: s.result.some((n) => n.testing)
            ? s.result.map((n) => (n.testing ? { ...n, testing: false } : n))
            : s.result,
        }))
        break
      case 'error':
        console.error('Error:', json.reason)
        if (json.reason === 'invalidsub' || json.reason === 'nonodes' || json.reason === 'norecoglink') {
          set({ loading: false })
        }
        break
    }
  },
}), {
  // 持久化用户配置 + 表格布局偏好;测试结果/连接等瞬时状态不落盘
  name: 'litespeedtest-options',
  partialize: (state) => ({
    options: state.options,
    columnOrder: state.columnOrder,
    columnSizing: state.columnSizing,
  }),
  // 用默认值补齐历史存档中缺失的字段,避免新增选项时读到 undefined
  merge: (persisted, current) => {
    const p = (persisted ?? {}) as Partial<TestState>
    return {
      ...current,
      options: { ...current.options, ...(p.options ?? {}) },
      columnOrder: p.columnOrder ?? current.columnOrder,
      columnSizing: p.columnSizing ?? current.columnSizing,
    }
  },
}))

