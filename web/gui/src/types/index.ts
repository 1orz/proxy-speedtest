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
  testMode: number
  downloadSize: string
  downloadUrl: string
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
  traffic?: number
  link?: string
  protocol?: string
  data?: string
  servers?: WebSocketMessage[]
  reason?: string
  path?: string
}

export type SpeedTestMode = 'all' | 'pingonly' | 'speedonly'
export type SortMethod = 'rspeed' | 'speed' | 'ping' | 'rping' | 'none'
export type Language = 'en' | 'cn'
export type Theme = 'rainbow' | 'original'

