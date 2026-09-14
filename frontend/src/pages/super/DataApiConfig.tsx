import { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Message, Tag, Alert, Typography, Divider } from '@arco-design/web-react';
import { api } from '../../api';

// 第三方数据 API（Just One API）配置（总后台 super）：
// 配置 token 后，抖音/小红书的同行主页、视频/笔记数据由 API 稳定抓取。
// 配置存全局（tenant_id=0），所有分站共用同一 token。
export default function DataApiConfig() {
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<{ enabled: boolean; token_masked: string; base_url: string; serper_enabled: boolean; serper_masked: string } | null>(null);
  const [token, setToken] = useState('');
  const [serperKey, setSerperKey] = useState('');

  const load = async () => {
    try {
      const c = await api.dataApiConfig();
      setConfig(c || null);
    } catch { /* 忽略 */ }
  };

  useEffect(() => { load(); }, []);

  const save = async () => {
    const t = token.trim();
    const sk = serperKey.trim();
    if (!t && !sk) { Message.warning('请输入 API Token 或 Serper Key'); return; }
    setLoading(true);
    try {
      const res = await api.dataApiSaveConfig(t, sk);
      setToken('');
      setSerperKey('');
      setConfig(res as any);
      Message.success('数据 API Token 已保存');
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <Card title="第三方数据 API" style={{ borderRadius: 12, marginBottom: 16 }}>
        <Typography.Text type="secondary">
          配置后，抖音 / 小红书的同行主页、视频/笔记数据（昵称、粉丝、点赞、评论等）由 API 稳定抓取，不再受平台反爬影响。
        </Typography.Text>
      </Card>

      <Card style={{ borderRadius: 12 }}>
        <Alert
          type="info"
          style={{ marginBottom: 16 }}
          content="未配置时，系统回退到无头浏览器 / 估算数据（可能抓不到或显示为「估算数据」）。配置 token 后自动切换为 API 稳定抓取。"
        />
        <Form layout="vertical" style={{ maxWidth: 560 }}>
          <Form.Item label="API 服务地址">
            <Input value={config?.base_url || 'https://api.justoneapi.com'} disabled />
          </Form.Item>
          <Form.Item label="当前状态">
            {config?.enabled ? (
              <Tag color="green">已配置（{config?.token_masked}）</Tag>
            ) : (
              <Tag color="orange">未配置（回退无头浏览器 / 估算）</Tag>
            )}
          </Form.Item>
          <Form.Item
            label="API Token"
            extra="注册地址：https://docs.justoneapi.com（抖音 + 小红书共用同一个 token）"
          >
            <Input.Password
              value={token}
              onChange={setToken}
              placeholder="粘贴 Just One API 的 Token"
              autoComplete="new-password"
            />
          </Form.Item>
          <Divider />
          <div style={{ fontWeight: 600, marginBottom: 8 }}>Google SERP（国际搜索优化）</div>
          <Alert
            type="info"
            style={{ marginBottom: 16 }}
            content="Google 关键词分析数据源。注册地址：https://serper.dev（2500 次免费额度，超出约 $0.30/千次）"
          />
          <Form.Item label="Serper 当前状态">
            {config?.serper_enabled ? (
              <Tag color="green">已配置（{config?.serper_masked}）</Tag>
            ) : (
              <Tag color="orange">未配置（Google 关键词分析不可用）</Tag>
            )}
          </Form.Item>
          <Form.Item label="Serper API Key" extra="留空表示不修改已保存的 Key">
            <Input.Password
              value={serperKey}
              onChange={setSerperKey}
              placeholder="粘贴 Serper.dev 的 API Key"
              autoComplete="new-password"
            />
          </Form.Item>
          <Button type="primary" loading={loading} onClick={save}>保存 Token</Button>
        </Form>
      </Card>
    </div>
  );
}
