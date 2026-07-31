# proxy-speedtest 待办与交接文档 (TODO)

> 生成于 2026-07-17,用于跨会话交接。
> ⚠️ 生成本文的上一会话工具通道出现异常(见文末「安全备注」),文中标注「需复核」的项,请在新会话用干净通道重新核对后再执行,勿直接采信细节。

---

## 0. 快速上手(给新会话)

- 项目:`proxy-speedtest`,基于 xray-core 的代理测速工具(Go)。三种用法:CLI / Web GUI(默认 10888) / gRPC。
- 前端:`web/gui`,**Vue3 + Vite + TypeScript**(element-plus / echarts / pinia)。
- 关键入口文件:
  - `main.go` —— flag 解析、模式分发
  - `web/server.go` —— Web 服务
  - `api/rpc/liteserver/server.go` —— gRPC 服务
  - `download/download.go` —— 下载测速实现
  - `web/profile.go` —— 测速主流程(**上一会话未干净读过**)
- 开工前先做文末「§7 需复核清单」。

---

## 1. 已完成(已落盘,待评审/提交)

### 1.1 服务监听地址绑定 `-bind` ✅
- 需求:让 Web(10888)与 gRPC 能只绑定到指定 IP / 接口(如只让 tailscale 访问),不暴露公网。
- 改动:
  - `main.go`:新增 `-bind` flag(空 = 所有接口,向后兼容);Web / gRPC 调用处传入。
  - `web/server.go`:`ServeFile(port, bind)`;新增导出函数 `ResolveBindAddress(bind)`(空→全接口;合法 IP→原样;否则按接口名解析首个可用 IP)。
  - `api/rpc/liteserver/server.go`:`StartServer(port, bind)`,复用 `web.ResolveBindAddress`。
  - `README.md`:补充 `-bind` 说明(特性、Web/gRPC 示例、参数表)。
- 状态:已在 **Windows 实测通过**;**未提交**(用户自行 commit)。
- 行为:**fail-closed** —— 绑错 / 接口不存在会 `log.Fatalln` 退出,不会降级暴露 0.0.0.0。
- ⚠️ 需复核:README 写入过程读回有污染,确认最终内容无重复 / 损坏。

---

## 2. 待优化

### 2.1 `ResolveBindAddress` 空字符串二义性加固 · 优先级 低
- 现状:空串同时表示「全接口」和「出错」,靠调用方查 `err` 区分。当前两个调用方都查了,安全。
- 风险:未来新增监听入口若漏检 `err`,`net.JoinHostPort("", port)` 会静默变 0.0.0.0 暴露公网(fail-open)。
- 目标:把「是否绑定」与「监听 host」解耦,或出错返回明确不可用值,让 API 无法 fail-open。

### 2.2 lockfile / Dockerfile 前端构建清理 · 优先级 高(疑似 CI 报错源)
- 现状:
  - `web/gui` 下 `package-lock.json` 与 `pnpm-lock.yaml` **并存**。
  - `Dockerfile`:`COPY web/gui/package.json web/gui/package-lock.json`(只拷这俩)+ RUN 的 `if/elif` 探测 pnpm/yarn/npm。因 pnpm-lock / yarn.lock 未被 COPY,实际锁死 `npm ci`,pnpm/yarn 分支是**死代码**。
  - `package.json` 无 `packageManager` 字段。
- 风险:若 pnpm 为真源、package-lock 过期,Docker 内 `npm ci` 硬失败 → `docker-image.yml` 报错。
- 目标(最优):统一单一包管理器 + 单一 lockfile + `--frozen-lockfile`/`npm ci` + 删死分支 + `package.json` 声明 `packageManager`。
- 待决策:**前端用 pnpm 还是 npm?**
  - 用 pnpm:删 `package-lock.json`;COPY `pnpm-lock.yaml`;`corepack enable && pnpm install --frozen-lockfile`。
  - 用 npm:删 `pnpm-lock.yaml`;`npm ci`;确保 `package-lock.json` 同步。
- ⚠️ 需复核:`.dockerignore` 精确内容(读取被污染)。

---

## 3. 待调查(技术问题)

