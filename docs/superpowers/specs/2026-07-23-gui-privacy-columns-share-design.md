# 前端增强:隐私保护 / 自定义列 / 下载列改名 / Key 不预填 / 矢量分享图片

日期:2026-07-23
范围:`web/gui`(纯前端 React + TypeScript,不改 Go 后端)

## 目标

在测速前端 GUI 上新增/调整五项功能:

1. **一键隐私保护**:一键遮蔽界面与导出图片里的 IP、服务器地址。
2. **自定义列显示**:表格列可勾选显示/隐藏,默认全显示,偏好持久化。
3. **「速度」列改名为「下载」**。
4. **Worker Key 不预填**:选 Worker 端点只填 URL,Key 留空。
5. **前端矢量出图**:真·矢量 SVG 分享卡片 + 「分享图片」菜单(下载/复制 × PNG/SVG)。

## 非目标

- 不改后端 `/renderImage`、WebSocket 协议或任何 Go 代码。前端不再调用 `/renderImage`,该端点暂保留(后续可单独清理)。
- 不脱敏「复制/导出/二维码」里的订阅 link(那是用户主动动作,且 link 脱敏即失效)。

---

## 1. 一键隐私保护

### 脱敏函数(`src/lib/utils.ts`)

```
maskSensitive(s: string): string
```

规则:保留字符串**首 2 个字符**和**末 1 个字符**;保留分隔符 `.` 和 `:`;其余字符按「连续被遮蔽的一段折叠成单个 `*`」处理。

- `123.123.123.123` → `12*.*.*.*3`
- `hk1.example.com` → `hk*.*.*m`

实现:遍历字符,`keep[i]` 为真当 `i ∈ {0, 1, len-1}` 或该字符是分隔符(`.`/`:`);逐字符输出被保留的字符,遇到非保留字符的极大连续段输出单个 `*`。若没有任何字符被遮蔽(串太短,如 `len ≤ 3`),原样返回。

### 开关与存储

- store 顶层新增持久化字段 `privacy: boolean`(默认 `false`),setter `setPrivacy`,并纳入 `partialize` / `merge`(与 `columnOrder`/`columnSizing` 同级)。

### 入口(`src/components/Header.tsx`)

- 语言/主题按钮旁加一个眼睛图标按钮:`privacy` 为真显示 `EyeOff`,为假显示 `Eye`;点击 `setPrivacy(!privacy)`;`title`/`aria-label` 走 i18n。

### 作用点

- **`IPInfoCard`**:`privacy` 为真时,`ip` 走 `maskSensitive`,`geo` 行**整行隐藏**(prose 不套遮蔽规则)。
- **`ResultTable` 服务器列**:`privacy` 为真时单元格文本走 `maskSensitive`。
- **分享图片**:卡片头部出口 IP 同样走 `maskSensitive`,geo 隐藏。图片不含节点 server 地址。

---

## 2. 自定义列显示

- store 顶层新增持久化字段 `columnVisibility: Record<string, boolean>`(默认 `{}` = 全显示),setter `setColumnVisibility`,纳入 `partialize`/`merge`。
- `ResultTable`:TanStack 表启用 `state.columnVisibility` + `onColumnVisibilityChange`(写回 store)。
- 各数据列设 `enableHiding: true`;`select` 列设 `enableHiding: false`(始终显示)。
- 搜索框旁加独立 **「列」** 按钮(`Columns` 图标)→ `DropdownMenu`,用 `DropdownMenuCheckboxItem` 列出 `table.getAllLeafColumns().filter(c => c.getCanHide())`,勾选即 `column.toggleVisibility(!!v)`。列名复用 `col.*` 文案(上传列存在时才在清单出现)。
- `resetTableLayout` 同时重置 `columnVisibility: {}`(连同 `columnOrder`/`columnSizing`)。

## 3. 「速度」→「下载」

- `src/lib/i18n.ts`:`col.speed` 的 `cn` 由「速度」改「下载」,`en` 由「Speed」改「Download」。仅文案。

## 4. Worker Key 不预填(`src/components/TestForm.tsx`)

- 删除常量 `WORKER_KEY_DEFAULT`。
- 下载端点选 `worker` 分支:`setOptions({ downloadSize: 'worker', downloadUrl: WORKER_DOWN_URL })`(去掉 `workerKey: ... || WORKER_KEY_DEFAULT`)。
- 上传端点选 `worker` 分支:`setOptions({ uploadSize: 'worker', uploadUrl: WORKER_UP_URL })`(去掉 workerKey 预填)。
- 用户已手填的 `workerKey` 不动。

