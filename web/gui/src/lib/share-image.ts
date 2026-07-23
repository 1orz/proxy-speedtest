// share-image.ts —— 前端「真·矢量」测速结果卡片生成器,仅用于导出(下载/复制 PNG·SVG)。
// 屏幕上的结果视图是交互式表格(ResultCard),这里只负责把当前排序/可见列快照渲染成 SVG。
// 纯字符串拼 SVG(文字 + 色块),不引用外部资源:SVG 干净可缩放,栅格化到 canvas 不污染。

import { getSpeed, getSpeedColor } from './utils'

export type ColKey = 'group' | 'name' | 'server' | 'protocol' | 'ping' | 'download' | 'upload'

export interface ShareCol {
  key: ColKey
  label: string
}

// 单元格显示语义:good=成功(绿) bad=失败(红) muted=占位/零(灰) speed=有效速度(彩色色块)
export type CellTone = 'good' | 'bad' | 'muted' | 'speed'
export interface Cell {
  text: string
  tone: CellTone
}

export interface ShareRow {
  group: string
  name: string
  server: string // 调用方已按隐私脱敏
  protocol: string
  ping: Cell
  download: Cell
  upload: Cell
}

export interface ShareInput {
  appearance: 'dark' | 'light'
  theme: 'rainbow' | 'original'
  title: string
  version?: string
  subtitle: string
  stats: { label: string; value: string }[]
  ipTitle: string
  ipLines: { label: string; value: string }[] // 调用方已按隐私脱敏;空数组则不渲染该块
  columns: ShareCol[] // 有序、已过滤为可见列
  rows: ShareRow[]
  footer: string
}

export interface ShareSvg {
  svg: string
  width: number
  height: number
}

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'
const SANS = 'ui-sans-serif, -apple-system, "Segoe UI", Roboto, "PingFang SC", "Microsoft YaHei", sans-serif'

const COL_WIDTH: Record<ColKey, number> = {
  group: 130,
  name: 260,
  server: 200,
  protocol: 96,
  ping: 100,
  download: 150,
  upload: 150,
}

const MONO_COLS: Record<ColKey, boolean> = {
  group: false, name: false, server: true, protocol: false, ping: true, download: true, upload: true,
}

function palette(appearance: 'dark' | 'light') {
  return appearance === 'light'
    ? { bg: '#ffffff', panel: '#f1f5f9', text: '#0f172a', muted: '#64748b', border: '#e2e8f0', accent: '#2563eb', success: '#16a34a', danger: '#dc2626' }
    : { bg: '#0f172a', panel: '#1e293b', text: '#e2e8f0', muted: '#94a3b8', border: '#334155', accent: '#60a5fa', success: '#4ade80', danger: '#f87171' }
}

