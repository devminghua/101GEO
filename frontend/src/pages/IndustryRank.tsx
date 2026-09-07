import { useEffect, useState } from 'react';
import { Card, Radio, Spin, Message, Tag, Skeleton } from '@arco-design/web-react';
import { api } from '../api';

// 行业颜色（每个行业一种颜色区分）
const INDUSTRY_COLORS = [
  '#165DFF', // 汽车 蓝
  '#F53F3F', // 手机 红
  '#722ED1', // 电脑办公 紫
  '#FF7D00', // 家用电器 橙
  '#F5319D', // 化妆品 粉
  '#00B42A', // 旅游景点 绿
  '#B37FEB', // 家具家居 淡紫
  '#14C9C9', // 家装平台 青
  '#0E42D2', // 房产 深蓝
  '#F7BA1E', // 婴幼儿奶粉 黄
  '#86909C', // 高校 灰
];

// 指数值格式化：>=100000 显示 xxxk（参考百度指数），否则千分位
const fmtIndex = (v: number) => {
  if (v >= 100000) {
    return `${Math.round(v / 1000).toLocaleString()}k`;
  }
  return Math.round(v).toLocaleString();
};

// 排名徽章颜色：1 金 2 银 3 铜，其余灰
const rankBadge = (rank: number) => {
  if (rank === 1) return { background: '#F7BA1E', color: '#fff' };
  if (rank === 2) return { background: '#C9CDD4', color: '#fff' };
  if (rank === 3) return { background: '#F5A08B', color: '#fff' };
  return { background: 'var(--color-fill-2)', color: 'var(--color-text-3)' };
};

export default function IndustryRank() {
  const [period, setPeriod] = useState<'day' | 'week'>('day');
  const [metric, setMetric] = useState('brand');
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const r: any = await api.industryRank(period, metric);
      setData(r);
    } catch (e: any) {
      Message.error('行业排行加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [period, metric]);

  const industries = data?.industries || [];
  const metrics = data?.metrics || [
    { key: 'brand', label: '品牌指数' },
    { key: 'search', label: '品牌搜索指数' },
    { key: 'news', label: '品牌资讯指数' },
    { key: 'interact', label: '品牌互动指数' },
  ];

  return (
    <div>
      {/* 顶部说明 + 切换 */}
      <Card style={{ marginBottom: 16, borderRadius: 12 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 16 }}>
          <div style={{ flex: 1, minWidth: 260 }}>
            <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--geo-text)' }}>行业排行</div>
            <div style={{ fontSize: 13, color: '#86909C', marginTop: 6, lineHeight: 1.6 }}>
              盘点各行业品牌的指数排名与上升下降趋势，反映品牌在行业中的位置和变化。
            </div>
            {data?.updated_at && (
              <div style={{ fontSize: 12, color: '#86909C', marginTop: 6 }}>更新时间：{data.updated_at}</div>
            )}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, alignItems: 'flex-end' }}>
            <Radio.Group type="button" value={metric} onChange={(v) => setMetric(v as any)} size="small">
              {metrics.map((m: any) => (
                <Radio key={m.key} value={m.key}>{m.label}</Radio>
              ))}
            </Radio.Group>
          </div>
        </div>
      </Card>

      {/* 行业卡片网格 */}
      {loading ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: 16 }}>
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Card key={i} style={{ borderRadius: 12 }}>
              <Skeleton animation>
                <Skeleton text={{ rows: 6, width: ['40%', '90%', '70%', '80%', '60%', '75%'] }} style={{ marginTop: 8 }} />
              </Skeleton>
            </Card>
          ))}
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: 16 }}>
          {industries.map((ind: any, idx: number) => {
            const color = INDUSTRY_COLORS[idx % INDUSTRY_COLORS.length];
            const items = ind.items || [];
            return (
              <Card
                key={ind.code}
                style={{ borderRadius: 12, overflow: 'hidden' }}
                bodyStyle={{ padding: 0 }}
                title={
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                    <span style={{ width: 10, height: 10, borderRadius: 3, background: color, display: 'inline-block', flex: '0 0 auto' }} />
                    <span style={{ fontSize: 16, fontWeight: 700 }}>{ind.name}行业排行</span>
                    <span style={{ fontSize: 12, color: '#86909C' }}>{ind.en}</span>
                  </div>
                }
              >
                {/* 榜单列表 */}
                <div style={{ padding: '4px 0 8px' }}>
                  {items.length === 0 && (
                    <div style={{ padding: '24px 0', textAlign: 'center', color: '#86909C', fontSize: 13 }}>暂无数据</div>
                  )}
                  {items.map((it: any) => {
                    const badge = rankBadge(it.rank);
                    return (
                      <div
                        key={it.rank}
                        style={{
                          display: 'flex', alignItems: 'center', gap: 12,
                          padding: '10px 20px', borderBottom: '1px solid var(--color-border-2)',
                        }}
                      >
                        <span
                          style={{
                            width: 22, height: 22, borderRadius: 6, display: 'inline-flex',
                            alignItems: 'center', justifyContent: 'center',
                            fontSize: 12, fontWeight: 700, flex: '0 0 auto',
                            background: badge.background, color: badge.color,
                          }}
                        >
                          {it.rank}
                        </span>
                        <span style={{ flex: 1, fontSize: 14, color: 'var(--geo-text)' }}>{it.brand}</span>
                        <span style={{ fontSize: 14, fontWeight: 600, color }}>
                          {fmtIndex(it.value)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* 数据来源说明 */}
      <div style={{ marginTop: 16, fontSize: 12, color: '#86909C', textAlign: 'center' }}>
        数据参考百度指数行业排行（index.baidu.com），每日定时更新
      </div>
    </div>
  );
}
