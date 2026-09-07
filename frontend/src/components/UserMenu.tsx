// 侧栏左下角悬浮用户栏：头像/Logo + 客户名称
//   - 点头像/名称 → 用户菜单（平台色紫蓝渐变，0.5s 推拉动画）
import { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { Message } from '@arco-design/web-react';
import {
  IconUser, IconGift, IconStar, IconSettings,
  IconPalette, IconRefresh, IconExport,
} from '@arco-design/web-react/icon';
import { getThemeMode, setThemeMode, subscribeTheme, type ThemeMode } from '../theme';

interface Props {
  user: any;
  onLogout: () => void;
  collapsed?: boolean;
}

const POPOVER_W = 240; // 用户菜单宽度

export default function UserMenu({ user, onLogout, collapsed = false }: Props) {
  const navigate = useNavigate();
  const isSuper = user?.role === 'super';
  const [open, setOpen] = useState(false);
  const [avatarFailed, setAvatarFailed] = useState(false);
  // 外观（浅色/深色）：初值从 localStorage 读，切换时同步写 DOM 属性 + 持久化
  const [themeMode, setThemeModeState] = useState<ThemeMode>(getThemeMode);
  useEffect(() => subscribeTheme(setThemeModeState), []);
  const changeTheme = (m: ThemeMode) => {
    setThemeModeState(m);
    setThemeMode(m); // 落 body[arco-theme] + localStorage，Arco 组件与 --geo-* 变量随之切换
  };
  const triggerRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; bottom: number }>({ left: 0, bottom: 0 });

  const username: string = user?.username || '';
  // 客户名称：第一行直接显示客户名（SaaS 后台更新名称后此处同步显示），无租户上下文时回落到账号
  const tenantName: string = user?.tenant_name || username || '未登录';
  // 入口展示名优先用客户名称
  const display: string = tenantName;
  const initial = (display || 'U').slice(0, 1).toUpperCase();
  // 头像优先调用客户 Logo（SaaS 后台设置），未设置则回落到账号头像
  const avatar: string = user?.tenant_logo || user?.avatar || '';

  // 头像 URL 变化时重置失败标记（换账号/后台补生成后能重新加载）
  useEffect(() => setAvatarFailed(false), [avatar]);

  // 计算弹层位置：左对齐触发入口，右侧留出安全边距
  const recalc = () => {
    const r = triggerRef.current?.getBoundingClientRect();
    if (!r) return;
    setPos({ left: r.left, bottom: window.innerHeight - r.top + 8 });
  };

  useEffect(() => {
    if (!open) return;
    recalc();
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node;
      if (triggerRef.current?.contains(t) || popoverRef.current?.contains(t)) return;
      setOpen(false);
    };
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
      }
    };
    const onResize = () => recalc();
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onEsc);
    window.addEventListener('resize', onResize);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onEsc);
      window.removeEventListener('resize', onResize);
    };
  }, [open]);

  // 总后台（super）不呼出上拉菜单
  const toggleUser = () => {
    if (isSuper) return;
    setOpen(!open);
  };

  const userLeft = Math.max(12, Math.min(pos.left, window.innerWidth - POPOVER_W - 12));

  return (
    <>
      {/* 悬浮触发栏：头像 + 用户名 + 铃铛 */}
      <div
        ref={triggerRef}
        style={{
          position: 'relative',
          // margin-top:auto 在 flex 列布局中把整条悬浮栏顶到侧栏最底部
          margin: 'auto 10px 12px',
          display: 'flex',
          alignItems: 'center',
          gap: collapsed ? 0 : 10,
          padding: collapsed ? '8px' : '10px 12px',
          justifyContent: collapsed ? 'center' : 'flex-start',
          cursor: 'pointer',
          userSelect: 'none',
          borderRadius: 12,
          background: open ? 'var(--color-fill-2)' : 'var(--color-fill-1)',
          boxShadow: '0 2px 10px rgba(15,23,42,.06)',
          border: '1px solid var(--color-border-1)',
          transition: 'background 0.15s, box-shadow 0.15s',
          flexShrink: 0,
        }}
        onMouseEnter={(e) => (e.currentTarget as HTMLDivElement).style.boxShadow = '0 4px 16px rgba(79,70,229,.16)'}
        onMouseLeave={(e) => (e.currentTarget as HTMLDivElement).style.boxShadow = '0 2px 10px rgba(15,23,42,.06)'}
      >
        {/* 头像区：NFT 数字头像 + 用户名 → 用户菜单 */}
        <div
          onClick={toggleUser}
          title={display}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            flex: 1,
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          {avatar && !avatarFailed ? (
            <img
              src={avatar}
              alt={display}
              onError={() => setAvatarFailed(true)}
              style={{
                width: 32,
                height: 32,
                borderRadius: '50%',
                objectFit: 'cover',
                flexShrink: 0,
                background: 'var(--color-fill-2)',
                boxShadow: '0 2px 8px rgba(79,70,229,.28)',
              }}
            />
          ) : (
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: '50%',
                background: 'linear-gradient(135deg, #4F46E5, #7B61FF)',
                color: '#fff',
                fontSize: 14,
                fontWeight: 800,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
                boxShadow: '0 2px 8px rgba(79,70,229,.3)',
              }}
            >
              {initial}
            </div>
          )}
          {!collapsed && (
            <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
              <div
                style={{
                  fontSize: 14,
                  fontWeight: 600,
                  color: 'var(--color-text-1)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  lineHeight: 1.2,
                }}
              >
                {display}
              </div>
            </div>
          )}
          {!collapsed && (
            <div
              style={{
                fontSize: 9,
                color: 'var(--color-text-3)',
                transform: open ? 'rotate(180deg)' : 'rotate(0deg)',
                transition: 'transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1)',
                flexShrink: 0,
              }}
            >
              ▲
            </div>
          )}
        </div>
      </div>

      {/* 用户菜单弹层 */}
      {createPortal(
        <div
          ref={popoverRef}
          style={{
            position: 'fixed',
            left: userLeft,
            bottom: pos.bottom,
            width: POPOVER_W,
            minWidth: 200,
            // 面板底色走 --geo-surface（深色模式自动变深），描边与文字同理
            background: 'var(--geo-surface)',
            border: '1px solid var(--color-border-2)',
            borderRadius: 14,
            boxShadow: '0 16px 40px rgba(79,70,229,.20), 0 4px 12px rgba(15,23,42,.10)',
            padding: '6px 0',
            color: 'var(--geo-text)',
            zIndex: 1000,
            transformOrigin: 'bottom left',
            transform: open ? 'translateY(0) scaleY(1)' : 'translateY(16px) scaleY(0.6)',
            opacity: open ? 1 : 0,
            transition: 'transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1), opacity 0.4s ease',
            pointerEvents: open ? 'auto' : 'none',
            overflow: 'hidden',
          }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          <Row icon={<IconUser style={{ fontSize: 16 }} />}>
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500 }}>{tenantName}</span>
          </Row>

          <Divider />

          <Row
            icon={<IconGift style={{ fontSize: 16 }} />}
            onClick={() => {
              setOpen(false);
              navigate('/invite');
            }}
          >
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0 }}>邀约奖励</span>
            <span style={{ display: 'inline-flex', alignItems: 'center', flexShrink: 0, color: 'var(--color-text-3)', fontSize: 11, whiteSpace: 'nowrap' }}>
              <span style={{ marginRight: 2 }}>650 积分/人</span>
              <Chevron />
            </span>
          </Row>

          <Row
            icon={<IconStar style={{ fontSize: 16 }} />}
            onClick={() => {
              setOpen(false);
              navigate('/growth');
            }}
          >
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0 }}>成长计划</span>
            <span style={{ display: 'inline-flex', alignItems: 'center', flexShrink: 0, color: 'var(--color-text-3)', fontSize: 11, whiteSpace: 'nowrap' }}>
              <span style={{ marginRight: 2 }}>每日签到</span>
              <Chevron />
            </span>
          </Row>

          <Divider />

          <Row
            icon={<IconSettings style={{ fontSize: 16 }} />}
            onClick={() => {
              setOpen(false);
              navigate('/settings');
            }}
          >
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500 }}>系统设置</span>
          </Row>

          <Row icon={<IconPalette style={{ fontSize: 16 }} />}>
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500 }}>外观</span>
            <div
              onClick={(e) => e.stopPropagation()}
              style={{
                display: 'flex',
                background: 'var(--color-fill-2)',
                borderRadius: 10,
                padding: 3,
                gap: 2,
              }}
            >
              <ToggleBtn active={themeMode === 'light'} onClick={() => changeTheme('light')}>
                浅色
              </ToggleBtn>
              <ToggleBtn active={themeMode === 'dark'} onClick={() => changeTheme('dark')}>
                深色
              </ToggleBtn>
            </div>
          </Row>

          <Row
            icon={<IconRefresh style={{ fontSize: 16 }} />}
            onClick={() => {
              setOpen(false);
              Message.success('已是最新版本');
            }}
          >
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500 }}>检查更新</span>
          </Row>

          <Divider />

          <Row
            icon={<IconExport style={{ fontSize: 16 }} />}
            onClick={() => {
              setOpen(false);
              onLogout();
            }}
            highlight
          >
            <span style={{ flex: 1, fontSize: 14, fontWeight: 500 }}>退出登陆</span>
          </Row>
        </div>,
        document.body,
      )}
    </>
  );
}

