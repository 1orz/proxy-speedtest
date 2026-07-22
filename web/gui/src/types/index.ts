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
}

export interface TestOptions {
  subscription: string
  concurrency: number
  threads: number
  timeout: number
  unique: boolean
  groupname: string
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

