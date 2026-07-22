import { useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { Download, Upload } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'
import { bytesToSize } from '@/lib/utils'

const MAX_BPS = 125 * 1024 * 1024 // ~1 Gbps 满量程
const SPARK_LEN = 64
const START_ANGLE = 225 // 表盘起点(左下 7:30),缺口在底部
const SWEEP = 270 // 扫过 270°
// 刻度标签(MB/s),用 sqrt 压缩后的比例定位,呈现 OpenSpeedTest 式非线性刻度感
const SCALE_MARKS = [1, 5, 10, 25, 50, 100]

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
  // 在 effect 里同步 target(而非渲染期写 ref),满足 React 规则;rAF 循环读取最新值
  useEffect(() => {
    targetRef.current = target
  }, [target])
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
  const t = useI18n()
  const currentTestingId = useTestStore((s) => s.currentTestingId)
  const currentDirection = useTestStore((s) => s.currentDirection)
  const liveDownloadBps = useTestStore((s) => s.liveDownloadBps)
  const liveUploadBps = useTestStore((s) => s.liveUploadBps)
  const result = useTestStore((s) => s.result)
  const testCount = useTestStore((s) => s.testCount)

  const isUp = currentDirection === 'up'
  const current = isUp ? liveUploadBps : liveDownloadBps
  const node = result.find((n) => n.id === currentTestingId)
  const total = result.length
  const progress = total > 0 ? Math.min(100, Math.floor((testCount / total) * 100)) : 0

  const displayed = useEased(current)
  const frac = speedFraction(displayed)

  // 火花线样本(换节点/换方向时清空;否则追加实时样本)
  const [samples, setSamples] = useState<number[]>([])
  const dirKey = `${currentTestingId}-${currentDirection}`
  const prevKeyRef = useRef(dirKey)
  useEffect(() => {
    if (prevKeyRef.current !== dirKey) {
      prevKeyRef.current = dirKey
      setSamples([current])
    } else {
      setSamples((prev) => [...prev.slice(-(SPARK_LEN - 1)), current])
    }
  }, [current, dirKey])

  // 表盘几何
  const size = 300
  const cx = size / 2
  const cy = size / 2
  const r = 120
  const strokeW = 18
  const track = arcPath(cx, cy, r, START_ANGLE, START_ANGLE + SWEEP)
  const prog = arcPath(cx, cy, r, START_ANGLE, START_ANGLE + Math.max(0.0001, frac) * SWEEP)
  const tip = polar(cx, cy, r, START_ANGLE + frac * SWEEP)

  const gradId = isUp ? 'liveGradUp' : 'liveGradDown'
  const fillId = isUp ? 'liveFillUp' : 'liveFillDown'
  const glowId = 'liveGlow'
  const accent = isUp ? 'text-violet-400' : 'text-cyan-400'

  // 峰值(当前方向本轮)
  const peak = samples.length ? Math.max(...samples) : 0

  // 实时曲线(面积图):自动缩放到本轮峰值,底部为基线
  const sparkMax = Math.max(1, peak) * 1.12
  const pts = samples.map((v, i) => {
    const x = samples.length > 1 ? (i / (samples.length - 1)) * 100 : 0
    const y = 40 - Math.min(1, v / sparkMax) * 36
    return { x, y }
  })
  const lineD = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ')
  const areaD = pts.length > 1 ? `${lineD} L 100 40 L 0 40 Z` : ''
  const last = pts[pts.length - 1]

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
              {t('live.testing')}
              <span className="ml-1 font-medium text-foreground">
                {node ? node.remark || t('live.node', { id: currentTestingId ?? 0 }) : t('live.preparing')}
              </span>
              {node?.protocol && <span className="ml-1 text-xs text-muted-foreground">· {node.protocol}</span>}
            </span>
            <span className={`flex shrink-0 items-center gap-1 font-medium ${accent}`}>
              {isUp ? <Upload className="h-4 w-4" /> : <Download className="h-4 w-4" />}
              {isUp ? t('live.uploading') : t('live.downloading')}
            </span>
          </div>

          {/* 表盘(响应式:窄屏自动缩小,不会被卡片裁切) */}
          <div className="relative w-full" style={{ maxWidth: size }}>
            <svg viewBox={`0 0 ${size} ${size}`} className="h-auto w-full overflow-visible">
              <defs>
                <linearGradient id="liveGradDown" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0%" stopColor="#22d3ee" />
                  <stop offset="100%" stopColor="#3b82f6" />
                </linearGradient>
                <linearGradient id="liveGradUp" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0%" stopColor="#a855f7" />
                  <stop offset="100%" stopColor="#ec4899" />
                </linearGradient>
                <linearGradient id="liveFillDown" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#22d3ee" stopOpacity="0.45" />
                  <stop offset="100%" stopColor="#22d3ee" stopOpacity="0" />
                </linearGradient>
                <linearGradient id="liveFillUp" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#a855f7" stopOpacity="0.45" />
                  <stop offset="100%" stopColor="#a855f7" stopOpacity="0" />
                </linearGradient>
                <filter id="liveGlow" x="-50%" y="-50%" width="200%" height="200%">
                  <feGaussianBlur stdDeviation="4" result="b" />
                  <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>
              </defs>

              {/* 轨道 */}
              <path d={track} fill="none" stroke="currentColor" strokeWidth={strokeW} strokeLinecap="round" className="text-muted-foreground/15" />

              {/* 刻度标签(置于弧外,避免与中心大数字拥挤) */}
              {SCALE_MARKS.map((mb) => {
                const f = speedFraction(mb * 1024 * 1024)
                const p = polar(cx, cy, r + 13, START_ANGLE + f * SWEEP)
                return (
                  <text
                    key={mb}
                    x={p.x}
                    y={p.y}
                    textAnchor="middle"
                    dominantBaseline="middle"
                    className="fill-muted-foreground/60"
                    fontSize={10}
                  >
                    {mb}
                  </text>
                )
              })}

              {/* 进度弧 */}
              <path
                d={prog}
                fill="none"
                stroke={`url(#${gradId})`}
                strokeWidth={strokeW}
                strokeLinecap="round"
                filter={`url(#${glowId})`}
                style={{ transition: 'stroke 0.4s ease' }}
              />
              {/* 弧头光点 */}
              {frac > 0.001 && (
                <circle cx={tip.x} cy={tip.y} r={7} className="fill-background" stroke={`url(#${gradId})`} strokeWidth={3} filter={`url(#${glowId})`} />
              )}
            </svg>

            {/* 中心数字(文字用前景色,颜色身份由弧线承载) */}
            <div className="absolute inset-0 flex flex-col items-center justify-center">
              <div className="text-4xl font-bold tabular-nums text-foreground">
                {bytesToSize(displayed)}
              </div>
              <div className="mt-1 text-sm font-medium text-muted-foreground">/s</div>
            </div>
          </div>

          {/* 实时曲线 + 峰值 */}
          <div className="w-full">
            <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
              <span className={accent}>{isUp ? t('live.upload') : t('live.download')}</span>
              <span>{t('live.peak')} {bytesToSize(peak)}/s</span>
            </div>
            <svg width="100%" height="56" viewBox="0 0 100 40" preserveAspectRatio="none" className="overflow-visible">
              {/* 基线网格(内敛) */}
              <line x1="0" y1="20" x2="100" y2="20" stroke="currentColor" strokeWidth={0.4} className="text-muted-foreground/15" vectorEffect="non-scaling-stroke" />
              {areaD && <path d={areaD} fill={`url(#${fillId})`} stroke="none" />}
              {pts.length > 1 && (
                <path d={lineD} fill="none" stroke={`url(#${gradId})`} strokeWidth={2} vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
              )}
              {last && pts.length > 1 && (
                <circle cx={last.x} cy={last.y} r={2.4} className="fill-background" stroke={`url(#${gradId})`} strokeWidth={1.6} vectorEffect="non-scaling-stroke" />
              )}
            </svg>
          </div>

          {/* 进度 */}
          <div className="w-full">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>{t('dash.done', { n: testCount })}</span>
              <span>{t('dash.totalNodes', { n: total })}</span>
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
