import { useMemo, useState, useCallback } from 'react'
import { motion } from 'motion/react'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import { Gauge, Columns3, Share2, Download, Copy, MoreHorizontal, FileJson, ArrowUp, ArrowDown } from 'lucide-react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'
import { tt } from '@/lib/i18n'
import { useVersion } from '@/hooks/useVersion'
import { cn, getSpeed, getSpeedColor, maskSensitive, copyToClipboard, downloadFile, bytesToSize, formatSeconds } from '@/lib/utils'
import {
  buildShareSvg,
  svgToPngBlob,
  svgToBlob,
  triggerDownload,
  copyBlobToClipboard,
  canCopyImage,
  type Cell,
  type CellTone,
  type ColKey,
  type ShareInput,
} from '@/lib/share-image'
import type { TestNode } from '@/types'

const COL_LABEL_KEY: Record<ColKey, string> = {
  group: 'col.group', name: 'col.remark', server: 'col.server', protocol: 'col.protocol',
  ping: 'col.ping', download: 'col.speed', upload: 'col.upload',
}

// 速度数值(用于排序):无效/占位记 -1,排到最后
function spdVal(s: string | undefined): number {
  const v = getSpeed(s ?? '')
  return isNaN(v) ? -1 : v
}

function toneTextClass(tone: CellTone): string {
  return tone === 'good' ? 'text-success' : tone === 'bad' ? 'text-red-500' : 'text-muted-foreground'
}

