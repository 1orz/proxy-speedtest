import { useState, useCallback, useEffect, type DragEvent } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { Play, Square, Upload, X, Settings2, FileText, Plus } from 'lucide-react'
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
import type { SpeedTestMode, HeaderEntry } from '@/types'

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
  { key: 'worker', label: { en: 'Cloudflare Worker (self-hosted · needs key)', cn: 'Cloudflare Worker（自建 · 需 Key）' }, url: 'https://cf-sp.orbitintel.com/__down?bytes=524288000' },
] as const

const DEFAULT_ENDPOINT = 'cloudflare'

// 自建 Cloudflare Worker 端点:仅预填 URL,鉴权用「自定义请求头」里的 X-Speedtest-Key。下载 500MiB / 上传走 __up。
const WORKER_DOWN_URL = 'https://cf-sp.orbitintel.com/__down?bytes=524288000'
const WORKER_UP_URL = 'https://cf-sp.orbitintel.com/__up'

// 上传测速端点(POST 接收即丢弃)。key 必须与后端 download.GetUploadURL 的 case 一致。
// 可用的公共 sink 稀缺:CF __up 为 Anycast 就近、首选;DLPTest 为美国固定备选。
const UPLOAD_ENDPOINTS = [
  { key: 'cloudflare', label: { en: 'Cloudflare __up (Global Anycast · nearest, preferred)', cn: 'Cloudflare __up（全球 Anycast · 就近,首选）' }, url: 'https://speed.cloudflare.com/__up' },
  { key: 'dlptest', label: { en: 'DLPTest (USA, fallback)', cn: 'DLPTest（美国,备选）' }, url: 'https://dlptest.com/api/http-post/' },
  { key: 'worker', label: { en: 'Cloudflare Worker (self-hosted · needs key)', cn: 'Cloudflare Worker（自建 · 需 Key）' }, url: WORKER_UP_URL },
] as const

