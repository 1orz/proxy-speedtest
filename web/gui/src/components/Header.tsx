import { motion } from 'motion/react'
import { Zap, Sun, Moon, Languages } from 'lucide-react'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'

export function Header() {
  const t = useI18n()
  const { options, setOptions } = useTestStore()

  const toggleTheme = () =>
    setOptions({ appearance: options.appearance === 'dark' ? 'light' : 'dark' })
  const toggleLang = () =>
    setOptions({ language: options.language === 'cn' ? 'en' : 'cn' })

  return (
    <motion.header
      initial={{ opacity: 0, y: -20 }}
      animate={{ opacity: 1, y: 0 }}
      className="relative z-10 py-6"
    >
      <div className="container mx-auto px-4">
        <div className="flex items-center justify-between">
          <motion.div
            className="flex items-center gap-3"
            whileHover={{ scale: 1.02 }}
          >
            <div className="relative">
              <div className="absolute inset-0 bg-primary/30 blur-xl rounded-full" />
              <div className="relative bg-gradient-to-br from-primary to-accent p-2.5 rounded-xl">
                <Zap className="w-6 h-6 text-white" />
              </div>
            </div>
            <div>
              <h1 className="text-2xl font-bold bg-gradient-to-r from-primary via-foreground to-accent bg-clip-text text-transparent">
                LiteSpeedTest
              </h1>
              <p className="text-xs text-muted-foreground">
                {t('app.subtitle')}
              </p>
            </div>
          </motion.div>

          <div className="flex items-center gap-2">
            <motion.button
              type="button"
              onClick={toggleLang}
              aria-label="language"
              title={options.language === 'cn' ? 'English' : '中文'}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-secondary/50 hover:bg-secondary transition-colors text-sm font-medium"
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
            >
              <Languages className="w-4 h-4" />
              {t('lang.toggle')}
            </motion.button>

            <motion.button
              type="button"
              onClick={toggleTheme}
              aria-label="theme"
              title={options.appearance === 'dark' ? t('theme.toLight') : t('theme.toDark')}
              className="flex items-center justify-center p-2 rounded-lg bg-secondary/50 hover:bg-secondary transition-colors"
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
            >
              {options.appearance === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
            </motion.button>

            <motion.a
              href="https://github.com/1orz/proxy-speedtest"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-secondary/50 hover:bg-secondary transition-colors"
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
            >
              <svg viewBox="0 0 24 24" className="w-5 h-5" fill="currentColor" aria-hidden="true">
                <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
              </svg>
              <span className="text-sm font-medium">GitHub</span>
            </motion.a>
          </div>
        </div>
      </div>
    </motion.header>
  )
}
