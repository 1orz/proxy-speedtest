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
import { ChevronDown, ChevronUp, Copy, Download, QrCode, FileJson, MoreHorizontal } from 'lucide-react'
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
import type { TestNode } from '@/types'

export function ResultTable() {
  const { result, selectedNodes, setSelectedNodes, options, totalTraffic, totalTime } = useTestStore()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [qrDialogOpen, setQrDialogOpen] = useState(false)

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
        size: 40,
      },
      {
        accessorKey: 'remark',
        header: '节点名称',
        cell: ({ row }) => (
          <div className="max-w-[300px] truncate font-medium" title={row.original.remark}>
            {row.original.remark}
          </div>
        ),
        size: 300,
      },
      {
        accessorKey: 'server',
        header: '服务器',
        cell: ({ row }) => (
          <div className="text-muted-foreground font-mono text-xs">
            {row.original.server}
          </div>
        ),
        size: 180,
      },
      {
        accessorKey: 'protocol',
        header: '协议',
        cell: ({ row }) => (
          <span className="px-2 py-1 rounded-md bg-secondary text-xs font-medium uppercase">
            {row.original.protocol}
          </span>
        ),
        size: 100,
      },
      {
        accessorKey: 'ping',
        header: 'Ping',
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
        header: '平均速度',
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
      {
        accessorKey: 'maxspeed',
        header: '最大速度',
        cell: ({ row }) => {
          const speed = row.original.maxspeed
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
          const speedA = getSpeed(a.original.maxspeed)
          const speedB = getSpeed(b.original.maxspeed)
          return (isNaN(speedB) ? -1 : speedB) - (isNaN(speedA) ? -1 : speedA)
        },
        size: 120,
      },
    ],
    [options.theme]
  )

  const table = useReactTable({
    data: result,
    columns,
    state: {
      sorting,
      globalFilter,
      rowSelection,
    },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
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
  })

  const handleCopyLinks = useCallback(async () => {
    const links = selectedNodes.map(n => n.link).join('\n')
    await copyToClipboard(links)
    alert('链接已复制到剪贴板')
  }, [selectedNodes])

  const handleCopyAvailable = useCallback(async () => {
    const links = result.filter(n => typeof n.ping === 'number' && n.ping > 0).map(n => n.link).join('\n')
    await copyToClipboard(links)
    alert('可用节点链接已复制到剪贴板')
  }, [result])

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
                测试结果
                <span className="text-sm font-normal text-muted-foreground">
                  ({result.length} 节点)
                </span>
              </CardTitle>
              <div className="flex items-center gap-3">
                <Input
                  placeholder="搜索节点..."
                  value={globalFilter}
                  onChange={(e) => setGlobalFilter(e.target.value)}
                  className="w-64"
                />
                {result.length > 0 && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" className="gap-2">
                        <MoreHorizontal className="w-4 h-4" />
                        操作
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-48">
                      <DropdownMenuItem onClick={handleCopyAvailable}>
                        <Copy className="w-4 h-4 mr-2" />
                        复制可用节点
                      </DropdownMenuItem>
                      {selectedNodes.length > 0 && (
                        <>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={handleCopyLinks}>
                            <Copy className="w-4 h-4 mr-2" />
                            复制选中 ({selectedNodes.length})
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={handleExportNodes}>
                            <Download className="w-4 h-4 mr-2" />
                            导出选中节点
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={handleShowQRCode}>
                            <QrCode className="w-4 h-4 mr-2" />
                            显示二维码
                          </DropdownMenuItem>
                        </>
                      )}
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onClick={handleExportResult}>
                        <FileJson className="w-4 h-4 mr-2" />
                        导出结果 JSON
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id} className="border-b border-border/50 bg-secondary/30">
                      {headerGroup.headers.map((header) => (
                        <th
                          key={header.id}
                          className={cn(
                            'px-4 py-3 text-left text-sm font-medium text-muted-foreground',
                            header.column.getCanSort() && 'cursor-pointer select-none hover:text-foreground'
                          )}
                          style={{ width: header.getSize() }}
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          <div className="flex items-center gap-2">
                            {flexRender(header.column.columnDef.header, header.getContext())}
                            {header.column.getIsSorted() === 'asc' && <ChevronUp className="w-4 h-4" />}
                            {header.column.getIsSorted() === 'desc' && <ChevronDown className="w-4 h-4" />}
                          </div>
                        </th>
                      ))}
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
                        <td key={cell.id} className="px-4 py-3 text-sm">
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
  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>节点二维码</DialogTitle>
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
                {node.ping}ms | {node.speed} | {node.maxspeed}
              </p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}

