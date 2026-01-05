import { useState, useCallback, type DragEvent } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { Play, Square, Upload, X, Settings2, FileText, Wand2 } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useTestStore } from '@/store/test-store'
import type { SpeedTestMode, PingMethod } from '@/types'

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

  const [activeTab, setActiveTab] = useState('basic')
  const [uploadedFile, setUploadedFile] = useState<File | null>(null)
  const [fileContent, setFileContent] = useState('')
  const [isDragging, setIsDragging] = useState(false)
  const [generateJSON, setGenerateJSON] = useState('')

  const handleFileUpload = useCallback((file: File) => {
    if (file.size > 10 * 1024 * 1024) {
      alert('文件大小不能超过 10MB')
      return
    }
    const reader = new FileReader()
    reader.onloadend = () => {
      setFileContent(reader.result as string)
      setUploadedFile(file)
      setOptions({ subscription: file.name })
    }
    reader.readAsText(file)
  }, [setOptions])

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
    setOptions({ subscription: '' })
  }, [setOptions])

  const handleSubmit = useCallback(() => {
    if (!options.subscription) {
      alert('请输入订阅链接或上传配置文件')
      return
    }

    reset()
    useTestStore.setState({ loading: true })

    const wsUrl = `ws://${window.location.host}/test`
    connect(wsUrl)

    // 等待连接建立后发送数据
    const checkAndSend = () => {
      const ws = useTestStore.getState().ws
      if (ws && ws.readyState === WebSocket.OPEN) {
        const data = {
          testMode: 2,
          subscription: uploadedFile ? fileContent : options.subscription,
          group: options.groupname || '?empty?',
          speedtestMode: options.speedtestMode,
          pingMethod: options.pingMethod,
          sortMethod: options.sortMethod,
          unique: options.unique,
          concurrency: options.concurrency,
          timeout: options.timeout,
          language: options.language,
          fontSize: options.fontSize,
          theme: options.theme,
        }
        send(JSON.stringify(data))
      } else {
        setTimeout(checkAndSend, 100)
      }
    }
    checkAndSend()
  }, [options, uploadedFile, fileContent, reset, connect, send])

  const handleTerminate = useCallback(() => {
    disconnect()
    reset()
  }, [disconnect, reset])

  const handleGenerateResult = useCallback(async () => {
    if (!generateJSON) return
    try {
      const response = await fetch(`${window.location.protocol}//${window.location.host}/generateResult`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: generateJSON,
      })
      const data = await response.text()
      useTestStore.setState({ picdata: data })
    } catch (error) {
      console.error('Generate result failed:', error)
    }
  }, [generateJSON])

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
            测速配置
          </CardTitle>
        </CardHeader>
        <CardContent className="p-6">
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="grid w-full grid-cols-4 mb-6">
              <TabsTrigger value="basic" className="gap-2">
                <Settings2 className="w-4 h-4" />
                基础
              </TabsTrigger>
              <TabsTrigger value="advanced" className="gap-2">
                <Wand2 className="w-4 h-4" />
                高级
              </TabsTrigger>
              <TabsTrigger value="export" className="gap-2">
                <FileText className="w-4 h-4" />
                导出
              </TabsTrigger>
              <TabsTrigger value="generate" className="gap-2">
                <Wand2 className="w-4 h-4" />
                生成
              </TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="space-y-6">
              <BasicSettings
                options={options}
                setOptions={setOptions}
                loading={loading}
                uploadedFile={uploadedFile}
                isDragging={isDragging}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onFileUpload={handleFileUpload}
                onClearFile={clearFile}
              />
              <ActionButtons
                loading={loading}
                onSubmit={handleSubmit}
                onTerminate={handleTerminate}
              />
            </TabsContent>

            <TabsContent value="advanced" className="space-y-6">
              <BasicSettings
                options={options}
                setOptions={setOptions}
                loading={loading}
                uploadedFile={uploadedFile}
                isDragging={isDragging}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onFileUpload={handleFileUpload}
                onClearFile={clearFile}
              />
              <AdvancedSettings options={options} setOptions={setOptions} loading={loading} />
              <ActionButtons
                loading={loading}
                onSubmit={handleSubmit}
                onTerminate={handleTerminate}
              />
            </TabsContent>

            <TabsContent value="export" className="space-y-6">
              <ExportSettings options={options} setOptions={setOptions} />
            </TabsContent>

            <TabsContent value="generate" className="space-y-6">
              <div className="space-y-4">
                <label className="text-sm font-medium text-muted-foreground">结果数据 (JSON)</label>
                <textarea
                  value={generateJSON}
                  onChange={(e) => setGenerateJSON(e.target.value)}
                  placeholder="粘贴测速结果 JSON 数据..."
                  className="w-full h-48 rounded-lg border border-input bg-secondary/50 px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-primary/50"
                />
                <Button onClick={handleGenerateResult} disabled={!generateJSON || loading}>
                  <Wand2 className="w-4 h-4 mr-2" />
                  生成图片
                </Button>
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </motion.div>
  )
}

interface SettingsProps {
  options: {
    subscription: string
    concurrency: number
    timeout: number
    unique: boolean
    groupname: string
    speedtestMode: SpeedTestMode
    pingMethod: PingMethod
    sortMethod: 'rspeed' | 'speed' | 'ping' | 'rping' | 'none'
    language: 'en' | 'cn'
    fontSize: number
    theme: 'rainbow' | 'original'
  }
  setOptions: (options: Partial<SettingsProps['options']>) => void
  loading?: boolean
}

