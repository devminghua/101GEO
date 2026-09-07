import { useState, useEffect } from 'react'
import Taro from '@tarojs/taro'
import { View, Text } from '@tarojs/components'
import { listMessages, readMessage, readAllMessages, getToken, MsgItem } from '../../api'

function relTime(iso: string): string {
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const diff = Date.now() - t
  const min = 60 * 1000
  const hour = 60 * min
  const day = 24 * hour
  if (diff < min) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / min)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`
  const d = new Date(t)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export default function Messages() {
  const [list, setList] = useState<MsgItem[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    if (!getToken()) {
      Taro.switchTab({ url: '/pages/home/index' })
      return
    }
    setLoading(true)
    try {
      setList(await listMessages())
    } catch (e: any) {
      Taro.showToast({ title: e.message || '加载失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onRead = async (m: MsgItem) => {
    if (m.read) return
    try {
      await readMessage(m.id)
      setList((prev) => prev.map((x) => (x.id === m.id ? { ...x, read: true } : x)))
    } catch (e: any) {
      Taro.showToast({ title: e.message || '操作失败', icon: 'none' })
    }
  }

  const onReadAll = async () => {
    try {
      await readAllMessages()
      setList((prev) => prev.map((x) => ({ ...x, read: true })))
      Taro.showToast({ title: '已全部标记为已读', icon: 'success' })
    } catch (e: any) {
      Taro.showToast({ title: e.message || '操作失败', icon: 'none' })
    }
  }

  return (
    <View style={{ padding: '12rpx 12rpx 40rpx' }}>
      <View className='card' style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <View style={{ fontSize: 30, fontWeight: 700 }}>消息中心</View>
        <Text style={{ color: '#4f46e5', fontSize: 26 }} onClick={onReadAll}>全部已读</Text>
      </View>

      {list.length === 0 ? (
        <View className='card' style={{ textAlign: 'center', color: '#86909c' }}>
          {loading ? '加载中…' : '暂无消息'}
        </View>
      ) : (
        list.map((m) => (
          <View key={m.id} className='card' style={{ margin: '16rpx 24rpx', padding: '28rpx 32rpx', opacity: m.read ? 0.6 : 1 }} onClick={() => onRead(m)}>
            <View style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <View style={{ display: 'flex', alignItems: 'center', gap: '12rpx' }}>
                {!m.read && <View style={{ width: 14, height: 14, borderRadius: '50%', background: '#f53f3f' }} />}
                <Text style={{ fontSize: 28, fontWeight: m.read ? 400 : 700 }}>{m.title}</Text>
              </View>
              <Text style={{ fontSize: 22, color: '#86909c' }}>{relTime(m.created_at)}</Text>
            </View>
            <View style={{ fontSize: 26, color: '#4e5969', marginTop: 12, lineHeight: 1.6 }}>{m.content}</View>
          </View>
        ))
      )}

      <View className='card' style={{ textAlign: 'center' }}>
        <Text style={{ color: '#4f46e5', fontSize: 26, fontWeight: 600 }} onClick={load}>刷新</Text>
      </View>
    </View>
  )
}
