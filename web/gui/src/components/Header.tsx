import { motion } from 'motion/react'
import { Zap, Github } from 'lucide-react'

export function Header() {
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
              <h1 className="text-2xl font-bold bg-gradient-to-r from-primary via-white to-accent bg-clip-text text-transparent">
                LiteSpeedTest
              </h1>
              <p className="text-xs text-muted-foreground">
                高性能代理节点测速工具
              </p>
            </div>
          </motion.div>
          
          <motion.a
            href="https://github.com/xxf098/LiteSpeedTest"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-secondary/50 hover:bg-secondary transition-colors"
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
          >
            <Github className="w-5 h-5" />
            <span className="text-sm font-medium">GitHub</span>
          </motion.a>
        </div>
      </div>
    </motion.header>
  )
}

