import { useState } from 'react';
import { Card, Input, Button, Message, Typography, Space } from '@arco-design/web-react';
import { IconSafe, IconCheckCircleFill } from '@arco-design/web-react/icon';
import { api } from '../api';

// 单机版卡密激活页：未激活 / 已到期时唯一入口。激活成功后展示到期时间与初始管理员密码。
export default function LicenseActivate({
  status,
}: {
  status: { activated: boolean; expired?: boolean; expire_at?: string; tier_name?: string };
}) {
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ tier_name: string; expire_at: string; super_username: string; super_password: string } | null>(null);

  const doActivate = async () => {
    const c = code.trim();
    if (!c) {
      Message.warning('请输入卡密');
      return;
    }
    setLoading(true);
    try {
      const r = await api.licenseActivate(c);
      setResult(r);
      Message.success('激活成功');
    } catch (e: any) {
      Message.error(e.message || '激活失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg,var(--geo-login-a) 0%,var(--geo-login-b) 60%,var(--geo-login-c) 100%)',
        padding: 20,
      }}
    >
      <Card style={{ width: 460, borderRadius: 16, boxShadow: '0 12px 40px rgba(22,93,255,0.12)' }}>
        <div style={{ textAlign: 'center', marginBottom: 20 }}>
          <div style={{ fontSize: 34, fontWeight: 800, letterSpacing: -0.5, lineHeight: 1.1, marginBottom: 6 }}>
            <span style={{ color: 'var(--geo-text)' }}>Link</span>
            <span style={{ color: '#4F46E5' }}>Geo</span>
          </div>
          <div style={{ fontSize: 13, color: '#86909c', letterSpacing: 1 }}>生成式引擎优化平台</div>
        </div>

        {result ? (
          <div>
            <div style={{ textAlign: 'center', padding: '12px 0 20px' }}>
              <IconCheckCircleFill style={{ fontSize: 44, color: '#00b42a' }} />
              <div style={{ fontSize: 16, fontWeight: 500, marginTop: 12 }}>
                激活成功 · {result.tier_name}
              </div>
              <div style={{ fontSize: 13, color: '#86909c', marginTop: 6 }}>
                有效期至 {result.expire_at}
              </div>
            </div>
            <div
              style={{
                background: 'var(--color-fill-2)',
                borderRadius: 12,
                padding: 14,
                fontSize: 13,
                lineHeight: 1.8,
              }}
            >
              <div>管理员账号：<Typography.Text copyable code>{result.super_username}</Typography.Text></div>
              <div>
                初始密码：
                <Typography.Text copyable code>{result.super_password}</Typography.Text>
              </div>
              <div style={{ color: '#86909c', marginTop: 6 }}>
                请妥善保存，登录后点击右上角「退出登录」下拉菜单中的「修改密码」即可修改。
              </div>
            </div>
            <Button type="primary" long size="large" style={{ marginTop: 20 }} onClick={() => { window.location.hash = '#/login'; window.location.reload(); }}>
              前往登录
            </Button>
          </div>
        ) : (
          <div>
            {status.expired && status.expire_at ? (
              <div
                style={{
                  background: 'var(--color-fill-2)',
                  borderRadius: 12,
                  padding: '10px 14px',
                  fontSize: 13,
                  color: 'var(--color-text-2)',
                  marginBottom: 16,
                }}
              >
                软件已到期（{status.expire_at}），请输入新的卡密续费激活。
              </div>
            ) : (
              <div
                style={{
                  background: 'var(--color-fill-2)',
                  borderRadius: 12,
                  padding: '10px 14px',
                  fontSize: 13,
                  color: 'var(--color-text-2)',
                  marginBottom: 16,
                }}
              >
                请输入卡密激活软件。卡密由您的服务商提供，激活后开始计时。
              </div>
            )}
            <Input
              size="large"
              placeholder="卡密，如 LG-XXXXX-XXXXX-..."
              value={code}
              onChange={(v) => setCode(v.toUpperCase())}
              prefix={<IconSafe />}
              onPressEnter={doActivate}
              style={{ fontFamily: 'var(--font-mono)', letterSpacing: 0.5 }}
            />
            <Space direction="vertical" style={{ width: '100%', marginTop: 20 }}>
              <Button type="primary" long size="large" loading={loading} onClick={doActivate}>
                立即激活
              </Button>
              <Typography.Text type="secondary" style={{ fontSize: 12, textAlign: 'center', display: 'block' }}>
                激活即代表同意服务条款，请勿将卡密转借他人
              </Typography.Text>
            </Space>
          </div>
        )}
      </Card>
    </div>
  );
}
