// 消息中心内页（老板需求 2026-09-13）：站内信提醒可点开内页查看全部内容。
// 展示完整推送列表（含已读），未读高亮，点击展开全文并标记已读。
// 已读能力复用既有接口（api.readNotification / readAllNotifications），不另起第二套口径；
// 时间展示复用 NotificationPopover 导出的 relTime。
import { useCallback, useEffect, useState } from 'react';
import { Message, Spin } from '@arco-design/web-react';
import { IconCheck, IconNotification } from '@arco-design/web-react/icon';
import { api, NotificationItem } from '../api';
import { relTime } from '../components/NotificationPopover';

export default function Notifications() {
  const [list, setList] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      // 后端 limit 上限 100；站内信为低频运营推送，一次拉全量足够
      const data = (await api.listNotifications(100)) || [];
      setList(Array.isArray(data) ? data : []);
    } catch {
      Message.error('加载站内信失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const unreadCount = list.filter((n) => !n.read).length;

  const openItem = async (item: NotificationItem) => {
    // 手风琴：再次点击收起
    if (expanded === item.id) {
      setExpanded(null);
      return;
    }
    setExpanded(item.id);
    if (item.read) return;
    // 乐观更新 + 落库标记已读
    setList((prev) => prev.map((n) => (n.id === item.id ? { ...n, read: true } : n)));
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
    try {
      await api.readAllNotifications();
      Message.success(`已将 ${unread} 条标记为已读`);
    } catch {
      load();
      Message.error('操作失败，请重试');
    }
  };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 860, margin: '0 auto' }}>
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
        <IconNotification style={{ fontSize: 20, color: '#4F46E5' }} />
        <span style={{ fontSize: 18, fontWeight: 600, color: 'var(--geo-text)' }}>消息中心</span>
        <span style={{ flex: 1 }} />
        {unreadCount > 0 && (
          <span
            onClick={readAll}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 4,
              fontSize: 13,
              color: '#4F46E5',
              cursor: 'pointer',
              userSelect: 'none',
            }}
          >
            <IconCheck style={{ fontSize: 13 }} />
            全部已读（{unreadCount}）
          </span>
        )}
      </div>
      <div style={{ fontSize: 13, color: '#86909c', marginBottom: 16 }}>
        后台推送的平台通知与公告，共 {list.length} 条{unreadCount > 0 ? `，未读 ${unreadCount} 条` : ''}
      </div>

      {/* 列表 */}
      {loading ? (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
          <Spin size={24} />
        </div>
      ) : list.length === 0 ? (
        <div
          style={{
            padding: '70px 0',
            textAlign: 'center',
            color: '#86909c',
            fontSize: 13,
            background: 'var(--geo-surface)',
            borderRadius: 14,
          }}
        >
          <div style={{ fontSize: 34, marginBottom: 10, opacity: 0.5 }}>🔔</div>
          暂无站内信
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {list.map((n) => {
            const isOpen = expanded === n.id;
            return (
              <div
                key={n.id}
                onClick={() => openItem(n)}
                style={{
                  background: 'var(--geo-surface)',
                  borderRadius: 14,
                  border: '1px solid #f0f0f2',
                  borderLeft: n.read ? '3px solid transparent' : '3px solid #4F46E5',
                  padding: '14px 18px',
                  cursor: 'pointer',
                  transition: 'box-shadow 0.15s',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLDivElement).style.boxShadow = '0 4px 14px rgba(15,23,42,.06)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLDivElement).style.boxShadow = 'none';
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  {!n.read && (
                    <span
                      style={{
                        width: 7,
                        height: 7,
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
                      fontSize: 14,
                      fontWeight: n.read ? 500 : 600,
                      color: n.read ? '#4e5969' : '#1d2129',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {n.title}
                  </span>
                  <span style={{ fontSize: 12, color: '#a9aeb8', flexShrink: 0 }}>
                    {relTime(n.created_at)}
                  </span>
                </div>
                <div
                  style={{
                    marginTop: 8,
                    fontSize: 13,
                    color: '#4e5969',
                    lineHeight: 1.8,
                    whiteSpace: isOpen ? 'pre-wrap' : undefined,
                    wordBreak: 'break-word',
                    display: isOpen ? 'block' : '-webkit-box',
                    WebkitLineClamp: isOpen ? 'unset' : 2,
                    WebkitBoxOrient: 'vertical',
                    overflow: isOpen ? 'visible' : 'hidden',
                    maxHeight: isOpen ? 420 : undefined,
                    overflowY: isOpen ? 'auto' : undefined,
                  }}
                >
                  {n.content || '（无正文）'}
                </div>
                {!isOpen && (
                  <div style={{ marginTop: 6, fontSize: 12, color: '#4F46E5' }}>点击查看全部内容</div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
