import { useCallback } from 'react'
import { useTestStore } from '@/store/test-store'
import { tt, type TKey } from '@/lib/i18n'

// useI18n 返回绑定当前语言的翻译函数。用 useCallback 保证同一语言下 t 引用稳定,
// 这样把 t 放进 useMemo/useCallback 依赖数组时,只有语言切换才触发重算(而非每次渲染)。
export function useI18n() {
  const lang = useTestStore((s) => s.options.language)
  return useCallback((key: TKey, vars?: Record<string, string | number>) => tt(lang, key, vars), [lang])
}
