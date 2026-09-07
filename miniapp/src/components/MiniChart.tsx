import { useEffect } from 'react'
import Taro from '@tarojs/taro'
import { Canvas } from '@tarojs/components'

// 轻量 Canvas 图表：环形图 / 水平堆叠柱状图 / 饼图
// 兼容微信小程序（canvas 2d）与 H5（浏览器 canvas）

const COLORS = ['#4F46E5', '#165DFF', '#00B42A', '#F77234', '#14C9C9', '#F53F3F', '#FF7D00', '#86909C']

export interface DonutData {
  value: number // 百分比 0-100
  centerText?: string
}

export interface HbarItem {
  name: string
  value: number // 主段（可见度）
  value2: number // 次段（TOP3）
}

export interface PieItem {
  name: string
  value: number
}

interface Props {
  type: 'donut' | 'hbar' | 'pie'
  donut?: DonutData
  hbar?: HbarItem[]
  pie?: PieItem[]
  height?: number
}

function draw(ctx: any, type: string, props: Props, w: number, h: number) {
  ctx.clearRect(0, 0, w, h)

  if (type === 'donut' && props.donut) {
    drawDonut(ctx, props.donut, w, h)
  } else if (type === 'hbar' && props.hbar) {
    drawHbar(ctx, props.hbar, w, h)
  } else if (type === 'pie' && props.pie) {
    drawPie(ctx, props.pie, w, h)
  }
}

function drawDonut(ctx: any, d: DonutData, w: number, h: number) {
  const cx = w / 2
  const cy = h / 2
  const r = Math.min(w, h) / 2 - 12
  const lw = Math.max(16, r * 0.36)

  // 背景环
  ctx.beginPath()
  ctx.arc(cx, cy, r, 0, Math.PI * 2)
  ctx.strokeStyle = '#EFF0F4'
  ctx.lineWidth = lw
  ctx.stroke()

  // 前景环（按比例）
  const start = -Math.PI / 2
  const end = start + (Math.PI * 2 * Math.min(d.value, 100)) / 100
  if (d.value > 0) {
    ctx.beginPath()
    ctx.arc(cx, cy, r, start, end)
    ctx.strokeStyle = '#4F46E5'
    ctx.lineCap = 'round'
    ctx.lineWidth = lw
    ctx.stroke()
  }

  // 中心文字
  ctx.fillStyle = '#1D2129'
  ctx.font = 'bold 28px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(`${d.value}%`, cx, cy - 6)
  ctx.fillStyle = '#86909C'
  ctx.font = '12px sans-serif'
  ctx.fillText(d.centerText || '', cx, cy + 20)
}

function drawHbar(ctx: any, items: HbarItem[], w: number, h: number) {
  const maxVal = Math.max(...items.map((i) => i.value + i.value2), 1)
  const barH = 18
  const gap = 26
  const labelW = 60
  const chartW = w - labelW - 30

  items.forEach((it, idx) => {
    const y = idx * gap + 8
    // 标签
    ctx.fillStyle = '#4E5969'
    ctx.font = '12px sans-serif'
    ctx.textAlign = 'right'
    ctx.textBaseline = 'middle'
    ctx.fillText(it.name.length > 5 ? it.name.slice(0, 5) : it.name, labelW - 8, y + barH / 2)

    // 主段
    const w1 = (it.value / maxVal) * chartW
    ctx.fillStyle = '#4F46E5'
    ctx.beginPath()
    roundRect(ctx, labelW, y, Math.max(w1, it.value > 0 ? 4 : 0), barH, 4)
    ctx.fill()

    // 次段（堆叠）
    const w2 = (it.value2 / maxVal) * chartW
    if (it.value2 > 0) {
      ctx.fillStyle = '#9A93FF'
      ctx.beginPath()
      roundRect(ctx, labelW + Math.max(w1, it.value > 0 ? 4 : 0), y, Math.max(w2, 2), barH, 4)
      ctx.fill()
    }

    // 数值
    ctx.fillStyle = '#86909C'
    ctx.font = '11px sans-serif'
    ctx.textAlign = 'left'
    ctx.fillText(`${it.value}%`, labelW + w1 + w2 + 6, y + barH / 2)
  })
}

function drawPie(ctx: any, items: PieItem[], w: number, h: number) {
  const cx = w / 2
  const cy = h / 2
  const r = Math.min(w, h) / 2 - 8
  const total = items.reduce((s, i) => s + i.value, 0) || 1

  let start = -Math.PI / 2
  items.forEach((it, idx) => {
    const angle = (it.value / total) * Math.PI * 2
    ctx.beginPath()
    ctx.moveTo(cx, cy)
    ctx.arc(cx, cy, r, start, start + angle)
    ctx.closePath()
    ctx.fillStyle = COLORS[idx % COLORS.length]
    ctx.fill()
    start += angle
  })

  // 中心挖空成环形（可选），这里保留实心饼图
}

function roundRect(ctx: any, x: number, y: number, w: number, h: number, r: number) {
  const rr = Math.min(r, h / 2)
  ctx.moveTo(x + rr, y)
  ctx.lineTo(x + w - rr, y)
  ctx.arcTo(x + w, y, x + w, y + rr, rr)
  ctx.lineTo(x + w, y + h - rr)
  ctx.arcTo(x + w, y + h, x + w - rr, y + h, rr)
  ctx.lineTo(x + rr, y + h)
  ctx.arcTo(x, y + h, x, y + h - rr, rr)
  ctx.lineTo(x, y + rr)
  ctx.arcTo(x, y, x + rr, y, rr)
}

export default function MiniChart({ type, donut, hbar, pie, height = 200 }: Props) {
  const id = `chart-${type}-${Math.random().toString(36).slice(2, 8)}`

  useEffect(() => {
    const paint = (canvas: any, ctx: any, w: number, h: number) => {
      draw(ctx, type, { type, donut, hbar, pie }, w, h)
    }

    if (process.env.TARO_ENV === 'h5') {
      // H5：直接拿 canvas 元素
      const el = document.getElementById(id) as HTMLCanvasElement
      if (el) {
        const ctx = el.getContext('2d')
        if (ctx) paint(el, ctx, el.width, el.height)
      }
    } else {
      // 小程序：canvas 2d 节点
      const query = Taro.createSelectorQuery()
      query
        .select(`#${id}`)
        .fields({ node: true, size: true })
        .exec((res: any) => {
          const info = res && res[0]
          if (!info || !info.node) return
          const canvas = info.node
          const dpr = Taro.getSystemInfoSync().pixelRatio || 2
          canvas.width = info.width * dpr
          canvas.height = info.height * dpr
          const ctx = canvas.getContext('2d')
          ctx.scale(dpr, dpr)
          paint(canvas, ctx, info.width, info.height)
        })
    }
  }, [type, JSON.stringify(donut), JSON.stringify(hbar), JSON.stringify(pie)])

  return (
    <Canvas
      type='2d'
      id={id}
      canvasId={id}
      style={{ width: '100%', height: `${height}px` }}
    />
  )
}
