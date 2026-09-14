import { useState } from 'react';
import { Card, Input, Button, Message, Table, Tag, Space, Typography, Empty, Spin, Alert, Tabs } from '@arco-design/web-react';
import { IconSearch, IconPublic } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

/* ================================================================
 * 国际搜索优化 · 收录查询（P3 / v1.0.68）
 *  - site:domain 走 SERP 引擎（Serper/SerpAPI），立即可用
 *  - 接入 GSC / Naver Search Advisor 官方数据后自动升级为权威收录数
 * ================================================================ */

interface IndexItem { title: string; url: string; domain: string; snippet: string; }
interface IndexResult { domain: string; engine: string; count: number; count_exact: boolean; items: IndexItem[]; page_count: number; }

export default function IntlIndex() {
  const { t } = useTranslation();
  const [engine, setEngine] = useState('google');
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<IndexResult | null>(null);
  const [errMsg, setErrMsg] = useState('');

  const query = async () => {
    const d = domain.trim().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!d) { Message.warning(t('intlIdx.emptyDomain')); return; }
    setLoading(true);
    setErrMsg('');
    try {
      const r: any = await api.intlIndexCount({ domain: d, engine });
      setResult(r);
    } catch (e: any) {
      setErrMsg(e?.message || t('intlIdx.failed'));
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#4285F4,#34A853,#FBBC05,#EA4335)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('intlIdx.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('intlIdx.sub')}</div>

      <Tabs activeTab={engine} onChange={setEngine} style={{ marginBottom: 12 }}>
        <Tabs.TabPane key="google" title="🇺🇸 Google" />
        <Tabs.TabPane key="naver" title="🇰🇷 Naver" />
      </Tabs>

      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
        <Space style={{ width: '100%' }}>
          <Input
            value={domain}
            onChange={setDomain}
            onPressEnter={query}
            placeholder={t('intlIdx.placeholder')}
            prefix={<IconPublic style={{ color: '#86909C' }} />}
            size="large"
            style={{ borderRadius: 10, flex: 1 }}
          />
          <Button type="primary" size="large" loading={loading} icon={<IconSearch />} onClick={query}>
            {t('intlIdx.query')}
          </Button>
        </Space>
        <div style={{ fontSize: 12, color: '#86909c', marginTop: 8 }}>{t('intlIdx.quotaNote')}</div>
      </Card>

      {errMsg && <Alert type="error" style={{ marginBottom: 16, borderRadius: 10 }} content={errMsg} />}

      {result && (
        <>
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }} bodyStyle={{ padding: 20 }}>
            <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', alignItems: 'baseline' }}>
              <div>
                <div style={{ color: '#86909C', fontSize: 12 }}>{t('intlIdx.total')}</div>
                <div style={{ fontSize: 32, fontWeight: 700, color: '#165DFF' }}>{result.count.toLocaleString()}</div>
              </div>
              <div>
                <div style={{ color: '#86909C', fontSize: 12 }}>{t('intlIdx.firstPage')}</div>
                <div style={{ fontSize: 24, fontWeight: 600, color: 'var(--geo-text)' }}>{result.page_count}</div>
              </div>
              <Tag color={result.count_exact ? 'green' : 'orange'} size="small">
                {result.count_exact ? t('intlIdx.exact') : t('intlIdx.approx')}
              </Tag>
            </div>
          </Card>

          <Card title={t('intlIdx.firstPageItems')} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
            {result.items.length > 0 ? (
              <Table
                rowKey="url"
                data={result.items}
                pagination={false}
                columns={[
                  { title: t('intlIdx.colTitle'), dataIndex: 'title', ellipsis: true },
                  { title: t('intlIdx.colUrl'), dataIndex: 'url', ellipsis: true, render: (v) => <Typography.Text copyable style={{ fontSize: 12 }}>{v}</Typography.Text> },
                ]}
              />
            ) : (
              <Empty description={t('intlIdx.noItems')} />
            )}
          </Card>
        </>
      )}

      {!result && !loading && !errMsg && (
        <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
          <Empty description={t('intlIdx.empty')} />
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
