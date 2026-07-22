import { useState } from 'react'
import { motion } from 'motion/react'
import { Image as ImageIcon, Download, RefreshCw } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'

export function ResultImage() {
  const t = useI18n()
  const picdata = useTestStore((s) => s.picdata)
  const hasResult = useTestStore((s) => s.result.length > 0)
  const loading = useTestStore((s) => s.loading)
  const regenerateImage = useTestStore((s) => s.regenerateImage)
  const [busy, setBusy] = useState(false)

  // 测速产生过结果就显示本卡片(即便还没图,也能手动生成)
  if (!hasResult && !picdata) {
    return null
  }

  const handleDownload = () => {
    if (!picdata) return
    const link = document.createElement('a')
    link.href = picdata
    link.download = `speedtest_result_${Date.now()}.png`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const handleRegenerate = async () => {
    setBusy(true)
    const ok = await regenerateImage()
    setBusy(false)
    if (!ok) alert(t('image.failed'))
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.4 }}
    >
      <Card>
        <CardHeader className="border-b border-border/50">
          <div className="flex items-center justify-between gap-3">
            <CardTitle className="flex items-center gap-2">
              <ImageIcon className="w-5 h-5 text-primary" />
              {t('image.title')}
            </CardTitle>
            <div className="flex items-center gap-2">
              <Button onClick={handleRegenerate} variant="outline" size="sm" disabled={busy || loading}>
                <RefreshCw className={`w-4 h-4 mr-2 ${busy ? 'animate-spin' : ''}`} />
                {busy ? t('image.regenerating') : t('image.regenerate')}
              </Button>
              {picdata && (
                <Button onClick={handleDownload} variant="outline" size="sm">
                  <Download className="w-4 h-4 mr-2" />
                  {t('image.download')}
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-4">
          {picdata ? (
            <div className="flex justify-center">
              <img
                src={picdata}
                alt="Speed Test Result"
                className="max-w-full h-auto rounded-lg border border-border shadow-lg"
              />
            </div>
          ) : (
            <p className="text-sm text-muted-foreground text-center py-6">{t('image.hint')}</p>
          )}
        </CardContent>
      </Card>
    </motion.div>
  )
}
