import { useState } from 'react';
import { Card, Input, Button, Message, Table, Tag, Space, Typography, Empty, Spin, Alert, Tabs } from '@arco-design/web-react';
import { IconSearch, IconPublic } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

/* ================================================================
 * 国际搜索优化 · 差距诊断（P4 / v1.0.69）
 *  - 复用国际关键词分析管线：输入关键词 + 我方域名 → 我方 vs 同行 TOP 差距对比
 * ================================================================ */

interface PeerRow { domain: string; name: string; count: number; pages: number[]; bestRank: number; avgRank: number; pattern: string; }
interface AdviceRow { title: string; desc: string; }

export default function IntlGap() {
  const { t } = useTranslation();
  const [engine, setEngine] = useState('google');
  const [keyword, setKeyword] = useState('');
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [errMsg, setErrMsg] = useState('');

  const analyze = async () => {
    const kw = keyword.trim();
    if (!kw) { Message.warning(t('intlGap.emptyKeyword')); return; }
    setLoading(true);
    setErrMsg('');
    try {
      const d: any = await api.intlAnalyze({ keyword: kw, engine, my_domain: domain.trim() });
      setResult(d);
    } catch (e: any) {
      setErrMsg(e?.message || t('intlGap.failed'));
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  const myBest = result?.myBest || 0;
  const gap = myBest > 0 ? t('intlGap.myRank', { rank: myBest }) : t('intlGap.notRanked');

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#722ED1,#F5319D)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('intlGap.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('intlGap.sub')}</div>

      <Tabs activeTab={engine} onChange={setEngine} style={{ marginBottom: 12 }}>
        <Tabs.TabPane key="google" title="🇺🇸 Google" />
        <Tabs.TabPane key="naver" title="🇰🇷 Naver" />
      </Tabs>

      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
        <Space style={{ width: '100%' }} direction="vertical" size={10}>
          <Input
            value={keyword}
            onChange={setKeyword}
            onPressEnter={analyze}
            placeholder={t('intlGap.keywordPlaceholder')}
            prefix={<IconSearch style={{ color: '#86909C' }} />}
            size="large"
            style={{ borderRadius: 10 }}
          />
          <Input
            value={domain}
            onChange={setDomain}
            onPressEnter={analyze}
            placeholder={t('intlGap.domainPlaceholder')}
            prefix={<IconPublic style={{ color: '#86909C' }} />}
            size="large"
            style={{ borderRadius: 10 }}
          />
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button type="primary" loading={loading} onClick={analyze}>{t('intlGap.analyze')}</Button>
          </div>
        </Space>
      </Card>

      {errMsg && <Alert type="error" style={{ marginBottom: 16, borderRadius: 10 }} content={errMsg} />}

      {result && (
        <>
          {/* 差距总览 */}
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }} bodyStyle={{ padding: 20 }}>
            <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', alignItems: 'baseline' }}>
              <div>
                <div style={{ color: '#86909C', fontSize: 12 }}>{t('intlGap.myBestLabel')}</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: myBest > 0 ? '#00B42A' : '#FF7D00' }}>{gap}</div>
              </div>
              <div>
                <div style={{ color: '#86909C', fontSize: 12 }}>{t('intlGap.peerCount')}</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: '#F53F3F' }}>{result.peers || 0}</div>
              </div>
              <div>
                <div style={{ color: '#86909C', fontSize: 12 }}>{t('intlGap.heat')}</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: '#FF7D00' }}>{result.heat || 0}</div>
              </div>
            </div>
          </Card>

          {/* 同行 TOP 表 */}
          {(result.peerRows || []).length > 0 && (
            <Card title={t('intlGap.peerTop')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
              <Table
                rowKey="domain"
                data={result.peerRows}
                pagination={false}
                columns={[
                  { title: t('intlGap.colPeer'), dataIndex: 'name', render: (v, r: PeerRow) => <div><b>{v}</b><div style={{ fontSize: 11, color: '#86909c' }}>{r.domain}</div></div> },
                  { title: t('intlGap.colCount'), dataIndex: 'count', width: 80, align: 'center' as const },
                  { title: t('intlGap.colBest'), dataIndex: 'bestRank', width: 90, align: 'center' as const, render: (v) => <Tag color="green">TOP{v}</Tag> },
                  { title: t('intlGap.colAvg'), dataIndex: 'avgRank', width: 90, align: 'center' as const },
                  { title: t('intlGap.colPattern'), dataIndex: 'pattern', ellipsis: true },
                ]}
              />
            </Card>
          )}

          {/* 差距建议 */}
          {(result.advices || []).length > 0 && (
            <Card title={t('intlGap.adviceTitle')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
              {result.advices.map((a: AdviceRow, i: number) => (
                <div key={i} style={{ padding: '8px 0', borderBottom: i < result.advices.length - 1 ? '1px solid var(--color-fill-2)' : 'none' }}>
                  <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--geo-text)' }}>💡 {a.title}</div>
                  <div style={{ fontSize: 13, color: '#4e5969', marginTop: 4, lineHeight: 1.7 }}>{a.desc}</div>
                </div>
              ))}
            </Card>
          )}
        </>
      )}

      {!result && !loading && !errMsg && (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty description={t('intlGap.empty')} />
        </Card>
      )}
      {loading && (
        <div style={{ padding: '40px 0', textAlign: 'center' }}>
          <Spin size={24} />
        </div>
      )}
    </div>
  );
}