// ResultCard 是唯一的「测速结果」视图:交互式表格(点表头排序、拖列边调宽)+ 卡片式表头
// (汇总统计 / 出口 IP / 版本徽标)。导出 PNG/SVG 通过 buildShareSvg 快照当前排序与可见列。
export function ResultCard() {
  const t = useI18n()
  const version = useVersion()
  const result = useTestStore((s) => s.result)
  const testOkCount = useTestStore((s) => s.testOkCount)
  const totalTraffic = useTestStore((s) => s.totalTraffic)
  const totalTime = useTestStore((s) => s.totalTime)
  const ipv4 = useTestStore((s) => s.ipv4)
  const ipv6 = useTestStore((s) => s.ipv6)
  const ipv4geo = useTestStore((s) => s.ipv4geo)
  const ipv6geo = useTestStore((s) => s.ipv6geo)
  const options = useTestStore((s) => s.options)
  const privacy = useTestStore((s) => s.privacy)
  const runUploadEnabled = useTestStore((s) => s.runUploadEnabled)
  const columnVisibility = useTestStore((s) => s.columnVisibility)
  const setColumnVisibility = useTestStore((s) => s.setColumnVisibility)
  const columnSizing = useTestStore((s) => s.columnSizing)
  const setColumnSizing = useTestStore((s) => s.setColumnSizing)
  const runMode = useTestStore((s) => s.runMode)

  const [sorting, setSorting] = useState<SortingState>([{ id: 'download', desc: true }])
  const [busy, setBusy] = useState(false)

  const mode = runMode // 用本次运行的模式快照,避免测完改表单模式导致列被回填成 '-'
  const theme = options.theme
  const hasUpload = runUploadEnabled || result.some((n) => !!n.uploadspeed)

  const colLabel = useCallback((key: ColKey) => t(COL_LABEL_KEY[key] as Parameters<typeof t>[0]), [t])

  // "测速中" 占位串在设置时按当时语言写入;切换语言后要能跨语言识别,故比对两种语言的值。
  const isTestingPlaceholder = useCallback((s: string) => s === tt('en', 'status.testing') || s === tt('cn', 'status.testing'), [])

  // 单元格显示语义(屏幕表格与导出 SVG 共用):待测/测速中/超时/失败(完全失败)/0B/s(能用但没速度)/有效速度。
  const pingDisplay = useCallback((n: TestNode): Cell => {
    if (typeof n.ping === 'string') return { text: t('status.testing'), tone: 'muted' } // 测速中(用当前语言)
    if (mode === 'speedonly') return { text: '-', tone: 'muted' } // 未测 ping
    if (n.ping > 0) return { text: `${n.ping} ms`, tone: 'good' }
    if (!n.tested) return { text: t('status.pending'), tone: 'muted' }
    return { text: t('status.timeout'), tone: 'bad' } // 超时
  }, [t, mode])

  const speedDisplay = useCallback((val: string | undefined, n: TestNode): Cell => {
    if (val && isTestingPlaceholder(val)) return { text: t('status.testing'), tone: 'muted' } // 测速中
    if (mode === 'pingonly') return { text: '-', tone: 'muted' } // 未测速
    const v = getSpeed(val ?? '')
    if (!isNaN(v) && v > 0) return { text: val as string, tone: 'speed' } // 有效速度
    if (!n.tested) return { text: t('status.pending'), tone: 'muted' }
    // 完全失败:节点连 ping 都挂(不可达)→ Failed;否则(ping 通或纯测速)零吞吐 → 0B/s(能用但没速度)
    if (mode !== 'speedonly' && typeof n.ping === 'number' && n.ping <= 0) return { text: t('status.failed'), tone: 'bad' }
    return { text: '0B/s', tone: 'muted' }
  }, [t, mode, isTestingPlaceholder])

  // 速度色块单元格(有效速度上色;失败红字;0速/占位灰字)
  const speedCellEl = useCallback((cell: Cell, testing?: boolean) => {
    if (cell.tone === 'speed') {
      const v = getSpeed(cell.text)
      const color = !isNaN(v) && v > 0 ? getSpeedColor(v, theme) : undefined
      return (
        <span
          className={cn('font-mono px-2 py-1 rounded', testing && 'animate-pulse')}
          style={color ? { backgroundColor: color, color: '#000' } : undefined}
        >
          {cell.text}
        </span>
      )
    }
    return <span className={cn('font-mono', testing && 'animate-pulse', toneTextClass(cell.tone))}>{cell.text}</span>
  }, [theme])

  const columns = useMemo<ColumnDef<TestNode>[]>(() => {
    const cols: ColumnDef<TestNode>[] = [
      {
        id: 'group', accessorKey: 'group', header: t('col.group'), size: 120,
        cell: ({ row }) => <span className="text-muted-foreground text-xs truncate block" title={row.original.group}>{row.original.group || '-'}</span>,
      },
      {
        id: 'name', accessorKey: 'remark', header: t('col.remark'), size: 220,
        cell: ({ row }) => <div className="max-w-full truncate font-medium" title={row.original.remark}>{row.original.remark}</div>,
      },
      {
        id: 'server', accessorKey: 'server', header: t('col.server'), size: 180, enableSorting: false,
        cell: ({ row }) => <div className="text-muted-foreground font-mono text-xs truncate">{privacy ? maskSensitive(row.original.server) : row.original.server}</div>,
      },
      {
        id: 'protocol', accessorKey: 'protocol', header: t('col.protocol'), size: 110,
        cell: ({ row }) => <span className="px-2 py-1 rounded-md bg-secondary text-xs font-medium uppercase">{row.original.protocol}</span>,
      },
      {
        id: 'ping', header: t('col.ping'), size: 104,
        accessorFn: (row) => (typeof row.ping === 'number' && row.ping > 0 ? row.ping : Number.POSITIVE_INFINITY),
        sortingFn: 'basic',
        cell: ({ row }) => {
          const d = pingDisplay(row.original)
          return <span className={cn('font-mono', row.original.testing && 'animate-pulse', toneTextClass(d.tone))}>{d.text}</span>
        },
      },
      {
        id: 'download', header: t('col.speed'), size: 140,
        accessorFn: (row) => spdVal(row.speed),
        sortingFn: 'basic',
        cell: ({ row }) => speedCellEl(speedDisplay(row.original.speed, row.original), row.original.testing),
      },
    ]
    if (hasUpload) {
      cols.push({
        id: 'upload', header: t('col.upload'), size: 140,
        accessorFn: (row) => spdVal(row.uploadspeed ?? ''),
        sortingFn: 'basic',
        cell: ({ row }) => speedCellEl(speedDisplay(row.original.uploadspeed, row.original), row.original.testing),
      })
    }
    return cols
  }, [t, privacy, hasUpload, pingDisplay, speedDisplay, speedCellEl])

  // group 列默认隐藏(多订阅时才有意义);其余默认显示。用户勾选写回持久化。
  const effectiveVisibility = useMemo(() => ({ group: false, ...columnVisibility }), [columnVisibility])

  const table = useReactTable({
    data: result,
    columns,
    state: { sorting, columnSizing, columnVisibility: effectiveVisibility },
    onSortingChange: setSorting,
    onColumnSizingChange: (u) => setColumnSizing(typeof u === 'function' ? u(columnSizing) : u),
    onColumnVisibilityChange: (u) => setColumnVisibility(typeof u === 'function' ? u(effectiveVisibility) : u),
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (row) => String(row.id),
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    defaultColumn: { minSize: 56, size: 120 },
  })

  // 依据当前排序 + 可见列生成分享 SVG(所见即所导出)
  const makeShare = useCallback(() => {
    const columns = table.getVisibleLeafColumns().map((col) => ({ key: col.id as ColKey, label: colLabel(col.id as ColKey) }))
    const nodes = table.getSortedRowModel().rows.map((r) => r.original)
    const ipLines: { label: string; value: string }[] = []
    if (ipv4) ipLines.push({ label: 'IPv4', value: privacy ? maskSensitive(ipv4) : ipv4 + (ipv4geo ? `  (${ipv4geo})` : '') })
    if (ipv6) ipLines.push({ label: 'IPv6', value: privacy ? maskSensitive(ipv6) : ipv6 + (ipv6geo ? `  (${ipv6geo})` : '') })
    const now = new Date()
    const pad = (x: number) => String(x).padStart(2, '0')
    const ts = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}`
    const input: ShareInput = {
      appearance: options.appearance,
      theme,
      title: 'LiteSpeedTest',
      version,
      subtitle: t('app.subtitle'),
      stats: [
        { label: t('share.statNodes'), value: String(result.length) },
        { label: t('dash.successRate'), value: `${testOkCount}/${result.length}` },
        { label: t('dash.traffic'), value: bytesToSize(totalTraffic) },
        { label: t('dash.duration'), value: formatSeconds(totalTime) },
      ],
      ipTitle: t('ip.title'),
      ipLines,
      columns,
      rows: nodes.map((n) => ({
        group: n.group || '',
        name: n.remark || '-',
        server: privacy ? maskSensitive(n.server) : n.server,
        protocol: n.protocol || '-',
        ping: pingDisplay(n),
        download: speedDisplay(n.speed, n),
        upload: speedDisplay(n.uploadspeed, n),
      })),
      footer: `Powered by LiteSpeedTest ${version} · ${ts}`,
    }
    return buildShareSvg(input)
  }, [table, colLabel, ipv4, ipv6, ipv4geo, ipv6geo, privacy, options.appearance, theme, version, t, result.length, testOkCount, totalTraffic, totalTime, pingDisplay, speedDisplay])

  const fileBase = () => `speedtest_result_${Date.now()}`

  const handleDownloadPng = async () => {
    setBusy(true)
    try {
      triggerDownload(await svgToPngBlob(makeShare()), `${fileBase()}.png`)
    } catch {
      alert(t('share.copyFailed'))
    } finally {
      setBusy(false)
    }
  }
  const handleDownloadSvg = () => triggerDownload(svgToBlob(makeShare().svg), `${fileBase()}.svg`)
  const handleCopyPng = async () => {
    setBusy(true)
    try {
      const blob = await svgToPngBlob(makeShare())
      if (canCopyImage()) {
        try {
          await copyBlobToClipboard(blob)
          alert(t('share.copied'))
        } catch {
          triggerDownload(blob, `${fileBase()}.png`)
          alert(t('share.copyPngFallback'))
        }
      } else {
        triggerDownload(blob, `${fileBase()}.png`)
        alert(t('share.copyPngFallback'))
      }
    } catch {
      alert(t('share.copyFailed'))
    } finally {
      setBusy(false)
    }
  }
  const handleCopySvg = async () => {
    try {
      await copyToClipboard(makeShare().svg)
      alert(t('share.copiedSvg'))
    } catch {
      alert(t('share.copyFailed'))
    }
  }

  const handleCopyAvailable = useCallback(async () => {
    const links = result.filter((n) => typeof n.ping === 'number' && n.ping > 0).map((n) => n.link).join('\n')
    await copyToClipboard(links)
    alert(t('table.copiedAvailable'))
  }, [t, result])

  const handleExportResult = useCallback(() => {
    const nodes = result.map((item) => ({
      id: item.id, group: item.group, remarks: item.remark, protocol: item.protocol,
      ping: `${item.ping}`,
      avg_speed: Math.floor(getSpeed(item.speed)) || 0,
      max_speed: Math.floor(getSpeed(item.maxspeed)) || 0,
      isok: typeof item.ping === 'number' && item.ping > 0,
    }))
    const data = { totalTraffic: bytesToSize(totalTraffic), totalTime: formatSeconds(totalTime), language: options.language, theme, sortMethod: options.sortMethod, nodes }
    downloadFile(JSON.stringify(data, null, 2), 'result.json')
  }, [result, totalTraffic, totalTime, options.language, options.sortMethod, theme])

  if (result.length === 0) return null

  const ipRows: { label: string; ip: string; geo: string }[] = []
  if (ipv4) ipRows.push({ label: 'IPv4', ip: privacy ? maskSensitive(ipv4) : ipv4, geo: privacy ? '' : ipv4geo })
  if (ipv6) ipRows.push({ label: 'IPv6', ip: privacy ? maskSensitive(ipv6) : ipv6, geo: privacy ? '' : ipv6geo })

  const stats = [
    { label: t('share.statNodes'), value: String(result.length) },
    { label: t('dash.successRate'), value: `${testOkCount}/${result.length}` },
    { label: t('dash.traffic'), value: bytesToSize(totalTraffic) },
    { label: t('dash.duration'), value: formatSeconds(totalTime) },
  ]

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.25 }}>
      <Card>
        <CardHeader className="border-b border-border/50 space-y-4">
          <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <Gauge className="w-5 h-5 text-primary shrink-0" />
                <span className="text-lg font-semibold">{t('table.title')}</span>
                <span className="px-1.5 py-0.5 rounded-md bg-primary/15 text-primary text-[10px] font-semibold leading-none">{version}</span>
                <span className="text-sm font-normal text-muted-foreground">{t('table.count', { n: result.length })}</span>
              </div>
              <p className="text-xs text-muted-foreground mt-1">{t('app.subtitle')}</p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="gap-2">
                    <Columns3 className="w-4 h-4" />
                    {t('table.columns')}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-44">
                  {table.getAllLeafColumns().filter((c) => c.getCanHide()).map((column) => (
                    <DropdownMenuCheckboxItem
                      key={column.id}
                      checked={column.getIsVisible()}
                      onCheckedChange={(v) => column.toggleVisibility(!!v)}
                      onSelect={(e) => e.preventDefault()}
                    >
                      {colLabel(column.id as ColKey)}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="gap-2" disabled={busy}>
                    <Share2 className="w-4 h-4" />
                    {t('share.button')}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuItem onClick={handleDownloadPng}><Download className="w-4 h-4 mr-2" />{t('share.downloadPng')}</DropdownMenuItem>
                  <DropdownMenuItem onClick={handleDownloadSvg}><Download className="w-4 h-4 mr-2" />{t('share.downloadSvg')}</DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={handleCopyPng}><Copy className="w-4 h-4 mr-2" />{t('share.copyPng')}</DropdownMenuItem>
                  <DropdownMenuItem onClick={handleCopySvg}><Copy className="w-4 h-4 mr-2" />{t('share.copySvg')}</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="gap-2">
                    <MoreHorizontal className="w-4 h-4" />
                    {t('table.actions')}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuLabel>{t('table.actions')}</DropdownMenuLabel>
                  <DropdownMenuItem onClick={handleCopyAvailable}><Copy className="w-4 h-4 mr-2" />{t('table.copyAvailable')}</DropdownMenuItem>
                  <DropdownMenuItem onClick={handleExportResult}><FileJson className="w-4 h-4 mr-2" />{t('table.exportJson')}</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          {/* 汇总统计(取代旧 Dashboard) */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
            {stats.map((s) => (
              <div key={s.label} className="rounded-lg bg-secondary/40 px-3 py-2 min-w-0">
                <div className="text-[11px] text-muted-foreground truncate">{s.label}</div>
                <div className="text-base font-bold truncate">{s.value}</div>
              </div>
            ))}
          </div>

          {/* 出口 IP(随隐私脱敏) */}
          {ipRows.length > 0 && (
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs">
              <span className="font-medium text-muted-foreground">{t('ip.title')}:</span>
              {ipRows.map((r) => (
                <span key={r.label} className="min-w-0 break-all">
                  <span className="font-semibold text-muted-foreground">{r.label}</span>{' '}
                  <span className="font-mono break-all">{r.ip}</span>
                  {r.geo && <span className="text-muted-foreground"> ({r.geo})</span>}
                </span>
              ))}
            </div>
          )}
        </CardHeader>

        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm" style={{ minWidth: table.getTotalSize(), tableLayout: 'fixed' }}>
              <thead>
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id} className="border-b border-border/50 bg-secondary/30">
                    {hg.headers.map((header) => {
                      const col = header.column
                      const canSort = col.getCanSort()
                      return (
                        <th key={header.id} className="relative px-3 py-2.5 text-left text-xs font-medium text-muted-foreground" style={{ width: header.getSize() }}>
                          <div
                            className={cn('flex items-center gap-1 truncate', canSort && 'cursor-pointer select-none hover:text-foreground')}
                            onClick={canSort ? col.getToggleSortingHandler() : undefined}
                          >
                            <span className="truncate">{flexRender(col.columnDef.header, header.getContext())}</span>
                            {col.getIsSorted() === 'asc' && <ArrowUp className="w-3.5 h-3.5 shrink-0" />}
                            {col.getIsSorted() === 'desc' && <ArrowDown className="w-3.5 h-3.5 shrink-0" />}
                          </div>
                          {col.getCanResize() && (
                            <div
                              onMouseDown={header.getResizeHandler()}
                              onTouchStart={header.getResizeHandler()}
                              onClick={(e) => e.stopPropagation()}
                              className={cn(
                                'absolute right-0 top-0 h-full w-1.5 cursor-col-resize touch-none select-none',
                                col.getIsResizing() ? 'bg-primary' : 'hover:bg-primary/40'
                              )}
                            />
                          )}
                        </th>
                      )
                    })}
                  </tr>
                ))}
              </thead>
              <tbody>
                {table.getRowModel().rows.map((row) => (
                  <tr key={row.id} className="border-b border-border/30 transition-colors hover:bg-secondary/30">
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="px-3 py-2.5 text-sm overflow-hidden" style={{ width: cell.column.getSize() }}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}
