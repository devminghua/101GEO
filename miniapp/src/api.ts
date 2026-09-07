import Taro from '@tarojs/taro'

// 后端地址：本地开发用 http://127.0.0.1:8080（微信开发者工具需关闭「校验合法域名」）
// 上线前改为 https://你的域名
export const BASE_URL = 'http://127.0.0.1:8080'

const TOKEN_KEY = 'linkgeo_miniapp_token'

export function getToken(): string {
  return Taro.getStorageSync(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  Taro.setStorageSync(TOKEN_KEY, token)
}

export function clearToken() {
  Taro.removeStorageSync(TOKEN_KEY)
}

// 获取微信登录 code：小程序环境走 wx.login；H5（浏览器演示）返回固定测试 code
export async function getWxCode(): Promise<string> {
  if (process.env.TARO_ENV === 'h5') {
    return 'h5-demo'
  }
  const { code } = await Taro.login()
  return code
}

// 通用请求封装：自动带 token，统一处理 code/msg
export async function request<T = any>(path: string, options: { method?: 'GET' | 'POST'; data?: any; auth?: boolean } = {}): Promise<T> {
  const { method = 'GET', data, auth = true } = options
  const header: any = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (auth && token) header.Authorization = 'Bearer ' + token

  const res = await Taro.request({
    url: BASE_URL + path,
    method,
    data,
    header,
  })

  const body = res.data as any
  if (body && body.code === 0) {
    return body.data as T
  }
  const msg = (body && body.msg) || '请求失败'
  throw new Error(msg)
}

// 微信登录：code → 已绑定返回 token，未绑定返回 need_bind
export async function miniappLogin(code: string) {
  return request<{ token?: string; need_bind?: boolean; open_id?: string }>('/api/miniapp/login', {
    method: 'POST',
    data: { code },
    auth: false,
  })
}

// 绑定分站账号
export async function miniappBind(code: string, username: string, password: string) {
  return request<{ token?: string }>('/api/miniapp/bind', {
    method: 'POST',
    data: { code, username, password },
    auth: false,
  })
}

// 效果总览聚合
export async function miniappHome() {
  return request<{
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
  }>('/api/miniapp/home')
}

// 竞品对标（复用 /geo/gaps）
export async function geoGaps() {
  return request<{
    competitor_sov: Array<{ name: string; mentions: number; first_mentions: number; questions: string[] }>
  }>('/api/geo/gaps?days=7')
}

// 效果归因（复用 /geo/compare）
export async function geoCompare() {
  return request<{
    brand_rate: [number, number]
    first_rate: [number, number]
    before_days: number
    after_days: number
    questions: Array<{ question: string; brand_rate: [number, number] }>
  }>('/api/geo/compare?before_days=30&after_days=7')
}

// 站内信（复用 /notifications）
export interface MsgItem {
  id: number
  title: string
  content: string
  read: boolean
  created_at: string
}

export async function listMessages(): Promise<MsgItem[]> {
  const r = await request<any[]>('/api/notifications')
  return (r || []).map((m: any) => ({
    id: m.id, title: m.title, content: m.content, read: m.read, created_at: m.created_at,
  }))
}

export async function readMessage(id: number) {
  return request(`/api/notifications/${id}/read`, { method: 'POST' })
}

export async function readAllMessages() {
  return request('/api/notifications/read_all', { method: 'POST' })
}
