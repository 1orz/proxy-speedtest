import { useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { Activity, Download, Upload } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { useTestStore } from '@/store/test-store'
import type { LiveNode } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'
import { bytesToSize, cn } from '@/lib/utils'

const MAX_BPS = 125 * 1024 * 1024 // ~1 Gbps 满量程
const SPARK_LEN = 64
const SPARK_INTERVAL_MS = 200 // 曲线采样节拍:固定节拍 → 横轴是均匀时间轴(约 12.8s 窗口)
const START_ANGLE = 225 // 表盘起点(左下 7:30),缺口在底部
const SWEEP = 270 // 扫过 270°
// 刻度标签(MB/s),用 sqrt 压缩后的比例定位,呈现 OpenSpeedTest 式非线性刻度感
const SCALE_MARKS = [1, 5, 10, 25, 50, 100]

// 并发数超过这个值就不再一节点一表盘,改为只显示一个总速率表盘。
// 再多的表盘既排不下,也没人看得过来。
const MAX_GAUGES = 2

// 表盘的展示阶段。idle = 该表盘位暂时没有节点在跑。
type Phase = 'idle' | LiveNode['phase']

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

// rAF 缓动:displayed 平滑逼近 target(样本约 1/秒到达,插值出流畅的数字滚动/指针)。
// 返回 [显示值, 最新值的 ref],ref 供固定节拍的曲线采样读取,避免为此再多一次渲染。
//
// resetKey 变化(换节点 / 换方向)时立即归零:曲线组件读的就是这个 ref,上一段速率的
// 衰减尾巴会混进新窗口的首个样本,进而把峰值标签和纵轴量程钉在一个不属于本段的数值上。
// 归零动作放在 rAF 回调里而不是 effect 体内 —— 后者是同步 setState,会触发级联渲染。
function useEased(target: number, resetKey: string): [number, { current: number }] {
  const [val, setVal] = useState(0)
  const targetRef = useRef(target)
  const keyRef = useRef(resetKey)
  const seenKeyRef = useRef(resetKey)
  const valRef = useRef(0)
  const shownRef = useRef(0)
  // 在 effect 里同步外部输入(而非渲染期写 ref),满足 React 规则;rAF 循环读取最新值
  useEffect(() => {
    targetRef.current = target
  }, [target])
  useEffect(() => {
    keyRef.current = resetKey
  }, [resetKey])
  useEffect(() => {
    let raf = 0
    const tick = () => {
      if (seenKeyRef.current !== keyRef.current) {
        seenKeyRef.current = keyRef.current
        valRef.current = 0
      }
      const t = targetRef.current
      const cur = valRef.current
      // 收敛阈值取 1 字节:到位后不再 setState —— 否则这个 rAF 循环会在整场测速里
      // 每秒白跑 60 次渲染。比对的是「上次推给 state 的值」,归零后同样能推出去。
      const next = Math.abs(t - cur) < 1 ? t : cur + (t - cur) * 0.18
      valRef.current = next
      if (next !== shownRef.current) {
        shownRef.current = next
        setVal(next)
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [])
  return [val, valRef]
}

// GaugeDefs 是所有表盘共用的渐变/滤镜定义。SVG 的 url(#id) 引用在整个文档内解析,
// 所以只需要渲染一次,多个表盘都能引用到。
function GaugeDefs() {
  return (
    <svg width="0" height="0" className="absolute" aria-hidden>
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
    </svg>
  )
}

// Sparkline 是「实时曲线 + 峰值」。它自己按固定节拍采样,并持有采样窗口与纵轴量程;
// 父组件用 key 重挂来重置,所以这里不需要任何清空逻辑。
//
// 为什么是固定节拍:原先「速率值一变就追加一个样本」有两个毛病 —— 连续两秒速率完全
// 相同时根本不追加,曲线原地卡住;而每多一个点又把已有点在横轴上重新摊开一遍,整条线
// 看着像被反复压缩。固定节拍 + 定长时间窗(x 从右往左排)把两个问题一起消掉。
function Sparkline({
  valueRef,
  isUp,
  label,
  gradId,
  fillId,
  accent,
}: {
  valueRef: { current: number }
  isUp: boolean
  label: string
  gradId: string
  fillId: string
  accent: string
}) {
  const t = useI18n()
  // samples 与 scale 同一拍更新:纵轴量程若直接取窗口峰值,峰值一滑出窗口整条曲线
  // 就会突然纵向跳一下;这里「涨立刻跟上、跌缓慢回落」,量程连续变化。
  const [spark, setSpark] = useState<{ samples: number[]; scale: number }>({ samples: [], scale: 1 })

  useEffect(() => {
    const id = setInterval(() => {
      setSpark((prev) => {
        const samples = [...prev.samples.slice(-(SPARK_LEN - 1)), valueRef.current]
        const target = Math.max(1, ...samples) * 1.12
        const scale = target > prev.scale ? target : prev.scale + (target - prev.scale) * 0.08
        return { samples, scale }
      })
    }, SPARK_INTERVAL_MS)
    return () => clearInterval(id)
  }, [valueRef])

  const { samples, scale } = spark
  const peak = samples.length ? Math.max(...samples) : 0

  // 横轴是定长时间窗:最新的点永远贴右边,老点按固定步距左移;样本不足时曲线从右侧
  // 长出来,而不是被拉伸铺满整幅。
  const pts = samples.map((v, i) => ({
    x: 100 - ((samples.length - 1 - i) / (SPARK_LEN - 1)) * 100,
    y: 40 - Math.min(1, v / scale) * 36,
  }))
  const lineD = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ')
  const areaD = pts.length > 1 ? `${lineD} L 100 40 L ${pts[0].x.toFixed(2)} 40 Z` : ''
  const last = pts[pts.length - 1]

  return (
    <div className="w-full">
      <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
        <span className={accent}>{label}</span>
        <span>{t('live.peak')} {bytesToSize(peak)}/s</span>
      </div>
      {/* 曲线用 preserveAspectRatio="none" 拉满整幅宽度(横轴是时间,不需要等比)。
          代价是圆形会被同样拉扁 —— 原来画在 SVG 里的端点圆就成了一颗扁椭圆。
          所以端点改用下面这个按百分比定位的 DOM 圆点,它不受 viewBox 缩放影响。 */}
      <div className="relative">
        <svg width="100%" height="56" viewBox="0 0 100 40" preserveAspectRatio="none" className="block">
          {/* 基线网格(内敛) */}
          <line x1="0" y1="20" x2="100" y2="20" stroke="currentColor" strokeWidth={0.4} className="text-muted-foreground/15" vectorEffect="non-scaling-stroke" />
          {areaD && <path d={areaD} fill={`url(#${fillId})`} stroke="none" />}
          {pts.length > 1 && (
            <path d={lineD} fill="none" stroke={`url(#${gradId})`} strokeWidth={2} vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
          )}
        </svg>
        {last && pts.length > 1 && (
          <span
            className={cn(
              'pointer-events-none absolute h-2 w-2 rounded-full border-2 bg-background',
              isUp ? 'border-violet-400' : 'border-cyan-400'
            )}
            style={{ left: `${last.x}%`, top: `${(last.y / 40) * 100}%`, transform: 'translate(-50%, -50%)' }}
          />
        )}
      </div>
    </div>
  )
}

// Gauge 是一路速率的完整展示:标题行 + 表盘 + 实时曲线。既用于「单个节点」,
// 也用于并发数较高时的「总速率」——两者的区别只有喂进来的数值和标题。
function Gauge({
  title,
  subtitle,
  phase,
  bps,
  resetKey,
  compact,
  footer,
}: {
  title: string
  subtitle?: string
  phase: Phase
  bps: number
  resetKey: string
  compact?: boolean
  footer?: React.ReactNode
}) {
  const t = useI18n()
  const isUp = phase === 'up'
  const [displayed, displayedRef] = useEased(bps, resetKey)
  const frac = speedFraction(displayed)

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
  const accent = isUp ? 'text-violet-400' : 'text-cyan-400'

  const phaseLabel =
    phase === 'up' ? t('live.uploading')
      : phase === 'down' ? t('live.downloading')
        : phase === 'ping' ? t('live.pinging')
          : t('live.preparing')

  return (
    <div className="flex min-w-0 flex-col items-center gap-3">
      {/* 标题行:哪个节点(或总计)+ 当前阶段 */}
      <div className="flex w-full items-center justify-between gap-2 text-sm">
        <span className="min-w-0 truncate text-muted-foreground">
          <span className="font-medium text-foreground">{title}</span>
          {subtitle && <span className="ml-1 text-xs text-muted-foreground">· {subtitle}</span>}
        </span>
        <span className={cn('flex shrink-0 items-center gap-1 text-xs font-medium', phase === 'idle' ? 'text-muted-foreground' : accent)}>
          {phase === 'up' ? <Upload className="h-4 w-4" />
            : phase === 'down' ? <Download className="h-4 w-4" />
              : <Activity className="h-4 w-4" />}
          {phaseLabel}
        </span>
      </div>

      {/* 表盘(响应式:窄屏/双表盘时自动缩小,不会被卡片裁切) */}
      <div className="relative w-full" style={{ maxWidth: size }}>
        <svg viewBox={`0 0 ${size} ${size}`} className="h-auto w-full overflow-visible">
          {/* 轨道 */}
          <path d={track} fill="none" stroke="currentColor" strokeWidth={strokeW} strokeLinecap="round" className="text-muted-foreground/15" />

          {/* 刻度标签(置于弧外,避免与中心大数字拥挤) */}
          {SCALE_MARKS.map((mb) => {
            const f = speedFraction(mb * 1024 * 1024)
            const p = polar(cx, cy, r + 13, START_ANGLE + f * SWEEP)
            return (
              <text key={mb} x={p.x} y={p.y} textAnchor="middle" dominantBaseline="middle" className="fill-muted-foreground/60" fontSize={10}>
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
            filter="url(#liveGlow)"
            style={{ transition: 'stroke 0.4s ease' }}
          />
          {/* 弧头光点 */}
          {frac > 0.001 && (
            <circle cx={tip.x} cy={tip.y} r={7} className="fill-background" stroke={`url(#${gradId})`} strokeWidth={3} filter="url(#liveGlow)" />
          )}
        </svg>

        {/* 中心数字(文字用前景色,颜色身份由弧线承载) */}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <div className={cn('font-bold tabular-nums text-foreground', compact ? 'text-2xl sm:text-3xl' : 'text-4xl')}>
            {bytesToSize(displayed)}
          </div>
          <div className="mt-1 text-sm font-medium text-muted-foreground">/s</div>
        </div>
      </div>

      {footer}

      {/* 实时曲线 + 峰值。key=resetKey:换节点/换方向直接重挂,采样窗口自然清零。 */}
      <Sparkline
        key={resetKey}
        valueRef={displayedRef}
        isUp={isUp}
        label={isUp ? t('live.upload') : t('live.download')}
        gradId={gradId}
        fillId={fillId}
        accent={accent}
      />
    </div>
  )
}

export function LiveMeter() {
  const t = useI18n()
  const live = useTestStore((s) => s.live)
  const result = useTestStore((s) => s.result)
  const testCount = useTestStore((s) => s.testCount)
  const runConcurrency = useTestStore((s) => s.runConcurrency)

  const total = result.length
  const progress = total > 0 ? Math.min(100, Math.floor((testCount / total) * 100)) : 0

  const active = Object.values(live)
  // 并发数决定形态:≤2 时一个节点一个表盘(各测各的,互不干扰);更多时并排放不下,
  // 也没法逐个盯,改为只显示一个「总速率」表盘 —— 各节点当前样本之和才是真实的总吞吐。
  const aggregate = runConcurrency > MAX_GAUGES
  const totalDown = active.reduce((sum, n) => (n.phase === 'down' ? sum + n.bps : sum), 0)
  const totalUp = active.reduce((sum, n) => (n.phase === 'up' ? sum + n.bps : sum), 0)

  const nodeOf = (id: number) => result.find((n) => n.id === id)
  const titleOf = (n: LiveNode) => nodeOf(n.id)?.remark || t('live.node', { id: n.id })

  // 表盘位固定为 min(并发数, 2) 个,空位显示「准备中」而不是把布局挤掉一格。
  const slots = aggregate ? [] : Array.from({ length: Math.min(runConcurrency, MAX_GAUGES) }, (_, i) => active.find((n) => n.slot === i))

  // 卡片底色:只有「单表盘且正在上传」才切到紫色,多表盘时各路方向不一,保持中性青色。
  const cardIsUp = !aggregate && slots.length === 1 && slots[0]?.phase === 'up'

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
            background: cardIsUp
              ? 'radial-gradient(120% 80% at 50% 0%, rgba(167,139,250,0.12), transparent)'
              : 'radial-gradient(120% 80% at 50% 0%, rgba(34,211,238,0.12), transparent)',
          }}
        />
        <GaugeDefs />
        <div className="relative flex flex-col items-center gap-4">
          {aggregate ? (
            <Gauge
              title={t('live.total')}
              subtitle={t('live.activeNodes', { n: active.length })}
              phase={active.length ? 'down' : 'idle'}
              bps={totalDown + totalUp}
              resetKey="aggregate"
              footer={
                // 总速率把上下行加在了一起,这里拆开给出各自的分量,避免只看一个数字
                // 分不清是谁贡献的。
                <div className="flex w-full items-center justify-center gap-3 text-xs">
                  <span className="flex items-center gap-1 rounded-md bg-secondary/50 px-2 py-1 text-cyan-400">
                    <Download className="h-3.5 w-3.5" />
                    <span className="font-mono">{bytesToSize(totalDown)}/s</span>
                  </span>
                  <span className="flex items-center gap-1 rounded-md bg-secondary/50 px-2 py-1 text-violet-400">
                    <Upload className="h-3.5 w-3.5" />
                    <span className="font-mono">{bytesToSize(totalUp)}/s</span>
                  </span>
                </div>
              }
            />
          ) : (
            <div className={cn('grid w-full gap-x-6 gap-y-4', slots.length > 1 && 'sm:grid-cols-2')}>
              {slots.map((n, i) => (
                <Gauge
                  key={i}
                  title={n ? titleOf(n) : t('live.preparing')}
                  subtitle={n ? nodeOf(n.id)?.protocol : undefined}
                  phase={n?.phase ?? 'idle'}
                  bps={n?.bps ?? 0}
                  // 表盘位换了节点、或同一节点换了方向,都重置缓动与曲线窗口。
                  resetKey={`${i}-${n?.id ?? 'idle'}-${n?.phase ?? 'idle'}`}
                  compact={slots.length > 1}
                />
              ))}
            </div>
          )}

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