function Row({
  children,
  icon,
  onClick,
  highlight = false,
}: {
  children: React.ReactNode;
  icon: React.ReactNode;
  onClick?: () => void;
  highlight?: boolean;
}) {
  return (
    <div
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '10px 14px',
        cursor: onClick ? 'pointer' : 'default',
        color: highlight ? '#F53F3F' : 'var(--geo-text)',
        transition: 'background 0.15s',
      }}
      onMouseEnter={(e) => {
        if (onClick) (e.currentTarget as HTMLDivElement).style.background = 'rgba(79,70,229,.08)';
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLDivElement).style.background = 'transparent';
      }}
    >
      <span style={{ width: 18, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: highlight ? '#F53F3F' : '#4F46E5' }}>
        {icon}
      </span>
      {children}
    </div>
  );
}

function Divider() {
  return <div style={{ height: 1, background: 'var(--color-border-1)', margin: '4px 12px' }} />;
}

function Chevron() {
  return <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>›</span>;
}

function ToggleBtn({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <span
      onClick={onClick}
      style={{
        padding: '3px 12px',
        fontSize: 12,
        borderRadius: 8,
        background: active ? '#4F46E5' : 'transparent',
        color: active ? '#fff' : 'var(--color-text-2)',
        fontWeight: active ? 600 : 500,
        cursor: 'pointer',
        transition: 'all 0.2s',
      }}
    >
      {children}
    </span>
  );
}
