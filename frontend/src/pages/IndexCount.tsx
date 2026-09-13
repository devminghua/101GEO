// 百度优化 · 收录查询
// 输入域名 → site:domain 抓取百度 SERP 首页 → 展示收录总数 + 首页收录条目。
// 收录总数解析「百度为您找到相关结果约 N 个」；抓不到时用首页条目数近似并标注。
import { useState } from 'react';
import { Button, Card, Empty, Input, Message, Spin, Tag } from '@arco-design/web-react';
import { IconSearch } from '@arco-design/web-react/icon';
import { api } from '../api';
import QuotaBadge from '../components/QuotaBadge';

interface IndexItem {
  title: string;
  url: string;
  domain: string;
  snippet: string;
}

interface IndexResult {
  domain: string;
  count: number;
  count_exact: boolean;
  items: IndexItem[];
  page_count: number;
}

export default function IndexCount() {
  const [domain, setDomain] = useState('');
  const [result, setResult] = useState<IndexResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  const run = async () => {
    const d = domain.trim();
    if (!d) {
      Message.warning('请填写要查询的网站域名');
      return;
    }
    setLoading(true);
    setSearched(true);
    setResult(null);
    try {
      const r: any = await api.baiduIndexCount({ domain: d });
      setResult(r);
      if (r.count > 0) {
        Message.success(`查询完成：${r.count_exact ? `收录约 ${r.count.toLocaleString()} 个页面` : `首页 ${r.page_count} 条收录（总数未能解析）`}`);
      } else {
        Message.info('该域名暂无收录记录，可能是新站或未被百度收录');
      }
    } catch (e: any) {
      Message.error(e?.message || '查询失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 960, margin: '0 auto' }}>
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#165DFF,#14C9C9,#FF7D00)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>收录查询</span>
        <Tag color="arcoblue" size="small">Beta</Tag>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>
        查询你的网站被百度收录了多少个页面（site: 查询，每次消耗 1 次查询配额）
        <QuotaBadge />
      </div>

      {/* 查询区 */}
      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <Input
            value={domain}
            onChange={setDomain}
            onPressEnter={run}
            placeholder="例如 link101.com.cn（自动去 www/协议）"
            style={{ flex: 1, minWidth: 260 }}
            prefix={<IconSearch />}
          />
          <Button type="primary" loading={loading} onClick={run} disabled={loading}>
            {loading ? '查询中…' : '查询收录'}
          </Button>
        </div>
        {searched && !loading && !result && (
          <div style={{ marginTop: 12, color: '#86909c', fontSize: 13 }}>无结果，请检查域名格式后重试</div>
        )}
      </Card>

      {/* 结果区 */}
      {loading && (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
          <Spin size={24} />
          <div style={{ color: '#86909c', fontSize: 13, marginTop: 10 }}>正在抓取百度搜索结果，约需 3~10 秒</div>
        </div>
      )}

      {result && !loading && (
        <>
          {/* 收录总数卡 */}
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ color: '#86909c', fontSize: 13 }}>{result.domain} 的百度收录</div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 4 }}>
                  <span style={{ fontSize: 34, fontWeight: 700, color: result.count > 0 ? '#165DFF' : '#86909c' }}>
                    {result.count > 0 ? result.count.toLocaleString() : '0'}
                  </span>
                  <span style={{ fontSize: 14, color: '#86909c' }}>个页面</span>
                  {!result.count_exact && result.count > 0 && (
                    <Tag color="orange" size="small">近似值（首页条数）</Tag>
                  )}
                </div>
              </div>
              <div style={{ flex: 1 }} />
              <div style={{ textAlign: 'right', color: '#86909c', fontSize: 12, lineHeight: 1.8 }}>
                {result.count > 0 && result.count <= 10 && '收录较少：建议增加内容更新频率并主动提交链接'}
                {result.count > 10 && result.count <= 100 && '收录正常：保持稳定更新，关注核心词排名'}
                {result.count > 100 && '收录良好：继续以内容矩阵扩大覆盖'}
                {result.count === 0 && '未收录：检查 robots.txt 与站点可访问性，到百度搜索资源平台提交站点'}
              </div>
            </div>
          </Card>

          {/* 首页收录条目 */}
          <Card
            title={`首页收录条目（${result.page_count} 条）`}
            bordered={false}
            style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
          >
            {result.items.length === 0 ? (
              <Empty description="首页未解析到条目" />
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {result.items.map((it, i) => (
                  <div key={i} style={{ borderBottom: '1px solid #f0f0f2', paddingBottom: 10 }}>
                    <a
                      href={it.url}
                      target="_blank"
                      rel="noreferrer"
                      style={{ fontSize: 14, fontWeight: 600, color: '#165DFF', textDecoration: 'none', wordBreak: 'break-all' }}
                    >
                      {it.title || it.url}
                    </a>
                    <div style={{ fontSize: 12, color: '#86909c', marginTop: 2, wordBreak: 'break-all' }}>{it.url}</div>
                    {it.snippet && (
                      <div style={{ fontSize: 13, color: '#4e5969', marginTop: 4, lineHeight: 1.6 }}>
                        {it.snippet}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </Card>
        </>
      )}
    </div>
  );
}
