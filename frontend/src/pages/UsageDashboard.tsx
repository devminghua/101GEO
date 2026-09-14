import { useEffect, useState } from 'react';
import { Card, Grid, Statistic, Table, Typography, Radio, Empty, Spin } from '@arco-design/web-react';
import { api } from '../api';
import EChart from '../components/EChart';
import { useTranslation } from 'react-i18next';

const { Row, Col } = Grid;

// 客户端「Token 用量」看板：统计本分站 AI 调用的真实 token 消耗
// （请求级 usage 记录，按平台 / 场景 / 天聚合）。
export default function UsageDashboard() {
  const { t } = useTranslation();
  const [days, setDays] = useState(7);
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api.usageOverview(days).then(setData).catch(() => setData(null)).finally(() => setLoading(false));
  }, [days]);

  const kpi = data?.kpi || {};
  const fmt = (n: number) => (n ?? 0).toLocaleString('en-US');

  // 按天趋势（折线 + 柱）
  const dailyOption = {
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 20, top: 30, bottom: 30 },
    xAxis: { type: 'category', data: (data?.daily || []).map((d: any) => d.key) },
    yAxis: [
      { type: 'value', name: 'Token' },
      { type: 'value', name: '调用次数', splitLine: { show: false } },
    ],
    series: [
      {
        name: 'Token 用量', type: 'bar', data: (data?.daily || []).map((d: any) => d.tokens),
        itemStyle: { color: '#4F46E5', borderRadius: [4, 4, 0, 0] },
      },
      {
        name: '调用次数', type: 'line', yAxisIndex: 1, data: (data?.daily || []).map((d: any) => d.calls),
        itemStyle: { color: '#F53F3F' },
      },
    ],
  };

  // 平台分布（饼图）
  const platformOption = {
    tooltip: { trigger: 'item', formatter: '{b}: {c} token ({d}%)' },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        data: (data?.by_platform || []).map((p: any) => ({ name: p.key, value: p.tokens })),
        label: { formatter: '{b}\n{d}%' },
      },
    ],
  };

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Card
            style={{ borderRadius: 12 }}
            title={t('usage.title')}
            extra={
              <Radio.Group value={days} onChange={setDays} type="button">
                <Radio value={1}>{t('usage.last1d')}</Radio>
                <Radio value={3}>{t('usage.last3d')}</Radio>
                <Radio value={7}>{t('usage.last7d')}</Radio>
                <Radio value={30}>{t('usage.last30d')}</Radio>
              </Radio.Group>
            }
          >
            {loading ? (
              <div style={{ textAlign: 'center', padding: 60 }}><Spin /></div>
            ) : !data ? (
              <Empty description={t('usage.empty')} />
            ) : (
              <>
                <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
                  <Col span={6}>
                    <Statistic title={t('usage.totalTokens')} value={fmt(kpi.tokens)} groupSeparator />
                  </Col>
                  <Col span={6}>
                    <Statistic title={t('usage.aiCalls')} value={fmt(kpi.calls)} groupSeparator />
                  </Col>
                  <Col span={6}>
                    <Statistic title={t('usage.inTokens')} value={fmt(kpi.in_tokens)} groupSeparator />
                  </Col>
                  <Col span={6}>
                    <Statistic title={t('usage.outTokens')} value={fmt(kpi.out_tokens)} groupSeparator />
                  </Col>
                </Row>
                <Row gutter={[16, 16]}>
                  <Col span={14}>
                    <Card title={t('usage.dailyTrend')} bordered style={{ borderRadius: 10 }}>
                      <EChart option={dailyOption} height={260} />
                    </Card>
                  </Col>
                  <Col span={10}>
                    <Card title={t('usage.platformDist')} bordered style={{ borderRadius: 10 }}>
                      <EChart option={platformOption} height={260} />
                    </Card>
                  </Col>
                </Row>
                <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
                  <Col span={24}>
                    <Card title={t('usage.platformDetail')} bordered style={{ borderRadius: 10 }}>
                      <Table
                        rowKey="key"
                        size="small"
                        pagination={false}
                        data={data?.by_platform || []}
                        columns={[
                          { title: '平台', dataIndex: 'key' },
                          { title: t('usage.model'), dataIndex: 'model' },
                          { title: t('usage.tokenUsage'), dataIndex: 'tokens', render: (v: number) => fmt(v) },
                          { title: t('usage.callCount'), dataIndex: 'calls', render: (v: number) => fmt(v) },
                        ]}
                      />
                      <Typography.Paragraph type="secondary" style={{ fontSize: 12, margin: '10px 0 0' }}>
                        {t('usage.byScene')}
                        {(data?.by_scene || []).map((s: any) => `${s.key} ${fmt(s.tokens)} token / ${fmt(s.calls)} 次`).join(' · ') || '暂无'}
                      </Typography.Paragraph>
                    </Card>
                  </Col>
                </Row>
                <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
                  <Col span={24}>
                    <Card title={t('usage.recentCalls')} bordered style={{ borderRadius: 10 }}>
                      <Table
                        rowKey="id"
                        size="small"
                        pagination={false}
                        data={data?.recent || []}
                        columns={[
                          { title: t('usage.time'), dataIndex: 'created_at', width: 180, render: (v: string) => (v || '').replace('T', ' ').slice(0, 19) },
                          { title: t('usage.platform'), dataIndex: 'platform_name', width: 140 },
                          { title: t('usage.model'), dataIndex: 'model', width: 170 },
                          { title: t('usage.scene'), dataIndex: 'scene', width: 110 },
                          { title: t('usage.input'), dataIndex: 'prompt_tokens', render: (v: number) => fmt(v) },
                          { title: t('usage.output'), dataIndex: 'completion_tokens', render: (v: number) => fmt(v) },
                          { title: t('usage.total'), dataIndex: 'total_tokens', render: (v: number) => fmt(v) },
                        ]}
                      />
                    </Card>
                  </Col>
                </Row>
              </>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
