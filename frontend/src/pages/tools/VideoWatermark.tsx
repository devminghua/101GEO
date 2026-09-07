import { useEffect, useState } from 'react';
import { Card, Input, Button, Message, Alert, Space, Tag, Typography, Spin } from '@arco-design/web-react';
import { IconLink, IconDownload } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 短视频去水印：前端本地解析（跟随重定向提取视频直链），不动用服务器流量。
// 扣费规则：Token > 2000 可用，每次解析扣 100 Token。
export default function VideoWatermark() {
  const [status, setStatus] = useState<any>(null);
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ title?: string; videoUrl?: string; images?: string[]; rawUrl?: string; hint?: string } | null>(null);

  useEffect(() => {
    api.toolStatus().then((s: any) => setStatus(s)).catch(() => {});
  }, []);

  const available = status?.available ?? false;
  const balance = status?.balance ?? 0;
  const cost = status?.cost ?? 100;

  // 解析：后端内部扣费 + 解析；解析失败自动退款（不扣客户 Token）
  const parse = async () => {
    const u = url.trim();
    if (!u) { Message.warning('请先粘贴短视频分享链接'); return; }
    if (!/^https?:\/\//i.test(u)) { Message.warning('请输入 http/https 开头的链接'); return; }
    if (!available) { Message.warning(`获客工具需 Token > ${status?.min_balance ?? 2000}，请先充值`); return; }

    setLoading(true);
    setResult(null);
    try {
      // 后端代理解析（内部扣费，失败自动退款）
      const data: any = await api.parseVideo(u);
      setResult({
        title: data?.title || (data?.video_id ? `视频 ID：${data.video_id}` : '解析结果'),
        videoUrl: data?.video_url || '',
        images: data?.images || [],
        rawUrl: data?.raw_url || '',
        hint: data?.hint || '',
      });
      if (data?.video_url || (data?.images?.length > 0)) Message.success('解析成功，已消耗 100 Token');
      else Message.warning('解析失败，本次未扣费');
      // 刷新余额
      api.toolStatus().then((s: any) => setStatus(s)).catch(() => {});
    } catch (e: any) {
      Message.error(e.message || '解析失败');
      api.toolStatus().then((s: any) => setStatus(s)).catch(() => {});
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <Card style={{ marginBottom: 16, borderRadius: 12 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 700 }}>短视频去水印</div>
            <Typography.Text type="secondary" style={{ fontSize: 13 }}>
              支持抖音 / 快手 / 小红书视频与图片，粘贴分享链接即可解析无水印直链。
            </Typography.Text>
          </div>
          <Space>
            <Tag color={available ? 'green' : 'orange'}>
              {available ? `Token ${balance}` : `Token ${balance}（需 > ${status?.min_balance ?? 2000}）`}
            </Tag>
            <Tag color="arcoblue">每次 {cost} Token</Tag>
          </Space>
        </div>
        {!available && (
          <Alert style={{ marginTop: 12 }} type="warning" content={`获客工具需 Token 余额高于 ${status?.min_balance ?? 2000}，请先到「充值」页面充值。`} />
        )}
      </Card>

      <Card title="解析链接" style={{ marginBottom: 16, borderRadius: 12 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input
            value={url}
            onChange={setUrl}
            prefix={<IconLink />}
            placeholder="粘贴抖音/快手/小红书分享链接，或视频/图片直链"
            size="large"
          />
          <Button type="primary" size="large" loading={loading} onClick={parse} disabled={!available}>
            解析并去水印（扣 {cost} Token）
          </Button>
        </Space>
      </Card>

      {result && (
        <Card title="解析结果" style={{ borderRadius: 12 }}>
          <Spin loading={false}>
            <Space direction="vertical" style={{ width: '100%' }} size={12}>
              {result.title && <div style={{ fontWeight: 600 }}>{result.title}</div>}
              {result.rawUrl && (
                <div style={{ fontSize: 13, color: '#86909C', wordBreak: 'break-all' }}>原始地址：{result.rawUrl}</div>
              )}
              {result.videoUrl ? (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <video src={result.videoUrl} controls style={{ width: '100%', maxHeight: 320, borderRadius: 8, background: '#000' }} />
                  <a href={result.videoUrl} target="_blank" rel="noreferrer" download>
                    <Button type="primary" icon={<IconDownload />}>下载无水印视频</Button>
                  </a>
                </Space>
              ) : result.images && result.images.length > 0 ? (
                <div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                    {result.images.map((img: string, i: number) => (
                      <a key={i} href={img} target="_blank" rel="noreferrer" download>
                        <img src={img} alt={`图片${i + 1}`} style={{ width: 140, height: 140, objectFit: 'cover', borderRadius: 8, border: '1px solid var(--color-border-2)' }} />
                      </a>
                    ))}
                  </div>
                  <Typography.Text type="secondary" style={{ fontSize: 12, marginTop: 8, display: 'block' }}>点击图片可下载（共 {result.images.length} 张）</Typography.Text>
                </div>
              ) : (
                <Typography.Text type="secondary">未提取到媒体直链</Typography.Text>
              )}
              {result.hint && <Alert type="info" content={result.hint} />}
            </Space>
          </Spin>
        </Card>
      )}
    </div>
  );
}
