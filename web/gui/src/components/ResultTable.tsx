import { useMemo, useState, useCallback } from 'react'
import { motion } from 'motion/react'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type RowSelectionState,
} from '@tanstack/react-table'
import { ChevronDown, ChevronUp, Copy, Download, QrCode, FileJson, MoreHorizontal, GripVertical, RotateCcw } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTestStore } from '@/store/test-store'
import { cn, getSpeed, getSpeedColor, copyToClipboard, downloadFile, bytesToSize, formatSeconds } from '@/lib/utils'
import { useI18n } from '@/hooks/useI18n'
import type { TestNode } from '@/types'

export function ResultTable() {
  const t = useI18n()
  const {
    result, selectedNodes, setSelectedNodes, options, totalTraffic, totalTime,
    runUploadEnabled, columnOrder, columnSizing, setColumnOrder, setColumnSizing, resetTableLayout,
  } = useTestStore()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [qrDialogOpen, setQrDialogOpen] = useState(false)
  const [dragCol, setDragCol] = useState<string | null>(null)

  // 本次运行启用了上传(或已有上传数据)时就显示上传列 —— 一开始即确定,不再中途蹦出
  const hasUpload = runUploadEnabled || result.some((n) => !!n.uploadspeed)

  const columns = useMemo<ColumnDef<TestNode>[]>(
    () => [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
            aria-label="Select all"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label="Select row"
            disabled={row.original.testing}
          />
        ),
        enableSorting: false,
        enableResizing: false,
        size: 44,
        minSize: 44,
      },
      {
        accessorKey: 'remark',
        header: t('col.remark'),
        cell: ({ row }) => (
          <div className="max-w-[300px] truncate font-medium" title={row.original.remark}>
            {row.original.remark}
          </div>
        ),
        size: 300,
      },
      {
        accessorKey: 'server',
        header: t('col.server'),
        cell: ({ row }) => (
          <div className="text-muted-foreground font-mono text-xs">
            {row.original.server}
          </div>
        ),
        size: 180,
      },
      {
        accessorKey: 'protocol',
        header: t('col.protocol'),
        cell: ({ row }) => (
          <span className="px-2 py-1 rounded-md bg-secondary text-xs font-medium uppercase">
            {row.original.protocol}
          </span>
        ),
        size: 100,
      },
      {
        accessorKey: 'ping',
        header: t('col.ping'),
        cell: ({ row }) => {
          const ping = row.original.ping
          const isNumber = typeof ping === 'number'
          const pingValue = isNumber ? ping : 0
          return (
            <span className={cn(
              'font-mono',
              row.original.testing && 'animate-pulse text-primary',
              isNumber && pingValue > 0 && 'text-success',
              isNumber && pingValue === 0 && 'text-muted-foreground'
            )}>
              {typeof ping === 'string' ? ping : `${ping} ms`}
            </span>
          )
        },
        sortingFn: (a, b) => {
          const pingA = typeof a.original.ping === 'number' ? a.original.ping : 0
          const pingB = typeof b.original.ping === 'number' ? b.original.ping : 0
          if (pingA === 0) return 1
          if (pingB === 0) return -1
          return pingA - pingB
        },
        size: 100,
      },
      {
        accessorKey: 'speed',
        header: t('col.speed'),
        cell: ({ row }) => {
          const speed = row.original.speed
          const speedValue = getSpeed(speed)
          const color = !isNaN(speedValue) && speedValue > 0 ? getSpeedColor(speedValue, options.theme) : undefined
          return (
            <span
              className={cn(
                'font-mono px-2 py-1 rounded',
                row.original.testing && 'animate-pulse text-primary'
              )}
              style={color ? { backgroundColor: color, color: '#000' } : undefined}
            >
              {speed}
            </span>
          )
        },
        sortingFn: (a, b) => {
          const speedA = getSpeed(a.original.speed)
          const speedB = getSpeed(b.original.speed)
          return (isNaN(speedB) ? -1 : speedB) - (isNaN(speedA) ? -1 : speedA)
        },
        size: 120,
      },
      ...(hasUpload
        ? ([
            {
              accessorKey: 'uploadspeed',
              header: t('col.upload'),
              cell: ({ row }) => {
                const speed = row.original.uploadspeed ?? ''
                const speedValue = getSpeed(speed)
                const color = !isNaN(speedValue) && speedValue > 0 ? getSpeedColor(speedValue, options.theme) : undefined
                return (
                  <span
                    className={cn('font-mono px-2 py-1 rounded', row.original.testing && 'animate-pulse text-primary')}
                    style={color ? { backgroundColor: color, color: '#000' } : undefined}
                  >
                    {speed || '-'}
                  </span>
                )
              },
              sortingFn: (a, b) => {
                const sa = getSpeed(a.original.uploadspeed ?? '')
                const sb = getSpeed(b.original.uploadspeed ?? '')
                return (isNaN(sb) ? -1 : sb) - (isNaN(sa) ? -1 : sa)
              },
              size: 120,
            },
          ] as ColumnDef<TestNode>[])
        : []),
    ],
    [t, options.theme, hasUpload]
  )

  // 当前所有列 id(顺序为定义顺序);上传列存在与否会变
  const columnIds = useMemo(
    () => columns.map((c) => (('id' in c && c.id) || ('accessorKey' in c && (c.accessorKey as string)) || '')),
    [columns]
  )
  // 有效列顺序 = 持久化顺序中仍存在的列 + 新增列(追加),且 select 永远第一
  const effectiveOrder = useMemo(() => {
    const known = new Set(columnIds)
    const ordered = columnOrder.filter((id) => known.has(id))
    const missing = columnIds.filter((id) => !ordered.includes(id))
    const merged = [...ordered, ...missing].filter((id) => id !== 'select')
    return ['select', ...merged]
  }, [columnOrder, columnIds])

  // 表头拖放重排:把 dragCol 移动到 targetId 之前
  const handleReorder = useCallback((targetId: string) => {
    setDragCol(null)
    if (!dragCol || dragCol === targetId || targetId === 'select' || dragCol === 'select') return
    const next = [...effectiveOrder]
    const from = next.indexOf(dragCol)
    const to = next.indexOf(targetId)
    if (from < 0 || to < 0) return
    next.splice(to, 0, next.splice(from, 1)[0])
    setColumnOrder(next)
  }, [dragCol, effectiveOrder, setColumnOrder])

  const table = useReactTable({
    data: result,
    columns,
    state: {
      sorting,
      globalFilter,
      rowSelection,
      columnOrder: effectiveOrder,
      columnSizing,
    },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    onColumnOrderChange: (updater) =>
      setColumnOrder(typeof updater === 'function' ? updater(effectiveOrder) : updater),
    onColumnSizingChange: (updater) =>
      setColumnSizing(typeof updater === 'function' ? updater(columnSizing) : updater),
    onRowSelectionChange: (updater) => {
      const newSelection = typeof updater === 'function' ? updater(rowSelection) : updater
      setRowSelection(newSelection)
      const selectedRows = Object.keys(newSelection)
        .filter(key => newSelection[key])
        .map(key => result[parseInt(key)])
        .filter(Boolean)
      setSelectedNodes(selectedRows)
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getRowId: (row) => String(row.id),
    enableRowSelection: (row) => !row.original.testing,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    defaultColumn: { minSize: 60, size: 120 },
  })

  const handleCopyLinks = useCallback(async () => {
    const links = selectedNodes.map(n => n.link).join('\n')
    await copyToClipboard(links)
    alert(t('table.copied'))
  }, [t, selectedNodes])

  const handleCopyAvailable = useCallback(async () => {
    const links = result.filter(n => typeof n.ping === 'number' && n.ping > 0).map(n => n.link).join('\n')
    await copyToClipboard(links)
    alert(t('table.copiedAvailable'))
  }, [t, result])

  const handleExportNodes = useCallback(() => {
    const data = selectedNodes.map(n => `# ${n.remark}\t${n.ping}\t${n.speed}\t${n.maxspeed}\n${n.link}`).join('\n')
    downloadFile(data, 'nodes.txt')
  }, [selectedNodes])

  const handleExportResult = useCallback(() => {
    const nodes = result.map(item => ({
      id: item.id,
      group: item.group,
      remarks: item.remark,
      protocol: item.protocol,
      ping: `${item.ping}`,
      avg_speed: Math.floor(getSpeed(item.speed)) || 0,
      max_speed: Math.floor(getSpeed(item.maxspeed)) || 0,
      isok: typeof item.ping === 'number' && item.ping > 0,
    }))
    const data = {
      totalTraffic: bytesToSize(totalTraffic),
      totalTime: formatSeconds(totalTime),
      language: options.language,
      fontSize: options.fontSize,
      theme: options.theme,
      sortMethod: options.sortMethod,
      nodes,
    }
    downloadFile(JSON.stringify(data, null, 2), 'result.json')
  }, [result, totalTraffic, totalTime, options])

  const handleShowQRCode = useCallback(() => {
    setQrDialogOpen(true)
  }, [])

  if (result.length === 0) {
    return null
  }

  return (
    <>
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.3 }}
      >
        <Card>
          <CardHeader className="border-b border-border/50">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <CardTitle className="flex items-center gap-2">
                {t('table.title')}
                <span className="text-sm font-normal text-muted-foreground">
                  {t('table.count', { n: result.length })}
                </span>
              </CardTitle>
              <div className="flex items-center gap-3">
                <Input
                  placeholder={t('table.search')}
                  value={globalFilter}
                  onChange={(e) => setGlobalFilter(e.target.value)}
                  className="w-64"
                />
                {result.length > 0 && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" className="gap-2">
                        <MoreHorizontal className="w-4 h-4" />
                        {t('table.actions')}
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-48">
                      <DropdownMenuItem onClick={handleCopyAvailable}>
                        <Copy className="w-4 h-4 mr-2" />
                        {t('table.copyAvailable')}
                      </DropdownMenuItem>
                      {selectedNodes.length > 0 && (
                        <>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={handleCopyLinks}>
                            <Copy className="w-4 h-4 mr-2" />
                            {t('table.copySelected', { n: selectedNodes.length })}
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={handleExportNodes}>
                            <Download className="w-4 h-4 mr-2" />
                            {t('table.exportSelected')}
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={handleShowQRCode}>
                            <QrCode className="w-4 h-4 mr-2" />
                            {t('table.showQr')}
                          </DropdownMenuItem>
                        </>
                      )}
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onClick={handleExportResult}>
                        <FileJson className="w-4 h-4 mr-2" />
                        {t('table.exportJson')}
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={resetTableLayout}>
                        <RotateCcw className="w-4 h-4 mr-2" />
                        {t('table.resetLayout')}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm" style={{ minWidth: table.getTotalSize(), tableLayout: 'fixed' }}>
                <thead>
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id} className="border-b border-border/50 bg-secondary/30">
                      {headerGroup.headers.map((header) => {
                        const col = header.column
                        const draggable = col.id !== 'select'
                        const canSort = col.getCanSort()
                        return (
                          <th
                            key={header.id}
                            className={cn(
                              'relative px-4 py-3 text-left text-sm font-medium text-muted-foreground',
                              dragCol && draggable && dragCol !== col.id && 'bg-primary/10'
                            )}
                            style={{ width: header.getSize() }}
                            onDragOver={draggable ? (e) => e.preventDefault() : undefined}
                            onDrop={draggable ? () => handleReorder(col.id) : undefined}
                          >
                            <div className="flex items-center gap-1.5 overflow-hidden">
                              {draggable && (
                                <span
                                  draggable
                                  onDragStart={() => setDragCol(col.id)}
                                  onDragEnd={() => setDragCol(null)}
                                  className="shrink-0 cursor-grab text-muted-foreground/40 hover:text-foreground active:cursor-grabbing"
                                  title={t('table.dragHint')}
                                >
                                  <GripVertical className="w-3.5 h-3.5" />
                                </span>
                              )}
                              <div
                                className={cn(
                                  'flex items-center gap-1 truncate',
                                  canSort && 'cursor-pointer select-none hover:text-foreground'
                                )}
                                onClick={canSort ? col.getToggleSortingHandler() : undefined}
                              >
                                {header.isPlaceholder ? null : flexRender(col.columnDef.header, header.getContext())}
                                {col.getIsSorted() === 'asc' && <ChevronUp className="w-4 h-4 shrink-0" />}
                                {col.getIsSorted() === 'desc' && <ChevronDown className="w-4 h-4 shrink-0" />}
                              </div>
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
                  {table.getRowModel().rows.map((row, index) => (
                    <motion.tr
                      key={row.id}
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: Math.min(index * 0.01, 0.5) }}
                      className={cn(
                        'border-b border-border/30 transition-colors',
                        'hover:bg-secondary/30',
                        row.getIsSelected() && 'bg-primary/10'
                      )}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <td
                          key={cell.id}
                          className="px-4 py-3 text-sm overflow-hidden"
                          style={{ width: cell.column.getSize() }}
                        >
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </motion.tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {qrDialogOpen && (
        <QRCodeDialog
          nodes={selectedNodes}
          open={qrDialogOpen}
          onClose={() => setQrDialogOpen(false)}
        />
      )}
    </>
  )
}

import { QRCodeSVG } from 'qrcode.react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface QRCodeDialogProps {
  nodes: TestNode[]
  open: boolean
  onClose: () => void
}

function QRCodeDialog({ nodes, open, onClose }: QRCodeDialogProps) {
  const t = useI18n()
  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('qr.title')}</DialogTitle>
        </DialogHeader>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 p-4">
          {nodes.map((node) => (
            <div
              key={node.id}
              className="flex flex-col items-center p-4 rounded-lg bg-secondary/50 border border-border"
            >
              <div className="bg-white p-2 rounded-lg mb-3">
                <QRCodeSVG value={node.link} size={160} />
              </div>
              <p className="text-sm font-medium text-center truncate w-full" title={node.remark}>
                {node.remark}
              </p>
              <p className="text-xs text-muted-foreground">
                {node.ping}ms | {node.speed}
              </p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}

