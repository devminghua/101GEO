import { useState, useEffect } from 'react'
import Taro from '@tarojs/taro'
import { View, Text } from '@tarojs/components'
import { geoCompare, getToken } from '../../api'

interface CompareData {
  brand_rate: [number, number]
  first_rate: [number, number]
  before_days: number
  after_days: number
  questions: Array<{ question: string; brand_rate: [number, number] }>
}

export default function Compare() {
  const [data, setData] = useState<CompareData | null>(null)
  const [loading, setLoading] = useState(true)

  const load = async () => {
    if (!getToken()) {
      Taro.switchTab({ url: '/pages/home/index' })
      return
    }
    setLoading(true)
    try {
      setData(await geoCompare())
    } catch (e: any) {
      Taro.showToast({ title: e.message || '加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const delta = (v: [number, number]) => {
    const d = v[1] - v[0]
    return d
  }

  const deltaView = (v: [number, number]) => {
    const d = delta(v)
    if (d > 0) return <Text style={{ color: '#f53f3f', fontWeight: 700 }}>▲ +{d.toFixed(1)}%</Text>
    if (d < 0) return <Text style={{ color: '#00b42a', fontWeight: 700 }}>▼ {d.toFixed(1)}%</Text>
    return <Text style={{ color: '#86909c' }}>— 持平</Text>
  }

  const metricCard = (label: string, v: [number, number]) => (
    <View className='card' style={{ margin: '16rpx 24rpx', padding: '28rpx 32rpx' }}>
      <View style={{ fontSize: 26, color: '#86909c' }}>{label}</View>
      <View style={{ display: 'flex', alignItems: 'baseline', gap: '16rpx', marginTop: 12 }}>
        <View style={{ flex: 1 }}>
          <View style={{ fontSize: 24, color: '#86909c' }}>优化前</View>
          <View style={{ fontSize: 40, fontWeight: 800, color: '#86909c' }}>{v[0].toFixed(1)}%</View>
        </View>
        <Text style={{ fontSize: 36, color: '#c9cdd4' }}>→</Text>
        <View style={{ flex: 1 }}>
          <View style={{ fontSize: 24, color: '#4f46e5' }}>优化后</View>
          <View style={{ fontSize: 40, fontWeight: 800, color: '#4f46e5' }}>{v[1].toFixed(1)}%</View>
        </View>
      </View>
      <View style={{ marginTop: 12 }}>{deltaView(v)}</View>
    </View>
  )

  return (
    <View style={{ padding: '12rpx 12rpx 40rpx' }}>
      <View className='card'>
        <View style={{ fontSize: 30, fontWeight: 700 }}>效果归因</View>
        <View style={{ color: '#86909c', fontSize: 24, marginTop: 8 }}>
          对比优化前后（近 {data?.before_days || 30} 天 vs 近 {data?.after_days || 7} 天）的提及率变化
        </View>
      </View>

      {loading && <View className='card' style={{ textAlign: 'center', color: '#86909c' }}>加载中…</View>}

      {data && (
        <>
          {metricCard('品牌出现率', data.brand_rate)}
          {metricCard('首推率', data.first_rate)}

          {(data.questions || []).length > 0 && (
            <View className='card'>
              <View style={{ fontWeight: 600, fontSize: 28, marginBottom: 20 }}>逐问题变化</View>
              {data.questions.map((q) => (
                <View key={q.question} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14rpx 0', borderBottom: '1px solid #f0f1f5' }}>
                  <Text style={{ fontSize: 26, flex: 1, marginRight: 16 }}>{q.question}</Text>
                  <View style={{ textAlign: 'right' }}>
                    <Text style={{ fontSize: 22, color: '#86909c' }}>{q.brand_rate[0].toFixed(1)}% → </Text>
                    <Text style={{ fontSize: 26, fontWeight: 700, color: '#4f46e5' }}>{q.brand_rate[1].toFixed(1)}%</Text>
                  </View>
                </View>
              ))}
            </View>
          )}
        </>
      )}

      <View className='card' style={{ textAlign: 'center' }}>
        <Text style={{ color: '#4f46e5', fontSize: 26, fontWeight: 600 }} onClick={load}>刷新</Text>
      </View>
    </View>
  )
}
