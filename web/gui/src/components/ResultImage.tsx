import { motion } from 'motion/react'
import { Image as ImageIcon, Download } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useTestStore } from '@/store/test-store'
import { useI18n } from '@/hooks/useI18n'

export function ResultImage() {
  const t = useI18n()
  const { picdata } = useTestStore()

  if (!picdata) {
    return null
  }

  const handleDownload = () => {
    const link = document.createElement('a')
    link.href = picdata
    link.download = `speedtest_result_${Date.now()}.png`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.4 }}
    >
      <Card>
        <CardHeader className="border-b border-border/50">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <ImageIcon className="w-5 h-5 text-primary" />
              {t('image.title')}
            </CardTitle>
            <Button onClick={handleDownload} variant="outline" size="sm">
              <Download className="w-4 h-4 mr-2" />
              {t('image.download')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="p-4">
          <div className="flex justify-center">
            <img
              src={picdata}
              alt="Speed Test Result"
              className="max-w-full h-auto rounded-lg border border-border shadow-lg"
            />
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

