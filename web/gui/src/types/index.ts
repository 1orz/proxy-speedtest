export interface TestNode {
  id: number
  group: string
  remark: string
  server: string
  protocol: string
  link: string
  loss: string
  ping: number | string
  speed: string
  maxspeed: string
  uploadspeed?: string
  maxuploadspeed?: string
  testing?: boolean
  tested?: boolean // 该节点的测试周期是否已结束(endone)。用于区分「待测(Pending)」与「已测但失败(N/A)」
}

export interface SubEntry {
  group: string // 组名,可空
  url: string // 订阅链接 / 节点链接 / base64 / clash
}

export interface HeaderEntry {
  name: string // 请求头名称(如 X-Speedtest-Token);为空则忽略
  value: string
}

export interface TestOptions {
  subscriptions: SubEntry[]
  headers: HeaderEntry[]
  concurrency: number
  threads: number
  timeout: number
  unique: boolean
  speedtestMode: 'all' | 'pingonly' | 'speedonly'
  sortMethod: 'rspeed' | 'speed' | 'ping' | 'rping' | 'none'
  language: 'en' | 'cn'
  fontSize: number
  theme: 'rainbow' | 'original'
  appearance: 'dark' | 'light'
  testMode: number
  downloadSize: string
  downloadUrl: string
  uploadEnable: boolean
  uploadSize: string
  uploadUrl: string
  // 自定义请求头:headersUnified=true 时上传与下载共用 downloadHeaders;false 时分别使用两份
  headersUnified: boolean
  downloadHeaders: HeaderEntry[]
  uploadHeaders: HeaderEntry[]
}

export interface WebSocketMessage {
  id: number
  info: string
  remarks?: string
  server?: string
  group?: string
  ping?: number
  lost?: string
  speed?: string
  maxspeed?: string
  uploadspeed?: string
  maxuploadspeed?: string
  traffic?: number
  link?: string
  protocol?: string
  data?: string
  servers?: WebSocketMessage[]
  reason?: string
  path?: string
  ipv4?: string
  ipv6?: string
  ipv4geo?: string
  ipv6geo?: string
}

export type SpeedTestMode = 'all' | 'pingonly' | 'speedonly'
export type SortMethod = 'rspeed' | 'speed' | 'ping' | 'rping' | 'none'
export type Language = 'en' | 'cn'
export type Theme = 'rainbow' | 'original'

