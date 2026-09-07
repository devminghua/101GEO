import { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Message, Tag, Alert, Typography } from '@arco-design/web-react';
import { api } from '../../api';

// 第三方数据 API（Just One API）配置（总后台 super）：
// 配置 token 后，抖音/小红书的同行主页、视频/笔记数据由 API 稳定抓取。
// 配置存全局（tenant_id=0），所有分站共用同一 token。
export default function DataApiConfig() {
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<{ enabled: boolean; token_masked: string; base_url: string } | null>(null);
  const [token, setToken] = useState('');

  const load = async () => {
    try {
      const c = await api.dataApiConfig();
      setConfig(c || null);
    } catch { /* 忽略 */ }
  };

  useEffect(() => { load(); }, []);

  const save = async () => {
    const t = token.trim();
    if (!t) { Message.warning('请输入 API Token'); return; }
    setLoading(true);
    try {
      const res = await api.dataApiSaveConfig(t);
      setToken('');
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
          <Button type="primary" loading={loading} onClick={save}>保存 Token</Button>
        </Form>
      </Card>
    </div>
  );
}
