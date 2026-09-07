// 站内信弹层：铃铛入口右侧弹出，展示 SaaS 端统一推送的消息。
// 已读状态由后端按用户维护，前端本地乐观更新。
import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Message, Spin } from '@arco-design/web-react';
import { IconNotification, IconCheck } from '@arco-design/web-react/icon';
import { api, NotificationItem } from '../api';

interface Props {
  open: boolean;
  left: number;
  top?: number;
  bottom?: number;
  onClose: () => void;
  onUnreadChange: (n: number) => void;
}

// 相对时间：1 分钟内「刚刚」，否则 N 分钟/小时/天前，超过 7 天显示日期
function relTime(iso: string): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '';
  const diff = Date.now() - t;
  const min = 60 * 1000;
  const hour = 60 * min;
  const day = 24 * hour;
  if (diff < min) return '刚刚';
  if (diff < hour) return `${Math.floor(diff / min)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`;
  const d = new Date(t);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export default function NotificationPopover({ open, left, top, bottom, onClose, onUnreadChange }: Props) {
  const [list, setList] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<number | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = (await api.listNotifications(20)) || [];
      setList(Array.isArray(data) ? data : []);
      onUnreadChange(data.filter((n: NotificationItem) => !n.read).length);
    } catch {
      /* 拉取失败保持旧列表 */
    } finally {
      setLoading(false);
    }
  }, [onUnreadChange]);

  // 打开时刷新；未读数由父组件轮询，这里只在打开时拉一次
  useEffect(() => {
    if (open) {
      load();
      setExpanded(null);
    }
  }, [open, load]);

  // 点击弹层外关闭（父组件的触发器会自己 stopPropagation）
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (boxRef.current?.contains(e.target as Node)) return;
      onClose();
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open, onClose]);

  const markRead = async (item: NotificationItem) => {
    // 展开/收起全文
    if (expanded === item.id) {
      setExpanded(null);
      return;
    }
    setExpanded(item.id);
    if (item.read) return;
    // 乐观更新
    setList((prev) => prev.map((n) => (n.id === item.id ? { ...n, read: true } : n)));
    onUnreadChange(Math.max(0, list.filter((n) => !n.read).length - 1));
    try {
      await api.readNotification(item.id);
    } catch {
      setList((prev) => prev.map((n) => (n.id === item.id ? { ...n, read: false } : n)));
      Message.error('标记已读失败');
    }
  };

  const readAll = async () => {
    const unread = list.filter((n) => !n.read).length;
    if (unread === 0) {
      Message.info('没有未读消息');
      return;
    }
    setList((prev) => prev.map((n) => ({ ...n, read: true })));
    onUnreadChange(0);
    try {
      await api.readAllNotifications();
      Message.success(`已将 ${unread} 条标记为已读`);
    } catch {
      load();
      Message.error('操作失败，请重试');
    }
  };

  return createPortal(
    <div
      ref={boxRef}
      style={{
        position: 'fixed',
        left,
        top: top ?? undefined,
        bottom: bottom ?? undefined,
        width: 320,
        maxWidth: 'calc(100vw - 24px)',
        background: 'var(--geo-surface)',
        borderRadius: 14,
        boxShadow: '0 16px 40px rgba(15,23,42,.18), 0 4px 12px rgba(0,0,0,.10)',
        color: 'var(--geo-text)',
        zIndex: 1001,
        transformOrigin: top != null ? 'top left' : 'bottom left',
        transform: open ? 'translateY(0) scale(1)' : `translateY(${top != null ? '-10px' : '10px'}) scale(0.92)`,
        opacity: open ? 1 : 0,
        transition: 'transform 0.32s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.24s ease',
        pointerEvents: open ? 'auto' : 'none',
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
        maxHeight: 460,
      }}
      onMouseDown={(e) => e.stopPropagation()}
    >
      {/* 头部 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '14px 16px 10px',
          borderBottom: '1px solid #f0f0f2',
          flexShrink: 0,
        }}
      >
        <IconNotification style={{ fontSize: 16, color: '#4F46E5' }} />
        <span style={{ flex: 1, fontSize: 14, fontWeight: 600 }}>站内信</span>
        <span
          onClick={readAll}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            fontSize: 12,
            color: '#4F46E5',
            cursor: 'pointer',
            padding: '2px 4px',
          }}
          title="全部标记为已读"
        >
          <IconCheck style={{ fontSize: 12 }} />
          全部已读
        </span>
      </div>

      {/* 列表 */}
      <div style={{ overflowY: 'auto', flex: 1, padding: '4px 0 6px' }}>
        {loading && list.length === 0 ? (
          <div style={{ padding: '28px 0', textAlign: 'center' }}>
            <Spin size={20} />
          </div>
        ) : list.length === 0 ? (
          <div style={{ padding: '34px 0', textAlign: 'center', color: '#86909c', fontSize: 13 }}>
            <div style={{ fontSize: 26, marginBottom: 8, opacity: 0.5 }}>🔔</div>
            暂无站内信
          </div>
        ) : (
          list.map((n) => {
            const isOpen = expanded === n.id;
            return (
              <div
                key={n.id}
                onClick={() => markRead(n)}
                style={{
                  padding: '10px 16px',
                  cursor: 'pointer',
                  borderLeft: n.read ? '3px solid transparent' : '3px solid #4F46E5',
                  background: isOpen ? 'var(--geo-tint-blue)' : 'transparent',
                  transition: 'background 0.15s',
                }}
                onMouseEnter={(e) => {
                  if (!isOpen) (e.currentTarget as HTMLDivElement).style.background = '#fafafa';
                }}
                onMouseLeave={(e) => {
                  if (!isOpen) (e.currentTarget as HTMLDivElement).style.background = 'transparent';
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  {!n.read && (
                    <span
                      style={{
                        width: 6,
                        height: 6,
                        borderRadius: '50%',
                        background: '#F53F3F',
                        flexShrink: 0,
                      }}
                    />
                  )}
                  <span
                    style={{
                      flex: 1,
                      minWidth: 0,
                      fontSize: 13,
                      fontWeight: n.read ? 500 : 600,
                      color: n.read ? '#4e5969' : '#1d2129',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {n.title}
                  </span>
                  <span style={{ fontSize: 11, color: '#a9aeb8', flexShrink: 0, marginLeft: 4 }}>
                    {relTime(n.created_at)}
                  </span>
                </div>
                <div
                  style={{
                    fontSize: 12,
                    color: '#86909c',
                    marginTop: 4,
                    lineHeight: 1.6,
                    display: '-webkit-box',
                    WebkitLineClamp: isOpen ? 'unset' : 2,
                    WebkitBoxOrient: 'vertical',
                    overflow: isOpen ? 'visible' : 'hidden',
                    wordBreak: 'break-word',
                  }}
                >
                  {n.content || '（无正文）'}
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>,
    document.body,
  );
}
