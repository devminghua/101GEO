// 统一的 API 请求封装（带登录鉴权）
const BASE = '/api';

export interface LoginResult {
  token: string;
  user: {
    id: number;
    username: string;
    nickname: string;
    role: 'super' | 'channel' | 'admin' | 'operator';
    tenant_id: number;
    tenant_name?: string;
    tenant_code?: string;
    brand_name?: string; // 分站自定义品牌名（system_name 分站级）
    tenant_logo?: string; // 分站 Logo
    open_months?: number;
    expire_at?: string | null;
    remain_days?: number; // -1=不限 0=已到期 >0=剩余天数
    features?: string[]; // 分站授权功能 key 列表；空/缺省=全部开放
    avatar?: string; // NFT 数字头像 URL（注册时自动生成，空则用首字母兜底）
  };
}

// 站内信（SaaS 总后台统一推送，客户端接收）
export interface NotificationItem {
  id: number;
  target_type: 'all' | 'tenant' | 'user';
  tenant_id: number;
  user_id: number;
  title: string;
  content: string;
  created_by: number;
  created_at: string;
  read?: boolean; // 仅客户端列表接口返回（总后台列表无此字段）
}

export function getToken(): string | null {
  return localStorage.getItem('geo_token');
}

export function setAuth(data: LoginResult) {
  localStorage.setItem('geo_token', data.token);
  localStorage.setItem('geo_user', JSON.stringify(data.user));
}

export function getStoredUser(): LoginResult['user'] | null {
  const raw = localStorage.getItem('geo_user');
  try {
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function clearAuth() {
  localStorage.removeItem('geo_token');
  localStorage.removeItem('geo_user');
}

async function request<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const res = await fetch(BASE + path, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...options,
  });
  const json = await res.json();
  if (json.code !== 0) {
    if (res.status === 401 || json.code === 401) {
      clearAuth();
      if (!location.hash.includes('/login')) location.hash = '#/login';
    }
    const err: any = new Error(json.msg || '请求失败');
    // 登录安全：锁定信息 / 剩余失败次数透传给调用方（登录页据此弹倒计时窗）
    err.locked = json.locked;
    err.lock_seconds = json.lock_seconds;
    err.lock_level = json.lock_level;
    err.remain_fails = json.remain_fails;
    throw err;
  }
  return json.data as T;
}

