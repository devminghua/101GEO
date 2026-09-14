import { useState } from 'react';
import { Card, Input, Button, Message, Tag, Space, Typography, Empty, Spin, Alert, Tabs, Radio } from '@arco-design/web-react';
import { IconFire, IconPublic } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import EChart from '../components/EChart';
import { api } from '../api';

/* ================================================================
 * 国际搜索优化 · 行业排行（P4 / v1.0.69）
 *  - Naver：官方 Datalab API（免费、权威、归一化趋势）
 *  - Google：SerpAPI google_trends
 * ================================================================ */

interface TrendPoint { period: string; value: number; }
interface TrendSeries { name: string; keywords: string[]; points: TrendPoint[]; }

export default function IntlRank() {
  const { t } = useTranslation();
  const [engine, setEngine] = useState('naver');
  const [keywords, setKeywords] = useState('');
  const [range, setRange] = useState('3m');
  const [loading, setLoading] = useState(false);
  const [series, setSeries] = useState<TrendSeries[] | null>(null);
  const [source, setSource] = useState('');
  const [errMsg, setErrMsg] = useState('');

  const query = async () => {
    const kws = keywords.split(/[,，]/).map((s) => s.trim()).filter(Boolean);
    if (kws.length === 0) { Message.warning(t('intlRank.emptyKeyword')); return; }
    if (kws.length > 5) { Message.warning(t('intlRank.max5')); return; }
    setLoading(true);
    setErrMsg('');
    try {
      const d: any = await api.intlTrends({ engine, keywords: kws, range });
      setSeries(d.series || []);
      setSource(d.source || '');
    } catch (e: any) {
      setErrMsg(e?.message || t('intlRank.failed'));
      setSeries(null);
    } finally {
      setLoading(false);
    }
  };

  const colors = ['#165DFF', '#F53F3F', '#00B42A', '#FF7D00', '#722ED1'];
  const chartOption = series && series.length > 0 ? {
    tooltip: { trigger: 'axis' as const },
    legend: { top: 0, textStyle: { color: 'var(--color-text-2)' } },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: {
      type: 'category' as const,
      data: series[0].points.map((p) => p.period.slice(5)),
      axisLabel: { color: 'var(--color-text-3)' },
    },
    yAxis: {
      type: 'value' as const,
      axisLabel: { color: 'var(--color-text-3)' },
      splitLine: { lineStyle: { color: 'var(--color-border-2)' } },
    },
    series: series.map((s, i) => ({
      name: s.name,
      type: 'line' as const,
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2.5 },
      itemStyle: { color: colors[i % colors.length] },
      data: s.points.map((p) => p.value),
    })),
  } : null;

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#FF7D00,#F53F3F)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('intlRank.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('intlRank.sub')}</div>

      <Tabs activeTab={engine} onChange={setEngine} style={{ marginBottom: 12 }}>
        <Tabs.TabPane key="naver" title="🇰🇷 Naver（官方 Datalab）" />
        <Tabs.TabPane key="google" title="🇺🇸 Google Trends" />
      </Tabs>

      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
        <Space style={{ width: '100%' }} direction="vertical" size={10}>
          <Input
            value={keywords}
            onChange={setKeywords}
            onPressEnter={query}
            placeholder={t('intlRank.placeholder')}
            prefix={<IconFire style={{ color: '#FF7D00' }} />}
            size="large"
            style={{ borderRadius: 10 }}
          />
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
            <Radio.Group value={range} onChange={setRange} type="button" size="small">
              <Radio value="1m">1M</Radio>
              <Radio value="3m">3M</Radio>
              <Radio value="6m">6M</Radio>
              <Radio value="12m">12M</Radio>
            </Radio.Group>
            <Button type="primary" loading={loading} onClick={query}>{t('intlRank.query')}</Button>
          </div>
          <div style={{ fontSize: 12, color: '#86909c' }}>
            {engine === 'naver' ? t('intlRank.naverNote') : t('intlRank.googleNote')}
          </div>
        </Space>
      </Card>

      {errMsg && <Alert type="error" style={{ marginBottom: 16, borderRadius: 10 }} content={errMsg} />}

      {series && chartOption && (
        <Card
          title={t('intlRank.chartTitle')}
          bordered={false}
          style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
          extra={source && <Tag color="green" size="small">{source}</Tag>}
        >
          <EChart option={chartOption} height={360} />
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 12 }}>
            {series.map((s, i) => (
              <Tag key={s.name} size="small" style={{ borderColor: colors[i % colors.length], color: colors[i % colors.length] }}>
                {s.name}
              </Tag>
            ))}
          </div>
        </Card>
      )}

      {!series && !loading && !errMsg && (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty description={t('intlRank.empty')} />
        </Card>
      )}
      {loading && (
        <div style={{ padding: '40px 0', textAlign: 'center' }}>
          <Spin size={24} />
          <div style={{ color: '#86909c', fontSize: 13, marginTop: 12 }}>{t('intlRank.loadingTip')}</div>
        </div>
      )}
    </div>
  );
}
