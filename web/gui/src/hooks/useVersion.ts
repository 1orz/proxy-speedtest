import { useEffect, useState } from 'react'

// 只保留版本号本身,去掉 git describe 的提交后缀:v1.6.0-4-gb31d493 → v1.6.0(含 -dirty 也一并去掉)。
function cleanVersion(v: string): string {
  return v.replace(/-\d+-g[0-9a-f]+(-dirty)?$/i, '').replace(/-dirty$/i, '')
}

const BUILD_VERSION = cleanVersion(`v${__APP_VERSION__}`)

// 跨组件共享的一次性 /version 拉取(模块级缓存,避免 Header 与结果卡片重复请求)。
let cached: string | null = null
let inflight: Promise<string> | null = null

export function useVersion(): string {
  const [version, setVersion] = useState<string>(cached ?? BUILD_VERSION)
  useEffect(() => {
    if (cached) return // 已有缓存:state 初始值即为它,无需再 setState
    let alive = true
    if (!inflight) {
      inflight = fetch('/version')
        .then((r) => (r.ok ? r.json() : null))
        .then((d) => {
          cached = d?.version ? cleanVersion(String(d.version)) : BUILD_VERSION
          return cached
        })
        .catch(() => {
          cached = BUILD_VERSION
          return cached
        })
    }
    inflight.then((v) => {
      if (alive) setVersion(v)
    })
    return () => {
      alive = false
    }
  }, [])
  return version
}
