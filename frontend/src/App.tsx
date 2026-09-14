import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Layout, Menu, Button, Tag, Dropdown, Space, Modal, Form, Input, Message, Card } from '@arco-design/web-react';
import {
  IconDashboard,
  IconApps,
  IconStorage,
  IconUserGroup,
  IconExport,
  IconDown,
  IconSearch,
  IconPublic,
  IconPlayArrow,
  IconBook,
  IconTrophy,
  IconCommon,
  IconBulb,
  IconLock,
  IconNotification,
  IconGift,
  IconTool,
  IconCustomerService,
  IconPhone,
  IconSettings,
  IconSafe,
} from '@arco-design/web-react/icon';
import { Routes, Route, useNavigate, useLocation, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import Dashboard from './pages/Dashboard';
import Keywords from './pages/Keywords';
import Platforms from './pages/Platforms';
import Tasks from './pages/Tasks';
import ContentPublish from './pages/ContentPublish';
import Report from './pages/Report';
import BaiduKeywords from './pages/BaiduKeywords';
import SiteAudit from './pages/SiteAudit';
import GapDiagnose from './pages/GapDiagnose';
import IndustryRank from './pages/IndustryRank';
import RankMonitor from './pages/RankMonitor';
import IndexCount from './pages/IndexCount';
import InvitePage from './pages/InvitePage';
import GrowthPage from './pages/GrowthPage';
import VideoWatermark from './pages/tools/VideoWatermark';
import VideoToText from './pages/tools/VideoToText';
import DouyinGain from './pages/DouyinGain';
import XhsGain from './pages/XhsGain';
import KuaishouGain from './pages/KuaishouGain';
import CreationCenter from './pages/CreationCenter';
import GeoIntel from './pages/GeoIntel';
import Points from './pages/Points';
import SuperNotifications from './pages/super/Notifications';
import Settings from './pages/Settings';
import LicenseActivate from './pages/LicenseActivate';
import SuperOverview from './pages/super/Overview';
import SuperCustomers from './pages/super/Customers';
import SuperChannels from './pages/super/Channels';
import ChannelCustomers from './pages/super/ChannelCustomers';
import ChannelProfile from './pages/super/ChannelProfile';
import Cards from './pages/super/Cards';
import SmsConfig from './pages/super/SmsConfig';
import DataApiConfig from './pages/super/DataApiConfig';
import HelpDocConfig from './pages/super/HelpDocConfig';
import HelpDoc from './pages/HelpDoc';
import Cases from './pages/Cases';
import IntlKeywords from './pages/IntlKeywords';
import CasesAdmin from './pages/super/CasesAdmin';
import Plans from './pages/super/Plans';
import UserMenu from './components/UserMenu';
import LanguageSwitcher from './components/LanguageSwitcher';
import NotificationPopover from './components/NotificationPopover';
import NoticePopup from './components/NoticePopup';
import Notifications from './pages/Notifications';
import AiAssistant from './components/AiAssistant';
import { getStoredUser, clearAuth, api, NotificationItem } from './api';
import { hasFeature } from './features';

const { Sider, Header, Content } = Layout;
const MenuItem = Menu.Item;
const SubMenu = Menu.SubMenu;

// 客户端右下角客服悬浮：电话 + 微信二维码弹窗（未配置电话/二维码时不显示）
function FloatingService({ phone, qr }: { phone?: string; qr?: string }) {
  const [open, setOpen] = useState(false);
  if (!phone && !qr) return null;
  return (
    <div style={{ position: 'fixed', right: 24, bottom: 24, zIndex: 1000, display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 12 }}>
      {open && (
        <Card style={{ width: 240, borderRadius: 16, boxShadow: '0 8px 30px rgba(0,0,0,0.16)' }}>
          <div style={{ fontSize: 15, fontWeight: 600, marginBottom: 12 }}>技术客服</div>
          {phone && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, marginBottom: qr ? 12 : 0, color: 'var(--color-text-1)' }}>
              <IconPhone style={{ color: '#165DFF' }} />
              <span style={{ fontWeight: 600 }}>{phone}</span>
            </div>
          )}
          {qr && (
            <div style={{ textAlign: 'center' }}>
              <img src={qr} alt="微信二维码" style={{ width: 160, height: 160, objectFit: 'contain', borderRadius: 8, border: '1px solid var(--color-border-2)' }} />
              <div style={{ fontSize: 12, color: '#86909C', marginTop: 6 }}>微信扫码联系客服</div>
            </div>
          )}
        </Card>
      )}
      <div
        onClick={() => setOpen(!open)}
        style={{
          width: 52, height: 52, borderRadius: '50%', background: 'linear-gradient(135deg,#4F46E5,#7B61FF)',
          color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 24, cursor: 'pointer', boxShadow: '0 6px 20px rgba(79,70,229,0.4)', userSelect: 'none',
        }}
        title="联系客服"
      >
        <IconCustomerService />
      </div>
    </div>
  );
}

