import { useState } from 'react';
import { Card, Button, Input, Message, Spin, Space, Tag, Alert, Typography, Grid, Progress, Table } from '@arco-design/web-react';
import { IconLaunch, IconRefresh, IconBug } from '@arco-design/web-react/icon';
import { api } from '../api';

const { Title, Text } = Typography;
const { Row: GridRow, Col: GridCol } = Grid;

const LAYER_META: any = {
  access: { color: '#F53F3F', desc: '抓取器能拿到内容吗' },
  direct: { color: '#FF7D00', desc: '抓取器找得到、认得清每个 URL 吗' },
  understand: { color: '#165DFF', desc: '机器读得懂这是什么实体吗' },
  citable: { color: '#00B42A', desc: 'robots / WAF / sitemap / llms.txt' },
};

function statusTag(s: string) {
  if (s === 'ok') return <Tag color="green">正常</Tag>;
  if (s === 'warn') return <Tag color="orange">有隐患</Tag>;
  return <Tag color="red">不达标</Tag>;
}

// 站点体检（四层：访问 → 定向 → 理解 → 可引用）
export default function SiteAudit() {
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any>(null);
  // Nmap 端口扫描（老板 2026-09-14 需求）
  const [scanTarget, setScanTarget] = useState('');
  const [scanLoading, setScanLoading] = useState(false);
  const [scanResult, setScanResult] = useState<any>(null);
  const [scanErr, setScanErr] = useState('');

  const run = async () => {
    if (!url.trim()) { Message.warning('请输入站点 URL'); return; }
    setLoading(true);
    setData(null);
    try {
      const r: any = await api.siteAudit({ url: url.trim() });
      setData(r);
    } catch (e: any) {
      Message.error(e.message || '体检失败');
    } finally {
      setLoading(false);
    }
  };

  const runScan = async () => {
    const target = (scanTarget || url).trim().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!target) { Message.warning('请输入要扫描的网站域名'); return; }
    setScanLoading(true);
    setScanErr('');
    setScanResult(null);
    try {
      const r: any = await api.sitePortScan({ url: target });
      setScanResult(r);
    } catch (e: any) {
      setScanErr(e?.message || '端口扫描失败');
    } finally {
      setScanLoading(false);
    }
  };

  const levelColor: any = { excellent: 'green', good: 'green', medium: 'orange', poor: 'red' };
  const gradeMeta: any = [
    { k: 'A', label: '可直接被引用', color: '#00B42A' },
    { k: 'B', label: '基本可用', color: '#165DFF' },
    { k: 'C', label: '需要改造', color: '#FF7D00' },
    { k: 'D', label: '等于不存在', color: '#F53F3F' },
  ];
  const totalBlocks: number = data ? (Object.values(data.grade_dist || {}) as number[]).reduce((s, v) => s + (Number(v) || 0), 0) : 0;

  return (
    <div style={{ paddingTop: 4 }}>
      <Card style={{ borderRadius: 12, marginBottom: 16 }}>
        <Space style={{ width: '100%' }}>
          <Input
            value={url}
            onChange={setUrl}
            onPressEnter={run}
            placeholder="输入站点 URL，如 https://example.com"
            style={{ width: 420 }}
          />
          <Button type="primary" icon={<IconLaunch />} loading={loading} onClick={run}>开始体检</Button>
        </Space>
        <div style={{ marginTop: 8, fontSize: 13, color: '#86909C' }}>
          按「访问 → 定向 → 理解 → 可引用」四层加权体检（权重 30/18/32/20），每层依赖上一层，访问层失败时下游一切优化在引擎侧不可见。
          与「GEO 智能 → ⑧ 网站审计」共用同一套评分核心与历史记录，两个入口结果完全一致。
        </div>
      </Card>

      {/* Nmap 端口扫描（独立能力卡） */}
      <Card
        title={<span><IconBug style={{ marginRight: 6, color: '#FF7D00' }} />端口扫描（Nmap）</span>}
        style={{ borderRadius: 12, marginBottom: 16 }}
      >
        <Space style={{ width: '100%' }}>
          <Input
            value={scanTarget}
            onChange={setScanTarget}
            onPressEnter={runScan}
            placeholder="默认使用上方站点域名，也可单独填写"
            style={{ width: 420 }}
          />
          <Button type="outline" icon={<IconLaunch />} loading={scanLoading} onClick={runScan}>开始扫描</Button>
        </Space>
        <div style={{ marginTop: 8, fontSize: 13, color: '#86909C' }}>
          对站点域名做常用端口与服务识别（Nmap 快扫 + 版本探测，约 10-60 秒）。仅允许扫描您自己的域名，每日 5 次。
        </div>

        {scanLoading && <div style={{ textAlign: 'center', padding: 30 }}><Spin size={20} tip="Nmap 扫描中，约需 10-60 秒…" /></div>}
        {scanErr && <Alert type="error" style={{ marginTop: 12 }} content={scanErr} />}
        {scanResult && (
          <div style={{ marginTop: 12 }}>
            <Space size="large" style={{ marginBottom: 12 }}>
              <Tag color="arcoblue" size="large">{scanResult.host} → {scanResult.ip}</Tag>
              <Tag color={scanResult.open_count > 0 ? 'orange' : 'green'} size="large">
                开放端口 {scanResult.open_count} 个
              </Tag>
              <Text type="secondary" style={{ fontSize: 12 }}>耗时 {scanResult.elapsed}</Text>
            </Space>
            {scanResult.ports.length > 0 ? (
              <Table
                rowKey={(r: any) => `${r.port}-${r.protocol}`}
                data={scanResult.ports}
                pagination={false}
                size="small"
                columns={[
                  { title: '端口', dataIndex: 'port', width: 80 },
                  { title: '协议', dataIndex: 'protocol', width: 70 },
                  { title: '状态', dataIndex: 'state', width: 90, render: (v) => <Tag color={v === 'open' ? 'green' : 'gray'} size="small">{v === 'open' ? '开放' : v}</Tag> },
                  { title: '服务', dataIndex: 'service', width: 120 },
                  { title: '版本', dataIndex: 'version', ellipsis: true },
                ]}
              />
            ) : (
              <Alert type="success" style={{ marginTop: 8 }} content="未发现开放端口（全部关闭或被防火墙过滤），站点暴露面小，安全性良好。" />
            )}
          </div>
        )}
      </Card>

      {loading && <div style={{ textAlign: 'center', padding: 60 }}><Spin size={24} tip="正在抓取站点并逐层体检…" /></div>}

      {data && (
        <>
          <Card style={{ borderRadius: 12, marginBottom: 16 }}>
            <Space size="large" align="center">
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>综合评分</Text>
                <div style={{ fontSize: 40, fontWeight: 700, color: '#4F46E5', lineHeight: 1.1 }}>{data.score}</div>
              </div>
              <Tag color={levelColor[data.level]} size="large" style={{ fontSize: 14 }}>{data.level === 'excellent' ? '优秀' : data.level === 'good' ? '良好' : data.level === 'medium' ? '中等' : '较差'}</Tag>
              <div style={{ flex: 1 }}>
                <Text style={{ fontSize: 13 }}>{data.overall_note}</Text>
                <div style={{ fontSize: 12, color: '#86909C', marginTop: 4 }}>{data.url}</div>
              </div>
            </Space>
            {(data.layers || []).length > 0 && (
              <div style={{ marginTop: 16, display: 'flex', gap: 16, flexWrap: 'wrap' }}>
                {data.layers.map((l: any) => (
                  <div key={l.key} style={{ flex: '1 1 150px', minWidth: 150 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 4 }}>
                      <span><b style={{ color: LAYER_META[l.key]?.color }}>{l.label}</b></span>
                      <span style={{ color: '#86909C' }}>{l.score ?? '-'}<span style={{ opacity: 0.6 }}>/{l.weight ?? '-'}</span></span>
                    </div>
                    <Progress percent={l.weight ? ((l.score || 0) / l.weight) * 100 : 0} size="small" color={LAYER_META[l.key]?.color} formatText={() => ''} />
                  </div>
                ))}
              </div>
            )}
          </Card>

          <GridRow gutter={[16, 16]}>
            <GridCol span={16}>
              {data.layers.map((l: any) => (
                <Card key={l.key} style={{ borderRadius: 12, marginBottom: 12 }} title={
                  <Space>
                    <span style={{ width: 8, height: 8, borderRadius: '50%', background: LAYER_META[l.key]?.color, display: 'inline-block' }} />
                    <b>{l.label}</b>
                    <Text type="secondary" style={{ fontWeight: 400, fontSize: 12 }}>{LAYER_META[l.key]?.desc}</Text>
                    {statusTag(l.status)}
                  </Space>
                }>
                  {l.checks.map((c: any, i: number) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 0', borderBottom: i < l.checks.length - 1 ? '1px solid var(--color-border-1)' : 'none' }}>
                      <span style={{ width: 6, height: 6, borderRadius: '50%', background: c.status === 'ok' ? '#00B42A' : c.status === 'warn' ? '#FF7D00' : '#F53F3F', flexShrink: 0 }} />
                      <Text style={{ width: 130, flexShrink: 0, fontSize: 13 }}>{c.name}</Text>
                      <Text type="secondary" style={{ fontSize: 13, flex: 1 }}>{c.note}</Text>
                      {statusTag(c.status)}
                    </div>
                  ))}
                </Card>
              ))}
            </GridCol>
            <GridCol span={8}>
              <Card title="全站抽取块缺口" style={{ borderRadius: 12, marginBottom: 12 }}>
                <div style={{ fontSize: 12, color: '#86909C', marginBottom: 12 }}>GEO 最大的单项杠杆：检索按段落选材，整页无一段自包含可引的内容会被点名。</div>
                {gradeMeta.map((g) => {
                  const n: number = Number(data.grade_dist?.[g.k]) || 0;
                  return (
                    <div key={g.k} style={{ marginBottom: 14 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13 }}>
                        <span><b style={{ color: g.color }}>{g.k}</b> · {g.label}</span>
                        <b>{n}</b>
                      </div>
                      <Progress percent={totalBlocks ? (n / totalBlocks) * 100 : 0} size="small" color={g.color} />
                    </div>
                  );
                })}
              </Card>
              <Alert type="info" content="体检结果已自动保存，可到「GEO 智能 → ⑦ 网站审计」查看历史记录。低分项会生成优化行动工单。" />
            </GridCol>
          </GridRow>
        </>
      )}
    </div>
  );
}
