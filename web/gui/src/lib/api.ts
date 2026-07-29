// api.ts —— 后端接口地址的相对解析。
//
// 构建产物用了相对 base(vite.config.ts 里 base: './'),静态资源写成 ./assets/...,
// 所以同一份 dist 可以直接挂在二级路径下(如反代到 https://host/speedtest/)。
// 但接口地址如果还写死成 '/version'、'/test',在二级路径下就会打到站点根目录去,
// 于是这里统一按「当前文档所在目录」解析,和浏览器解析 ./assets/... 的规则完全一致:
// 资源能加载出来,接口就一定打得中。
//
//   页面 /              → apiUrl('version') = /version
//   页面 /speedtest/     → apiUrl('version') = /speedtest/version
//   页面 /a/b/index.html → apiUrl('version') = /a/b/version
//
// 注意:
//   1) path 必须是相对路径(不要带前导 '/'),否则又变回绝对地址;
//   2) 后端只注册了根级路由,所以反代必须把前缀剥掉(nginx 的 proxy_pass 带尾斜杠),
//      并且要带尾斜杠访问(/speedtest/ 而不是 /speedtest)——详见 README「Deploy under a sub-path」。
//      少了尾斜杠时接口和 ./assets/... 会一起解析到上一级,行为一致但都拿不到东西。
// document.baseURI 会尊重 <base href>,前缀已剥离、只是入口少了尾斜杠时可以拿它兜底。

/** 普通 HTTP 接口地址(相对当前页面目录解析)。 */
export function apiUrl(path: string): string {
  return new URL(path, document.baseURI).toString()
}

/** WebSocket 接口地址;按页面协议选择 ws/wss,HTTPS 部署下才不会被混合内容策略拦截。 */
export function wsUrl(path: string): string {
  const u = new URL(path, document.baseURI)
  const proto = u.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${u.host}${u.pathname}${u.search}`
}