### 3.1 测速时长 vs 小文件 · 优先级 中
- 问题:测试超时时长较长、但下载文件较小时,剩余时长如何处理?
- 初步(基于 `download/download.go`):下载循环读到 EOF 即结束,无「续下」逻辑 → 小文件提前结束,采样不足,不占满 timeout,峰值速度可能不稳。
- 未确认:GUI 实际测速走 `web/profile.go`(未干净读过),需确认其下载实现是否一致。
- 若确认为问题:考虑在 timeout 内循环 / 持续下载测量,或选更大的测速文件。

---

## 4. 待新增(重大方向)

### 4.1 tsnet 集成(独立模式) · 优先级 待定(需先决策)
- 目标:用 tailscale 的 **tsnet** 让服务直接开在 tailnet 内,本地零暴露,且不依赖宿主机装 tailscale(用户态节点)。比 `-bind` 更彻底,适合 Docker。
- 方案:新增 `-tsnet` 模式,与 `-bind` 并存。gRPC 把 `net.Listen` 换成 `tsSrv.Listen`;Web 用 `tsSrv.Listen` + `http.Serve(ln, nil)`。
- 待决策:① 给 Web / gRPC / both;② auth key 方式(`TS_AUTHKEY` 环境变量 / ephemeral key);③ 是否用 build tag 隔离 tsnet 依赖(避免拖累默认构建与 gomobile)。
- 代价:依赖膨胀(约几十 MB,需验证)、需 auth key、tailnet 多一个节点、需状态目录。

---

## 5. 前端 GUI 改造(web/gui,Vue3+Vite+TS) · 优先级 中
> 前置:上一会话未成功探查 `web/gui/src` 结构,开工前先摸清组件划分。

- **5.1** 砍掉「导出 / 生成」功能(鸡肋),连带清理相关代码。
- **5.2** 去掉「基础 / 高级」模式区分:直接采用高级模式,且**不显示**「高级」字样。
- **5.3** 并发数控制:预设按钮 **1 / 3 / 5** + 一个自定义输入框。
- **5.4** 测试项改为 segmented(切换)样式:**左 = Ping Only / 中 = 测速+TCPing / 右 = SpeedOnly**;ping 方式使用 Google 的真实 TCPing。
- **5.5** 测速端点:**去掉默认**;显示「当前用的是谁」+ 显示测速链接;链接**禁止修改**,仅当选「自定义」时才允许输入。
- **5.6** 前端输入项写入 **localStorage**,下次打开复用。

---

## 6. 代码清洁度(用户要求:干净、无废弃功能) · 优先级 中
> 需系统过一遍源码后给完整清单再删。已知候选:

- `Dockerfile`:pnpm/yarn 死分支(见 §2.2)。
- `transport/dialer/`:`RegisterDialerController` 扩展点从未被调用;`sockopt_linux.go` 仅 `setReusePort`。需确认是否废弃(注意:该包目前仅被 `dns/client.go` 使用)。
- `download/download.go`:`WSDownload` 是空实现(`return 0, nil`)。
- 前端:导出 / 生成功能(见 §5.1)。

---

## 7. 需复核清单(新会话开局先做)
> 因上一会话通道被污染,用干净通道重新读取核对以下项,再据此执行:

- [ ] `README.md`(确认 `-bind` 段落无重复 / 损坏)
- [ ] `.dockerignore`(精确内容,是否排除 lockfile)
- [ ] `Dockerfile`(lockfile 处理段)
- [ ] `web/gui/` 顶层(确认 lockfile 现状)+ `web/gui/src` 结构
- [ ] `web/profile.go`(测速下载实现,回答 §3.1)
- [ ] `git status`(确认 `-bind` 改动仍在、未提交)

---

## 8. 安全备注
上一会话工具通道多次异常:输出污染(重复行、乱序、混入非文件的旁白文本)、`mkdir '(eval):1'` 故障、Glob 失效,系统一度以「疑似 context manipulation / prompt injection」暂停工具执行。判断**更可能是插件 / 中断导致的会话状态损坏,而非外部攻击**,但新会话仍应以干净读取为准。已落盘的 `-bind` 改动应仍在(Edit 成功写入)。
