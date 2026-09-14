import { useCallback, useEffect, useState } from 'react';
import { Button, Card, Empty, Message, Spin, Table, Tag, Tabs } from '@arco-design/web-react';
import { IconRefresh, IconArrowUp, IconArrowDown, IconMinus, IconPlusCircle, IconCloseCircle } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

/* ================================================================
 * 国际排名监控（独立于百度版）：Google / Naver 排名看板
 *  - 数据与百度完全隔离（engine 维度独立存储与独立配额池）
 *  - 百度「中文归中文」，本页四语言国际语境
 * ================================================================ */

interface OverviewItem {
  keyword: string;
  site_id: number;
  site_domain: string;
  site_name: string;
  latest_rank: number;
  latest_date: string;
  prev_rank: number;
  prev_date: string;
  trend: 'up' | 'down' | 'flat' | 'new' | 'none';
}

export default function IntlRankMonitor() {
  const { t } = useTranslation();
  const [engine, setEngine] = useState('google');
  const [items, setItems] = useState<OverviewItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [hint, setHint] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const d: any = await api.rankOverview(engine);
      setItems(d.items || []);
      setHint(d.hint || '');
    } catch {
      Message.error(t('irm.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [engine, t]);

  useEffect(() => {
    load();
  }, [load]);

  const trendMeta: Record<string, { icon: any; color: string; label: string }> = {
    up: { icon: <IconArrowUp />, color: '#F53F3F', label: t('irm.up') },
    down: { icon: <IconArrowDown />, color: '#00B42A', label: t('irm.down') },
    flat: { icon: <IconMinus />, color: '#86909C', label: t('irm.flat') },
    new: { icon: <IconPlusCircle />, color: '#165DFF', label: t('irm.new') },
    none: { icon: <IconCloseCircle />, color: '#C9CDD4', label: t('irm.none') },
  };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#4285F4,#34A853,#FBBC05,#EA4335)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('irm.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('irm.sub')}</div>

      <Tabs activeTab={engine} onChange={setEngine} style={{ marginBottom: 12 }}>
        <Tabs.TabPane key="google" title="🇺🇸 Google" />
        <Tabs.TabPane key="naver" title="🇰🇷 Naver" />
      </Tabs>

      <Card
        bordered={false}
        style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
        extra={
          <Button icon={<IconRefresh />} loading={loading} onClick={load} size="small">
            {t('irm.refresh')}
          </Button>
        }
      >
        {items.length === 0 ? (
          <Empty description={hint || t('irm.empty')} style={{ padding: '60px 0' }} />
        ) : (
          <Table
            rowKey={(r) => `${r.keyword}-${r.site_domain}`}
            data={items}
            loading={loading}
            pagination={false}
            columns={[
              { title: t('irm.colKeyword'), dataIndex: 'keyword', ellipsis: true, render: (v) => <b>{v}</b> },
              { title: t('irm.colDomain'), dataIndex: 'site_domain', ellipsis: true },
              {
                title: t('irm.colLatest'),
                dataIndex: 'latest_rank',
                width: 110,
                align: 'center' as const,
                render: (v, r: OverviewItem) => (v > 0 ? <Tag color="green">TOP{v}</Tag> : <span style={{ color: '#C9CDD4' }}>—</span>),
              },
              {
                title: t('irm.colTrend'),
                dataIndex: 'trend',
                width: 130,
                align: 'center' as const,
                render: (v) => {
                  const m = trendMeta[v] || trendMeta.none;
                  return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, color: m.color, fontWeight: 600 }}>{m.icon}{m.label}</span>;
                },
              },
              { title: t('irm.colDate'), dataIndex: 'latest_date', width: 110, align: 'center' as const },
            ]}
          />
        )}
      </Card>

      <div style={{ marginTop: 12, fontSize: 12, color: '#86909c' }}>
        {engine === 'google' ? t('irm.googleNote') : t('irm.naverNote')}
      </div>
    </div>
  );
}
