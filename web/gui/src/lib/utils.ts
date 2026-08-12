import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// bytesToSize 把字节数格式化成人类可读的量级。
//
// 两个下标/取整的坑必须挡住,否则实时表盘会闪出鬼数值 —— 它喂进来的是缓动动画的
// **浮点**中间值,不是整数:
//   1) 0 < bytes < 1 时 log 得负数,Math.floor 后 i = -1,sizes[-1] 是 undefined,
//      于是缓动收敛到 0 的那几帧界面上会出现 "512.0 undefined";
//   2) i 越过数组末尾(≥1PB,或拿到脏数据)同样是 undefined;
//   3) B 档直接插值原始浮点,会打印 "123.45678901234567 B"。
export function bytesToSize(bytes: number): string {
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const exp = Math.floor(Math.log(bytes) / Math.log(1024))
  const i = Math.min(sizes.length - 1, Math.max(0, exp))
  if (i === 0) return `${Math.round(bytes)} ${sizes[i]}`
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`
}

export function formatSeconds(seconds: number): string {
  const totalTime = seconds > 0 ? seconds : 0
  const hours = Math.floor(totalTime / 3600)
  const minutes = Math.floor((totalTime % 3600) / 60)
  const secs = totalTime % 60
  
  let result = `${secs}s`
  if (minutes > 0) result = `${minutes}m ${result}`
  if (hours > 0) result = `${hours}h ${result}`
  return result
}

export function getSpeed(speed: string): number {
  const value = parseFloat(speed.slice(0, -2))
  const unit = speed.slice(-2)
  if (unit === 'MB') return value * 1048576
  if (unit === 'KB') return value * 1024
  return parseFloat(speed.slice(0, -1))
}

export function getSpeedColor(speed: number, theme: 'rainbow' | 'original' = 'rainbow'): string {
  const themes = {
    rainbow: {
      colorgroup: [
        [255, 255, 255],
        [102, 255, 102],
        [255, 255, 102],
        [255, 178, 102],
        [255, 102, 102],
        [226, 140, 255],
        [102, 204, 255],
        [102, 102, 255]
      ],
      bounds: [0, 64 * 1024, 512 * 1024, 4 * 1024 * 1024, 16 * 1024 * 1024, 24 * 1024 * 1024, 32 * 1024 * 1024, 40 * 1024 * 1024]
    },
    original: {
      colorgroup: [
        [255, 255, 255],
        [128, 255, 0],
        [255, 255, 0],
        [255, 128, 192],
        [255, 0, 0]
      ],
      bounds: [0, 64 * 1024, 512 * 1024, 4 * 1024 * 1024, 16 * 1024 * 1024]
    }
  }

  const { colorgroup, bounds } = themes[theme]

  const getColor = (lc: number[], rc: number[], level: number): number[] => {
    return [
      Math.floor(lc[0] * (1 - level) + rc[0] * level),
      Math.floor(lc[1] * (1 - level) + rc[1] * level),
      Math.floor(lc[2] * (1 - level) + rc[2] * level)
    ]
  }

  for (let i = 0; i < bounds.length - 1; i++) {
    if (speed >= bounds[i] && speed <= bounds[i + 1]) {
      const color = getColor(
        colorgroup[i],
        colorgroup[i + 1],
        (speed - bounds[i]) / (bounds[i + 1] - bounds[i])
      )
      return `rgb(${color[0]}, ${color[1]}, ${color[2]})`
    }
  }

  const lastColor = colorgroup[colorgroup.length - 1]
  return `rgb(${lastColor[0]}, ${lastColor[1]}, ${lastColor[2]})`
}

// maskSensitive 按「保留首 2 字符 + 末 1 字符、保留 . 和 : 分隔符、其余每个连续段折叠成单个 *」脱敏。
// 例:123.123.123.123 -> 12*.*.*.*3;hk1.example.com -> hk*.*.*m。串太短(无字符需遮蔽)时原样返回。
export function maskSensitive(s: string): string {
  if (!s) return s
  const len = s.length
  const isDelim = (c: string) => c === '.' || c === ':'
  let out = ''
  let masking = false
  for (let i = 0; i < len; i++) {
    const c = s[i]
    const keep = i === 0 || i === 1 || i === len - 1 || isDelim(c)
    if (keep) {
      out += c
      masking = false
    } else if (!masking) {
      out += '*'
      masking = true
    }
    // 处于遮蔽段中的后续字符已由单个 * 代表,直接跳过
  }
  return out
}

export async function copyToClipboard(text: string): Promise<void> {
  if (navigator.clipboard) {
    await navigator.clipboard.writeText(text)
  } else {
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.left = '-999999px'
    textArea.style.top = '-999999px'
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()
    document.execCommand('copy')
    textArea.remove()
  }
}

export function downloadFile(data: string, filename: string): void {
  const blob = new Blob([data], { type: 'text/plain;charset=utf-8;' })
  const link = document.createElement('a')
  const url = URL.createObjectURL(blob)
  const date = new Date()
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  const jsonDate = date.toJSON().slice(0, 19).replace(/-/g, '').replace(/T/g, '').replace(/:/g, '')
  
  link.setAttribute('href', url)
  link.setAttribute('download', `${filename}_${jsonDate}`)
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

