import { useState, useCallback, useEffect, type DragEvent } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { Play, Square, Upload, X, Settings2, FileText } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'
import type { SpeedTestMode } from '@/types'

// 下载测速端点预设。key 必须与后端 download.GetDownloadURL 的 case 保持一致,
// url 仅用于前端只读展示"当前测速链接",让用户清楚测的是哪个目标。
const DOWNLOAD_ENDPOINTS = [
  { key: 'cloudflare', label: { en: 'Cloudflare (Global Anycast · 10MB, best single-thread)', cn: 'Cloudflare（全球 Anycast · 10MB，单线程首选）' }, url: 'https://speed.cloudflare.com/__down?bytes=10000000' },
  { key: 'hetzner-de', label: { en: 'Hetzner Germany (1GB)', cn: 'Hetzner 德国（1GB）' }, url: 'https://fsn1-speed.hetzner.com/1GB.bin' },
  { key: 'hetzner-us', label: { en: 'Hetzner USA (1GB)', cn: 'Hetzner 美国（1GB）' }, url: 'https://ash-speed.hetzner.com/1GB.bin' },
  { key: 'linode-jp', label: { en: 'Linode Tokyo (100MB)', cn: 'Linode 东京（100MB）' }, url: 'https://speedtest.tokyo2.linode.com/100MB-tokyo2.bin' },
  { key: 'vultr-sg', label: { en: 'Vultr Singapore (100MB)', cn: 'Vultr 新加坡（100MB）' }, url: 'https://sgp-ping.vultr.com/vultr.com.100MB.bin' },
  { key: 'ovh-eu', label: { en: 'OVH Europe (1GB)', cn: 'OVH 欧洲（1GB）' }, url: 'https://proof.ovh.net/files/1Gb.dat' },
  { key: 'datapacket-us', label: { en: 'DataPacket USA (100MB)', cn: 'DataPacket 美国（100MB）' }, url: 'http://lax.download.datapacket.com/100mb.bin' },
  { key: 'huawei-cn', label: { en: 'Huawei Cloud Mirror · China (2.3GB)', cn: '华为云镜像 · 国内（2.3GB）' }, url: 'https://mirrors.huaweicloud.com/ubuntu-releases/bionic/ubuntu-18.04.6-desktop-amd64.iso' },
] as const

const DEFAULT_ENDPOINT = 'cloudflare'

// 上传测速端点(POST 接收即丢弃)。key 必须与后端 download.GetUploadURL 的 case 一致。
// 可用的公共 sink 稀缺:CF __up 为 Anycast 就近、首选;DLPTest 为美国固定备选。
const UPLOAD_ENDPOINTS = [
  { key: 'cloudflare', label: { en: 'Cloudflare __up (Global Anycast · nearest, preferred)', cn: 'Cloudflare __up（全球 Anycast · 就近,首选）' }, url: 'https://speed.cloudflare.com/__up' },
  { key: 'dlptest', label: { en: 'DLPTest (USA, fallback)', cn: 'DLPTest（美国,备选）' }, url: 'https://dlptest.com/api/http-post/' },
] as const

const CONCURRENCY_PRESETS = [1, 3, 5] as const
const THREAD_PRESETS = [1, 2, 4, 8, 16, 32, 64, 128] as const

// 三段切换的测试项:左 Ping Only / 中 测速 + Tcping / 右 Speed Only
const TEST_MODES: { value: SpeedTestMode; label: string }[] = [
  { value: 'pingonly', label: 'Ping Only' },
  { value: 'all', label: '测速 + Tcping' },
  { value: 'speedonly', label: 'Speed Only' },
]

