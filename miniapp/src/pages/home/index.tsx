import { useState, useEffect } from 'react'
import Taro from '@tarojs/taro'
import { View, Text } from '@tarojs/components'
import { miniappHome, getToken, clearToken } from '../../api'
import MiniChart from '../../components/MiniChart'

interface HomeData {
  brand_rate: number
  top3_rate: number
  cite_rate: number
  comp_sov: number
  brand_sov: number
  trend: number[]
  platforms: Array<{ name: string; visibility: number; top3_rate: number; total: number }>
  unread: number
  total_checks: number
  last_run_at: string
}

const EMPTY: HomeData = {
  brand_rate: 0, top3_rate: 0, cite_rate: 0, comp_sov: 0, brand_sov: 0,
  trend: [], platforms: [], unread: 0, total_checks: 0, last_run_at: '',
}

export default function Home() {
  const [data, setData] = useState<HomeData>(EMPTY)
  const [loading, setLoading] = useState(true)

  const load = async () => {
    if (!getToken()) {
      Taro.reLaunch({ url: '/pages/login/index' })
      return
    }
    setLoading(true)
    try {
      setData(await miniappHome())
    } catch (e: any) {
      if (e.message && e.message.indexOf('未登录') >= 0) {
        clearToken()
        Taro.reLaunch({ url: '/pages/login/index' })
        return
      }
      Taro.showToast({ title: e.message || '加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const platformColor = ['#4F46E5', '#165DFF', '#00B42A', '#F77234', '#14C9C9', '#F53F3F', '#FF7D00']

  return (
    <View style={{ padding: '12rpx 12rpx 40rpx' }}>
      {/* 头部 */}
      <View className='card' style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <View>
          <View style={{ fontSize: 36, fontWeight: 800 }}>
            <Text className='brand-text'>Link</Text>
            <Text className='accent'>Geo</Text>
            <Text style={{ fontSize: 28, fontWeight: 600, color: '#1d2129' }}> 效果总览</Text>
          </View>
          <View style={{ color: '#86909c', fontSize: 24, marginTop: 8 }}>
            {data.last_run_at ? `最近巡检 ${data.last_run_at}` : '尚未巡检'}
          </View>
        </View>
        {data.unread > 0 && (
          <View style={{ background: '#f53f3f', color: '#fff', borderRadius: 16, padding: '4rpx 16rpx', fontSize: 22 }}>
            {data.unread} 未读
          </View>
        )}
      </View>

      {/* 环形图：品牌出现率 + 关键指标 */}
      <View className='card'>
        <View style={{ fontSize: 28, fontWeight: 600, marginBottom: 16 }}>品牌出现率</View>
        <View style={{ display: 'flex', alignItems: 'center' }}>
          <View style={{ flex: 1, height: 320 }}>
            <MiniChart type='donut' donut={{ value: data.brand_rate, centerText: '品牌出现率' }} height={320} />
          </View>
          <View style={{ flex: 1, paddingLeft: 16 }}>
            <View style={{ marginBottom: 24 }}>
              <View style={{ fontSize: 24, color: '#86909c' }}>推荐率 TOP3</View>
              <View style={{ fontSize: 40, fontWeight: 800, color: '#165dff' }}>{data.top3_rate}%</View>
            </View>
            <View style={{ marginBottom: 24 }}>
              <View style={{ fontSize: 24, color: '#86909c' }}>引用率</View>
              <View style={{ fontSize: 40, fontWeight: 800, color: '#00b42a' }}>{data.cite_rate}%</View>
            </View>
            <View>
              <View style={{ fontSize: 24, color: '#86909c' }}>竞品声量</View>
              <View style={{ fontSize: 40, fontWeight: 800, color: '#f77234' }}>{data.comp_sov}%</View>
            </View>
          </View>
        </View>
      </View>

      {/* 水平堆叠柱：各平台可见度 */}
      <View className='card'>
        <View style={{ fontSize: 28, fontWeight: 600, marginBottom: 8 }}>各平台可见度</View>
        <View style={{ color: '#86909c', fontSize: 22, marginBottom: 16 }}>
          深色 = 可见度 · 浅色 = 推荐 TOP3
        </View>
        {data.platforms.length > 0 ? (
          <MiniChart
            type='hbar'
            height={data.platforms.length * 26 + 16}
            hbar={data.platforms.map((p) => ({ name: p.name, value: p.visibility, value2: p.top3_rate }))}
          />
        ) : (
          <View style={{ color: '#86909c', textAlign: 'center', padding: '40rpx 0' }}>暂无平台数据</View>
        )}
      </View>

      {/* 饼图：各平台份额 */}
      <View className='card'>
        <View style={{ fontSize: 28, fontWeight: 600, marginBottom: 16 }}>平台覆盖份额</View>
        {data.platforms.length > 0 ? (
          <>
            <MiniChart
              type='pie'
              height={360}
              pie={data.platforms.map((p) => ({ name: p.name, value: p.visibility }))}
            />
            <View style={{ display: 'flex', flexWrap: 'wrap', marginTop: 16 }}>
              {data.platforms.map((p, i) => (
                <View key={p.name} style={{ display: 'flex', alignItems: 'center', marginRight: 24, marginBottom: 12 }}>
                  <View style={{ width: 16, height: 16, borderRadius: 4, background: platformColor[i % platformColor.length], marginRight: 8 }} />
                  <Text style={{ fontSize: 22, color: '#4e5969' }}>{p.name} {p.visibility}%</Text>
                </View>
              ))}
            </View>
          </>
        ) : (
          <View style={{ color: '#86909c', textAlign: 'center', padding: '40rpx 0' }}>暂无平台数据</View>
        )}
      </View>

      {/* 品牌声量 + 刷新 */}
      <View className='card' style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <View>
          <View style={{ fontSize: 24, color: '#86909c' }}>品牌声量（近 7 天）</View>
          <View style={{ fontSize: 40, fontWeight: 800, color: '#4f46e5' }}>{data.brand_sov}</View>
        </View>
        <Text style={{ color: '#4f46e5', fontSize: 26, fontWeight: 600 }} onClick={load}>{loading ? '加载中…' : '刷新'}</Text>
      </View>
    </View>
  )
}
