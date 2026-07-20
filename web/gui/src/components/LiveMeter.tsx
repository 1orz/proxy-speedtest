import { useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { Download, Upload } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { useTestStore } from '@/store/test-store'
import { bytesToSize, getSpeedColor } from '@/lib/utils'

const MAX_BPS = 125 * 1024 * 1024 // ~1 Gbps 满量程
const SPARK_LEN = 40
const START_ANGLE = 225 // 表盘起点(左下 7:30),缺口在底部
const SWEEP = 270 // 扫过 270°

// 速率 → 表盘填充比例。sqrt 压缩,让低速段也有可见变化。
function speedFraction(bps: number): number {
  if (bps <= 0) return 0
  return Math.min(1, Math.sqrt(bps / MAX_BPS))
}

function polar(cx: number, cy: number, r: number, deg: number) {
  const rad = ((deg - 90) * Math.PI) / 180
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) }
}

function arcPath(cx: number, cy: number, r: number, startAngle: number, endAngle: number): string {
  const start = polar(cx, cy, r, endAngle)
  const end = polar(cx, cy, r, startAngle)
  const largeArc = endAngle - startAngle <= 180 ? '0' : '1'
  return `M ${start.x.toFixed(2)} ${start.y.toFixed(2)} A ${r} ${r} 0 ${largeArc} 0 ${end.x.toFixed(2)} ${end.y.toFixed(2)}`
}

// rAF 缓动:displayed 平滑逼近 target(样本约 1/秒到达,插值出流畅的数字滚动/指针)
function useEased(target: number): number {
  const [val, setVal] = useState(0)
  const targetRef = useRef(target)
  const valRef = useRef(0)
  targetRef.current = target
  useEffect(() => {
    let raf = 0
    const tick = () => {
      const t = targetRef.current
      const cur = valRef.current
      const next = Math.abs(t - cur) < 1024 ? t : cur + (t - cur) * 0.18
      valRef.current = next
      setVal(next)
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [])
  return val
}

export function LiveMeter() {
  const currentTestingId = useTestStore((s) => s.currentTestingId)
  const currentDirection = useTestStore((s) => s.currentDirection)
  const liveDownloadBps = useTestStore((s) => s.liveDownloadBps)
  const liveUploadBps = useTestStore((s) => s.liveUploadBps)
  const result = useTestStore((s) => s.result)
  const testCount = useTestStore((s) => s.testCount)
  const theme = useTestStore((s) => s.options.theme)

  const isUp = currentDirection === 'up'
  const current = isUp ? liveUploadBps : liveDownloadBps
  const node = result.find((n) => n.id === currentTestingId)
  const total = result.length
  const progress = total > 0 ? Math.min(100, Math.floor((testCount / total) * 100)) : 0

  const displayed = useEased(current)
  const frac = speedFraction(displayed)
  const speedColor = getSpeedColor(displayed, theme)

  // 火花线样本(换节点/换方向时清空;否则追加实时样本)
  const [samples, setSamples] = useState<number[]>([])
  const dirKey = `${currentTestingId}-${currentDirection}`
  const prevKeyRef = useRef(dirKey)
  useEffect(() => {
    // ref 改写放在 effect 体内(而非 setState updater 内)保持 updater 纯净,
    // 避免 StrictMode 下 updater 被双调用导致换节点/换方向时火花线不清空。
    if (prevKeyRef.current !== dirKey) {
      prevKeyRef.current = dirKey
      setSamples([current])
    } else {
      setSamples((prev) => [...prev.slice(-(SPARK_LEN - 1)), current])
    }
  }, [current, dirKey])

  // 表盘几何
  const size = 260
  const cx = size / 2
  const cy = size / 2
  const r = 108
  const track = arcPath(cx, cy, r, START_ANGLE, START_ANGLE + SWEEP)
  const prog = arcPath(cx, cy, r, START_ANGLE, START_ANGLE + Math.max(0.0001, frac) * SWEEP)

  const gradId = isUp ? 'liveGradUp' : 'liveGradDown'
  const accent = isUp ? 'text-violet-400' : 'text-cyan-400'

  // 火花线路径
  const sparkMax = Math.max(1, ...samples)
  const sparkPts = samples
    .map((v, i) => {
      const x = samples.length > 1 ? (i / (samples.length - 1)) * 100 : 0
      const y = 30 - (v / sparkMax) * 28
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')

  return (
    <motion.div
      initial={{ opacity: 0, y: -12, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: -12, scale: 0.98 }}
      transition={{ duration: 0.3 }}
    >
      <Card className="relative overflow-hidden p-6">
        <div
          className="pointer-events-none absolute inset-0 opacity-60"
          style={{
            background: isUp
              ? 'radial-gradient(120% 80% at 50% 0%, rgba(167,139,250,0.12), transparent)'
              : 'radial-gradient(120% 80% at 50% 0%, rgba(34,211,238,0.12), transparent)',
          }}
        />
        <div className="relative flex flex-col items-center gap-3">
          {/* 当前节点 + 阶段 */}
          <div className="flex w-full items-center justify-between text-sm">
            <span className="truncate text-muted-foreground">
              正在测速:
              <span className="ml-1 font-medium text-foreground">
                {node ? node.remark || `节点 #${currentTestingId}` : '准备中…'}
              </span>
              {node?.protocol && <span className="ml-1 text-xs text-muted-foreground">· {node.protocol}</span>}
            </span>
            <span className={`flex shrink-0 items-center gap-1 font-medium ${accent}`}>
              {isUp ? <Upload className="h-4 w-4" /> : <Download className="h-4 w-4" />}
              {isUp ? '上传中' : '下载中'}
            </span>
          </div>

          {/* 表盘 */}
          <div className="relative" style={{ width: size, height: size * 0.82 }}>
            <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="overflow-visible">
              <defs>
                <linearGradient id="liveGradDown" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0%" stopColor="#22d3ee" />
                  <stop offset="100%" stopColor="#3b82f6" />
                </linearGradient>
                <linearGradient id="liveGradUp" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0%" stopColor="#a855f7" />
                  <stop offset="100%" stopColor="#ec4899" />
                </linearGradient>
              </defs>
              <path d={track} fill="none" stroke="currentColor" strokeWidth={14} strokeLinecap="round" className="text-muted/25" />
              <path
                d={prog}
                fill="none"
                stroke={`url(#${gradId})`}
                strokeWidth={14}
                strokeLinecap="round"
                style={{ transition: 'stroke 0.4s ease' }}
              />
            </svg>
            {/* 中心数字 */}
            <div className="absolute inset-0 flex flex-col items-center justify-center pt-2">
              <div className="text-4xl font-bold tabular-nums transition-colors" style={{ color: speedColor }}>
                {bytesToSize(displayed)}
                <span className="ml-1 text-lg font-medium text-muted-foreground">/s</span>
              </div>
              {/* 火花线 */}
              <svg width="120" height="32" viewBox="0 0 100 32" className="mt-2 opacity-80" preserveAspectRatio="none">
                {samples.length > 1 && (
                  <polyline points={sparkPts} fill="none" stroke={`url(#${gradId})`} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
                )}
              </svg>
            </div>
          </div>

          {/* 进度 */}
          <div className="w-full">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>已完成 {testCount}</span>
              <span>共 {total} 节点</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted/30">
              <motion.div
                className="h-full rounded-full bg-primary"
                initial={false}
                animate={{ width: `${progress}%` }}
                transition={{ duration: 0.4 }}
              />
            </div>
          </div>
        </div>
      </Card>
    </motion.div>
  )
}
