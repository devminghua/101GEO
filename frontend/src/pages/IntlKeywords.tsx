import { useState } from 'react';
import { Card, Input, Button, Message, Table, Tag, Space, Typography, Empty, Spin, Alert, Grid, Tabs } from '@arco-design/web-react';
import { IconSearch, IconGoogle } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

const { Row, Col } = Grid;

/* ================================================================
 * 国际搜索优化 · Google 关键词分析（P1 / v1.0.66）
 *  - 复用后端统一分析管线（同行识别/霸屏统计/建议生成与百度同口径）
 *  - SERP 数据源：Serper.dev（总后台「数据 API」页配置 Key）
 * ================================================================ */

interface RankItem { rank: number; type: string; peer: boolean; name: string; domain: string; title: string; url: string; snippet: string; suspected?: boolean; suspectHints?: string; }
interface PageInfo { page: number; density: number; ads: number; organicPeers: number; organicOther: number; items: RankItem[]; parseErrTag?: string; }
interface PeerRow { domain: string; name: string; count: number; pages: number[]; bestRank: number; avgRank: number; pattern: string; feats: string[]; hits?: string[]; }
interface AdviceRow { title: string; desc: string; }
interface IntlResult {
  keyword: string; depth: number; peers: number; myBest: number; heat: number;
  pages: PageInfo[]; peerRows: PeerRow[]; advices: AdviceRow[];
  suspicious?: { domain: string; title: string; url: string; pages: number[]; hints: string }[];
  totalItems?: number; myDomains?: string[]; costMs?: number; partialFail?: string; trendSummary?: string;
}