export function TestForm() {
  const {
    loading,
    options,
    setOptions,
    reset,
    connect,
    disconnect,
    send,
  } = useTestStore()

  const t = useI18n()

  const [uploadedFile, setUploadedFile] = useState<File | null>(null)
  const [fileContent, setFileContent] = useState('')
  const [isDragging, setIsDragging] = useState(false)

  // 迁移旧版持久化里已失效/更名的端点 key(如 cloudflare100/cloudflare200/cachefly100 已被替换)
  useEffect(() => {
    const known = DOWNLOAD_ENDPOINTS.some((e) => e.key === options.downloadSize)
    if (options.downloadSize !== 'custom' && !known) {
      setOptions({ downloadSize: DEFAULT_ENDPOINT, downloadUrl: '' })
    }
  }, [options.downloadSize, setOptions])

  const handleFileUpload = useCallback((file: File) => {
    if (file.size > 10 * 1024 * 1024) {
      alert(t('form.upload.tooLarge'))
      return
    }
    const reader = new FileReader()
    reader.onloadend = () => {
      setFileContent(reader.result as string)
      setUploadedFile(file)
      // 注意:不把文件名写进 options.subscription。subscription 会被持久化到 localStorage,
      // 而文件内容(fileContent)是非持久化的组件状态;若写入文件名,刷新后会残留一个
      // 无法使用的裸文件名,误导用户并导致后端解析失败。上传态改用 uploadedFile 判定。
    }
    reader.readAsText(file)
  }, [t])

  const handleDrop = useCallback((e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    const file = e.dataTransfer.files[0]
    if (file) handleFileUpload(file)
  }, [handleFileUpload])

  const handleDragOver = useCallback((e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback(() => {
    setIsDragging(false)
  }, [])

  const clearFile = useCallback(() => {
    setUploadedFile(null)
    setFileContent('')
  }, [])

  const handleSubmit = useCallback(() => {
    if (!uploadedFile && !options.subscription) {
      alert(t('form.alert.noSub'))
      return
    }
    if (options.downloadSize === 'custom' && !options.downloadUrl.trim()) {
      alert(t('form.alert.noCustomUrl'))
      return
    }

    reset()
    useTestStore.setState({ loading: true })

    const payload = JSON.stringify({
      testMode: 2,
      subscription: uploadedFile ? fileContent : options.subscription,
      group: options.groupname || '?empty?',
      speedtestMode: options.speedtestMode,
      sortMethod: options.sortMethod,
      unique: options.unique,
      concurrency: options.concurrency,
      threads: options.threads,
      timeout: options.timeout,
      language: options.language,
      fontSize: options.fontSize,
      theme: options.theme,
      appearance: options.appearance,
      downloadSize: options.downloadSize,
      downloadUrl: options.downloadUrl,
      uploadEnable: options.uploadEnable && options.speedtestMode !== 'pingonly',
      uploadSize: options.uploadSize,
    })

    // 按页面协议选择 ws/wss,HTTPS 部署下才不会被混合内容策略拦截
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    connect(`${proto}//${window.location.host}/test`)

    // 等待连接建立后发送;连接失败(ws 被 onerror/onclose 置空)或超过重试上限即停止,
    // 避免原先"永远 100ms 轮询"造成的死循环与定时器泄漏。
    let attempts = 0
    const checkAndSend = () => {
      const ws = useTestStore.getState().ws
      if (ws && ws.readyState === WebSocket.OPEN) {
        send(payload)
        return
      }
      if (!ws || ws.readyState === WebSocket.CLOSING || ws.readyState === WebSocket.CLOSED) {
        useTestStore.setState({ loading: false })
        return
      }
      if (++attempts > 50) {
        disconnect()
        alert(t('form.alert.connectFail'))
        return
      }
      setTimeout(checkAndSend, 100)
    }
    checkAndSend()
  }, [t, options, uploadedFile, fileContent, reset, connect, send, disconnect])

  const handleTerminate = useCallback(() => {
    disconnect()
    reset()
  }, [disconnect, reset])

  const currentEndpointUrl =
    DOWNLOAD_ENDPOINTS.find((e) => e.key === options.downloadSize)?.url ?? ''

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.1 }}
    >
      <Card className="overflow-hidden">
        <CardHeader className="bg-gradient-to-r from-primary/10 to-accent/10 border-b border-border/50">
          <CardTitle className="flex items-center gap-2">
            <Settings2 className="w-5 h-5 text-primary" />
            {t('form.title')}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-6 space-y-6">
          {/* 订阅链接输入 */}
          <div className="space-y-2">
            <label className="text-sm font-medium text-muted-foreground">{t('form.subscription')}</label>
            <Input
              value={uploadedFile ? uploadedFile.name : options.subscription}
              onChange={(e) => setOptions({ subscription: e.target.value })}
              placeholder={t('form.subscription.ph')}
              disabled={loading || !!uploadedFile}
            />
          </div>

          {/* 文件上传区域 */}
          <AnimatePresence mode="wait">
            {!uploadedFile && !options.subscription && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                className={`
                  relative border-2 border-dashed rounded-xl p-8 text-center transition-all duration-200
                  ${isDragging ? 'border-primary bg-primary/10' : 'border-border hover:border-primary/50'}
                `}
              >
                <input
                  type="file"
                  onChange={(e) => e.target.files?.[0] && handleFileUpload(e.target.files[0])}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                  disabled={loading}
                />
                <Upload className="w-10 h-10 mx-auto mb-3 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  {t('form.upload.dnd')}<span className="text-primary cursor-pointer">{t('form.upload.click')}</span>
                </p>
              </motion.div>
            )}

            {uploadedFile && (
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="flex items-center justify-between p-4 rounded-lg bg-secondary/50 border border-border"
              >
                <div className="flex items-center gap-3">
                  <FileText className="w-5 h-5 text-primary" />
                  <span className="text-sm font-medium">{uploadedFile.name}</span>
                </div>
                <Button variant="ghost" size="icon" onClick={clearFile} disabled={loading}>
                  <X className="w-4 h-4" />
                </Button>
              </motion.div>
            )}
          </AnimatePresence>

          {/* 并发数:预设 1/3/5 + 自定义 */}
          <div className="space-y-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.concurrency')}</label>
              <span className="text-xs text-muted-foreground">{t('form.concurrency.hint')}</span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {CONCURRENCY_PRESETS.map((n) => (
                <Button
                  key={n}
                  type="button"
                  size="sm"
                  variant={options.concurrency === n ? 'default' : 'outline'}
                  onClick={() => setOptions({ concurrency: n })}
                  disabled={loading}
                  className="w-12"
                >
                  {n}
                </Button>
              ))}
              <div className="flex items-center gap-2 pl-2">
                <span className="text-xs text-muted-foreground">{t('form.custom')}</span>
                <NumberField
                  min={1}
                  max={50}
                  fallback={2}
                  value={options.concurrency}
                  onChange={(n) => setOptions({ concurrency: n })}
                  disabled={loading}
                  className="w-24"
                />
              </div>
            </div>
          </div>

          {/* 下载线程数:单节点测速内的并行连接数(与"并发数"是两个维度) */}
          <div className="space-y-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.threads')}</label>
              <span className="text-xs text-muted-foreground">{t('form.threads.hint')}</span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {THREAD_PRESETS.map((n) => (
                <Button
                  key={n}
                  type="button"
                  size="sm"
                  variant={options.threads === n ? 'default' : 'outline'}
                  onClick={() => setOptions({ threads: n })}
                  disabled={loading}
                  className="w-12"
                >
                  {n}
                </Button>
              ))}
            </div>
          </div>

          {/* 测试项:三段切换 */}
          <div className="space-y-2">
            <label className="text-sm font-medium text-muted-foreground">{t('form.testItems')}</label>
            <div className="grid grid-cols-3 gap-1 rounded-lg bg-secondary/50 p-1">
              {TEST_MODES.map((m) => (
                <button
                  key={m.value}
                  type="button"
                  disabled={loading}
                  onClick={() => setOptions({ speedtestMode: m.value })}
                  className={cn(
                    'rounded-md px-3 py-2 text-sm font-medium transition-all disabled:opacity-50 disabled:pointer-events-none',
                    options.speedtestMode === m.value
                      ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/25'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {m.value === 'all' ? t('form.mode.all') : m.label}
                </button>
              ))}
            </div>
          </div>

          {/* 测试时长 + 去重 */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium text-muted-foreground">{t('form.duration')}</label>
              <NumberField
                min={5}
                max={60}
                fallback={15}
                value={options.timeout}
                onChange={(n) => setOptions({ timeout: n })}
                disabled={loading}
              />
            </div>
            <div className="flex items-end pb-2.5">
              <div className="flex items-center gap-3">
                <Checkbox
                  id="unique"
                  checked={options.unique}
                  onCheckedChange={(checked) => setOptions({ unique: !!checked })}
                  disabled={loading}
                />
                <label htmlFor="unique" className="text-sm font-medium cursor-pointer">
                  {t('form.unique')}
                </label>
              </div>
            </div>
          </div>

          {/* 自定义组名 */}
          <div className="space-y-2">
            <label className="text-sm font-medium text-muted-foreground">{t('form.groupname')}</label>
            <Input
              value={options.groupname}
              onChange={(e) => setOptions({ groupname: e.target.value })}
              placeholder={t('form.groupname.ph')}
              disabled={loading}
            />
          </div>

          {/* 下载测速端点 */}
          <div className="space-y-2">
            <label className="text-sm font-medium text-muted-foreground">{t('form.downloadEndpoint')}</label>
            <Select
              value={options.downloadSize}
              onValueChange={(v) => {
                if (v === 'custom') {
                  setOptions({ downloadSize: 'custom' })
                } else {
                  setOptions({ downloadSize: v, downloadUrl: '' })
                }
              }}
              disabled={loading}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DOWNLOAD_ENDPOINTS.map((e) => (
                  <SelectItem key={e.key} value={e.key}>
                    {options.language === 'cn' ? e.label.cn : e.label.en}
                  </SelectItem>
                ))}
                <SelectItem value="custom">{t('form.customUrl')}</SelectItem>
              </SelectContent>
            </Select>
            {options.downloadSize === 'custom' ? (
              <Input
                value={options.downloadUrl}
                onChange={(e) => setOptions({ downloadUrl: e.target.value })}
                placeholder={t('form.downloadUrl.ph')}
                disabled={loading}
                className="font-mono text-xs"
              />
            ) : (
              <Input
                value={currentEndpointUrl}
                readOnly
                tabIndex={-1}
                aria-label={t('form.currentUrl.aria')}
                className="font-mono text-xs text-muted-foreground cursor-not-allowed focus-visible:ring-0"
              />
            )}
            <p className="text-xs text-muted-foreground">
              {t('form.downloadEndpoint.hint')}
            </p>
          </div>

          {/* 上传测速:独立开关(仅测速模式可用,pingonly 下不适用) */}
          <div className="space-y-2">
            <div className="flex items-center gap-3 flex-wrap">
              <Checkbox
                id="uploadEnable"
                checked={options.uploadEnable && options.speedtestMode !== 'pingonly'}
                onCheckedChange={(checked) => setOptions({ uploadEnable: !!checked })}
                disabled={loading || options.speedtestMode === 'pingonly'}
              />
              <label htmlFor="uploadEnable" className="text-sm font-medium cursor-pointer">
                {t('form.uploadTest')}
              </label>
              <span className="text-xs text-muted-foreground">
                {options.speedtestMode === 'pingonly'
                  ? t('form.uploadTest.disabled')
                  : t('form.uploadTest.hint')}
              </span>
            </div>
            {options.uploadEnable && options.speedtestMode !== 'pingonly' && (
              <div className="space-y-2 pl-7">
                <Select
                  value={options.uploadSize}
                  onValueChange={(v) => setOptions({ uploadSize: v })}
                  disabled={loading}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {UPLOAD_ENDPOINTS.map((e) => (
                      <SelectItem key={e.key} value={e.key}>
                        {options.language === 'cn' ? e.label.cn : e.label.en}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  value={UPLOAD_ENDPOINTS.find((e) => e.key === options.uploadSize)?.url ?? ''}
                  readOnly
                  tabIndex={-1}
                  aria-label={t('form.currentUploadUrl.aria')}
                  className="font-mono text-xs text-muted-foreground cursor-not-allowed focus-visible:ring-0"
                />
                <p className="text-xs text-muted-foreground">
                  {t('form.uploadEndpoint.hint')}
                </p>
              </div>
            )}
          </div>

          <ActionButtons
            loading={loading}
            onSubmit={handleSubmit}
            onTerminate={handleTerminate}
          />
        </CardContent>
      </Card>
    </motion.div>
  )
}

// NumberField 是一个受控数字输入,允许输入过程中临时清空(不会像 `parseInt(v) || default`
// 那样在清空瞬间跳回默认值),因此可以直接把并发数改成 1。失焦时再做兜底与范围钳制。
function NumberField({
  value,
  min,
  max,
  fallback,
  onChange,
  disabled,
  className,
}: {
  value: number
  min: number
  max: number
  fallback: number
  onChange: (n: number) => void
  disabled?: boolean
  className?: string
}) {
  const [text, setText] = useState(String(value))

  useEffect(() => {
    setText(String(value))
  }, [value])

  return (
    <Input
      type="number"
      min={min}
      max={max}
      value={text}
      disabled={disabled}
      className={className}
      onChange={(e) => {
        setText(e.target.value)
        const n = parseInt(e.target.value, 10)
        if (!isNaN(n)) onChange(n)
      }}
      onBlur={() => {
        let n = parseInt(text, 10)
        if (isNaN(n)) n = fallback
        n = Math.min(max, Math.max(min, n))
        onChange(n)
        setText(String(n))
      }}
    />
  )
}

interface ActionButtonsProps {
  loading: boolean
  onSubmit: () => void
  onTerminate: () => void
}

function ActionButtons({ loading, onSubmit, onTerminate }: ActionButtonsProps) {
  const t = useI18n()
  return (
    <div className="flex gap-3 pt-4">
      <Button
        onClick={onSubmit}
        disabled={loading}
        className="flex-1"
        variant={loading ? 'secondary' : 'default'}
      >
        {loading ? (
          <>
            <motion.div
              animate={{ rotate: 360 }}
              transition={{ duration: 1, repeat: Infinity, ease: 'linear' }}
              className="w-4 h-4 border-2 border-current border-t-transparent rounded-full"
            />
            {t('form.testing')}
          </>
        ) : (
          <>
            <Play className="w-4 h-4" />
            {t('form.start')}
          </>
        )}
      </Button>
      <Button
        onClick={onTerminate}
        disabled={!loading}
        variant="destructive"
      >
        <Square className="w-4 h-4" />
        {t('form.terminate')}
      </Button>
    </div>
  )
}
