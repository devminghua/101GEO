// 登录后自动弹窗：展示后台最新推送的一条未读站内信。
// 产品逻辑（老板需求 2026-09-13）：
//   1) 客户端登录后若有未读推送 → 居中弹窗展示标题 + 全文；
//   2) 「我知道了」关闭并标记该条已读；「查看全部消息」跳转消息中心内页；
//   3) 同一会话同一条只弹一次（sessionStorage 去重），避免刷新页面反复打扰。
// 已读能力完全复用既有接口 api.readNotification，不另起第二套口径。
import { Modal } from '@arco-design/web-react';
import { IconNotification, IconRight } from '@arco-design/web-react/icon';
import { NotificationItem } from '../api';
import { relTime } from './NotificationPopover';

interface Props {
  notice: NotificationItem | null;
  onClose: (markRead: boolean) => void;
  onViewAll: () => void;
}

export default function NoticePopup({ notice, onClose, onViewAll }: Props) {
  return (
    <Modal
      visible={!!notice}
      onCancel={() => onClose(true)}
      footer={null}
      unmountOnExit
      style={{ width: 460, maxWidth: 'calc(100vw - 32px)' }}
      modalRender={(node) => (
        <div style={{ borderRadius: 16, overflow: 'hidden' }}>{node}</div>
      )}
    >
      {notice && (
        <div>
          {/* 品牌色顶栏 */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '14px 20px',
              background: '#4F46E5',
              color: '#fff',
            }}
          >
            <IconNotification style={{ fontSize: 16 }} />
            <span style={{ fontSize: 14, fontWeight: 600, flex: 1 }}>平台通知</span>
            <span style={{ fontSize: 12, opacity: 0.85 }}>
              {notice.created_at ? new Date(notice.created_at).toLocaleString('zh-CN', { hour12: false }) : ''}
            </span>
          </div>

          {/* 正文区 */}
          <div style={{ padding: '18px 20px 6px' }}>
            <div
              style={{
                fontSize: 16,
                fontWeight: 600,
                color: '#1d2129',
                lineHeight: 1.5,
                wordBreak: 'break-word',
              }}
            >
              {notice.title}
            </div>
            <div
              style={{
                marginTop: 12,
                fontSize: 13,
                color: '#4e5969',
                lineHeight: 1.8,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                maxHeight: 300,
                overflowY: 'auto',
              }}
            >
              {notice.content || '（无正文）'}
            </div>
          </div>

          {/* 底部操作 */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 20px 16px',
            }}
          >
            <span
              onClick={onViewAll}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 2,
                fontSize: 13,
                color: '#4F46E5',
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              查看全部消息
              <IconRight style={{ fontSize: 12 }} />
            </span>
            <span
              onClick={() => onClose(true)}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                minWidth: 96,
                padding: '7px 20px',
                borderRadius: 8,
                background: '#4F46E5',
                color: '#fff',
                fontSize: 13,
                fontWeight: 500,
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              我知道了
            </span>
          </div>
        </div>
      )}
    </Modal>
  );
}
