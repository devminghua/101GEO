// 百度优化 · 排名监控看板
// 聚合展示全部启用监控词的各站点排名：最新排名、较上次变化（升/降/持平/新上榜/掉榜）。
// 数据来自「关键词分析」页每次分析自动落库的排名快照，无需额外抓取。
// 自适应：≥840px 宽屏用表格；窄屏切换为卡片流（每词一卡，竖排信息），页头自动换行。
import { useCallback, useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Card, Empty, Message, Spin, Table, Tag } from '@arco-design/web-react';
import { IconRefresh, IconArrowUp, IconArrowDown, IconMinus, IconPlusCircle, IconCloseCircle } from '@arco-design/web-react/icon';
import { api } from '../api';

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

const TREND_META: Record<string, { text: string; color: string; icon: any }> = {
  up: { text: '上升', color: 'red', icon: IconArrowUp },
  down: { text: '下降', color: 'green', icon: IconArrowDown },
  flat: { text: '持平', color: 'gray', icon: IconMinus },
  new: { text: '新上榜', color: 'arcoblue', icon: IconPlusCircle },
  none: { text: '暂无数据', color: 'gray', icon: IconMinus },
};

// 响应式切换：≥840px 表格，窄屏卡片流（CSS 媒体查询控制显隐，双形态同数据源）
function ResponsiveStyle() {
  return (
    <style>{`
      @media (max-width: 839px) {
        .rank-monitor-table { display: none !important; }
        .rank-monitor-cards { display: grid !important; }
        .rank-monitor-page { padding: 12px 10px 32px !important; }
      }
      @media (min-width: 840px) {
        .rank-monitor-cards { display: none !important; }
      }
      .rank-monitor-cards {
        display: none;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: 12px;
      }
    `}</style>
  );
}

function TrendTag({ trend }: { trend: string }) {
  const meta = TREND_META[trend] || TREND_META.none;
  const Icon = meta.icon;
  return (
    <Tag color={meta.color} style={{ display: 'inline-flex', alignItems: 'center', gap: 2, margin: 0 }}>
      <Icon style={{ fontSize: 12 }} />
      {meta.text}
    </Tag>
  );
}

export default function RankMonitor() {
  const [params] = useSearchParams();
  const engine = params.get('engine') || 'baidu';
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
      Message.error('加载排名监控失败');
    } finally {
      setLoading(false);
    }
  }, [engine]);

  useEffect(() => {
    load();
  }, [load]);

  const columns = [
    {
      title: '监控关键词',
      dataIndex: 'keyword',
      width: 200,
      render: (v: string) => <span style={{ fontWeight: 600 }}>{v}</span>,
    },
    {
      title: '网站',
      dataIndex: 'site_domain',
      width: 200,
      render: (_: string, r: OverviewItem) => (
        <span style={{ color: '#4e5969' }}>
          {r.site_name ? `${r.site_name}（${r.site_domain}）` : r.site_domain}
        </span>
      ),
    },
    {
      title: '最新排名',
      dataIndex: 'latest_rank',
      width: 120,
      render: (v: number) =>
        v > 0 ? (
          <span style={{ fontWeight: 700, fontSize: 15, color: v <= 10 ? '#F53F3F' : '#1d2129' }}>
            第 {v} 名
          </span>
        ) : (
          <span style={{ color: '#86909c' }}>未上榜</span>
        ),
    },
    {
      title: '较上次',
      dataIndex: 'trend',
      width: 130,
      render: (_: string, r: OverviewItem) => <TrendTag trend={r.trend} />,
    },
    {
      title: '上次排名',
      dataIndex: 'prev_rank',
      width: 150,
      render: (v: number, r: OverviewItem) =>
        r.prev_date
          ? v > 0
            ? `第 ${v} 名（${r.prev_date}）`
            : `未上榜（${r.prev_date}）`
          : '—',
    },
  ];

  return (
    <div className="rank-monitor-page" style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <ResponsiveStyle />
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#165DFF,#14C9C9,#FF7D00)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>排名监控</span>
        <Tag color="arcoblue" size="small">Beta</Tag>
        <span style={{ flex: 1, minWidth: 20 }} />
        <Button icon={<IconRefresh />} onClick={load} loading={loading}>
          刷新
        </Button>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>
        聚合全部监控关键词的百度排名变化：数据来自「关键词分析」每次分析自动落库的快照，无需额外消耗查询次数
      </div>

      {loading ? (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
          <Spin size={24} />
        </div>
      ) : items.length === 0 ? (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty
            description={
              <span>
                {hint || '暂无监控关键词'}
                <br />
                <span style={{ color: '#86909c', fontSize: 12 }}>
                  请先到「百度优化 → 关键词分析」的「我的网站 / 关键词配置」中添加监控词，分析后这里会自动出现排名变化
                </span>
              </span>
            }
          />
        </Card>
      ) : (
        <>
          {/* 宽屏：表格 */}
          <Card bordered={false} className="rank-monitor-table" style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
            <Table
              rowKey={(r: OverviewItem) => `${r.keyword}|${r.site_domain}`}
              columns={columns}
              data={items}
              loading={loading}
              pagination={{ pageSize: 20, showTotal: true }}
              scroll={{ x: 800 }}
            />
          </Card>

          {/* 窄屏：卡片流（每词一卡，信息竖排） */}
          <div className="rank-monitor-cards">
            {items.map((r) => (
              <Card key={`${r.keyword}|${r.site_domain}`} bordered={false} style={{ borderRadius: 14, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <span style={{ fontSize: 15, fontWeight: 700, color: 'var(--geo-text)', wordBreak: 'break-all' }}>
                    {r.keyword}
                  </span>
                  <TrendTag trend={r.trend} />
                </div>
                <div style={{ fontSize: 12, color: '#86909c', marginTop: 6, wordBreak: 'break-all' }}>
                  {r.site_name ? `${r.site_name}（${r.site_domain}）` : r.site_domain}
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginTop: 10, flexWrap: 'wrap', gap: 8 }}>
                  <span>
                    <span style={{ fontSize: 12, color: '#86909c' }}>最新排名 </span>
                    {r.latest_rank > 0 ? (
                      <span style={{ fontWeight: 800, fontSize: 18, color: r.latest_rank <= 10 ? '#F53F3F' : '#1d2129' }}>
                        第 {r.latest_rank} 名
                      </span>
                    ) : (
                      <span style={{ color: '#86909c' }}>未上榜</span>
                    )}
                  </span>
                  <span style={{ fontSize: 12, color: '#86909c' }}>
                    上次：
                    {r.prev_date
                      ? r.prev_rank > 0
                        ? `第 ${r.prev_rank} 名`
                        : '未上榜'
                      : '—'}
                  </span>
                </div>
              </Card>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