export default function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(false);
  const [user, setUser] = useState(getStoredUser());
  const [sysInfo, setSysInfo] = useState<any>(null);
  // 单机版卡密授权状态：null=加载中；license_mode=true 且未激活/已到期时仅渲染激活页
  const [license, setLicense] = useState<any>(null);
  // 当前分站 Token 余额（仅客户端非 super 展示），侧栏紫蓝卡片用
  const [tokenBalance, setTokenBalance] = useState<number | null>(null);
  // 帮助文档目录树（客户端「使用指南」下拉子菜单用）
  const [helpTree, setHelpTree] = useState<any[]>([]);
  // 修改密码弹窗
  const [pwdForm] = Form.useForm();
  const [pwdVisible, setPwdVisible] = useState(false);
  const [pwdSaving, setPwdSaving] = useState(false);

  // 站内信（顶栏铃铛，客户端）
  const [notifOpen, setNotifOpen] = useState(false);
  const [unread, setUnread] = useState(0);
  const notifRef = useRef<HTMLSpanElement>(null);
  const [notifPos, setNotifPos] = useState({ left: 0, top: 0 });
  const [working, setWorking] = useState(false);

  // 登录后自动弹窗：展示后台最新推送的一条未读站内信（老板需求 2026-09-13）。
  // 仅客户端（非 super，与铃铛轮询口径一致）；同一条消息每次浏览器会话只弹一次。
  const [noticePopup, setNoticePopup] = useState<NotificationItem | null>(null);
  useEffect(() => {
    if (!user || user.role === 'super') return;
    let alive = true;
    (async () => {
      try {
        const list: any = await api.listNotifications(20);
        if (!alive || !Array.isArray(list)) return;
        // 列表按时间倒序，第一个未读即「最新一条未读推送」
        const latest = list.find((n: any) => !n.read);
        if (!latest) return;
        // 同一会话已弹过的不再弹（记录最近 20 条已弹 id）
        const seenKey = 'notice_popup_seen';
        const seenArr = (sessionStorage.getItem(seenKey) || '').split(',').filter(Boolean).map(Number);
        if (seenArr.includes(latest.id)) return;
        sessionStorage.setItem(seenKey, [...seenArr, latest.id].slice(-20).join(','));
        setNoticePopup(latest);
      } catch {
        /* 拉取失败不打扰用户 */
      }
    })();
    return () => { alive = false; };
  }, [user?.id, user?.role]);

  // 关闭弹窗：标记该条已读；goAll=true 时跳转消息中心内页查看全部
  const dismissNotice = async (goAll: boolean) => {
    const n = noticePopup;
    setNoticePopup(null);
    if (!n) {
      if (goAll) navigate('/notifications');
      return;
    }
    if (!n.read) {
      setUnread((u) => Math.max(0, u - 1));
      try {
        await api.readNotification(n.id);
      } catch {
        /* 静默失败：未读数下次轮询会纠正 */
      }
    }
    if (goAll) navigate('/notifications');
  };

  // 站内信未读数轮询（仅客户端非 super）
  useEffect(() => {
    if (!user || user.role === 'super') return;
    let alive = true;
    const poll = async () => {
      try {
        const r: any = await api.unreadNotificationCount();
        if (alive && r && typeof r.unread === 'number') setUnread(r.unread);
      } catch {
        /* 忽略网络偶发失败 */
      }
    };
    poll();
    const timer = setInterval(poll, 60 * 1000);
    return () => { alive = false; clearInterval(timer); };
  }, [user?.id, user?.role]);

  const toggleNotif = () => {
    const r = notifRef.current?.getBoundingClientRect();
    if (r) {
      setNotifPos({ left: Math.max(12, r.right - 320 - 12), top: r.bottom + 8 });
    }
    setNotifOpen(!notifOpen);
  };

  // 工作状态轮询：巡检任务运行中 -> 绿点转圈；空闲 -> 静止绿点（仅客户端）
  useEffect(() => {
    if (!user || user.role === 'super') return;
    let alive = true;
    const poll = async () => {
      try {
        const list: any[] = await api.listTasks();
        if (alive) setWorking(Array.isArray(list) && list.some((t: any) => t.status === 'running'));
      } catch {
        /* 忽略网络偶发失败 */
      }
    };
    poll();
    const timer = setInterval(poll, 10 * 1000);
    return () => { alive = false; clearInterval(timer); };
  }, [user?.id, user?.role]);

  // 读取系统品牌配置（名称 / Logo / 版权），供侧边栏与页脚展示；保存后通过事件刷新
  useEffect(() => {
    const load = async () => {
      try {
        const s: any = await api.systemInfo();
        setSysInfo(s || {});
      } catch {
        /* 品牌信息读取失败时使用默认展示 */
      }
    };
    load();
    window.addEventListener('geo-system-info-updated', load);
    return () => window.removeEventListener('geo-system-info-updated', load);
  }, []);

  // 查询单机版卡密授权状态（公开接口）；容器/未启用授权时返回 activated=true
  useEffect(() => {
    api
      .licenseStatus()
      .then((s: any) => setLicense(s || { license_mode: false, activated: true }))
      .catch(() => setLicense({ license_mode: false, activated: true }));
  }, []);

  // 登录后读取 Token 余额（侧栏紫蓝卡片），并在路由/用户变化时刷新
  useEffect(() => {
    if (!user || user.role === 'super') {
      setTokenBalance(null);
      return;
    }
    let alive = true;
    const loadBalance = async () => {
      try {
        const r: any = await api.getPoints();
        if (alive && r && typeof r.balance === 'number') setTokenBalance(r.balance);
      } catch {
        /* 余额读取失败时保留旧值 */
      }
    };
    loadBalance();
    // 离开/回到点卡中心后立即再读一次，确保充值页提交的余额是最新
    const onVisible = () => {
      if (document.visibilityState === 'visible') loadBalance();
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      alive = false;
      document.removeEventListener('visibilitychange', onVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id, user?.role, location.pathname]);

  // 帮助文档目录树（客户端「使用指南」下拉子菜单）：登录时加载 + 每 60 秒轮询刷新，
  // 保证 SaaS 后台修改帮助文档后，已登录的客户端 1 分钟内自动同步
  useEffect(() => {
    if (!user || user.role === 'super') { setHelpTree([]); return; }
    const load = () => api.helpTree().then((t) => setHelpTree(t || [])).catch(() => {});
    load();
    const timer = setInterval(load, 60 * 1000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id, user?.role]);

  // 登录后每 10 分钟刷新一次账号信息（同步服务有效期的剩余天数）
  useEffect(() => {
    if (!user) return;
    let alive = true;
    const refresh = async () => {
      try {
        const u: any = await api.me();
        if (u && alive) {
          setUser((prev: any) => (prev ? { ...prev, ...u } : prev));
          const stored = getStoredUser();
          if (stored) localStorage.setItem('geo_user', JSON.stringify({ ...stored, ...u }));
        }
      } catch {
        /* 忽略网络偶发失败 */
      }
    };
    refresh();
    const timer = setInterval(refresh, 10 * 60 * 1000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, []);

  // 卡密授权状态加载中：显示 loading，避免登录页闪跳
  if (license === null) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#86909c',
          fontSize: 14,
        }}
      >
        正在加载…
      </div>
    );
  }

  // 单机版未激活 / 已到期：仅渲染卡密激活页（软件级锁定）
  if (license.license_mode && (!license.activated || license.expired)) {
    return <LicenseActivate status={license} />;
  }

  if (!user) {
    // 未登录：登录页 / 注册页（自助注册短信验证）
    return (
      <Routes>
        <Route path="/register" element={<Register onSuccess={() => setUser(getStoredUser())} />} />
        <Route path="*" element={<Login onSuccess={() => setUser(getStoredUser())} />} />
      </Routes>
    );
  }

  const isSuper = user.role === 'super';
  const isChannel = user.role === 'channel';
  const isOperator = user.role === 'operator';
  const sysName = (sysInfo && sysInfo.system_name) || '';
  const sysLogo = (sysInfo && sysInfo.system_logo) || '';
  const copyright = (sysInfo && sysInfo.copyright) || '';
  // 产品版本号（后端 /api/system/info 下发，每次更新记一次版本号）
  const version = (sysInfo && sysInfo.version) || '';
  // 客户端顶栏品牌名：分站自定义（brand_name）优先，空则默认 LinkGeo
  const brandName = (user && user.brand_name) || 'LinkGeo';

  const menus = isChannel
    ? [
        // 渠道后台：管理自己渠道下的分站（客户管理全能力）+ 品牌/客服设置
        { key: '/channel/customers', label: t('menu.channelCustomers'), icon: <IconApps /> },
        { key: '/channel/profile', label: t('menu.channelProfile'), icon: <IconSettings /> },
        { key: '/settings', label: t('menu.accountSecurity'), icon: <IconSafe /> },
      ]
    : isSuper
    ? [
        { key: '/super/overview', label: t('menu.superOverview'), icon: <IconStorage /> },
        // 分站（客户）与账号管理融合为一个入口
        { key: '/super/customers', label: t('menu.channelCustomers'), icon: <IconApps /> },
        // 渠道管理：渠道商自建分站 + 品牌/客服
        { key: '/super/channels', label: t('menu.superChannels'), icon: <IconUserGroup /> },
        // AI 平台已收归总后台统一管理（tenant_id=0 全局平台），仅 super 可配置
        { key: '/super/platforms', label: t('menu.superPlatforms'), icon: <IconCommon /> },
        // 扫码支付（微信/支付宝）配置已并入「系统设置」Tab，不再单独挂菜单
        // 站内信：SaaS 端统一推送，客户端左下角铃铛收取
        { key: '/super/notifications', label: t('menu.superNotifications'), icon: <IconNotification /> },
        // 卡密管理：密钥对 + 批量生成/导出卡密（软件授权）
        { key: '/super/cards', label: t('menu.superCards'), icon: <IconLock /> },
        // 短信设置：注册短信验证开关 + 短信服务商配置
        { key: '/super/sms', label: t('menu.superSms'), icon: <IconNotification /> },
        // 第三方数据 API：抖音/小红书稳定数据抓取 token 配置
        { key: '/super/data-api', label: t('menu.superDataApi'), icon: <IconCommon /> },
        // 帮助文档：客户端「使用指南」内容编辑
        { key: '/super/help-doc', label: t('menu.superHelpDoc'), icon: <IconBook /> },
        // 成功案例：SaaS 端上传，客户端「成功案例」页展示
        { key: '/super/cases', label: t('menu.superCases'), icon: <IconTrophy /> },
        // 价格套餐：充值中心「选择套餐」的套餐配置
        { key: '/super/plans', label: t('menu.superPlans'), icon: <IconGift /> },
        // 系统设置：品牌名称/Logo/版权 + 账号与安全 + 登录日志
        { key: '/settings', label: t('menu.superSettings'), icon: <IconSettings /> },
      ]
    : [
        { key: '/dashboard', label: t('menu.dashboard'), icon: <IconDashboard />, feature: 'dashboard' },
        // GEO 智能中心：GEO 智能 + 关键词监控 + AI 平台 + 巡检任务 + 内容投放 + 生成报告
        // （2026-09-13 老板拍板：五项归纳为 GEO 智能下拉，侧栏瘦身）
        // 父组不挂 feature（常显），各子项保留各自 feature 权限控制
        { key: '/geo-center', label: t('menu.geoCenter'), icon: <IconBulb />, children: [
          { key: '/geo-intel', label: t('menu.geoIntel'), feature: 'geo_intel' },
          { key: '/platforms', label: t('menu.superPlatforms'), feature: 'platforms' },
          { key: '/keywords', label: t('menu.keywords'), feature: 'keywords' },
          { key: '/tasks', label: t('menu.tasks'), feature: 'tasks' },
          { key: '/content', label: t('menu.content'), feature: 'content' },
          { key: '/report', label: t('menu.report'), feature: 'report' },
        ] },
        // 国际搜索优化：Google/Naver（P1：Google 关键词分析，feature 同 baidu）
        { key: '/intl', label: t('menu.intl'), icon: <IconPublic />, feature: 'baidu', children: [
          { key: '/intl-google', label: 'Google', children: [
            { key: '/intl-google-keywords', label: t('menu.intlGoogleKeywords') },
            { key: '/intl-google-rank', label: t('menu.rankMonitor') },
          ]},
        ]},
        { key: '/baidu', label: t('menu.baidu'), icon: <IconSearch />, feature: 'baidu', children: [
          { key: '/baidu-keywords', label: t('menu.baiduKeywords') },
          { key: '/baidu-rank-monitor', label: t('menu.rankMonitor') },
          { key: '/baidu-index-count', label: t('menu.indexCount') },
          { key: '/baidu-audit', label: t('menu.siteAudit') },
          { key: '/baidu-gaps', label: t('menu.gapDiagnose') },
          { key: '/baidu-industry-rank', label: t('menu.industryRank') },
        ] },
        // 短视频获客：抖音 + 快手 + 小红书（2026-09-13 老板拍板收纳为一个栏目）
        { key: '/short-video', label: t('menu.shortVideo'), icon: <IconPlayArrow />, children: [
          { key: '/douyin', label: t('menu.douyin'), feature: 'douyin' },
          { key: '/kuaishou', label: t('menu.kuaishou'), feature: 'kuaishou' },
          { key: '/xhs', label: t('menu.xhs'), feature: 'xhs' },
        ] },
        { key: '/creation', label: t('menu.creation'), icon: <IconCommon />, feature: 'creation' },
        // Token 用量已集成到充值中心页签（v1.0.57），不再独立挂菜单
        // 成功案例：SaaS 端上传，客户端展示（v1.0.61）
        { key: '/cases', label: t('menu.superCases'), icon: <IconTrophy /> },
        // 使用指南：SaaS 后台编辑的帮助文档（图文 + B 站视频），下拉「分类 → 文档」
        { key: '/help', label: t('menu.help'), icon: <IconBook />, children: helpTree.length > 0 ? helpTree.map((c: any) => ({
          key: `/help?cat=${c.id}`, label: c.name,
          children: (c.docs && c.docs.length > 0) ? c.docs.map((d: any) => ({ key: `/help?doc=${d.id}`, label: d.title })) : undefined,
        })) : undefined },
        // 系统中心：固定最后（2026-09-13 老板拍板：系统中心/充值中心/消息中心收纳且置底）
        { key: '/system-center', label: t('menu.systemCenter'), icon: <IconSettings />, feature: 'settings', children: [
          { key: '/settings', label: t('menu.superSettings') },
          { key: '/points', label: t('menu.points') },
          { key: '/notifications', label: t('menu.notifications') },
        ] },
        // 获客工具：暂时隐藏（待第三方解析 API 接入后恢复）
        // { key: '/tools', label: t('menu.tools'), icon: <IconTool />, feature: 'tools', children: [
        //   { key: '/tools/watermark', label: '短视频去水印' },
        //   { key: '/tools/video2text', label: '视频转文案' },
        // ] },
      ].filter((m) => (m as any).feature === undefined || hasFeature(user.features, (m as any).feature));

  // 保留前两段路径，兼容 /tools/watermark、/tools/video2text 这类二级菜单
  const _parts = location.pathname.split('/').filter(Boolean);
  const isBackend = isSuper || isChannel;
  const selectedRaw = '/' + (_parts.length ? _parts.slice(0, 2).join('/') : (isBackend ? 'super' : 'dashboard'));
  let selectedKey = isBackend
    ? menus.find((m) => m.key === location.pathname)?.key || (isChannel ? '/channel/customers' : '/super/overview')
    : selectedRaw;

  const handleMenu = (key: string) => {
    if (key === '/settings') return navigate(key);
    if (isSuper && key === '/dashboard') return navigate('/super/overview');
    if (isChannel && key === '/dashboard') return navigate('/channel/customers');
    navigate(key);
  };

  const logout = () => {
    clearAuth();
    localStorage.removeItem('geo_super_backup');
    setUser(null);
  };

  // 模拟登录进入分站后台后，一键切回总后台
  const backToSuper = () => {
    const raw = localStorage.getItem('geo_super_backup');
    if (raw) {
      try {
        const b = JSON.parse(raw);
        localStorage.setItem('geo_token', b.token);
        localStorage.setItem('geo_user', JSON.stringify(b.user));
      } catch {
        /* 备份数据异常时忽略 */
      }
      localStorage.removeItem('geo_super_backup');
      window.location.reload();
    }
  };

  // 退出登录带确认步骤
  const confirmLogout = () => {
    Modal.confirm({
      title: '退出登录',
      content: '确认退出当前登录账号？',
      okText: '退出',
      cancelText: '取消',
      okButtonProps: { status: 'danger' },
      onOk: () => logout(),
    });
  };

  // 修改当前登录账号密码（弹窗）
  const doChangePwd = async () => {
    const v = await pwdForm.validate();
    setPwdSaving(true);
    try {
      await api.changePassword(v.old_password, v.new_password);
      Message.success('密码已修改，请使用新密码重新登录');
      setPwdVisible(false);
      pwdForm.resetFields();
      clearAuth();
      setUser(null);
      location.hash = '#/login';
    } catch (e: any) {
      Message.error(e.message || '修改密码失败');
    } finally {
      setPwdSaving(false);
    }
  };

  // 右上角退出登录下拉菜单：修改密码 / 退出登录
  const userDroplist = (
    <Menu
      onClickMenuItem={(key) => {
        if (key === 'pwd') {
          pwdForm.resetFields();
          setPwdVisible(true);
        } else if (key === 'back-super') {
          backToSuper();
        } else if (key === 'logout') {
          confirmLogout();
        }
      }}
    >
      {!!localStorage.getItem('geo_super_backup') && (
        <MenuItem key="back-super">↩ 切回总后台</MenuItem>
      )}
      {!isOperator && (
        <MenuItem key="pwd">
          <IconLock style={{ marginRight: 8 }} />
          修改密码
        </MenuItem>
      )}
      <MenuItem key="logout" style={{ color: 'rgb(var(--danger-6))' }}>
        <IconExport style={{ marginRight: 8 }} />
        退出登录
      </MenuItem>
    </Menu>
  );

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={220}
        style={{
          background: 'var(--geo-surface)',
          borderRight: '1px solid var(--color-border-2)',
          // 纵向弹性布局：菜单占满剩余空间，底部用户悬浮栏被 margin-top:auto 顶到最底部
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div
          style={{
            height: 64,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            borderBottom: '1px solid var(--color-border-2)',
            overflow: 'hidden',
            padding: collapsed ? '0' : '0 11px',
            flexShrink: 0,
          }}
        >
          {sysLogo ? (
            <img
              src={sysLogo}
              alt="logo"
              style={{
                height: collapsed ? 28 : 36,
                maxWidth: collapsed ? 28 : 180,
                objectFit: 'contain',
                flexShrink: 0,
              }}
            />
          ) : collapsed ? (
            <div
              style={{
                width: 28,
                height: 28,
                borderRadius: 8,
                background: 'linear-gradient(135deg, #4F46E5, #7B61FF)',
                color: '#fff',
                fontSize: 13,
                fontWeight: 800,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              Lg
            </div>
          ) : (
            <>
              <div
                style={{
                  fontSize: 30,
                  fontWeight: 800,
                  letterSpacing: -0.5,
                  lineHeight: 1.15,
                  color: 'var(--geo-text)',
                  whiteSpace: 'nowrap',
                }}
              >
                {brandName === 'LinkGeo' ? (
                  <>
                    <span style={{ color: 'var(--geo-text)' }}>Link</span>
                    <span style={{ color: '#4F46E5' }}>Geo</span>
                  </>
                ) : (
                  <span style={{ color: 'var(--geo-text)' }}>{brandName}</span>
                )}
              </div>
              <div
                style={{
                  fontSize: 11,
                  color: '#86909c',
                  letterSpacing: 1.5,
                  marginTop: 2,
                  whiteSpace: 'nowrap',
                }}
              >
                生成式引擎优化平台
              </div>
            </>
          )}
        </div>
        {/* 侧栏 Token 余额卡片：仅展开态 + 客户端展示（渠道后台不显示，渠道余额在品牌设置页展示） */}
        {!collapsed && user && user.role !== 'super' && user.role !== 'channel' && (
          <div
            onClick={() => navigate('/points')}
            style={{
              margin: '12px 11px 4px',
              padding: '18px 18px 14px',
              borderRadius: 16,
              background: 'linear-gradient(135deg, #4F46E5 0%, #7B61FF 100%)',
              color: '#fff',
              cursor: 'pointer',
              boxShadow: '0 6px 18px rgba(79, 70, 229, 0.18)',
              userSelect: 'none',
            }}
            title="前往点卡中心"
          >
            <div style={{ fontSize: 13, fontWeight: 500, opacity: 0.92, letterSpacing: 0.5 }}>
              Token
            </div>
            <div style={{ display: 'flex', alignItems: 'baseline', marginTop: 6, marginBottom: 14, lineHeight: 1 }}>
              {(() => {
                const s = (tokenBalance ?? 0).toLocaleString('en-US');
                // 按金额位数自适应字号，保证完整显示不截断
                const fs = s.length <= 4 ? 32 : s.length <= 6 ? 26 : s.length <= 8 ? 21 : s.length <= 10 ? 18 : 15;
                return (
                  <span style={{ fontSize: fs, fontWeight: 800, letterSpacing: -0.8, whiteSpace: 'nowrap' }}>
                    {s}
                  </span>
                );
              })()}
            </div>
            <div
              onClick={(e) => {
                e.stopPropagation();
                navigate('/points');
              }}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                height: 36,
                borderRadius: 10,
                background: 'rgba(255, 255, 255, 0.18)',
                fontSize: 14,
                fontWeight: 500,
                transition: 'background 0.15s',
              }}
              onMouseEnter={(e) => ((e.currentTarget as HTMLDivElement).style.background = 'rgba(255, 255, 255, 0.28)')}
              onMouseLeave={(e) => ((e.currentTarget as HTMLDivElement).style.background = 'rgba(255, 255, 255, 0.18)')}
            >
              <span style={{ fontSize: 18, lineHeight: 1, marginTop: -1 }}>⊕</span>
              <span>兑换</span>
            </div>
          </div>
        )}
        <Menu
          selectedKeys={[selectedKey]}
          onClickMenuItem={handleMenu}
          style={{ border: 'none', flex: 1, overflowY: 'auto', minHeight: 0 }}
        >
          {isSuper && (
            <MenuItem key="super-group" disabled style={{ color: 'var(--color-text-3)', fontSize: 12, padding: '10px 16px 4px' }}>
              总管理后台
            </MenuItem>
          )}
          {isChannel && (
            <MenuItem key="channel-group" disabled style={{ color: 'var(--color-text-3)', fontSize: 12, padding: '10px 16px 4px' }}>
              渠道管理后台
            </MenuItem>
          )}
          {menus.map((m: any) => m.children ? (
            /* 子菜单先按 feature 过滤；全部无权限时整个父组隐藏（避免空下拉） */
            (() => {
              const visible = m.children.filter((c: any) => c.feature === undefined || hasFeature(user.features, c.feature));
              if (visible.length === 0) return null;
              return (
                <SubMenu key={m.key} title={<span>{m.icon} {m.label}</span>}>
                  {visible.map((c: any) => c.children ? (
                    <SubMenu key={c.key} title={<span className="geo-menu-icon-ph" aria-hidden />}>
                      {c.children.map((s: any) => (
                        <MenuItem key={s.key}><span className="geo-menu-icon-ph" aria-hidden />{s.label}</MenuItem>
                      ))}
                    </SubMenu>
                  ) : (
                    /* 子菜单文字与父级栏目文字对齐：父级有图标，子项补同宽占位（v1.0.59） */
                    <MenuItem key={c.key}><span className="geo-menu-icon-ph" aria-hidden />{c.label}</MenuItem>
                  ))}
                </SubMenu>
              );
            })()
          ) : (
            <MenuItem key={m.key}>
              {m.icon} {m.label}
            </MenuItem>
          ))}
        </Menu>
        {version && (
          <div
            style={{
              padding: '6px 16px 10px',
              fontSize: 11,
              color: 'var(--color-text-4)',
              borderTop: '1px solid var(--color-border-2)',
            }}
          >
            v{version}
          </div>
        )}
      </Sider>
      <Layout>
        <Header
          style={{
            height: 64,
            background: 'var(--geo-surface)',
            borderBottom: '1px solid var(--color-border-2)',
            display: 'flex',
            alignItems: 'center',
            padding: '0 24px',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ fontSize: 16, fontWeight: 600, color: 'var(--geo-text)' }}>
            {isSuper
              ? 'Super · ' + (menuTitle(selectedKey, t) === 'LinkGeo' ? t('menu.superOverview') : menuTitle(selectedKey, t))
              : isChannel
              ? 'Channel · ' + (menuTitle(selectedKey, t) === 'LinkGeo' ? t('menu.channelCustomers') : menuTitle(selectedKey, t))
              : menuTitle(selectedKey, t)}
          </div>
          <Space>
            {/* 语言切换：中文 / English / 한국어 / 日本語 */}
            <LanguageSwitcher />
            {isSuper || isChannel ? (
              <>
                {isSuper && <Tag color="gold" size="small">{t('topbar.superAdmin')}</Tag>}
                {isChannel && <Tag color="purple" size="small">{t('topbar.channel')}</Tag>}
                <Dropdown droplist={userDroplist} trigger="click" position="br">
                  <Button size="small" icon={<IconExport />}>
                    {t('topbar.logout')} <IconDown />
                  </Button>
                </Dropdown>
              </>
            ) : (
              <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, fontWeight: 600, color: 'var(--geo-text)' }}>
                <span className={working ? 'geo-working-dot' : 'geo-working-idle'} />
                <span>{t('topbar.workingStatus')}</span>
                <span
                  ref={notifRef}
                  onClick={toggleNotif}
                  className={unread > 0 ? 'geo-bell geo-bell-shake' : 'geo-bell'}
                  title={unread > 0 ? `${t('topbar.notifications')} · ${unread} ${t('topbar.unread')}` : t('topbar.notifications')}
                  style={{
                    marginLeft: 3, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    width: 28, height: 28, borderRadius: 8, color: unread > 0 ? '#165DFF' : '#C9CDD4', transition: 'color .15s, background .15s',
                  }}
                >
                  <IconNotification style={{ fontSize: 18 }} />
                </span>
                {/* 右上角用户菜单：纯文字 + 点击下拉 */}
                <UserMenu
                  user={user}
                  onLogout={logout}
                  onChangePassword={() => {
                    pwdForm.resetFields();
                    setPwdVisible(true);
                  }}
                />
              </span>
            )}
            <Modal
              title="修改密码"
              visible={pwdVisible}
              onCancel={() => setPwdVisible(false)}
              onOk={doChangePwd}
              confirmLoading={pwdSaving}
              okText="确认修改"
              cancelText="取消"
              unmountOnExit
            >
              <Form form={pwdForm} layout="vertical" style={{ marginTop: 8 }}>
                <Form.Item
                  label="原密码"
                  field="old_password"
                  rules={[{ required: true, message: '请输入原密码' }]}
                >
                  <Input.Password prefix={<IconLock />} placeholder="请输入原密码" />
                </Form.Item>
                <Form.Item
                  label="新密码"
                  field="new_password"
                  rules={[
                    { required: true, message: '请输入新密码' },
                    { minLength: 8, message: '密码至少 8 位' },
                    { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' },
                  ]}
                >
                  <Input.Password prefix={<IconLock />} placeholder="至少 8 位，含字母和数字" />
                </Form.Item>
                <Form.Item
                  label="确认新密码"
                  field="confirm_password"
                  rules={[
                    { required: true, message: '请再次输入新密码' },
                    {
                      validator: (value, callback) => {
                        if (value && value !== pwdForm.getFieldValue('new_password')) {
                          callback('两次输入的密码不一致');
                        } else {
                          callback();
                        }
                      },
                    },
                  ]}
                >
                  <Input.Password prefix={<IconLock />} placeholder="再次输入新密码" />
                </Form.Item>
              </Form>
            </Modal>
          </Space>
        </Header>
        <Content style={{ padding: 20, background: 'var(--color-fill-2)' }}>
          <Routes>
            {isSuper ? (
              <>
                <Route path="/super/overview" element={<SuperOverview />} />
                {/* 分站+账号融合页 */}
                <Route path="/super/customers" element={<SuperCustomers />} />
                {/* 兼容旧入口：分站管理 / 账号管理 重定向到融合页 */}
                <Route path="/super/tenants" element={<Navigate to="/super/customers" replace />} />
                <Route path="/super/users" element={<Navigate to="/super/customers" replace />} />
                {/* 渠道管理：渠道商自建分站 + 品牌/客服 */}
                <Route path="/super/channels" element={<SuperChannels />} />
                {/* AI 平台统一由总后台管理（tenant_id=0 全局平台） */}
                <Route path="/super/platforms" element={<Platforms />} />
                {/* 扫码支付配置已并入「系统设置」Tab；旧地址保留兼容，重定向过去 */}
                <Route path="/super/pay" element={<Navigate to="/settings" replace />} />
                <Route path="/super/notifications" element={<SuperNotifications />} />
                <Route path="/super/cards" element={<Cards />} />
                <Route path="/super/sms" element={<SmsConfig />} />
                <Route path="/super/data-api" element={<DataApiConfig />} />
                <Route path="/super/help-doc" element={<HelpDocConfig />} />
                <Route path="/super/cases" element={<CasesAdmin />} />
                <Route path="/super/plans" element={<Plans />} />
                {/* 系统设置：品牌/版权/客服 + 短信 + OSS + 登录日志（super 复用 Settings 页） */}
                <Route path="/settings" element={<Settings />} />
                <Route path="*" element={<Navigate to="/super/overview" replace />} />
              </>
            ) : isChannel ? (
              <>
                {/* 渠道后台：客户管理（限定自己渠道）+ 品牌/客服设置 + 账号安全 */}
                <Route path="/channel/customers" element={<ChannelCustomers />} />
                <Route path="/channel/profile" element={<ChannelProfile />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="*" element={<Navigate to="/channel/customers" replace />} />
              </>
            ) : (
              <>
                <Route path="/" element={<Navigate to="/dashboard" replace />} />
                <Route
                  path="/dashboard"
                  element={hasFeature(user.features, 'dashboard') ? <Dashboard /> : <NoAccess feature="仪表盘" />}
                />
                <Route
                  path="/keywords"
                  element={hasFeature(user.features, 'keywords') ? <Keywords /> : <NoAccess feature="关键词监控" />}
                />
                <Route
                  path="/platforms"
                  element={hasFeature(user.features, 'platforms') ? <Platforms /> : <NoAccess feature="AI 平台" />}
                />
                {/* 点卡中心：查询余额与流水（AI 按次扣点） */}
                <Route path="/points" element={<Points />} />
                {/* 消息中心：站内信全部内容（登录后弹窗的「查看全部消息」落点） */}
                <Route path="/notifications" element={<Notifications />} />
                {/* Token 用量已集成到充值中心页签：旧链接重定向兼容 */}
                <Route path="/usage" element={<Navigate to="/points?tab=usage" replace />} />
                <Route
                  path="/tasks"
                  element={hasFeature(user.features, 'tasks') ? <Tasks /> : <NoAccess feature="巡检任务" />}
                />
                <Route
                  path="/content"
                  element={hasFeature(user.features, 'content') ? <ContentPublish /> : <NoAccess feature="内容投放" />}
                />
                <Route
                  path="/report"
                  element={hasFeature(user.features, 'report') ? <Report /> : <NoAccess feature="生成报告" />}
                />
                <Route
                  path="/baidu-keywords"
                  element={hasFeature(user.features, 'baidu') ? <BaiduKeywords /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/baidu-rank-monitor"
                  element={hasFeature(user.features, 'baidu') ? <RankMonitor /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/baidu-index-count"
                  element={hasFeature(user.features, 'baidu') ? <IndexCount /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/baidu-audit"
                  element={hasFeature(user.features, 'baidu') ? <SiteAudit /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/baidu-gaps"
                  element={hasFeature(user.features, 'baidu') ? <GapDiagnose /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/baidu-industry-rank"
                  element={hasFeature(user.features, 'baidu') ? <IndustryRank /> : <NoAccess feature="百度优化" />}
                />
                <Route
                  path="/douyin"
                  element={hasFeature(user.features, 'douyin') ? <DouyinGain /> : <NoAccess feature="抖音获客" />}
                />
                <Route
                  path="/kuaishou"
                  element={hasFeature(user.features, 'kuaishou') ? <KuaishouGain /> : <NoAccess feature="快手获客" />}
                />
                <Route
                  path="/xhs"
                  element={hasFeature(user.features, 'xhs') ? <XhsGain /> : <NoAccess feature="小红书获客" />}
                />
                <Route
                  path="/creation"
                  element={hasFeature(user.features, 'creation') ? <CreationCenter /> : <NoAccess feature="智能创作中心" />}
                />
                <Route path="/help" element={<HelpDoc />} />
                <Route path="/cases" element={<Cases />} />
                <Route path="/intl-google-keywords" element={<IntlKeywords />} />
                <Route path="/intl-google-rank" element={<RankMonitor />} />
                <Route
                  path="/geo-intel"
                  element={hasFeature(user.features, 'geo_intel') ? <GeoIntel /> : <NoAccess feature="智能中心" />}
                />
                <Route
                  path="/settings"
                  element={hasFeature(user.features, 'settings') ? <Settings /> : <NoAccess feature="系统设置" />}
                />
                <Route
                  path="/invite"
                  element={!isSuper ? <InvitePage /> : <Navigate to="/dashboard" replace />}
                />
                <Route
                  path="/growth"
                  element={!isSuper ? <GrowthPage /> : <Navigate to="/dashboard" replace />}
                />
                <Route
                  path="/tools/watermark"
                  element={!isSuper && hasFeature(user.features, 'tools') ? <VideoWatermark /> : <NoAccess feature="获客工具" />}
                />
                <Route
                  path="/tools/video2text"
                  element={!isSuper && hasFeature(user.features, 'tools') ? <VideoToText /> : <NoAccess feature="获客工具" />}
                />
                <Route path="*" element={<Navigate to="/dashboard" replace />} />
              </>
            )}
          </Routes>
        </Content>
        {copyright && (
          <div
            style={{
              textAlign: 'center',
              padding: '10px 16px',
              fontSize: 12,
              color: 'var(--color-text-3)',
              borderTop: '1px solid var(--color-border-2)',
              background: 'var(--color-bg-2)',
            }}
          >
            {copyright}
          </div>
        )}
      </Layout>
      <NotificationPopover
        open={notifOpen}
        left={notifPos.left}
        top={notifPos.top}
        onClose={() => setNotifOpen(false)}
        onUnreadChange={(n) => setUnread(n)}
      />
      {/* 登录后自动弹窗：后台最新推送的站内信（客户端） */}
      <NoticePopup notice={noticePopup} onClose={(markRead) => dismissNotice(false)} onViewAll={() => dismissNotice(true)} />
      {!isSuper && !isChannel && <FloatingService phone={sysInfo?.service_phone} qr={sysInfo?.service_wechat_qr} />}
      {/* AI 数据分析助手：右侧悬浮（有客服球时上移错开），仅客户端展示 */}
      {!isSuper && !isChannel && (
        <AiAssistant bottomOffset={sysInfo?.service_phone || sysInfo?.service_wechat_qr ? 92 : 24} />
      )}
    </Layout>
  );
}

// 未授权功能占位页：客户端直接访问未授权路由时展示
function NoAccess({ feature }: { feature: string }) {
  return (
    <Card style={{ borderRadius: 12 }}>
      <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-3)' }}>
        <div style={{ fontSize: 28, marginBottom: 12 }}>🔒</div>
        <div style={{ fontSize: 16, marginBottom: 8 }}>「{feature}」功能未授权</div>
        <div>该功能未在当前套餐中开放，请联系总后台开通</div>
      </div>
    </Card>
  );
}

function menuTitle(key: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    '/dashboard': 'menu.dashboard',
    '/points': 'menu.points',
    '/notifications': 'menu.notifications',
    '/keywords': 'menu.keywords',
    '/platforms': 'menu.platforms',
    '/super/platforms': 'menu.superPlatforms',
    '/super/notifications': 'menu.superNotifications',
    '/super/cards': 'menu.superCards',
    '/super/sms': 'menu.superSms',
    '/super/channels': 'menu.superChannels',
    '/channel/customers': 'menu.channelCustomers',
    '/channel/profile': 'menu.channelProfile',
    '/super/data-api': 'menu.superDataApi',
    '/super/help-doc': 'menu.superHelpDoc',
    '/help': 'menu.help',
    '/super/cases': 'menu.superCases',
    '/cases': 'menu.cases',
    '/super/plans': 'menu.superPlans',
    '/super/customers': 'menu.superCustomers',
    '/super/tenants': 'menu.superCustomers',
    '/super/users': 'menu.superUsers',
    '/tasks': 'menu.tasks',
    '/content': 'menu.content',
    '/report': 'menu.report',
    '/baidu-keywords': 'menu.baiduKeywords',
    '/baidu-rank-monitor': 'menu.rankMonitor',
    '/baidu-index-count': 'menu.indexCount',
    '/baidu-audit': 'menu.siteAudit',
    '/baidu-gaps': 'menu.gapDiagnose',
    '/baidu-industry-rank': 'menu.industryRank',
    '/douyin': 'menu.douyin',
    '/kuaishou': 'menu.kuaishou',
    '/xhs': 'menu.xhs',
    '/creation': 'menu.creation',
    '/geo-intel': 'menu.geoIntel',
    '/tools/watermark': 'menu.tools',
    '/tools/video2text': 'menu.tools',
    '/settings': 'menu.settings',
  };
  const k = map[key] || ({ '/intl-google-keywords': 'menu.intlGoogleKeywords' } as Record<string, string>)[key];
  return k ? t(k) : 'LinkGeo';
}