export default function IntlKeywords() {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const [engine, setEngine] = useState('google');
  const [analyzing, setAnalyzing] = useState(false);
  const [result, setResult] = useState<IntlResult | null>(null);
  const [errMsg, setErrMsg] = useState('');

  const analyze = async () => {
    const kw = keyword.trim();
    if (!kw) { Message.warning(t('intl.emptyKeyword')); return; }
    setAnalyzing(true);
    setErrMsg('');
    try {
      const d: any = await api.intlAnalyze({ keyword: kw, engine });
      setResult(d);
    } catch (e: any) {
      setErrMsg(e?.message || t('intl.analyzeFailed'));
      setResult(null);
    } finally {
      setAnalyzing(false);
    }
  };

  const totalOrganic = result ? result.pages.reduce((s, p) => s + (p.organicPeers || 0) + (p.organicOther || 0), 0) : 0;
  const totalAds = result ? result.pages.reduce((s, p) => s + (p.ads || 0), 0) : 0;
  const density = result && result.pages.length > 0
    ? Math.round(result.pages.reduce((s, p) => s + (p.density || 0), 0) / result.pages.length) : 0;

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#4285F4,#34A853,#FBBC05,#EA4335)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{engine === 'naver' ? t('intl.naverTitle') : t('intl.googleTitle')}</span>
        <Tag color={engine === 'naver' ? 'green' : 'arcoblue'} size="small">{engine === 'naver' ? 'Naver' : 'Google'}</Tag>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('intl.googleSub')}</div>

      {/* 引擎切换 */}
      <Tabs
        activeTab={engine}
        onChange={setEngine}
        style={{ marginBottom: 12 }}
      >
        <Tabs.TabPane key="google" title="🇺🇸 Google" />
        <Tabs.TabPane key="naver" title="🇰🇷 Naver" />
      </Tabs>

      {/* 输入区 */}
      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
        <Space style={{ width: '100%' }} direction="vertical" size={8}>
          <Input
            value={keyword}
            onChange={setKeyword}
            onPressEnter={analyze}
            placeholder={engine === 'naver' ? t('intl.naverPlaceholder') : t('intl.googlePlaceholder')}
            prefix={<IconGoogle style={{ color: '#86909C' }} />}
            size="large"
            style={{ borderRadius: 10 }}
          />
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
            <span style={{ fontSize: 12, color: '#86909c' }}>{t('intl.quotaNote')}</span>
            <Button type="primary" loading={analyzing} onClick={analyze}>
              {analyzing ? t('intl.analyzing') : t('intl.analyze')}
            </Button>
          </div>
        </Space>
      </Card>

      {/* 错误提示 */}
      {errMsg && <Alert type="error" style={{ marginBottom: 16, borderRadius: 10 }} content={errMsg} />}

      {/* 结果区 */}
      {result && (
        <>
          {/* 总览卡 */}
          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            {[
              { label: t('intl.peerCount'), value: result.peers, color: '#F53F3F' },
              { label: t('intl.myBestRank'), value: result.myBest > 0 ? `TOP${result.myBest}` : '—', color: '#00B42A' },
              { label: t('intl.heat'), value: result.heat, color: '#FF7D00' },
              { label: t('intl.density'), value: `${density}%`, color: '#165DFF' },
              { label: t('intl.organicAds'), value: `${totalOrganic}/${totalAds}`, color: '#722ED1' },
              { label: t('intl.costMs'), value: `${result.costMs || 0}ms`, color: '#86909C' },
            ].map((k) => (
              <Col key={k.label} xs={12} sm={8} md={4}>
                <Card bordered={false} style={{ borderRadius: 12, boxShadow: '0 2px 10px rgba(0,0,0,.04)' }} bodyStyle={{ padding: 14 }}>
                  <div style={{ color: '#86909C', fontSize: 12 }}>{k.label}</div>
                  <div style={{ fontSize: 22, fontWeight: 700, color: k.color, marginTop: 4 }}>{k.value}</div>
                </Card>
              </Col>
            ))}
          </Row>

          {/* 同行明细 */}
          {result.peerRows.length > 0 && (
            <Card title={t('intl.peerTable')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
              <Table
                rowKey="domain"
                data={result.peerRows}
                pagination={false}
                columns={[
                  { title: t('intl.colDomain'), dataIndex: 'name', render: (v, r: any) => <div><b>{v}</b><div style={{ fontSize: 11, color: '#86909c' }}>{r.domain}</div></div> },
                  { title: t('intl.colCount'), dataIndex: 'count', width: 80, align: 'center' as const },
                  { title: t('intl.colBest'), dataIndex: 'bestRank', width: 90, align: 'center' as const, render: (v) => <Tag color="green">TOP{v}</Tag> },
                  { title: t('intl.colAvg'), dataIndex: 'avgRank', width: 90, align: 'center' as const },
                  { title: t('intl.colPattern'), dataIndex: 'pattern', ellipsis: true },
                ]}
              />
            </Card>
          )}

          {/* 建议 */}
          {result.advices.length > 0 && (
            <Card title={t('intl.adviceTitle')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
              {result.advices.map((a, i) => (
                <div key={i} style={{ padding: '8px 0', borderBottom: i < result.advices.length - 1 ? '1px solid var(--color-fill-2)' : 'none' }}>
                  <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--geo-text)' }}>💡 {a.title}</div>
                  <div style={{ fontSize: 13, color: '#4e5969', marginTop: 4, lineHeight: 1.7 }}>{a.desc}</div>
                </div>
              ))}
            </Card>
          )}

          {/* 疑似同行 */}
          {result.suspicious && result.suspicious.length > 0 && (
            <Card title={t('intl.suspiciousTitle')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
              {result.suspicious.map((s, i) => (
                <div key={i} style={{ padding: '8px 0', display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', borderBottom: i < result.suspicious!.length - 1 ? '1px solid var(--color-fill-2)' : 'none' }}>
                  <div style={{ minWidth: 0 }}>
                    <b style={{ fontSize: 13 }}>{s.domain}</b>
                    <div style={{ fontSize: 12, color: '#86909c', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 560 }}>{s.title}</div>
                  </div>
                  <Tag color="orange" size="small">{s.hints}</Tag>
                </div>
              ))}
            </Card>
          )}

          {/* 趋势摘要 */}
          {result.trendSummary && (
            <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
              <Typography.Text style={{ fontSize: 13, color: '#4e5969', whiteSpace: 'pre-wrap' }}>{result.trendSummary}</Typography.Text>
            </Card>
          )}
        </>
      )}

      {!result && !analyzing && !errMsg && (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty description={t('intl.empty')} />
        </Card>
      )}
      {analyzing && (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
          <Spin size={24} />
          <div style={{ color: '#86909c', fontSize: 13, marginTop: 12 }}>{t('intl.analyzingTip')}</div>
        </div>
      )}
    </div>
  );
}
