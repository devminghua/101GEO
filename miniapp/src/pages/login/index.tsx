import { useState } from 'react'
import Taro from '@tarojs/taro'
import { View, Text, Input, Button } from '@tarojs/components'
import { miniappLogin, miniappBind, setToken, getWxCode } from '../../api'

export default function Login() {
  const [needBind, setNeedBind] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const goHome = () => {
    Taro.switchTab({ url: '/pages/home/index' })
  }

  // 微信登录：code → 后端判断是否已绑定
  const doWxLogin = async () => {
    setLoading(true)
    try {
      const code = await getWxCode()
      const r = await miniappLogin(code)
      if (r.need_bind) {
        setNeedBind(true)
      } else if (r.token) {
        setToken(r.token)
        goHome()
      }
    } catch (e: any) {
      Taro.showToast({ title: e.message || '登录失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  // 绑定分站账号
  const doBind = async () => {
    if (!username.trim() || !password) {
      Taro.showToast({ title: '请输入账号和密码', icon: 'none' })
      return
    }
    setLoading(true)
    try {
      const code = await getWxCode()
      const r = await miniappBind(code, username.trim(), password)
      if (r.token) {
        setToken(r.token)
        Taro.showToast({ title: '绑定成功', icon: 'success' })
        setTimeout(goHome, 600)
      }
    } catch (e: any) {
      Taro.showToast({ title: e.message || '绑定失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <View className='login-page' style={{ padding: '80rpx 48rpx' }}>
      <View style={{ textAlign: 'center', marginBottom: 64 }}>
        <View style={{ fontSize: 64, fontWeight: 800 }}>
          <Text className='brand-text'>Link</Text>
          <Text className='accent'>Geo</Text>
        </View>
        <View style={{ color: '#86909c', fontSize: 26, marginTop: 12 }}>生成式引擎优化平台</View>
      </View>

      {!needBind ? (
        <View className='card' style={{ textAlign: 'center' }}>
          <View style={{ fontSize: 32, fontWeight: 600, marginBottom: 12 }}>微信一键登录</View>
          <View style={{ color: '#86909c', fontSize: 24, marginBottom: 40 }}>
            登录后可随时查看品牌的 GEO 优化效果
          </View>
          <Button className='btn-primary' loading={loading} onClick={doWxLogin}>
            微信登录
          </Button>
        </View>
      ) : (
        <View className='card'>
          <View style={{ fontSize: 32, fontWeight: 600, marginBottom: 8 }}>首次使用，绑定账号</View>
          <View style={{ color: '#86909c', fontSize: 24, marginBottom: 40 }}>
            请输入你的 LinkGeo 分站账号完成绑定
          </View>
          <View style={{ marginBottom: 24 }}>
            <Text style={{ fontSize: 26, color: '#4e5969', marginBottom: 8, display: 'block' }}>账号</Text>
            <Input
              value={username}
              onInput={(e) => setUsername(e.detail.value)}
              placeholder='请输入登录账号'
              style={{ background: '#f5f6fa', borderRadius: 12, padding: '20rpx 24rpx', fontSize: 28 }}
            />
          </View>
          <View style={{ marginBottom: 40 }}>
            <Text style={{ fontSize: 26, color: '#4e5969', marginBottom: 8, display: 'block' }}>密码</Text>
            <Input
              value={password}
              password
              onInput={(e) => setPassword(e.detail.value)}
              placeholder='请输入登录密码'
              style={{ background: '#f5f6fa', borderRadius: 12, padding: '20rpx 24rpx', fontSize: 28 }}
            />
          </View>
          <Button className='btn-primary' loading={loading} onClick={doBind}>
            绑定并登录
          </Button>
          <View style={{ textAlign: 'center', marginTop: 24 }}>
            <Text style={{ color: '#4f46e5', fontSize: 26 }} onClick={() => setNeedBind(false)}>返回微信登录</Text>
          </View>
        </View>
      )}
    </View>
  )
}
