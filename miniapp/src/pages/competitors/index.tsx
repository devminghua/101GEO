import { useState, useEffect } from 'react'
import Taro from '@tarojs/taro'
import { View, Text } from '@tarojs/components'
import { geoGaps, getToken } from '../../api'

interface Comp {
  name: string
  mentions: number
  first_mentions: number
  questions: string[]
}

export default function Competitors() {
  const [comps, setComps] = useState<Comp[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    if (!getToken()) {
      Taro.switchTab({ url: '/pages/home/index' })
      return
    }
    setLoading(true)
    try {
      const r = await geoGaps()
      setComps(r.competitor_sov || [])
    } catch (e: any) {
      Taro.showToast({ title: e.message || '加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const maxMentions = Math.max(...comps.map((c) => c.mentions), 1)

  return (
    <View style={{ padding: '12rpx 12rpx 40rpx' }}>
      <View className='card'>
        <View style={{ fontSize: 30, fontWeight: 700 }}>竞品声量对标</View>
        <View style={{ color: '#86909c', fontSize: 24, marginTop: 8 }}>
          近 7 天，竞品在 AI 回答中被提及的次数与首推情况
        </View>
      </View>

      {comps.length === 0 ? (
        <View className='card' style={{ textAlign: 'center', color: '#86909c' }}>
          {loading ? '加载中…' : '暂无竞品数据，请先在总后台配置竞品'}
        </View>
      ) : (
        comps.map((c) => (
          <View key={c.name} className='card' style={{ padding: '28rpx 32rpx', margin: '16rpx 24rpx' }}>
            <View style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
              <Text style={{ fontSize: 30, fontWeight: 700 }}>{c.name}</Text>
              <Text style={{ fontSize: 40, fontWeight: 800, color: '#f77234' }}>{c.mentions}<Text style={{ fontSize: 24, color: '#86909c', fontWeight: 400 }}> 次提及</Text></Text>
            </View>
            <View style={{ background: '#f0f1f5', borderRadius: 8, height: 12, marginTop: 16, overflow: 'hidden' }}>
              <View style={{ background: 'linear-gradient(90deg,#f77234,#ff9a5a)', height: '100%', borderRadius: 8, width: `${Math.max(c.mentions / maxMentions * 100, 2)}%` }} />
            </View>
            <View style={{ display: 'flex', justifyContent: 'space-between', marginTop: 16, fontSize: 24, color: '#86909c' }}>
              <Text>首推 {c.first_mentions} 次</Text>
              <Text>覆盖 {c.questions.length} 个问题</Text>
            </View>
          </View>
        ))
      )}

      <View className='card' style={{ textAlign: 'center' }}>
        <Text style={{ color: '#4f46e5', fontSize: 26, fontWeight: 600 }} onClick={load}>{loading ? '加载中…' : '刷新'}</Text>
      </View>
    </View>
  )
}
