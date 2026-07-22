import { motion } from 'motion/react'
import { Activity, Gauge, Clock, Download, Wifi } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'
import { bytesToSize, formatSeconds } from '@/lib/utils'

export function Dashboard() {
  const t = useI18n()
  const { result, testCount, testOkCount, totalTraffic, totalTime, loading } = useTestStore()
  
  const progress = result.length > 0 ? Math.floor((testCount / result.length) * 100) : 0
  
  // 统计各协议数量
  const protocolStats = {
    vmess: result.filter(n => n.protocol.startsWith('vmess')).length,
    vless: result.filter(n => n.protocol.startsWith('vless')).length,
    trojan: result.filter(n => n.protocol.startsWith('trojan')).length,
    ss: result.filter(n => n.protocol === 'ss').length,
    ssr: result.filter(n => n.protocol === 'ssr').length,
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.2 }}
      className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4"
    >
      {/* 进度 */}
      <Card className="col-span-2 p-4 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-r from-primary/10 to-transparent" />
        <div className="relative">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <div className="p-2 rounded-lg bg-primary/20">
                <Activity className={`w-4 h-4 text-primary ${loading ? 'animate-pulse' : ''}`} />
              </div>
              <span className="text-sm font-medium text-muted-foreground">{t('dash.progress')}</span>
            </div>
            <span className="text-2xl font-bold text-primary">{progress}%</span>
          </div>
          <Progress value={progress} className="h-2" />
          <div className="flex justify-between mt-2 text-xs text-muted-foreground">
            <span>{t('dash.done', { n: testCount })}</span>
            <span>{t('dash.totalNodes', { n: result.length })}</span>
          </div>
        </div>
      </Card>

      {/* 成功率 */}
      <StatCard
        icon={Gauge}
        label={t('dash.successRate')}
        value={`${testOkCount}/${testCount || result.length}`}
        color="success"
        delay={0.1}
      />

      {/* 流量 */}
      <StatCard
        icon={Download}
        label={t('dash.traffic')}
        value={bytesToSize(totalTraffic)}
        color="accent"
        delay={0.15}
      />

      {/* 耗时 */}
      <StatCard
        icon={Clock}
        label={t('dash.duration')}
        value={formatSeconds(totalTime)}
        color="warning"
        delay={0.2}
      />

      {/* 协议统计 */}
      <Card className="p-4 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-info/10 to-transparent" />
        <div className="relative">
          <div className="flex items-center gap-2 mb-3">
            <div className="p-2 rounded-lg bg-info/20">
              <Wifi className="w-4 h-4 text-info" />
            </div>
            <span className="text-sm font-medium text-muted-foreground">{t('dash.protocols')}</span>
          </div>
          <div className="grid grid-cols-2 gap-1 text-xs">
            {Object.entries(protocolStats).map(([protocol, count]) => (
              count > 0 && (
                <div key={protocol} className="flex justify-between">
                  <span className="text-muted-foreground uppercase">{protocol}</span>
                  <span className="font-medium">{count}</span>
                </div>
              )
            ))}
          </div>
        </div>
      </Card>
    </motion.div>
  )
}

interface StatCardProps {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  color: 'primary' | 'success' | 'accent' | 'warning' | 'info'
  delay?: number
}

function StatCard({ icon: Icon, label, value, color, delay = 0 }: StatCardProps) {
  const colorClasses = {
    primary: 'from-primary/10 text-primary bg-primary/20',
    success: 'from-success/10 text-success bg-success/20',
    accent: 'from-accent/10 text-accent bg-accent/20',
    warning: 'from-warning/10 text-warning bg-warning/20',
    info: 'from-info/10 text-info bg-info/20',
  }

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ delay: 0.2 + delay }}
    >
      <Card className="p-4 relative overflow-hidden h-full">
        <div className={`absolute inset-0 bg-gradient-to-br ${colorClasses[color].split(' ')[0]} to-transparent`} />
        <div className="relative">
          <div className="flex items-center gap-2 mb-2">
            <div className={`p-2 rounded-lg ${colorClasses[color].split(' ').slice(1).join(' ')}`}>
              <Icon className={`w-4 h-4 ${colorClasses[color].split(' ')[1]}`} />
            </div>
            <span className="text-sm font-medium text-muted-foreground">{label}</span>
          </div>
          <p className={`text-xl font-bold ${colorClasses[color].split(' ')[1]}`}>{value}</p>
        </div>
      </Card>
    </motion.div>
  )
}

