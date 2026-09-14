import { useEffect, useState } from 'react';
import { Card, Empty, Spin, Tag, Message, Modal, Typography } from '@arco-design/web-react';
import { IconRight, IconTrophy } from '@arco-design/web-react/icon';
import DOMPurify from 'dompurify';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

interface CaseItem {
  id: number;
  title: string;
  summary: string;
  cover_url: string;
  tags: string;
}

/* ================================================================
 * 成功案例（客户端展示）：SaaS 端上传，本页只读。
 * 卡片流：封面 / 标题 / 摘要 / 标签，点击查看详情。
 * ================================================================ */
export default function Cases() {
  const { t } = useTranslation();
  const [list, setList] = useState<CaseItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [detail, setDetail] = useState<any>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const d: any = await api.caseList();
      setList(d || []);
    } catch {
      Message.error('加载成功案例失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const openDetail = async (id: number) => {
    setDetailLoading(true);
    setDetail({ id });
    try {
      const d: any = await api.caseDetail(id);
      setDetail(d);
    } catch (e: any) {
      Message.error(e?.message || '加载详情失败');
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const tagsOf = (t: string) =>
    (t || '')
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean);

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#F7BA1E,#FF9A2E,#FF7D00)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('cases.pageTitle')}</span>
        <Tag color="gold" size="small">{t('cases.selected')}</Tag>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>
        {t('cases.subtitle')}
      </div>

      {loading ? (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
          <Spin size={24} />
        </div>
      ) : list.length === 0 ? (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty description="{t('cases.empty')}" />
        </Card>
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
            gap: 16,
          }}
        >
          {list.map((c) => (
            <Card
              key={c.id}
              hoverable
              bordered={false}
              onClick={() => openDetail(c.id)}
              style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', cursor: 'pointer', overflow: 'hidden' }}
              bodyStyle={{ padding: 0 }}
            >
              {c.cover_url ? (
                <div
                  style={{
                    height: 150,
                    background: `url(${c.cover_url}) center/cover no-repeat, linear-gradient(135deg,#FFF7E6,#FFE7BA)`,
                    backgroundSize: 'cover',
                    backgroundColor: '#FFF7E6',
                  }}
                />
              ) : (
                <div
                  style={{
                    height: 120,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    background: 'linear-gradient(135deg,#FFF7E6 0%,#FFE7BA 100%)',
                  }}
                >
                  <IconTrophy style={{ fontSize: 40, color: '#F7BA1E' }} />
                </div>
              )}
              <div style={{ padding: '14px 16px 16px' }}>
                <div
                  style={{
                    fontSize: 15,
                    fontWeight: 700,
                    color: 'var(--geo-text)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 8,
                  }}
                >
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.title}</span>
                  <IconRight style={{ color: '#C9CDD4', flexShrink: 0 }} />
                </div>
                {c.summary && (
                  <div
                    style={{
                      fontSize: 13,
                      color: '#4e5969',
                      marginTop: 6,
                      lineHeight: 1.6,
                      display: '-webkit-box',
                      WebkitLineClamp: 2,
                      WebkitBoxOrient: 'vertical',
                      overflow: 'hidden',
                    }}
                  >
                    {c.summary}
                  </div>
                )}
                {tagsOf(c.tags).length > 0 && (
                  <div style={{ display: 'flex', gap: 6, marginTop: 10, flexWrap: 'wrap' }}>
                    {tagsOf(c.tags).map((t) => (
                      <Tag key={t} color="orange" size="small">
                        {t}
                      </Tag>
                    ))}
                  </div>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* 详情弹窗 */}
      <Modal
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <IconTrophy style={{ color: '#F7BA1E' }} />
            {detail?.title || t('cases.detail')}
          </span>
        }
        visible={!!detail}
        onCancel={() => setDetail(null)}
        footer={null}
        style={{ width: 640, maxWidth: '92vw' }}
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : (
          <div>
            {detail?.cover_url && (
              <div
                style={{
                  height: 180,
                  borderRadius: 10,
                  marginBottom: 14,
                  background: `url(${detail.cover_url}) center/cover no-repeat`,
                  backgroundColor: '#FFF7E6',
                }}
              />
            )}
            {detail?.summary && (
              <Typography.Paragraph style={{ color: '#4e5969', fontSize: 13 }}>
                {detail.summary}
              </Typography.Paragraph>
            )}
            {detail?.tags && (
              <div style={{ display: 'flex', gap: 6, marginBottom: 12, flexWrap: 'wrap' }}>
                {tagsOf(detail.tags).map((t: string) => (
                  <Tag key={t} color="orange" size="small">
                    {t}
                  </Tag>
                ))}
              </div>
            )}
            <div
              className="help-doc-content"
              style={{ fontSize: 14, lineHeight: 1.9, color: 'var(--geo-text)' }}
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(detail?.content || '') }}
            />
            {!detail?.content && (
              <div style={{ fontSize: 14, color: '#86909c' }}>{t('cases.noContent')}</div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
