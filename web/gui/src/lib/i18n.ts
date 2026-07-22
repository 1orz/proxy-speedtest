import type { Language } from '@/types'

// 全站中英文文案。`en` 为键的权威来源;`cn` 用 `typeof en` 约束,漏键会被 tsc 拦下。
const en = {
  'app.subtitle': 'High-performance proxy speed test',
  'lang.toggle': 'EN',
  'theme.toLight': 'Switch to light',
  'theme.toDark': 'Switch to dark',

  // TestForm
  'form.title': 'Test Configuration',
  'form.subscription': 'Subscription Link',
  'form.subscription.ph': 'Supports V2Ray/Trojan/SS/SSR/Clash/VLESS subscription links',
  'form.upload.dnd': 'Drag a config file here, or ',
  'form.upload.click': 'click to upload',
  'form.upload.tooLarge': 'File must not exceed 10MB',
  'form.concurrency': 'Concurrency',
  'form.concurrency.hint': 'Nodes tested simultaneously',
  'form.custom': 'Custom',
  'form.threads': 'Download Threads',
  'form.threads.hint': 'Parallel connections per node to aggregate throughput; use 1GB endpoints (Hetzner/OVH) for multi-thread — Cloudflare’s small file gets throttled',
  'form.testItems': 'Test Items',
  'form.mode.all': 'Speed + Tcping',
  'form.duration': 'Duration (s)',
  'form.unique': 'Remove duplicate nodes',
  'form.groupname': 'Custom Group Name',
  'form.groupname.ph': 'Optional, leave blank for default',
  'form.downloadEndpoint': 'Download Endpoint',
  'form.customUrl': 'Custom URL',
  'form.downloadUrl.ph': 'https://example.com/100mb.bin (large-file direct link reachable via proxy)',
  'form.downloadEndpoint.hint': 'Current URL shown above; presets are read-only. Choose “Custom URL” to enter your own large-file link.',
  'form.uploadTest': 'Test Upload Speed',
  'form.uploadTest.disabled': '(requires a speed test mode)',
  'form.uploadTest.hint': 'Runs an upload test after download; adds time per node',
  'form.uploadEndpoint.hint': 'Upload sinks are scarce (static endpoints 405 on POST); only these two are verified discard sinks.',
  'form.currentUrl.aria': 'Current test URL',
  'form.currentUploadUrl.aria': 'Current upload URL',
  'form.alert.noSub': 'Enter a subscription link or upload a config file',
  'form.alert.noCustomUrl': 'You selected “Custom URL” — please enter a download link',
  'form.alert.connectFail': 'Failed to connect to the test service, please retry',
  'form.start': 'Start Test',
  'form.testing': 'Testing...',
  'form.terminate': 'Stop',

  // Dashboard
  'dash.progress': 'Progress',
  'dash.done': 'Done {n}',
  'dash.totalNodes': '{n} nodes',
  'dash.successRate': 'Success Rate',
  'dash.traffic': 'Traffic Used',
  'dash.duration': 'Elapsed',
  'dash.protocols': 'Protocols',

  // ResultTable
  'col.remark': 'Node',
  'col.server': 'Server',
  'col.protocol': 'Protocol',
  'col.ping': 'Ping',
  'col.speed': 'Speed',
  'col.upload': 'Upload',
  'table.title': 'Results',
  'table.count': '({n} nodes)',
  'table.search': 'Search nodes...',
  'table.actions': 'Actions',
  'table.copyAvailable': 'Copy working nodes',
  'table.copySelected': 'Copy selected ({n})',
  'table.exportSelected': 'Export selected',
  'table.showQr': 'Show QR codes',
  'table.exportJson': 'Export JSON',
  'table.copied': 'Links copied to clipboard',
  'table.copiedAvailable': 'Working node links copied to clipboard',
  'qr.title': 'Node QR Codes',

  // LiveMeter
  'live.testing': 'Testing:',
  'live.node': 'Node #{id}',
  'live.preparing': 'Preparing…',
  'live.downloading': 'Downloading',
  'live.uploading': 'Uploading',

  // ResultImage
  'image.title': 'Exported Image',
  'image.download': 'Download',

  // IP card
  'ip.title': 'Public Egress IP',
  'ip.pending': 'Resolving…',
  'ip.none': 'N/A',

  // status placeholders (used in store)
  'status.testing': 'Testing...',
} as const

export type TKey = keyof typeof en