## 5. 前端矢量分享图片

### 生成器(`src/lib/share-image.ts`)

纯函数,输入当前测试快照,输出真·矢量 SVG 字符串:

```
buildShareSvg(input: ShareInput): string
```

`ShareInput` 含:`appearance`('dark'|'light')、`theme`、汇总(节点数/成功数/流量文本/耗时文本)、出口 IP(已按 `privacy` 处理:脱敏值或空、geo 隐藏)、节点列表(名称/协议/ping/下载 speed 字符串/上传 speed 字符串,`hasUpload` 决定是否含上传列)。

版式:标题带 + 汇总行 + 出口 IP 块 + 节点表。表列:名称 | 协议 | Ping | 下载 | 上传(可选)。下载/上传单元格为「色块 rect + 文字」,颜色用 `getSpeedColor(getSpeed(speed), theme)`。深浅色由 `appearance` 决定背景/前景/描边色。

布局用固定列 x 坐标与固定行高;长名称按最大字符数截断加省略号(纯字符串生成,不依赖 DOM 量宽)。SVG 宽高按列宽合计与行数算出。字体用通用 `font-family: ui-sans-serif, system-ui, ...`。

栅格化/复制(同文件导出工具函数):

- `svgToPngBlob(svg, scale=3): Promise<Blob>`:`new Image()` 载入 `data:image/svg+xml;charset=utf-8,<encodeURIComponent(svg)>` → 画到 `canvas`(宽高 × scale)→ `canvas.toBlob('image/png')`。
- 下载 SVG:`Blob([svg], {type:'image/svg+xml'})` → 触发下载。
- 下载 PNG:`svgToPngBlob` → 触发下载。
- 复制 PNG:`navigator.clipboard.write([new ClipboardItem({'image/png': blob})])`。
- 复制 SVG:`navigator.clipboard.writeText(svg)`(复制 SVG 源码文本;浏览器对 svg+xml 图片剪贴板支持差,文本最可靠)。

### UI(重写 `src/components/ResultImage.tsx`)

- 卡片标题旁一个 **「分享图片」** 按钮(`Share2`),`DropdownMenu` 展开 4 项:下载 PNG / 下载 SVG / 复制 PNG / 复制 SVG。
- 卡片主体内联渲染当前 SVG 预览(用 `dangerouslySetInnerHTML` 或 `img[src=data-uri]`)。
- 数据来自 store(`result`、`testOkCount`、`totalTraffic`、`totalTime`、`ipv4/ipv6/geo`、`options.appearance/theme/language`、`privacy`、`hasUpload`)。
- 复制成功/失败用 `alert()`(与现有 `table.copied` 风格一致)。
- 不再使用 `picdata` / `regenerateImage`。旧的「有结果才显示卡片」判据改为 `result.length > 0`。

### i18n 新增 key(`en` + `cn` 同步)

- `privacy.on` / `privacy.off`(按钮 title)、`ip.masked`(可选)
- `table.columns`(「列」按钮)
- `share.title`(卡片标题,替代 `image.title`)、`share.button`(分享图片)、`share.downloadPng`、`share.downloadSvg`、`share.copyPng`、`share.copySvg`、`share.copied`、`share.copyFailed`、`share.empty`(无结果占位)

## 数据流 / 存储小结

- 新增持久化 UI 偏好(store 顶层):`privacy`、`columnVisibility`。与既有 `columnOrder`/`columnSizing` 一样走 `partialize`/`merge`,`resetTableLayout` 重置列相关三项(不含 privacy)。
- 脱敏是纯展示层变换(`maskSensitive`),不改底层 `result`/`options` 数据,故切换开关即时生效、可逆。

## 测试 / 验证

- `pnpm build`(`tsc -b && vite build`)通过、`pnpm lint` 无新错误。
- 手动:开关隐私→IP 卡片/服务器列即时脱敏且符合示例;列勾选显示/隐藏并刷新后保留;下载列表头显示「下载」;选 Worker 端点 URL 填好而 Key 空;四种分享动作各自产出正确的 PNG/SVG(含隐私脱敏)。

## 风险

- `ClipboardItem` / `clipboard.write` 需 HTTPS 或 localhost;失败时提示 `share.copyFailed`。
- SVG→PNG 经 `Image` 载入 data-uri 属同源、无 `crossOrigin` 污染问题(不引用外部资源、不嵌图)。