interface BasicSettingsProps extends SettingsProps {
  uploadedFile: File | null
  isDragging: boolean
  onDrop: (e: DragEvent<HTMLDivElement>) => void
  onDragOver: (e: DragEvent<HTMLDivElement>) => void
  onDragLeave: () => void
  onFileUpload: (file: File) => void
  onClearFile: () => void
}

function BasicSettings({
  options,
  setOptions,
  loading,
  uploadedFile,
  isDragging,
  onDrop,
  onDragOver,
  onDragLeave,
  onFileUpload,
  onClearFile,
}: BasicSettingsProps) {
  return (
    <div className="space-y-6">
      {/* 订阅链接输入 */}
      <div className="space-y-2">
        <label className="text-sm font-medium text-muted-foreground">订阅链接</label>
        <Input
          value={uploadedFile ? uploadedFile.name : options.subscription}
          onChange={(e) => setOptions({ subscription: e.target.value })}
          placeholder="支持 V2Ray/Trojan/SS/SSR/Clash/VLESS 订阅链接"
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
            onDrop={onDrop}
            onDragOver={onDragOver}
            onDragLeave={onDragLeave}
            className={`
              relative border-2 border-dashed rounded-xl p-8 text-center transition-all duration-200
              ${isDragging ? 'border-primary bg-primary/10' : 'border-border hover:border-primary/50'}
            `}
          >
            <input
              type="file"
              onChange={(e) => e.target.files?.[0] && onFileUpload(e.target.files[0])}
              className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
              disabled={loading}
            />
            <Upload className="w-10 h-10 mx-auto mb-3 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              拖拽配置文件到此处，或<span className="text-primary cursor-pointer">点击上传</span>
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
            <Button variant="ghost" size="icon" onClick={onClearFile} disabled={loading}>
              <X className="w-4 h-4" />
            </Button>
          </motion.div>
        )}
      </AnimatePresence>

      {/* 并发数和测试项 */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">并发数</label>
          <Input
            type="number"
            min={1}
            max={50}
            value={options.concurrency}
            onChange={(e) => setOptions({ concurrency: parseInt(e.target.value) || 2 })}
            disabled={loading}
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">测试项</label>
          <Select
            value={options.speedtestMode}
            onValueChange={(v) => setOptions({ speedtestMode: v as SpeedTestMode })}
            disabled={loading}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部测试</SelectItem>
              <SelectItem value="pingonly">仅 Ping</SelectItem>
              <SelectItem value="speedonly">仅速度</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* 自定义组名 */}
      <div className="space-y-2">
        <label className="text-sm font-medium text-muted-foreground">自定义组名</label>
        <Input
          value={options.groupname}
          onChange={(e) => setOptions({ groupname: e.target.value })}
          placeholder="可选，留空使用默认值"
          disabled={loading}
        />
      </div>
    </div>
  )
}

function AdvancedSettings({ options, setOptions, loading }: SettingsProps) {
  return (
    <div className="space-y-6 pt-4 border-t border-border/50">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">测试时长 (秒)</label>
          <Input
            type="number"
            min={5}
            max={60}
            value={options.timeout}
            onChange={(e) => setOptions({ timeout: parseInt(e.target.value) || 15 })}
            disabled={loading}
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">Ping 方式</label>
          <Select
            value={options.pingMethod}
            onValueChange={(v) => setOptions({ pingMethod: v as PingMethod })}
            disabled={loading}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="googleping">Google Ping</SelectItem>
              <SelectItem value="tcping">TCP Ping</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Checkbox
          id="unique"
          checked={options.unique}
          onCheckedChange={(checked) => setOptions({ unique: !!checked })}
          disabled={loading}
        />
        <label htmlFor="unique" className="text-sm font-medium cursor-pointer">
          去除重复节点
        </label>
      </div>
    </div>
  )
}

function ExportSettings({ options, setOptions }: SettingsProps) {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">语言</label>
          <Select
            value={options.language}
            onValueChange={(v) => setOptions({ language: v as 'en' | 'cn' })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="en">English</SelectItem>
              <SelectItem value="cn">中文</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">字体大小</label>
          <Input
            type="number"
            min={12}
            max={36}
            value={options.fontSize}
            onChange={(e) => setOptions({ fontSize: parseInt(e.target.value) || 24 })}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">排序方式</label>
          <Select
            value={options.sortMethod}
            onValueChange={(v) => setOptions({ sortMethod: v as typeof options.sortMethod })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="rspeed">速度倒序</SelectItem>
              <SelectItem value="speed">速度顺序</SelectItem>
              <SelectItem value="rping">Ping 倒序</SelectItem>
              <SelectItem value="ping">Ping 顺序</SelectItem>
              <SelectItem value="none">默认</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium text-muted-foreground">主题</label>
          <Select
            value={options.theme}
            onValueChange={(v) => setOptions({ theme: v as 'rainbow' | 'original' })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="rainbow">Rainbow</SelectItem>
              <SelectItem value="original">Original</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  )
}

interface ActionButtonsProps {
  loading: boolean
  onSubmit: () => void
  onTerminate: () => void
}

function ActionButtons({ loading, onSubmit, onTerminate }: ActionButtonsProps) {
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
            测速中...
          </>
        ) : (
          <>
            <Play className="w-4 h-4" />
            开始测速
          </>
        )}
      </Button>
      <Button
        onClick={onTerminate}
        disabled={!loading}
        variant="destructive"
      >
        <Square className="w-4 h-4" />
        终止
      </Button>
    </div>
  )
}

