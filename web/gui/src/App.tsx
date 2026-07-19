import { Header } from '@/components/Header'
import { TestForm } from '@/components/TestForm'
import { Dashboard } from '@/components/Dashboard'
import { ResultTable } from '@/components/ResultTable'
import { ResultImage } from '@/components/ResultImage'
import { useTimer } from '@/hooks/useTimer'

function App() {
  useTimer()

  return (
    <div className="min-h-screen gradient-bg grid-pattern">
      <Header />
      
      <main className="container mx-auto px-4 pb-12 space-y-6">
        <TestForm />
        <Dashboard />
        <ResultTable />
        <ResultImage />
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
            proxy-speedtest
          </a>
        </p>
      </footer>
    </div>
  )
}

export default App
