import { useCallback, useEffect, useState } from 'react';
import {
  Card, Form, Input, Select, Radio, Button, Table, Message, Modal, Space, Tag, Typography, Spin,
} from '@arco-design/web-react';
import { IconNotification, IconRefresh, IconSend, IconHistory } from '@arco-design/web-react/icon';
import { api, NotificationItem } from '../../api';

const FormItem = Form.Item;

// 推送范围：全体客户端 / 指定租户（分站下所有账号）/ 指定账号
const SCOPES = [
  { value: 'all', label: '全体用户' },
  { value: 'tenant', label: '指定租户（分站）' },
  { value: 'user', label: '指定账号' },
];

function fmtTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

export default function SuperNotifications() {
  const [form] = Form.useForm();
  const [scope, setScope] = useState<'all' | 'tenant' | 'user'>('all');
  const [tenants, setTenants] = useState<any[]>([]);
  const [users, setUsers] = useState<any[]>([]);
  const [records, setRecords] = useState<NotificationItem[]>([]);
  const [pushing, setPushing] = useState(false);
  const [loading, setLoading] = useState(false);

  const loadTargets = useCallback(async () => {
    try {
      const [t, u] = await Promise.all([api.listTenants(), api.listUsers(true)]);
      setTenants(Array.isArray(t) ? t : []);
      setUsers(Array.isArray(u) ? u : []);
    } catch {
      /* 目标列表拉取失败不影响推送（可改用 ID 输入） */
    }
  }, []);

  const loadRecords = useCallback(async () => {
    setLoading(true);
    try {
      const list = await api.listSuperNotifications();
      setRecords(Array.isArray(list) ? list : []);
    } catch {
      /* 忽略 */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadTargets();
    loadRecords();
  }, [loadTargets, loadRecords]);

  // 范围 → 目标名称（记录列表展示用）
  const targetLabel = (n: NotificationItem) => {
    if (n.target_type === 'all') return '全体用户';
    if (n.target_type === 'tenant') {
      const t = tenants.find((x: any) => x.id === n.tenant_id);
      return `租户：${t ? (t.name || t.tenant_name || `#${n.tenant_id}`) : `#${n.tenant_id}`}`;
    }
    const u = users.find((x: any) => x.id === n.user_id);
    return `账号：${u ? (u.username || `#${n.user_id}`) : `#${n.user_id}`}`;
  };

  const submit = async () => {
    let values: any;
    try {
      values = await form.validate();
    } catch {
      return;
    }
    const title: string = (values.title || '').trim();
    const content: string = (values.content || '').trim();
    if (!title) {
      Message.warning('请填写标题');
      return;
    }
    const targetId = scope === 'all' ? undefined : Number(values.target_id);
    if (scope !== 'all' && !targetId) {
      Message.warning(scope === 'tenant' ? '请选择要推送的租户' : '请选择要推送的账号');
      return;
    }

    const scopeText = SCOPES.find((s) => s.value === scope)?.label || '';
    const targetName =
      scope === 'tenant'
        ? tenants.find((x: any) => x.id === targetId)?.name || `#${targetId}`
        : scope === 'user'
          ? users.find((x: any) => x.id === targetId)?.username || `#${targetId}`
          : '所有分站的全部账号';

    Modal.confirm({
      title: '确认推送站内信',
      content: (
        <div style={{ fontSize: 13, lineHeight: 1.8 }}>
          <div>推送范围：{scopeText}{scope !== 'all' ? `（${targetName}）` : ''}</div>
          <div>标题：{title}</div>
          <div style={{ color: '#86909c' }}>推送后客户端铃铛将出现未读红点。</div>
        </div>
      ),
      okText: '确认推送',
      cancelText: '取消',
      onOk: async () => {
        setPushing(true);
        try {
          await api.pushNotification({ target_type: scope, target_id: targetId, title, content });
          Message.success('推送成功');
          form.setFieldsValue({ title: '', content: '' });
          loadRecords();
        } catch (e: any) {
          Message.error(e?.message || '推送失败');
        } finally {
          setPushing(false);
        }
      },
    });
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    {
      title: '范围',
      dataIndex: 'target_type',
      width: 180,
      render: (v: string, r: NotificationItem) => targetLabel(r),
    },
    { title: '标题', dataIndex: 'title', width: 200, ellipsis: true },
    { title: '内容', dataIndex: 'content', ellipsis: true },
    { title: '推送时间', dataIndex: 'created_at', width: 150, render: (v: string) => fmtTime(v) },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title heading={5} style={{ margin: 0, flex: 1 }}>
          站内信推送
        </Typography.Title>
        <Button icon={<IconRefresh />} onClick={loadRecords}>
          刷新
        </Button>
      </div>

      <Card title={<span><IconNotification style={{ marginRight: 6 }} />推送新消息</span>} style={{ marginBottom: 16 }}>
        <Form form={form} layout="vertical" style={{ maxWidth: 720 }} initialValues={{ scope: 'all' }}>
          <FormItem label="推送范围" required>
            <Radio.Group
              value={scope}
              onChange={(v) => {
                setScope(v as any);
                form.setFieldsValue({ target_id: undefined });
              }}
              options={SCOPES}
            />
          </FormItem>

          {scope === 'tenant' && (
            <FormItem
              label="选择租户（分站）"
              field="target_id"
              rules={[{ required: true, message: '请选择租户' }]}
            >
              <Select
                placeholder="按名称搜索租户"
                showSearch
                allowClear
                filterOption={(input: string, option: any) =>
                  String(option?.children ?? '').toLowerCase().includes(input.toLowerCase())
                }
                options={tenants.map((t: any) => ({
                  value: t.id,
                  label: `${t.name || t.tenant_name || '未命名'}${t.code ? `（${t.code}）` : ''}`,
                }))}
              />
            </FormItem>
          )}

          {scope === 'user' && (
            <FormItem
              label="选择账号"
              field="target_id"
              rules={[{ required: true, message: '请选择账号' }]}
            >
              <Select
                placeholder="按账号名搜索"
                showSearch
                allowClear
                filterOption={(input: string, option: any) =>
                  String(option?.children ?? '').toLowerCase().includes(input.toLowerCase())
                }
                options={users
                  .filter((u: any) => u.role !== 'super')
                  .map((u: any) => ({
                    value: u.id,
                    label: `${u.username}${u.tenant_name ? ` · ${u.tenant_name}` : ''}`,
                  }))}
              />
            </FormItem>
          )}

          <FormItem
            label="标题"
            field="title"
            rules={[{ required: true, message: '请填写标题' }]}
          >
            <Input placeholder="如：系统维护通知" maxLength={128} showWordLimit />
          </FormItem>

          <FormItem label="内容" field="content">
            <Input.TextArea placeholder="填写正文，客户端点击消息可展开全文" rows={4} maxLength={1000} showWordLimit />
          </FormItem>

          <FormItem style={{ marginBottom: 0 }}>
            <Space>
              <Button type="primary" icon={<IconSend />} loading={pushing} onClick={submit}>
                立即推送
              </Button>
              <Button onClick={() => form.resetFields()}>重置</Button>
            </Space>
          </FormItem>
        </Form>
      </Card>

      <Card
        title={<span><IconHistory style={{ marginRight: 6 }} />推送记录</span>}
        extra={<Tag color="arcoblue">最近 100 条</Tag>}
      >
        <Spin loading={loading} style={{ display: 'block' }}>
          <Table
            rowKey="id"
            columns={columns}
            data={records}
            pagination={{ pageSize: 10, sizeCanChange: true }}
            noDataElement="暂无推送记录"
            scroll={{ x: 900 }}
          />
        </Spin>
      </Card>
    </div>
  );
}