function esc(s: string): string {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

// 宽字符(CJK/韩文/全角/emoji)约按 1 个 em 计,拉丁约 0.55 em —— 仅用于截断与色块宽度估算。
function charWidth(ch: string, fs: number): number {
  const cp = ch.codePointAt(0) ?? 0
  const wide =
    cp >= 0x1100 &&
    (cp <= 0x115f ||
      (cp >= 0x2e80 && cp <= 0xa4cf) ||
      (cp >= 0xac00 && cp <= 0xd7a3) ||
      (cp >= 0xf900 && cp <= 0xfaff) ||
      (cp >= 0xfe30 && cp <= 0xfe4f) ||
      (cp >= 0xff00 && cp <= 0xff60) ||
      (cp >= 0xffe0 && cp <= 0xffe6) ||
      cp >= 0x1f000)
  return fs * (wide ? 1.0 : 0.55)
}

function estWidth(text: string, fs: number): number {
  let w = 0
  for (const ch of text) w += charWidth(ch, fs)
  return w
}

function truncate(text: string, maxW: number, fs: number): string {
  let w = 0
  let out = ''
  for (const ch of text) {
    const cw = charWidth(ch, fs)
    if (w + cw > maxW) return out + '…'
    w += cw
    out += ch
  }
  return out
}

function textEl(
  x: number,
  y: number,
  content: string,
  opts: { size: number; color: string; weight?: number; mono?: boolean; anchor?: 'start' | 'middle' | 'end' }
): string {
  const weight = opts.weight ? ` font-weight="${opts.weight}"` : ''
  const family = opts.mono ? MONO : SANS
  const anchor = opts.anchor ? ` text-anchor="${opts.anchor}"` : ''
  return `<text x="${x}" y="${y}" font-family='${family}' font-size="${opts.size}" fill="${opts.color}"${weight}${anchor}>${esc(content)}</text>`
}

export function buildShareSvg(input: ShareInput): ShareSvg {
  const c = palette(input.appearance)
  const P = 32
  const gap = 12
  const cols = input.columns

  const tableW = cols.reduce((s, col) => s + COL_WIDTH[col.key], 0)
  const width = P * 2 + tableW

  const colX: number[] = []
  {
    let x = P
    for (const col of cols) {
      colX.push(x)
      x += COL_WIDTH[col.key]
    }
  }

  const rowH = 34
  const headerH = 38
  const parts: string[] = []
  let y = P

  // 标题 + 版本徽标
  parts.push(textEl(P, y + 24, input.title, { size: 22, color: c.text, weight: 700 }))
  if (input.version) {
    const bx = P + estWidth(input.title, 22) + 10
    const bw = estWidth(input.version, 11) + 14
    parts.push(`<rect x="${bx}" y="${y + 8}" width="${bw}" height="18" rx="5" fill="${c.accent}" fill-opacity="0.15"/>`)
    parts.push(textEl(bx + 7, y + 21, input.version, { size: 11, color: c.accent, weight: 600, mono: true }))
  }
  if (input.subtitle) parts.push(textEl(P, y + 44, input.subtitle, { size: 12, color: c.muted }))
  y += 62

  // 汇总 chips
  if (input.stats.length > 0) {
    const n = input.stats.length
    const chipW = (tableW - gap * (n - 1)) / n
    const chipH = 46
    input.stats.forEach((s, i) => {
      const cx = P + i * (chipW + gap)
      parts.push(`<rect x="${cx}" y="${y}" width="${chipW}" height="${chipH}" rx="8" fill="${c.panel}"/>`)
      parts.push(textEl(cx + 12, y + 19, s.label, { size: 11, color: c.muted }))
      parts.push(textEl(cx + 12, y + 37, s.value, { size: 16, color: c.text, weight: 700 }))
    })
    y += chipH + 18
  }

  // 出口 IP 块
  if (input.ipLines.length > 0) {
    parts.push(textEl(P, y + 12, input.ipTitle, { size: 12, color: c.accent, weight: 700 }))
    y += 22
    for (const line of input.ipLines) {
      parts.push(textEl(P, y + 12, line.label, { size: 12, color: c.muted, weight: 700 }))
      parts.push(textEl(P + 56, y + 12, line.value, { size: 13, color: c.text, mono: true }))
      y += 22
    }
    y += 10
  }

  // 表头
  parts.push(`<rect x="${P}" y="${y}" width="${tableW}" height="${headerH}" rx="8" fill="${c.panel}"/>`)
  cols.forEach((col, i) => {
    parts.push(textEl(colX[i] + 12, y + 24, col.label, { size: 12, color: c.muted, weight: 600 }))
  })
  y += headerH

  // 数据行
  input.rows.forEach((row, ri) => {
    const rowTop = y + ri * rowH
    if (ri > 0) {
      parts.push(`<line x1="${P}" y1="${rowTop}" x2="${P + tableW}" y2="${rowTop}" stroke="${c.border}" stroke-width="1" stroke-opacity="0.5"/>`)
    }
    const baseline = rowTop + rowH / 2 + 5
    cols.forEach((col, i) => {
      const x = colX[i]
      const w = COL_WIDTH[col.key]
      if (col.key === 'download' || col.key === 'upload') {
        const cell = col.key === 'download' ? row.download : row.upload
        parts.push(toneCell(x, rowTop, rowH, w, cell, input.theme, c))
        return
      }
      if (col.key === 'ping') {
        const color = row.ping.tone === 'good' ? c.success : row.ping.tone === 'bad' ? c.danger : c.muted
        parts.push(textEl(x + 12, baseline, truncate(row.ping.text || '-', w - 20, 13), { size: 13, color, mono: true }))
        return
      }
      const raw = String((row as unknown as Record<string, unknown>)[col.key] ?? '')
      let color = c.text
      let weight: number | undefined
      if (col.key === 'name') weight = 500
      else if (col.key === 'protocol' || col.key === 'group' || col.key === 'server') color = c.muted
      parts.push(textEl(x + 12, baseline, truncate(raw || '-', w - 20, 13), { size: 13, color, weight, mono: MONO_COLS[col.key] }))
    })
  })
  y += input.rows.length * rowH

  // 页脚
  parts.push(`<line x1="${P}" y1="${y + 12}" x2="${P + tableW}" y2="${y + 12}" stroke="${c.border}" stroke-width="1"/>`)
  parts.push(textEl(P, y + 32, input.footer, { size: 11, color: c.muted }))
  y += 44

  const height = y + P - 4
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">` +
    `<rect x="0" y="0" width="${width}" height="${height}" fill="${c.bg}"/>` +
    parts.join('') +
    `</svg>`
  return { svg, width, height }
}

// toneCell 渲染下载/上传单元格:speed→彩色色块+黑字;bad→红字(Failed);muted→灰字(0B/s / 占位)。
function toneCell(
  x: number,
  rowTop: number,
  rowH: number,
  w: number,
  cell: Cell,
  theme: 'rainbow' | 'original',
  c: ReturnType<typeof palette>
): string {
  const baseline = rowTop + rowH / 2 + 5
  if (cell.tone === 'speed') {
    const v = getSpeed(cell.text)
    if (!isNaN(v) && v > 0) {
      const color = getSpeedColor(v, theme)
      const pillH = 22
      const pillTop = rowTop + (rowH - pillH) / 2
      const pillW = Math.min(w - 16, Math.max(48, estWidth(cell.text, 12) + 20))
      return (
        `<rect x="${x + 8}" y="${pillTop}" width="${pillW}" height="${pillH}" rx="6" fill="${color}"/>` +
        textEl(x + 8 + pillW / 2, pillTop + 15, cell.text, { size: 12, color: '#000', weight: 600, mono: true, anchor: 'middle' })
      )
    }
  }
  const color = cell.tone === 'bad' ? c.danger : c.muted
  return textEl(x + 12, baseline, truncate(cell.text || '-', w - 20, 13), { size: 13, color, mono: true })
}

// ---- 导出/复制工具 ----

export function svgToBlob(svg: string): Blob {
  return new Blob([svg], { type: 'image/svg+xml;charset=utf-8' })
}

// 用 data URL 载入 SVG 再画到高 DPI canvas;data URL 同源不污染 canvas,toBlob 可用。
export function svgToPngBlob(share: ShareSvg, scale = 3): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = Math.round(share.width * scale)
      canvas.height = Math.round(share.height * scale)
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('canvas 2d context unavailable'))
        return
      }
      ctx.scale(scale, scale)
      ctx.drawImage(img, 0, 0)
      canvas.toBlob((blob) => {
        if (blob) resolve(blob)
        else reject(new Error('canvas.toBlob returned null'))
      }, 'image/png')
    }
    img.onerror = () => reject(new Error('failed to load SVG for rasterization'))
    img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(share.svg)
  })
}

export function triggerDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// 复制位图到剪贴板(image/png)。需要安全上下文(HTTPS/localhost),不支持则由调用方回退到下载。
export function canCopyImage(): boolean {
  return typeof ClipboardItem !== 'undefined' && !!navigator.clipboard?.write
}

export async function copyBlobToClipboard(blob: Blob): Promise<void> {
  if (!canCopyImage()) throw new Error('clipboard image write unsupported')
  await navigator.clipboard.write([new ClipboardItem({ [blob.type]: blob })])
}