export const api = {
  // 鉴权
  login: (body: { username: string; password: string; captcha_id?: string; slide_x?: number; track?: number[] }) =>
    request<LoginResult>('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
  me: () => request('/auth/me'),
  // 滑动解锁（登录前置，无图像纯滑块）
  getCaptcha: () => request<{ id: string }>('/auth/captcha'),

  // 自助注册：发送短信/邮箱验证码 + 提交注册（注册即开通试用并直接登录）
  smsSend: (phone: string) =>
    request<{ phone: string; debug_code?: string }>('/auth/sms-code', { method: 'POST', body: JSON.stringify({ phone }) }),
  emailCode: (email: string) =>
    request<{ email: string; debug_code?: string }>('/auth/email-code', { method: 'POST', body: JSON.stringify({ email }) }),
  register: (body: { username: string; phone: string; email?: string; code: string; password: string; company_name: string; ref?: string; agreed?: boolean }) =>
    request<LoginResult>('/auth/register', { method: 'POST', body: JSON.stringify(body) }),
  // 注册配置（公开）：自助注册开关 + 验证方式（sms/email/off，前端注册页据此动态渲染）+ 网站注册安全协议
  registerConfig: () =>
    request<{
      register_enabled: boolean;
      verify_mode: 'sms' | 'email' | 'off';
      sms_required: boolean;
      agreement_enabled: boolean;
      agreement_title: string;
      agreement_content: string;
      agreement_updated: string;
    }>('/auth/register-config'),

  // 账号有效期（登录页查询，匿名可用）& 客户端修改密码
  authExpiry: (username: string) => request(`/auth/expiry?username=${encodeURIComponent(username)}`),
  changePassword: (old_password: string, new_password: string) =>
    request('/auth/change-password', { method: 'POST', body: JSON.stringify({ old_password, new_password }) }),

  // 仪表盘
  dashboardOverview: () => request('/dashboard/overview'),
  dashboardSummary: () => request('/dashboard/summary'),
  dashboardTrend: (days = 30) => request(`/dashboard/trend?days=${days}`),
  dashboardPlatforms: () => request('/dashboard/platforms'),
  dashboardKeywords: () => request('/dashboard/keywords'),
  platformHealth: () => request('/platforms/health'),

  // 关键词
  listKeywords: () => request('/keywords'),
  getCategories: () => request('/keywords/categories'),
  createKeyword: (body: any) => request('/keywords', { method: 'POST', body: JSON.stringify(body) }),
  bulkCreateKeywords: (body: any) => request('/keywords/bulk', { method: 'POST', body: JSON.stringify(body) }),
  updateKeyword: (id: number, body: any) => request(`/keywords/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteKeyword: (id: number) => request(`/keywords/${id}`, { method: 'DELETE' }),
  batchDeleteKeywords: (ids: number[]) => request('/keywords/batch-delete', { method: 'POST', body: JSON.stringify({ ids }) }),

  // 平台（已收归总后台统一管理，仅 super 可用；分站不再展示/编辑）
  listPlatforms: () => request('/platforms'),
  // Token 用量看板（客户端）
  usageOverview: (days: number = 7) => request(`/usage/overview?days=${days}`),
  platformTemplates: () => request('/platforms/templates'),
  createPlatform: (body: any) => request('/platforms', { method: 'POST', body: JSON.stringify(body) }),
  updatePlatform: (id: number, body: any) => request(`/platforms/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deletePlatform: (id: number) => request(`/platforms/${id}`, { method: 'DELETE' }),
  testPlatform: (id: number) => request(`/platforms/${id}/test`, { method: 'POST', body: '{}' }),

  // 点卡计费（分站）
  getPoints: () => request<{ balance: number; records: any[] }>('/points'),

  // 充值卡密兑换 token（分站，二选一充值方式之一）
  redeemCard: (code: string) =>
    request<{ points: number }>('/card/redeem', { method: 'POST', body: JSON.stringify({ code }) }),

  // ===== 站内信（客户端接收；SaaS 总后台统一推送）=====
  listNotifications: (limit = 20) =>
    request<NotificationItem[]>(`/notifications?limit=${limit}`),
  unreadNotificationCount: () => request<{ unread: number }>('/notifications/unread_count'),
  readNotification: (id: number) => request(`/notifications/${id}/read`, { method: 'POST', body: '{}' }),
  readAllNotifications: () => request('/notifications/read_all', { method: 'POST', body: '{}' }),
  // 总后台：推送站内信 / 查看推送记录
  pushNotification: (body: { target_type: 'all' | 'tenant' | 'user'; target_id?: number; title: string; content: string }) =>
    request<{ id: number }>('/super/notifications', { method: 'POST', body: JSON.stringify(body) }),
  listSuperNotifications: () => request<NotificationItem[]>('/super/notifications'),

  // 点卡自助扫码充值（微信/支付宝）：下单 + 轮询状态
  rechargeCreate: (body: { channel: 'wechat' | 'alipay'; points: number }) =>
    request<{ order_no: string; channel: string; code_url: string; amount_fen: number; points: number; expire_time: string }>(
      '/pay/recharge', { method: 'POST', body: JSON.stringify(body) }),
  rechargeStatus: (orderNo: string) =>
    request<{ order_no: string; status: number; channel: string; amount_fen: number; points: number; expire_time: string; pay_time?: string | null }>(
      `/pay/recharge/${encodeURIComponent(orderNo)}`),

  // 价格套餐：客户端查套餐 + 选套餐下单
  listRechargePlans: () =>
    request<{ id: number; name: string; points: number; price_fen: number; orig_fen: number; tag: string; enabled: boolean; sort: number }[]>('/recharge/plans'),
  rechargePlanOrder: (id: number, channel: string) =>
    request<{ order_no: string; channel: string; code_url: string; amount_fen: number; points: number; expire_time: string }>(
      `/recharge/plans/${id}/order`, { method: 'POST', body: JSON.stringify({ channel }) }),

  // 总后台：价格套餐管理
  listAllPlans: () =>
    request<{ id: number; name: string; points: number; price_fen: number; orig_fen: number; tag: string; enabled: boolean; sort: number }[]>('/super/plans'),
  createPlan: (body: any) => request('/super/plans', { method: 'POST', body: JSON.stringify(body) }),
  updatePlan: (id: number, body: any) => request(`/super/plans/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deletePlan: (id: number) => request(`/super/plans/${id}`, { method: 'DELETE' }),

  // 总后台支付设置：微信/支付宝点卡扫码充值配置（读写均密钥脱敏）
  payGetConfig: () =>
    request<Record<string, string>>('/super/pay/config'),
  paySaveConfig: (body: Record<string, string>) =>
    request('/super/pay/config', { method: 'POST', body: JSON.stringify(body) }),

  // 任务
  runTask: (mode = 'manual') => request('/tasks/run', { method: 'POST', body: JSON.stringify({ mode }) }),
  listTasks: () => request('/tasks'),
  taskDetail: (id: number) => request(`/tasks/${id}`),

  // 结果 / 报告 / 设置
  generateReport: (days = 7) => request(`/report?days=${days}`),
  reportData: (days = 7) => request(`/report/data?days=${days}`),
  getSettings: () => request('/settings'),
  saveSettings: (body: any) => request('/settings', { method: 'POST', body: JSON.stringify(body) }),

  // 系统信息（名称 / Logo / 版权 / 客服电话与二维码）：读公开，写仅 super/admin
  systemInfo: () => request('/system/info'),
  myBrandInfo: () => request<{ system_name: string; copyright: string }>('/system/my-brand'),
  saveSystemInfo: (body: { system_name?: string; system_logo?: string; copyright?: string; service_phone?: string; service_wechat_qr?: string }) =>
    request('/system/info', { method: 'POST', body: JSON.stringify(body) }),
  uploadSystemLogo: async (file: File): Promise<{ url: string }> => {
    const token = getToken();
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch(`${BASE}/system/upload-logo`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    });
    const json = await res.json();
    if (json.code !== 0) throw new Error(json.msg || '上传失败');
    return json.data;
  },
  uploadImage: async (file: File, type = 'image'): Promise<{ url: string }> => {
    const token = getToken();
    const fd = new FormData();
    fd.append('file', file);
    fd.append('type', type);
    const res = await fetch(`${BASE}/system/upload-image`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    });
    const json = await res.json();
    if (json.code !== 0) throw new Error(json.msg || '上传失败');
    return json.data;
  },

  // GEO 智能中心：品牌事实库 / 竞品 / 风险词 / 引用溯源 / 六项指标 / 缺口 / 行动 / 审计 / 生成器
  listFacts: () => request('/facts'),
  createFact: (body: any) => request('/facts', { method: 'POST', body: JSON.stringify(body) }),
  updateFact: (id: number, body: any) => request(`/facts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteFact: (id: number) => request(`/facts/${id}`, { method: 'DELETE' }),
  listCompetitors: () => request('/competitors'),
  createCompetitor: (body: any) => request('/competitors', { method: 'POST', body: JSON.stringify(body) }),
  updateCompetitor: (id: number, body: any) => request(`/competitors/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteCompetitor: (id: number) => request(`/competitors/${id}`, { method: 'DELETE' }),
  listRiskWords: () => request('/risk-words'),
  createRiskWord: (body: any) => request('/risk-words', { method: 'POST', body: JSON.stringify(body) }),
  deleteRiskWord: (id: number) => request(`/risk-words/${id}`, { method: 'DELETE' }),
  listCitations: (params = '') => request(`/citations?${params}`),
  citationDomains: (days = 30) => request(`/citations/domains?days=${days}`),
  sourceGaps: (days = 30) => request(`/geo/source-gaps?days=${days}`),
  geoChannels: () => request<{ name: string; market: string; weight: string; priority: string; what: string; pace: string }[]>('/geo/channels'),
  geoSetupCheck: () => request<{ ready: boolean; checks: Array<{ key: string; label: string; ok: boolean; hint: string }> }>('/geo/setup-check'),

  // 获客工具（前端本地解析，服务器仅扣费）
  toolStatus: () => request<{ available: boolean; balance: number; min_balance: number; cost: number }>('/tools/status'),
  toolConsume: (tool: string) => request<{ balance: number }>('/tools/consume', { method: 'POST', body: JSON.stringify({ tool }) }),
  parseVideo: (url: string) => request<{ type?: string; video_url: string; images?: string[]; video_id?: string; title?: string; cover?: string; raw_url?: string; hint?: string; balance?: number }>('/tools/parse-video', { method: 'POST', body: JSON.stringify({ url }) }),

  // 邀约奖励
  inviteSummary: () => request<{ invite_code: string; total_invited: number; total_reward: number; reward_per: number; records: Array<{ id: number; phone: string; status: number; reward_points: number; company_name: string; created_at: string }> }>('/invite/summary'),
  // 成长计划
  growthSummary: () => request<{ points: number; level: number; level_name: string; next_need: number; streak: number; today_checked: boolean; checkins: Array<{ day: string; points: number; streak: number }> }>('/growth/summary'),
  growthCheckin: () => request<{ points: number; streak: number }>('/growth/checkin', { method: 'POST', body: '{}' }),
  geoIntel: (days = 7) => request(`/geo/intel?days=${days}`),
  geoGaps: (days = 7) => request(`/geo/gaps?days=${days}`),
  geoCompare: (beforeDays = 30, afterDays = 7) => request(`/geo/compare?before_days=${beforeDays}&after_days=${afterDays}`),
  listActions: (params = '') => request(`/geo/actions?${params}`),
  generateActions: () => request('/geo/actions/generate', { method: 'POST', body: '{}' }),
  updateAction: (id: number, body: any) => request(`/geo/actions/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteAction: (id: number) => request(`/geo/actions/${id}`, { method: 'DELETE' }),
  runAudit: (url: string) => request('/geo/audit', { method: 'POST', body: JSON.stringify({ url }) }),
  listAudits: () => request('/geo/audits'),
  genLLMS: () => request('/geo/llms'),
  genSchema: () => request('/geo/schema'),

  // AI 优化员（operator）：管理仅 super/admin
  listOperators: () => request('/operator/list'),
  createOperator: (body: { tenant_id?: number; username: string; password: string; nickname?: string }) =>
    request('/operator/create', { method: 'POST', body: JSON.stringify(body) }),
  deleteOperator: (id: number) => request(`/operator/${id}`, { method: 'DELETE' }),

  // 登录日志（仅 super/admin）：?limit=&status=success|fail
  listLoginLogs: (params = '') => request(`/auth/login-logs?${params}`),

  // 一键导出报告文档（md / html / docx），通过浏览器下载
  exportReport: async (format: 'md' | 'html' | 'docx', days = 7): Promise<string> => {
    const token = getToken();
    const res = await fetch(`${BASE}/report/export?format=${format}&days=${days}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) {
      let msg = `导出失败（${res.status}）`;
      try {
        const j = await res.json();
        msg = j.msg || msg;
      } catch { /* ignore */ }
      throw new Error(msg);
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    const cd = res.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename="?([^";]+)"?/);
    a.href = url;
    a.download = m ? m[1] : `geo-report-${Date.now()}.${format}`;
    a.click();
    URL.revokeObjectURL(url);
    return a.download;
  },

  // 百度分析（实时抓取 SERP → 同行识别 → 归因 → 建议）
  baiduAnalyze: (body: { keyword: string; depth: number; my_domain?: string; my_domains?: string[] }) =>
    request('/baidu/analyze', { method: 'POST', body: JSON.stringify(body) }),
  baiduSuggest: (body: { keyword: string; source?: string }) =>
    request<{ keyword: string; words: string[]; baidu_count: number; google_count: number }>('/baidu/suggest', { method: 'POST', body: JSON.stringify(body) }),
  siteAudit: (body: { url: string }) =>
    request<{ url: string; host: string; score: number; level: string; layers: any[]; grade_dist: Record<string, number>; overall_note: string }>('/baidu/site-audit', { method: 'POST', body: JSON.stringify(body) }),
  gapDiagnose: () =>
    request<{ content_gap: number; content_questions: string[]; channel_gap: number; channel_list: string[]; fact_gap: number; fact_list: string[] }>('/baidu/gap-diagnose'),

  // 百度指数行业排行
  industryRank: (period = 'day', metric = 'brand') =>
    request<{
      period: string; metric: string;
      industries: Array<{ code: string; name: string; en: string; stat_date: string; items: Array<{ rank: number; brand: string; value: number }> }>;
      metrics: Array<{ key: string; label: string }>;
      updated_at: string;
    }>(`/baidu/industry-rank?period=${period}&metric=${metric}`),

  // 百度关键词分析 · 客户网站配置（按租户隔离）
  baiduListSites: () => request('/baidu/sites'),
  baiduCreateSite: (body: { domain: string; name?: string; enabled?: boolean }) =>
    request('/baidu/sites', { method: 'POST', body: JSON.stringify(body) }),
  baiduUpdateSite: (id: number, body: any) =>
    request(`/baidu/sites/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  baiduDeleteSite: (id: number) => request(`/baidu/sites/${id}`, { method: 'DELETE' }),

  // 百度关键词分析 · 监控关键词配置（按租户隔离）
  baiduListMonitorKeywords: (site_id?: number) =>
    request(`/baidu/monitor-keywords${site_id ? `?site_id=${site_id}` : ''}`),
  baiduCreateMonitorKeyword: (body: { keyword: string; site_id?: number; enabled?: boolean }) =>
    request('/baidu/monitor-keywords', { method: 'POST', body: JSON.stringify(body) }),
  baiduUpdateMonitorKeyword: (id: number, body: any) =>
    request(`/baidu/monitor-keywords/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  baiduDeleteMonitorKeyword: (id: number) => request(`/baidu/monitor-keywords/${id}`, { method: 'DELETE' }),

  // 百度关键词分析 · 排名历史查询（keyword + days）
  baiduRankHistory: (keyword: string, days = 30) =>
    request(`/baidu/rank-history?keyword=${encodeURIComponent(keyword)}&days=${days}`),

  /* ===== 抖音获客（半自动） ===== */
  // 1) 账号管理
  douyinListAccounts: () => request('/douyin/accounts'),
  douyinCreateAccount: (body: any) => request('/douyin/accounts', { method: 'POST', body: JSON.stringify(body) }),
  douyinUpdateAccount: (id: number, body: any) => request(`/douyin/accounts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  douyinDeleteAccount: (id: number) => request(`/douyin/accounts/${id}`, { method: 'DELETE' }),
  douyinRefreshAccount: (id: number) => request(`/douyin/accounts/${id}/refresh`, { method: 'POST', body: '{}' }),
  // 2) 同行追踪
  douyinListPeers: () => request('/douyin/peers'),
  douyinImportPeers: (links: string) => request('/douyin/peers/import', { method: 'POST', body: JSON.stringify({ links }) }),
  douyinRefreshAllPeers: () => request('/douyin/peers/refresh-all', { method: 'POST', body: '{}' }),
  douyinRefreshPeer: (id: number) => request(`/douyin/peers/${id}/refresh`, { method: 'POST', body: '{}' }),
  douyinDeletePeer: (id: number) => request(`/douyin/peers/${id}`, { method: 'DELETE' }),
  // 3) 视频数据
  douyinListVideos: (peerId?: number) => request(`/douyin/videos${peerId ? `?peer_id=${peerId}` : ''}`),
  douyinAnalysis: (peerId?: number) => request(`/douyin/analysis${peerId ? `?peer_id=${peerId}` : ''}`),
  // 4) 客户获取
  douyinListLeads: (params = '') => request(`/douyin/leads?${params}`),
  douyinParseLeads: (body: any) => request('/douyin/leads/parse', { method: 'POST', body: JSON.stringify(body) }),
  douyinCreateLead: (body: any) => request('/douyin/leads', { method: 'POST', body: JSON.stringify(body) }),
  douyinUpdateLead: (id: number, body: any) => request(`/douyin/leads/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  douyinDeleteLead: (id: number) => request(`/douyin/leads/${id}`, { method: 'DELETE' }),
  // 5) 话术库
  douyinListSlogans: () => request('/douyin/slogans'),
  douyinCreateSlogan: (body: any) => request('/douyin/slogans', { method: 'POST', body: JSON.stringify(body) }),
  douyinUpdateSlogan: (id: number, body: any) => request(`/douyin/slogans/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  douyinDeleteSlogan: (id: number) => request(`/douyin/slogans/${id}`, { method: 'DELETE' }),
  douyinUseSlogan: (id: number) => request(`/douyin/slogans/${id}/use`, { method: 'POST', body: '{}' }),
  // 6) 频率与安全 + 动作前置校验 + 打招呼（半自动）
  douyinGetSettings: () => request('/douyin/settings'),
  douyinSaveSettings: (body: any) => request('/douyin/settings', { method: 'POST', body: JSON.stringify(body) }),
  douyinPrecheck: (body: any) => request('/douyin/precheck', { method: 'POST', body: JSON.stringify(body) }),
  douyinGreet: (body: any) => request('/douyin/greet', { method: 'POST', body: JSON.stringify(body) }),
  douyinAiGenerate: (body: any) => request('/douyin/slogans/ai-generate', { method: 'POST', body: JSON.stringify(body) }),
  // 7) 动作日志
  douyinListLogs: (params = '') => request(`/douyin/logs?${params}`),

  /* ===== 小红书获客（半自动） ===== */
  // 1) 账号管理
  xhsListAccounts: () => request('/xhs/accounts'),
  xhsCreateAccount: (body: any) => request('/xhs/accounts', { method: 'POST', body: JSON.stringify(body) }),
  xhsUpdateAccount: (id: number, body: any) => request(`/xhs/accounts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  xhsDeleteAccount: (id: number) => request(`/xhs/accounts/${id}`, { method: 'DELETE' }),
  xhsRefreshAccount: (id: number) => request(`/xhs/accounts/${id}/refresh`, { method: 'POST', body: '{}' }),
  // 2) 同行追踪
  xhsListPeers: () => request('/xhs/peers'),
  xhsImportPeers: (links: string) => request('/xhs/peers/import', { method: 'POST', body: JSON.stringify({ links }) }),
  xhsRefreshPeer: (id: number) => request(`/xhs/peers/${id}/refresh`, { method: 'POST', body: '{}' }),
  xhsDeletePeer: (id: number) => request(`/xhs/peers/${id}`, { method: 'DELETE' }),
  // 3) 笔记数据
  xhsListNotes: (peerId?: number) => request(`/xhs/notes${peerId ? `?peer_id=${peerId}` : ''}`),
  xhsAnalysis: (peerId?: number) => request(`/xhs/analysis${peerId ? `?peer_id=${peerId}` : ''}`),
  // 4) 客户获取
  xhsListLeads: (params = '') => request(`/xhs/leads?${params}`),
  xhsParseLeads: (body: any) => request('/xhs/leads/parse', { method: 'POST', body: JSON.stringify(body) }),
  xhsCreateLead: (body: any) => request('/xhs/leads', { method: 'POST', body: JSON.stringify(body) }),
  xhsUpdateLead: (id: number, body: any) => request(`/xhs/leads/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  xhsDeleteLead: (id: number) => request(`/xhs/leads/${id}`, { method: 'DELETE' }),
  // 5) 话术库
  xhsListSlogans: () => request('/xhs/slogans'),
  xhsCreateSlogan: (body: any) => request('/xhs/slogans', { method: 'POST', body: JSON.stringify(body) }),
  xhsUpdateSlogan: (id: number, body: any) => request(`/xhs/slogans/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  xhsDeleteSlogan: (id: number) => request(`/xhs/slogans/${id}`, { method: 'DELETE' }),
  xhsUseSlogan: (id: number) => request(`/xhs/slogans/${id}/use`, { method: 'POST', body: '{}' }),
  // 6) AI 话术生成
  xhsAiGenerate: (body: any) => request('/xhs/slogans/ai-generate', { method: 'POST', body: JSON.stringify(body) }),
  // 7) 频率与安全 + 动作前置校验 + 打招呼（半自动）
  xhsGetSettings: () => request('/xhs/settings'),
  xhsSaveSettings: (body: any) => request('/xhs/settings', { method: 'POST', body: JSON.stringify(body) }),
  xhsPrecheck: (body: any) => request('/xhs/precheck', { method: 'POST', body: JSON.stringify(body) }),
  xhsGreet: (body: any) => request('/xhs/greet', { method: 'POST', body: JSON.stringify(body) }),
  // 8) 价值统计 + 通知管理员
  xhsValueStats: () => request('/xhs/value-stats'),
  xhsListNotifications: (unread = false) => request(`/xhs/notifications${unread ? '?unread=1' : ''}`),
  xhsMarkNotificationRead: (id: number) => request(`/xhs/notifications/${id}/read`, { method: 'POST', body: '{}' }),
  // 9) 动作日志
  xhsListLogs: (params = '') => request(`/xhs/logs?${params}`),

  // ===== 总后台（super）=====
  superOverview: () => request('/super/overview'),
  superOnlineCount: () => request('/super/online-count'),
  // ===== 渠道管理（super 端 + channel 端共用）=====
  listChannels: () => request('/super/channels'),
  createChannel: (body: any) => request('/super/channels', { method: 'POST', body: JSON.stringify(body) }),
  updateChannel: (id: number, body: any) => request(`/super/channels/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  updateChannelStatus: (id: number, status: number) => request(`/super/channels/${id}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),
  channelRecharge: (id: number, body: { amount: number; remark?: string }) =>
    request(`/super/channels/${id}/recharge`, { method: 'POST', body: JSON.stringify(body) }),
  superChannelSimulateLogin: (id: number) =>
    request<LoginResult>(`/super/channels/${id}/simulate-login`, { method: 'POST', body: '{}' }),
  superChannelPlaintext: (id: number) => request(`/super/channels/${id}/plaintext`),
  superChannelResetPassword: (id: number, pwd: string) =>
    request(`/super/channels/${id}/password`, { method: 'PUT', body: JSON.stringify({ password: pwd }) }),
  // 渠道后台：品牌/客服 + 客户管理（范围限定自己渠道）
  channelProfile: () => request('/channel/profile'),
  channelSaveProfile: (body: any) => request('/channel/profile', { method: 'PUT', body: JSON.stringify(body) }),
  channelListCustomers: () => request('/channel/customers'),
  channelCreateCustomer: (body: any) => request('/channel/customers', { method: 'POST', body: JSON.stringify(body) }),
  channelUpdateTenant: (id: number, body: any) => request(`/channel/customers/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  channelDeleteTenant: (id: number) => request(`/channel/customers/${id}`, { method: 'DELETE' }),
  channelSimulateLogin: (tenantId: number) =>
    request<LoginResult>(`/channel/customers/${tenantId}/simulate-login`, { method: 'POST', body: '{}' }),
  channelRechargeTenant: (id: number, body: { amount: number; remark?: string }) =>
    request(`/channel/customers/${id}/recharge`, { method: 'POST', body: JSON.stringify(body) }),
  channelResetPassword: (id: number, pwd: string) =>
    request(`/channel/users/${id}/password`, { method: 'PUT', body: JSON.stringify({ password: pwd }) }),
  channelExtendUser: (id: number, months: number, remark?: string, payMethod?: string) =>
    request(`/channel/users/${id}/extend`, { method: 'POST', body: JSON.stringify({ months, remark, pay_method: payMethod }) }),
  channelUpdateUserStatus: (id: number, status: number) =>
    request(`/channel/users/${id}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),
  channelUserPlaintext: (id: number) => request(`/channel/users/${id}/plaintext`),
  channelDeleteUser: (id: number) => request(`/channel/users/${id}`, { method: 'DELETE' }),
  // 融合客户管理：一个客户=一个分站+一个登录账号
  listCustomers: () => request('/super/customers'),
  createCustomer: (body: any) => request('/super/customers', { method: 'POST', body: JSON.stringify(body) }),
  listTenants: () => request('/super/tenants'),
  createTenant: (body: any) => request('/super/tenants', { method: 'POST', body: JSON.stringify(body) }),
  updateTenant: (id: number, body: any) => request(`/super/tenants/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteTenant: (id: number) => request(`/super/tenants/${id}`, { method: 'DELETE' }),
  simulateLogin: (tenantId: number) =>
    request<LoginResult>(`/super/tenants/${tenantId}/simulate-login`, { method: 'POST', body: '{}' }),
  listUsers: (all = true) => request(`/super/users?all=${all ? 1 : 0}`),
  createUser: (body: any) => request('/super/users', { method: 'POST', body: JSON.stringify(body) }),
  resetUserPassword: (id: number, pwd: string) =>
    request(`/super/users/${id}/password`, { method: 'PUT', body: JSON.stringify({ password: pwd }) }),
  // 续费/延长开通时长（到期预警一键续费；payMethod 收款方式：微信/支付宝/银行转账/现金/赠送）
  extendUser: (id: number, months: number, remark?: string, payMethod?: string) =>
    request(`/super/users/${id}/extend`, { method: 'POST', body: JSON.stringify({ months, remark, pay_method: payMethod }) }),
  // 告警通知：测试推送 / 预览当前告警内容
  notifyTest: () => request('/super/notify/test', { method: 'POST', body: '{}' }),
  notifyPreview: () => request('/super/notify/preview'),

  // 续费流水（对账用）
  listExtendRecords: (params?: { tenant_id?: number; month?: string; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.tenant_id) q.set('tenant_id', String(params.tenant_id));
    if (params?.month) q.set('month', params.month);
    if (params?.limit) q.set('limit', String(params.limit));
    const qs = q.toString();
    return request(`/super/extend-records${qs ? '?' + qs : ''}`);
  },
  updateUserStatus: (id: number, status: number) =>
    request(`/super/users/${id}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),
  deleteUser: (id: number) => request(`/super/users/${id}`, { method: 'DELETE' }),
  userPlaintext: (id: number) => request(`/super/users/${id}/plaintext`),
  // 点卡充值（总后台为分站充值）
  rechargeTenant: (id: number, body: { amount: number; remark?: string }) =>
    request(`/super/tenants/${id}/recharge`, { method: 'POST', body: JSON.stringify(body) }),

  // ===== 智能创作中心（creation）=====
  // 角色设定
  listRoles: () => request('/creation/roles'),
  createRole: (body: any) => request('/creation/roles', { method: 'POST', body: JSON.stringify(body) }),
  updateRole: (id: number, body: any) => request(`/creation/roles/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteRole: (id: number) => request(`/creation/roles/${id}`, { method: 'DELETE' }),
  // AI 助手对话
  listSessions: () => request('/creation/sessions'),
  createSession: (body: any) => request('/creation/sessions', { method: 'POST', body: JSON.stringify(body) }),
  deleteSession: (id: number) => request(`/creation/sessions/${id}`, { method: 'DELETE' }),
  listSessionMessages: (id: number) => request(`/creation/sessions/${id}/messages`),
  chatSend: (body: any) => request('/creation/chat', { method: 'POST', body: JSON.stringify(body) }),
  // 文案写作 / 脚本 / 小红书 / 深度学习 / 洗稿
  writeCopy: (body: any) => request('/creation/write', { method: 'POST', body: JSON.stringify(body) }),
  genScript: (body: any) => request('/creation/script', { method: 'POST', body: JSON.stringify(body) }),
  genXhsCopy: (body: any) => request('/creation/xhs', { method: 'POST', body: JSON.stringify(body) }),
  learnCopy: (body: any) => request('/creation/learn', { method: 'POST', body: JSON.stringify(body) }),
  xiegou: (body: any) => request('/creation/xiegou', { method: 'POST', body: JSON.stringify(body) }),
  // 图片生成
  genImage: (body: any) => request('/creation/image', { method: 'POST', body: JSON.stringify(body) }),
  // 生成记录 / 素材库
  listRecords: (kind = '') => request(`/creation/records${kind ? `?kind=${kind}` : ''}`),
  deleteRecord: (id: number) => request(`/creation/records/${id}`, { method: 'DELETE' }),
  listMaterials: () => request('/creation/materials'),
  saveMaterial: (body: any) => request('/creation/materials', { method: 'POST', body: JSON.stringify(body) }),
  deleteMaterial: (id: number) => request(`/creation/materials/${id}`, { method: 'DELETE' }),

  // ===== 内容投放（content）=====
  // 1) 媒体库
  listContentMedia: (params = '') => request(`/content/media${params ? `?${params}` : ''}`),
  createContentMedia: (body: any) => request('/content/media', { method: 'POST', body: JSON.stringify(body) }),
  updateContentMedia: (id: number, body: any) => request(`/content/media/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteContentMedia: (id: number) => request(`/content/media/${id}`, { method: 'DELETE' }),
  // 2) 软文生成（AI 写软文）
  generateContentArticle: (body: any) => request('/content/articles/generate', { method: 'POST', body: JSON.stringify(body) }),
  generateContentArticlesBatch: (body: any) => request('/content/articles/generate-batch', { method: 'POST', body: JSON.stringify(body) }),
  listContentArticles: (params = '') => request(`/content/articles${params ? `?${params}` : ''}`),
  createContentArticle: (body: any) => request('/content/articles', { method: 'POST', body: JSON.stringify(body) }),
  updateContentArticle: (id: number, body: any) => request(`/content/articles/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteContentArticle: (id: number) => request(`/content/articles/${id}`, { method: 'DELETE' }),
  // 3) 发布任务（自动/半自动）
  createContentTask: (body: any) => request('/content/tasks', { method: 'POST', body: JSON.stringify(body) }),
  createContentTasksBatch: (body: any) => request('/content/tasks/batch', { method: 'POST', body: JSON.stringify(body) }),
  updateContentTask: (id: number, body: any) => request(`/content/tasks/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  listContentTasks: (params = '') => request(`/content/tasks${params ? `?${params}` : ''}`),
  publishContentTask: (id: number) => request(`/content/tasks/${id}/publish`, { method: 'POST', body: '{}' }),
  finishContentTask: (id: number, body: any) => request(`/content/tasks/${id}/finish`, { method: 'POST', body: JSON.stringify(body) }),
  checkContentTask: (id: number) => request(`/content/tasks/${id}/check`, { method: 'POST', body: '{}' }),
  deleteContentTask: (id: number) => request(`/content/tasks/${id}`, { method: 'DELETE' }),
  // 4) 监控记录
  listContentMonitors: (params = '') => request(`/content/monitors${params ? `?${params}` : ''}`),
  // 5) 发稿平台配置
  getContentPublishConfig: () => request('/content/publish-config'),
  saveContentPublishConfig: (body: any) => request('/content/publish-config', { method: 'POST', body: JSON.stringify(body) }),
  // 6) 效果分析
  contentAnalysis: () => request('/content/analysis'),

  // ===== 单机版卡密授权（公开接口，无需登录）=====
  licenseStatus: () =>
    request<{
      license_mode: boolean;
      activated: boolean;
      tier?: number;
      tier_name?: string;
      activated_at?: string;
      expire_at?: string;
      remain_days?: number;
      expired?: boolean;
    }>('/license/status'),
  licenseActivate: (code: string) =>
    request<{ tier_name: string; expire_at: string; super_username: string; super_password: string }>(
      '/license/activate', { method: 'POST', body: JSON.stringify({ code }) }),

  // ===== 总后台卡密管理（super）=====
  licenseKey: () => request<{ public_key: string; has_private: boolean }>('/super/license/key'),
  regenerateLicenseKey: () => request<{ public_key: string }>('/super/license/key', { method: 'POST', body: '{}' }),
  importLicenseKey: (private_key: string) =>
    request<{ public_key: string }>('/super/license/key/import', { method: 'POST', body: JSON.stringify({ private_key }) }),
  generateCards: (body: { tier: number; count: number; points?: number; remark?: string }) =>
    request<{ count: number; tier: number; tier_name: string; points: number; codes: string[] }>(
      '/super/cards/generate', { method: 'POST', body: JSON.stringify(body) }),
  listCards: () =>
    request<{ id: number; tier: number; code: string; points: number; status: number; remark: string; created_at: string }[]>(
      '/super/cards'),

  // 注册验证设置（总后台 super）：验证方式（短信/邮箱/关闭）+ 短信服务商 + SMTP 配置
  smsGetConfig: () =>
    request<{ verify_mode: 'sms' | 'email' | 'off'; sms_required: boolean; provider: string; access_key_id: string; access_key_secret: string; sign_name: string; template_code: string; smtp_host: string; smtp_port: number; smtp_user: string; smtp_pass: string; smtp_from: string; agreement_enabled: boolean; agreement_title: string; agreement_content: string; agreement_updated: string }>(
      '/super/sms/config'),
  smsSaveConfig: (body: { verify_mode?: string; sms_required: boolean; provider: string; access_key_id: string; access_key_secret: string; sign_name: string; template_code: string; smtp_host?: string; smtp_port?: number; smtp_user?: string; smtp_pass?: string; smtp_from?: string; agreement_enabled?: boolean; agreement_title?: string; agreement_content?: string }) =>
    request('/super/sms/config', { method: 'POST', body: JSON.stringify(body) }),

  // 第三方数据 API（Just One API）token 配置（总后台 super）：抖音/小红书稳定数据抓取
  dataApiConfig: () =>
    request<{ enabled: boolean; token_masked: string; base_url: string }>('/super/data-api/config'),
  dataApiSaveConfig: (token: string) =>
    request<{ enabled: boolean; token_masked: string }>('/super/data-api/config', { method: 'POST', body: JSON.stringify({ token }) }),

  // 帮助文档（使用教程）：客户端只读 + SaaS 后台管理
  helpTree: () =>
    request<{ id: number; name: string; docs: { id: number; title: string }[] }[]>('/help/tree'),
  helpDocDetail: (id: number) =>
    request<{ id: number; category_id: number; title: string; content: string }>(`/help/doc/${id}`),
  superHelpCategories: () =>
    request<any[]>('/super/help/categories'),
  superHelpCreateCategory: (name: string) =>
    request('/super/help/categories', { method: 'POST', body: JSON.stringify({ name }) }),
  superHelpUpdateCategory: (id: number, name: string) =>
    request(`/super/help/categories/${id}`, { method: 'PUT', body: JSON.stringify({ name }) }),
  superHelpDeleteCategory: (id: number) =>
    request(`/super/help/categories/${id}`, { method: 'DELETE' }),
  superHelpDocs: (categoryId?: number) =>
    request<{ id: number; category_id: number; title: string; sort_order: number }[]>(
      `/super/help/docs${categoryId ? `?category_id=${categoryId}` : ''}`),
  superHelpDoc: (id: number) =>
    request<{ id: number; category_id: number; title: string; content: string; sort_order: number }>(`/super/help/doc/${id}`),
  superHelpSaveDoc: (body: { id?: number; category_id: number; title: string; content: string }) =>
    request('/super/help/docs', { method: 'POST', body: JSON.stringify(body) }),
  superHelpDeleteDoc: (id: number) =>
    request(`/super/help/docs/${id}`, { method: 'DELETE' }),
};