// 并发数 / 下载线程数改为离散滑动块的档位
const CONCURRENCY_STEPS = [1, 2, 3, 5, 8, 16, 32, 50] as const
const THREAD_STEPS = [1, 2, 4, 8, 16, 32, 64] as const
const MAX_SUBS = 10 // 订阅条目上限

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
      // 注意:不把文件名写进订阅项。文件内容(fileContent)是非持久化的组件状态;
      // 上传态改用 uploadedFile 判定,刷新后不会残留无法使用的裸文件名。
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

  // 订阅条目增删改(均通过 setOptions 更新持久化的 subscriptions)
  const setEntry = useCallback((i: number, patch: Partial<{ group: string; url: string }>) => {
    setOptions({ subscriptions: options.subscriptions.map((e, idx) => (idx === i ? { ...e, ...patch } : e)) })
  }, [options.subscriptions, setOptions])

  const addEntry = useCallback(() => {
    if (options.subscriptions.length >= MAX_SUBS) return
    setOptions({ subscriptions: [...options.subscriptions, { group: '', url: '' }] })
  }, [options.subscriptions, setOptions])

  const removeEntry = useCallback((i: number) => {
    const next = options.subscriptions.filter((_, idx) => idx !== i)
    setOptions({ subscriptions: next.length > 0 ? next : [{ group: '', url: '' }] })
  }, [options.subscriptions, setOptions])

  const handleSubmit = useCallback(() => {
    // 有效订阅条目(URL 非空);上传文件视为一条原始内容订阅
    const entries = options.subscriptions.filter((e) => e.url.trim())
    if (!uploadedFile && entries.length === 0) {
      alert(t('form.alert.noSub'))
      return
    }
    if (options.downloadSize === 'custom' && !options.downloadUrl.trim()) {
      alert(t('form.alert.noCustomUrl'))
      return
    }

    const uploadEnable = options.uploadEnable && options.speedtestMode !== 'pingonly'
    reset()
    // 本次是否测上传、以及测试模式在开始时即快照,让结果展示不受之后表单改动影响
    useTestStore.setState({ loading: true, runUploadEnabled: uploadEnable, runMode: options.speedtestMode })

    const subscriptions = uploadedFile
      ? [{ group: '', subscription: fileContent }]
      : entries.map((e) => ({ group: e.group.trim(), subscription: e.url.trim() }))

    // 自定义请求头:统一模式下上传复用下载那份;非统一分别取。名称为空的行忽略。
    const trimHeaders = (arr: HeaderEntry[]) =>
      arr.filter((h) => h.name.trim()).map((h) => ({ name: h.name.trim(), value: h.value }))
    const downloadHeaders = trimHeaders(options.downloadHeaders)
    const uploadHeaders = trimHeaders(options.headersUnified ? options.downloadHeaders : options.uploadHeaders)

    const payload = JSON.stringify({
      testMode: 2,
      subscriptions,
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
      uploadEnable,
      uploadSize: options.uploadSize,
      uploadUrl: options.uploadUrl,
      downloadHeaders,
      uploadHeaders,
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
        {/* 响应式栅格:手机单列堆叠,PC(md+)双列,长控件跨两列,避免 PC 下过于松散 */}
        <CardContent className="p-6 grid grid-cols-1 md:grid-cols-2 gap-x-5 gap-y-5">
          {/* 订阅列表:多条 (组名, 订阅链接),+ 添加,上限 10;组名可空 */}
          <div className="space-y-2 md:col-span-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.subscription')}</label>
              <span className="text-xs text-muted-foreground">{t('form.subMax', { n: MAX_SUBS })}</span>
            </div>
            {options.subscriptions.map((entry, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  value={entry.group}
                  onChange={(e) => setEntry(i, { group: e.target.value })}
                  placeholder={t('form.group.ph')}
                  disabled={loading || !!uploadedFile}
                  className="w-32 sm:w-40 shrink-0"
                />
                <Input
                  value={entry.url}
                  onChange={(e) => setEntry(i, { url: e.target.value })}
                  placeholder={t('form.subscription.ph')}
                  disabled={loading || !!uploadedFile}
                  className="flex-1 min-w-0 font-mono text-xs"
                />
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => removeEntry(i)}
                  disabled={loading || !!uploadedFile || options.subscriptions.length <= 1}
                  className="shrink-0"
                  aria-label="remove"
                >
                  <X className="w-4 h-4" />
                </Button>
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={addEntry}
              disabled={loading || !!uploadedFile || options.subscriptions.length >= MAX_SUBS}
              className="gap-2"
            >
              <Plus className="w-4 h-4" />
              {t('form.addSub')}
            </Button>
          </div>

          {/* 文件上传区域 */}
          <div className="md:col-span-2">
            <AnimatePresence mode="wait">
              {!uploadedFile && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  onDrop={handleDrop}
                  onDragOver={handleDragOver}
                  onDragLeave={handleDragLeave}
                  className={`
                    relative border-2 border-dashed rounded-xl p-6 text-center transition-all duration-200
                    ${isDragging ? 'border-primary bg-primary/10' : 'border-border hover:border-primary/50'}
                  `}
                >
                  <input
                    type="file"
                    onChange={(e) => e.target.files?.[0] && handleFileUpload(e.target.files[0])}
                    className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                    disabled={loading}
                  />
                  <Upload className="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
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
          </div>

          {/* 并发数:离散滑动块 */}
          <div className="space-y-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.concurrency')}</label>
              <span className="text-xs text-muted-foreground">{t('form.concurrency.hint')}</span>
            </div>
            <StepSlider steps={CONCURRENCY_STEPS} value={options.concurrency} onChange={(n) => setOptions({ concurrency: n })} disabled={loading} />
          </div>

          {/* 下载线程数:单节点测速内的并行连接数(与"并发数"是两个维度) */}
          <div className="space-y-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.threads')}</label>
              <span className="text-xs text-muted-foreground">{t('form.threads.hint')}</span>
            </div>
            <StepSlider steps={THREAD_STEPS} value={options.threads} onChange={(n) => setOptions({ threads: n })} disabled={loading} />
          </div>

          {/* 测试项:三段切换 */}
          <div className="space-y-2 md:col-span-2">
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
          <div className="grid grid-cols-2 gap-4 md:col-span-2">
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

          {/* 下载测速端点 */}
          <div className="space-y-2 md:col-span-2 lg:col-span-1">
            <label className="text-sm font-medium text-muted-foreground">{t('form.downloadEndpoint')}</label>
            <Select
              value={options.downloadSize}
              onValueChange={(v) => {
                if (v === 'custom') {
                  setOptions({ downloadSize: 'custom' })
                } else if (v === 'worker') {
                  setOptions({
                    downloadSize: 'worker',
                    downloadUrl: WORKER_DOWN_URL,
                  })
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
            {options.downloadSize === 'custom' || options.downloadSize === 'worker' ? (
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
            {options.downloadSize === 'worker' && (
              <p className="text-xs text-warning">{t('form.worker.note')}</p>
            )}
            <p className="text-xs text-muted-foreground">
              {t('form.downloadEndpoint.hint')}
            </p>
          </div>

          {/* 上传测速:独立开关(仅测速模式可用,pingonly 下不适用) */}
          <div className="space-y-2 md:col-span-2 lg:col-span-1">
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
                  onValueChange={(v) => {
                    if (v === 'worker') {
                      setOptions({ uploadSize: 'worker', uploadUrl: WORKER_UP_URL })
                    } else {
                      setOptions({ uploadSize: v, uploadUrl: '' })
                    }
                  }}
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
                {options.uploadSize === 'worker' ? (
                  <>
                    <Input
                      value={options.uploadUrl}
                      onChange={(e) => setOptions({ uploadUrl: e.target.value })}
                      placeholder={WORKER_UP_URL}
                      disabled={loading}
                      className="font-mono text-xs"
                    />
                    <p className="text-xs text-warning">{t('form.worker.note')}</p>
                  </>
                ) : (
                  <Input
                    value={UPLOAD_ENDPOINTS.find((e) => e.key === options.uploadSize)?.url ?? ''}
                    readOnly
                    tabIndex={-1}
                    aria-label={t('form.currentUploadUrl.aria')}
                    className="font-mono text-xs text-muted-foreground cursor-not-allowed focus-visible:ring-0"
                  />
                )}
                <p className="text-xs text-muted-foreground">
                  {t('form.uploadEndpoint.hint')}
                </p>
              </div>
            )}
          </div>

          {/* 自定义请求头(可选):可统一或分别设置下载/上传 */}
          <div className="space-y-3 md:col-span-2">
            <div className="flex items-baseline gap-2 flex-wrap">
              <label className="text-sm font-medium text-muted-foreground">{t('form.headers')}</label>
              <span className="text-xs text-muted-foreground">{t('form.headers.hint')}</span>
            </div>
            <div className="flex items-center gap-3">
              <Checkbox
                id="headersUnified"
                checked={options.headersUnified}
                onCheckedChange={(checked) => setOptions({ headersUnified: !!checked })}
                disabled={loading}
              />
              <label htmlFor="headersUnified" className="text-sm font-medium cursor-pointer">
                {t('form.headers.unified')}
              </label>
            </div>
            {options.headersUnified ? (
              <HeaderList
                entries={options.downloadHeaders}
                onChange={(next) => setOptions({ downloadHeaders: next })}
                disabled={loading}
                t={t}
              />
            ) : (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <div className="text-xs font-semibold text-muted-foreground">{t('form.headers.download')}</div>
                  <HeaderList
                    entries={options.downloadHeaders}
                    onChange={(next) => setOptions({ downloadHeaders: next })}
                    disabled={loading}
                    t={t}
                  />
                </div>
                <div className="space-y-2">
                  <div className="text-xs font-semibold text-muted-foreground">{t('form.headers.upload')}</div>
                  <HeaderList
                    entries={options.uploadHeaders}
                    onChange={(next) => setOptions({ uploadHeaders: next })}
                    disabled={loading}
                    t={t}
                  />
                </div>
              </div>
            )}
          </div>

          <div className="md:col-span-2">
            <ActionButtons
              loading={loading}
              onSubmit={handleSubmit}
              onTerminate={handleTerminate}
            />
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

// StepSlider 是离散档位滑动块(原生 range,无额外依赖)。value 不在档位内时吸附到最近档。
// 当前值以气泡形式显示在滑块「上方」并随滑块位置移动(首/末档做边界钳制),下方为档位刻度。
function StepSlider({
  steps,
  value,
  onChange,
  disabled,
}: {
  steps: readonly number[]
  value: number
  onChange: (n: number) => void
  disabled?: boolean
}) {
  let idx = steps.indexOf(value)
  if (idx < 0) {
    // 吸附到最近档位
    idx = 0
    let best = Infinity
    steps.forEach((s, i) => {
      const d = Math.abs(s - value)
      if (d < best) {
        best = d
        idx = i
      }
    })
  }
  const last = steps.length - 1
  const pct = last > 0 ? (idx / last) * 100 : 0
  // 首档左对齐、末档右对齐,其余居中,避免气泡溢出容器边界
  const tx = idx === 0 ? '0%' : idx === last ? '-100%' : '-50%'
  return (
    <div className="space-y-1.5">
      <div className="relative h-6">
        <span
          className="absolute bottom-0 rounded-md bg-primary/15 px-1.5 py-0.5 font-mono text-xs font-semibold leading-none text-primary"
          style={{ left: `${pct}%`, transform: `translateX(${tx})` }}
        >
          {value}
        </span>
      </div>
      <input
        type="range"
        min={0}
        max={last}
        step={1}
        value={idx}
        disabled={disabled}
        onChange={(e) => onChange(steps[parseInt(e.target.value, 10)])}
        className="w-full accent-primary cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
      />
      <div className="flex justify-between px-0.5 text-[10px] text-muted-foreground">
        {steps.map((s) => (
          <span key={s}>{s}</span>
        ))}
      </div>
    </div>
  )
}

// HeaderList 渲染一组可增删的「名称 / 值」请求头行 + 添加按钮。受控:变更通过 onChange 返回新数组。
function HeaderList({
  entries,
  onChange,
  disabled,
  t,
}: {
  entries: HeaderEntry[]
  onChange: (next: HeaderEntry[]) => void
  disabled?: boolean
  t: ReturnType<typeof useI18n>
}) {
  const set = (i: number, patch: Partial<HeaderEntry>) =>
    onChange(entries.map((h, idx) => (idx === i ? { ...h, ...patch } : h)))
  const add = () => onChange([...entries, { name: '', value: '' }])
  const remove = (i: number) => onChange(entries.filter((_, idx) => idx !== i))
  return (
    <div className="space-y-2">
      {entries.map((h, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            value={h.name}
            onChange={(e) => set(i, { name: e.target.value })}
            placeholder={t('form.header.namePh')}
            disabled={disabled}
            className="w-36 sm:w-44 shrink-0 font-mono text-xs"
          />
          <Input
            value={h.value}
            onChange={(e) => set(i, { value: e.target.value })}
            placeholder={t('form.header.valuePh')}
            disabled={disabled}
            className="flex-1 min-w-0 font-mono text-xs"
          />
          <Button variant="ghost" size="icon" onClick={() => remove(i)} disabled={disabled} className="shrink-0" aria-label="remove header">
            <X className="w-4 h-4" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={add} disabled={disabled} className="gap-2">
        <Plus className="w-4 h-4" />
        {t('form.addHeader')}
      </Button>
    </div>
  )
}

// NumberField 是一个受控数字输入,允许输入过程中临时清空(不会像 `parseInt(v) || default`
// 那样在清空瞬间跳回默认值)。失焦时再做兜底与范围钳制。
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
    <div className="flex gap-3 pt-2">
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
