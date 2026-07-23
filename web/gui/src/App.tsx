import { useEffect } from 'react'
import { AnimatePresence } from 'motion/react'
import { Header } from '@/components/Header'
import { TestForm } from '@/components/TestForm'
import { LiveMeter } from '@/components/LiveMeter'
import { ResultCard } from '@/components/ResultCard'
import { useTimer } from '@/hooks/useTimer'
import { useVersion } from '@/hooks/useVersion'
import { useTestStore } from '@/store/test-store'

function App() {
  useTimer()
  const version = useVersion()
  const loading = useTestStore((s) => s.loading)
  const appearance = useTestStore((s) => s.options.appearance)

  // 把主题写到 <html data-theme> 上,驱动 index.css 的浅色覆盖。
  useEffect(() => {
    document.documentElement.dataset.theme = appearance
  }, [appearance])

  return (
    <div className="min-h-screen gradient-bg grid-pattern">
      <Header />

      <main className="container mx-auto px-4 pb-12 space-y-6">
        <TestForm />
        <AnimatePresence>{loading && <LiveMeter key="live-meter" />}</AnimatePresence>
        <ResultCard />
      </main>

      {/* 底部装饰 */}
      <footer className="py-6 text-center text-sm text-muted-foreground">
        <p>
          Powered by{' '}
          <a
            href="https://github.com/1orz/proxy-speedtest"
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            LiteSpeedTest
          </a>{' '}
          <span className="text-muted-foreground">{version}</span>
        </p>
      </footer>
    </div>
  )
}

export default App
