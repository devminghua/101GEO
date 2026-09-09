import { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Tag,
  Select,
  Space,
  Message,
  Popconfirm,
  Switch,
  Typography,
  InputNumber,
  Tooltip,
} from '@arco-design/web-react';
import { IconPlus, IconEye } from '@arco-design/web-react/icon';
import { api } from '../../api';

export default function Users({ embedded = false }: { embedded?: boolean }) {
  const [list, setList] = useState<any[]>([]);
  const [tenants, setTenants] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [saving, setSaving] = useState(false);
  const [pwdUser, setPwdUser] = useState<any>(null);
  const [pwdSaving, setPwdSaving] = useState(false);
  const [plainUser, setPlainUser] = useState<any>(null); // 正在查看明文密码的账号
  const [plainPwd, setPlainPwd] = useState('');
  const [plainLoading, setPlainLoading] = useState(false);
  const [form] = Form.useForm();
  const [pwdForm] = Form.useForm();
  // 实时监听"开通时长"，用于预览服务截止日期
  const months = Form.useWatch('open_months', form) ?? 12;

  const load = async () => {
    setLoading(true);
    try {
      const [users, ts] = await Promise.all([api.listUsers(true), api.listTenants()]);
      setList(users || []);
      setTenants(ts || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const tenantName = (tid: number) => {
    const t = tenants.find((x) => x.id === tid);
    return t ? `${t.name}（${t.code}）` : tid === 0 ? '总后台' : '-';
  };

  const openCreate = () => {
    form.resetFields();
    form.setFieldsValue({ open_months: 12 });
    setModalVisible(true);
  };

  const submit = async () => {
    const values = await form.validate();
    setSaving(true);
    try {
      await api.createUser({
        username: values.username,
        password: values.password,
        nickname: values.nickname,
        tenant_code: values.tenant_code,
        open_months: values.open_months,
      });
      Message.success('账号已创建并设置密码');
      setModalVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  // 小眼睛：查看客户明文密码（可逆加密解密）
  const showPlain = async (row: any) => {
    setPlainUser(row);
    setPlainPwd('');
    setPlainLoading(true);
    try {
      const data: any = await api.userPlaintext(row.id);
      setPlainPwd(data.plaintext);
    } catch (e: any) {
      Message.error(e.message);
      setPlainUser(null);
    } finally {
      setPlainLoading(false);
    }
  };

  const resetPwd = async () => {
    const values = await pwdForm.validate();
    setPwdSaving(true);
    try {
      await api.resetUserPassword(pwdUser.id, values.password);
      Message.success(`已重置「${pwdUser.username}」的密码`);
      setPwdUser(null);
      pwdForm.resetFields();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setPwdSaving(false);
    }
  };

  const toggleStatus = async (row: any, checked: boolean) => {
    try {
      await api.updateUserStatus(row.id, checked ? 1 : 0);
      Message.success(checked ? '账号已启用' : '账号已停用');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const remove = async (row: any) => {
    try {
      await api.deleteUser(row.id);
      Message.success('账号已删除');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // 服务有效期展示：已到期红 / 30天内橙 / 未到期绿 / 不限灰
  const renderExpiry = (_: any, row: any) => {
    if (!row.open_months || row.open_months <= 0) {
      return <Tag color="gray">不限</Tag>;
    }
    const days = row.remain_days;
    if (days <= 0) {
      return <Tag color="red">已到期</Tag>;
    }
    let color: 'green' | 'orange' | 'arcoblue' = 'green';
    if (days <= 30) color = 'orange';
    return (
      <div>
        <Tag color={color}>剩余 {days} 天</Tag>
        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
          开通 {row.open_months} 个月 · 至{' '}
          {row.expire_at ? new Date(row.expire_at).toLocaleDateString() : '-'}
        </div>
      </div>
    );
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '账号', dataIndex: 'username', width: 140 },
    { title: '昵称', dataIndex: 'nickname', render: (v: string) => v || '-' },
    {
      title: '角色',
      dataIndex: 'role',
      width: 100,
      render: (v: string) =>
        v === 'super' ? <Tag color="gold">总后台</Tag> : <Tag color="green">分站管理员</Tag>,
    },
    {
      title: '所属分站',
      dataIndex: 'tenant_id',
      render: (v: number) => tenantName(v),
    },
    {
      title: '服务有效期',
      dataIndex: 'open_months',
      width: 190,
      render: renderExpiry,
    },
    {
      title: '状态',
      width: 90,
      render: (_: any, row: any) =>
        row.role === 'super' ? (
          <Tag>正常</Tag>
        ) : (
          <Switch size="small" checked={row.status === 1} checkedText="启用" uncheckedText="停用" onChange={(checked) => toggleStatus(row, checked)} />
        ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 180,
      render: (v: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '操作',
      width: 300,
      render: (_: any, row: any) =>
        row.role === 'super' ? (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            内置总后台账号
          </Typography.Text>
        ) : (
          <Space>
            <Tooltip content="查看该客户当前明文密码">
              <Button size="mini" icon={<IconEye />} onClick={() => showPlain(row)}>
                查看密码
              </Button>
            </Tooltip>
            <Button size="mini" onClick={() => setPwdUser(row)}>
              重置密码
            </Button>
            <Popconfirm title={`确定删除账号「${row.username}」？`} onOk={() => remove(row)} okText="删除" cancelText="取消">
              <Button size="mini" status="danger">
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
    },
  ];

  return (
    <Card
      title={
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>账号管理</span>
          <Button type="primary" icon={<IconPlus />} onClick={openCreate}>
            新建账号
          </Button>
        </div>
      }
      style={{ borderRadius: 12 }}
      bordered
    >
      {!embedded && (
        <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 16, fontSize: 13 }}>
          为分站创建登录账号，选择开通时长（最长 36 个月），账号将按期限自动到期。可「查看密码」协助客户登录，或「重置密码」。账号登录后，其所有 API 调用均路由到该账号所属分站的独立数据空间。
        </Typography.Text>
      )}
      <Table rowKey="id" columns={columns} data={list} loading={loading} pagination={false} />

      {/* 新建账号 */}
      <Modal
        title="新建账号"
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={submit}
        confirmLoading={saving}
        okText="创建"
        cancelText="取消"
        maskClosable={false}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="所属分站"
            field="tenant_code"
            rules={[{ required: true, message: '请选择所属分站' }]}
          >
            <Select placeholder="选择分站标识" allowClear={false}>
              {tenants.map((t) => (
                <Select.Option key={t.code} value={t.code}>
                  {t.name}（{t.code}）
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label="账号" field="username" rules={[{ required: true, message: '请输入账号' }]}>
            <Input placeholder="登录账号，字母/数字，如 hongniang" />
          </Form.Item>
          <Form.Item
            label="初始密码"
            field="password"
            rules={[
              { required: true, message: '请设置密码' },
              { minLength: 8, message: '密码至少 8 位' },
              { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' },
            ]}
          >
            <Input.Password placeholder="至少 8 位，含字母和数字" />
          </Form.Item>
          <Form.Item label="昵称" field="nickname">
            <Input />
          </Form.Item>
          <Form.Item
            label="开通时长"
            field="open_months"
            rules={[{ required: true, message: '请选择开通时长' }]}
          >
            <InputNumber
              min={1}
              max={36}
              precision={0}
              suffix="个月"
              style={{ width: '100%' }}
              placeholder="1 - 36 个月"
            />
          </Form.Item>
          {!!months && months > 0 && (
            <div
              style={{
                padding: '8px 12px',
                borderRadius: 8,
                background: 'var(--color-fill-2)',
                fontSize: 12,
                color: 'var(--color-text-2)',
              }}
            >
              服务有效期：从今日起开通 {months} 个月，截止{' '}
              {(() => {
                const d = new Date();
                d.setMonth(d.getMonth() + Number(months));
                return d.toLocaleDateString();
              })()}
            </div>
          )}
        </Form>
      </Modal>

      {/* 重置密码 */}
      <Modal
        title={`重置密码 - ${pwdUser?.username || ''}`}
        visible={!!pwdUser}
        onCancel={() => {
          setPwdUser(null);
          pwdForm.resetFields();
        }}
        onOk={resetPwd}
        confirmLoading={pwdSaving}
        okText="重置"
        cancelText="取消"
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item
            label="新密码"
            field="password"
            rules={[
              { required: true, message: '请输入新密码' },
              { minLength: 8, message: '密码至少 8 位' },
              { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' },
            ]}
          >
            <Input.Password placeholder="至少 8 位，含字母和数字" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看明文密码（小眼睛） */}
      <Modal
        title={`查看密码 - ${plainUser?.username || ''}`}
        visible={!!plainUser}
        onCancel={() => setPlainUser(null)}
        footer={
          <Button onClick={() => setPlainUser(null)} type="primary">
            知道了
          </Button>
        }
        maskClosable={false}
        style={{ width: 420 }}
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          该客户当前登录密码如下（SaaS 端可查看，便于线下协助客户登录）：
        </Typography.Paragraph>
        {plainLoading ? (
          <Typography.Text type="secondary">解密中…</Typography.Text>
        ) : (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '16px 0',
              borderRadius: 10,
              background: 'var(--color-fill-2)',
              fontSize: 24,
              fontWeight: 600,
              fontFamily: 'Consolas, Menlo, monospace',
              letterSpacing: 2,
            }}
          >
            {plainPwd}
          </div>
        )}
      </Modal>
    </Card>
  );
}
