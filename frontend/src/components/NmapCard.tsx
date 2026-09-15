import { useState } from 'react';
import { Card, Button, Input, Message, Space, Tag, Alert, Table, Spin, Typography } from '@arco-design/web-react';
import { IconBug, IconLaunch } from '@arco-design/web-react/icon';
import { api } from '../api';

const { Text } = Typography;

/* ================================================================
 * 端口扫描（Nmap 功能）卡片——总后台「安全检测」专属（老板 2026-09-15 拍板）
 *  - Go 原生 TCP connect 扫描引擎（与 nmap -sT 同原理）
 * ================================================================ */

export default function NmapCard() {
  const [target, setTarget] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [err, setErr] = useState('');

  const run = async () => {
    const t = target.trim().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!t) { Message.warning('请输入要扫描的网站域名'); return; }
    setLoading(true);
    setErr('');
    setResult(null);
    try {
      const r: any = await api.sitePortScan({ url: t });
      setResult(r);
    } catch (e: any) {
      setErr(e?.message || '端口扫描失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card
      title={<span><IconBug style={{ marginRight: 6, color: '#FF7D00' }} />端口扫描（Nmap）</span>}
      style={{ borderRadius: 12 }}
    >
      <Space style={{ width: '100%' }}>
        <Input
          value={target}
          onChange={setTarget}
          onPressEnter={run}
          placeholder="输入要扫描的网站域名，如 example.com"
          style={{ width: 420 }}
        />
        <Button type="outline" icon={<IconLaunch />} loading={loading} onClick={run}>开始扫描</Button>
      </Space>
      <div style={{ marginTop: 8, fontSize: 13, color: '#86909C' }}>
        对站点域名做常用端口与服务识别（20 个常用端口 + 服务名 + banner 探测，约 10-60 秒）。仅允许扫描您自己的域名，每日 5 次。
      </div>

      {loading && <div style={{ textAlign: 'center', padding: 30 }}><Spin size={20} tip="端口扫描中，约需 10-60 秒…" /></div>}
      {err && <Alert type="error" style={{ marginTop: 12 }} content={err} />}
      {result && (
        <div style={{ marginTop: 12 }}>
          <Space size="large" style={{ marginBottom: 12 }}>
            <Tag color="arcoblue" size="large">{result.host} → {result.ip}</Tag>
            <Tag color={result.open_count > 0 ? 'orange' : 'green'} size="large">
              开放端口 {result.open_count} 个
            </Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>耗时 {result.elapsed}</Text>
          </Space>
          {result.ports.length > 0 ? (
            <Table
              rowKey={(r: any) => `${r.port}-${r.protocol}`}
              data={result.ports}
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
  );
}
