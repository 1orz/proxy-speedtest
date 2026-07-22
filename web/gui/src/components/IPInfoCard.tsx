import { motion } from 'motion/react'
import { Globe } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'

// IPInfoCard 展示测速机自身的公网出口(v4/v6 + 归属地),数据来自后端 ipinfo 消息。
// 未获取到任一地址时不渲染。
export function IPInfoCard() {
  const t = useI18n()
  // 只订阅 IP 相关字段,避免测速中每秒的样本更新触发本卡片重渲染。
  const ipv4 = useTestStore((s) => s.ipv4)
  const ipv6 = useTestStore((s) => s.ipv6)
  const ipv4geo = useTestStore((s) => s.ipv4geo)
  const ipv6geo = useTestStore((s) => s.ipv6geo)

  if (!ipv4 && !ipv6) {
    return null
  }

  const rows: { label: string; ip: string; geo: string }[] = []
  if (ipv4) rows.push({ label: 'IPv4', ip: ipv4, geo: ipv4geo })
  if (ipv6) rows.push({ label: 'IPv6', ip: ipv6, geo: ipv6geo })

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.25 }}
    >
      <Card className="p-4 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-info/10 to-transparent" />
        <div className="relative">
          <div className="flex items-center gap-2 mb-3">
            <div className="p-2 rounded-lg bg-info/20">
              <Globe className="w-4 h-4 text-info" />
            </div>
            <span className="text-sm font-medium text-muted-foreground">{t('ip.title')}</span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {rows.map((r) => (
              <div key={r.label} className="flex items-baseline gap-2 min-w-0">
                <span className="shrink-0 px-2 py-0.5 rounded-md bg-secondary text-xs font-semibold">
                  {r.label}
                </span>
                <div className="min-w-0">
                  <div className="font-mono text-sm truncate" title={r.ip}>{r.ip}</div>
                  {r.geo && (
                    <div className="text-xs text-muted-foreground truncate" title={r.geo}>{r.geo}</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>
    </motion.div>
  )
}
