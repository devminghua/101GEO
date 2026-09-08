import { useEffect, useState } from 'react';
import { Card, Button, Message, Table, Tag, Space, Typography } from '@arco-design/web-react';
import { IconCopy, IconUserAdd, IconGift } from '@arco-design/web-react/icon';
import { api } from '../api';

export default function InvitePage() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const r: any = await api.inviteSummary();
      setData(r);
    } catch (e: any) {
      Message.error('加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); /* eslint-disable-next-line */ }, []);

  const inviteUrl = data?.invite_code ? `${window.location.origin}/register?ref=${data.invite_code}` : '';

  const copyText = async (text: string, tip: string) => {
    try {
      await navigator.clipboard.writeText(text);
      Message.success(tip);
    } catch {
      // 兜底：老浏览器降级
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      Message.success(tip);
    }
  };

  const columns = [
    { title: '邀请时间', dataIndex: 'created_at', width: 140 },
    { title: '被邀请人', dataIndex: 'phone', width: 140 },
    {
      title: '客户名称', dataIndex: 'company_name',
      render: (v: string) => v || <Typography.Text type="secondary">待完善</Typography.Text>,
    },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (v: number) => (v === 1 ? <Tag color="green">已奖励</Tag> : <Tag color="orange">待注册</Tag>),
    },
    {
      title: '奖励 token', dataIndex: 'reward_points', width: 100,
      render: (v: number) => (v > 0 ? `+${v}` : '-'),
    },
  ];

  return (
    <div>
      <Card style={{ marginBottom: 16, borderRadius: 12, background: 'linear-gradient(135deg,#4F46E5,#7B61FF)', color: '#fff' }} bodyStyle={{ padding: '24px' }}>
        <div style={{ fontSize: 18, fontWeight: 700, color: '#fff' }}>邀约奖励</div>
        <div style={{ fontSize: 13, color: 'rgba(255,255,255,0.9)', marginTop: 6, lineHeight: 1.6 }}>
          邀请新客户注册，每成功邀请 1 位，你和对方都受益——你获得 {data?.reward_per ?? 500} token 奖励。
        </div>
        <div style={{ display: 'flex', gap: 48, marginTop: 20 }}>
          <div>
            <div style={{ fontSize: 12, color: 'rgba(255,255,255,0.85)' }}>累计已邀请</div>
            <div style={{ fontSize: 28, fontWeight: 700, color: '#fff', marginTop: 4 }}>{data?.total_invited ?? 0}</div>
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'rgba(255,255,255,0.85)' }}>累计奖励 token</div>
            <div style={{ fontSize: 28, fontWeight: 700, color: '#fff', marginTop: 4 }}>{data?.total_reward ?? 0}</div>
          </div>
        </div>
      </Card>

      <Card title="我的邀请链接" style={{ marginBottom: 16, borderRadius: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 260, background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 14px', fontSize: 14, wordBreak: 'break-all', color: 'var(--color-text-1)' }}>
            {inviteUrl || '加载中…'}
          </div>
          <Space>
            <Button type="primary" icon={<IconCopy />} onClick={() => copyText(inviteUrl, '链接已复制')}>复制链接</Button>
            <Button icon={<IconCopy />} onClick={() => copyText(data?.invite_code || '', '邀请码已复制')}>复制邀请码</Button>
          </Space>
        </div>
        <div style={{ fontSize: 12, color: '#86909C', marginTop: 10 }}>
          朋友打开链接或到注册页填入邀请码 <b>{data?.invite_code || ''}</b> 注册，即算邀请成功。
        </div>
      </Card>

      <Card title="邀请记录" style={{ borderRadius: 12 }} loading={loading}>
        <Table
          rowKey="id"
          columns={columns}
          data={data?.records || []}
          pagination={{ pageSize: 10 }}
          noDataElement={<EmptyInvite />}
        />
      </Card>
    </div>
  );
}

function EmptyInvite() {
  return (
    <div style={{ padding: '32px 0', textAlign: 'center', color: '#86909C' }}>
      <IconUserAdd style={{ fontSize: 36, marginBottom: 8 }} />
      <div>还没有邀请记录，快去分享你的邀请链接吧</div>
    </div>
  );
}