const cn: Record<TKey, string> = {
  'app.subtitle': '高性能代理节点测速工具',
  'lang.toggle': '中',
  'theme.toLight': '切换到浅色',
  'theme.toDark': '切换到深色',

  'form.title': '测速配置',
  'form.subscription': '订阅链接',
  'form.subscription.ph': '支持 V2Ray/Trojan/SS/SSR/Clash/VLESS 订阅链接',
  'form.upload.dnd': '拖拽配置文件到此处，或',
  'form.upload.click': '点击上传',
  'form.upload.tooLarge': '文件大小不能超过 10MB',
  'form.concurrency': '并发数',
  'form.concurrency.hint': '同时测试的节点数量',
  'form.custom': '自定义',
  'form.threads': '下载线程数',
  'form.threads.hint': '单个节点测速时的并行连接数,聚合吞吐;多线程请配 1GB 端点(Hetzner/OVH),Cloudflare 小文件多线程易被限流',
  'form.testItems': '测试项',
  'form.mode.all': '测速 + Tcping',
  'form.duration': '测试时长 (秒)',
  'form.unique': '去除重复节点',
  'form.groupname': '自定义组名',
  'form.groupname.ph': '可选，留空使用默认值',
  'form.downloadEndpoint': '下载测速端点',
  'form.customUrl': '自定义 URL',
  'form.downloadUrl.ph': 'https://example.com/100mb.bin （需可通过代理访问的大文件直链）',
  'form.downloadEndpoint.hint': '当前测速链接如上；预设不可修改，选“自定义 URL”后可填写自己的大文件直链。',
  'form.uploadTest': '测上传速度',
  'form.uploadTest.disabled': '（需开启测速项才可用）',
  'form.uploadTest.hint': '在下载之后追加上传测速,会增加每节点耗时',
  'form.uploadEndpoint.hint': '上传端点稀缺(静态端点 POST 会 405);仅这两个为实测可用的丢弃型 sink。',
  'form.currentUrl.aria': '当前测速链接',
  'form.currentUploadUrl.aria': '当前上传链接',
  'form.alert.noSub': '请输入订阅链接或上传配置文件',
  'form.alert.noCustomUrl': '已选择“自定义 URL”，请填写测速下载直链',
  'form.alert.connectFail': '连接测速服务失败，请重试',
  'form.start': '开始测速',
  'form.testing': '测速中...',
  'form.terminate': '终止',

  'dash.progress': '测试进度',
  'dash.done': '已完成 {n}',
  'dash.totalNodes': '共 {n} 节点',
  'dash.successRate': '成功率',
  'dash.traffic': '已用流量',
  'dash.duration': '测试耗时',
  'dash.protocols': '协议统计',

  'col.remark': '节点名称',
  'col.server': '服务器',
  'col.protocol': '协议',
  'col.ping': 'Ping',
  'col.speed': '速度',
  'col.upload': '上传',
  'table.title': '测试结果',
  'table.count': '({n} 节点)',
  'table.search': '搜索节点...',
  'table.actions': '操作',
  'table.copyAvailable': '复制可用节点',
  'table.copySelected': '复制选中 ({n})',
  'table.exportSelected': '导出选中节点',
  'table.showQr': '显示二维码',
  'table.exportJson': '导出结果 JSON',
  'table.copied': '链接已复制到剪贴板',
  'table.copiedAvailable': '可用节点链接已复制到剪贴板',
  'qr.title': '节点二维码',

  'live.testing': '正在测速：',
  'live.node': '节点 #{id}',
  'live.preparing': '准备中…',
  'live.downloading': '下载中',
  'live.uploading': '上传中',

  'image.title': '导出图片',
  'image.download': '下载图片',

  'ip.title': '公网出口 IP',
  'ip.pending': '获取中…',
  'ip.none': '未获取',

  'status.testing': '测试中...',
}

const messages: Record<Language, Record<TKey, string>> = { en, cn }

function interpolate(s: string, vars?: Record<string, string | number>): string {
  if (!vars) return s
  return s.replace(/\{(\w+)\}/g, (_, k) => (k in vars ? String(vars[k]) : `{${k}}`))
}

// tt 是非 React 的翻译函数(store 等处直接用);缺失语言回退到 en。
export function tt(lang: Language, key: TKey, vars?: Record<string, string | number>): string {
  const table = messages[lang] ?? messages.en
  return interpolate(table[key] ?? messages.en[key], vars)
}
