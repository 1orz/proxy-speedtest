import { useEffect, useRef } from 'react'
import { useTestStore } from '@/store/test-store'

export function useTimer() {
  const { loading, incrementTime } = useTestStore()
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    if (loading) {
      timerRef.current = window.setInterval(() => {
        incrementTime()
      }, 1000)
    } else {
      if (timerRef.current) {
        clearInterval(timerRef.current)
        timerRef.current = null
      }
    }

    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current)
      }
    }
  }, [loading, incrementTime])
}

