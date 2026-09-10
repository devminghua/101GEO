import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Tag, Space, Message, Popconfirm, Switch, Typography, InputNumber,
} from '@arco-design/web-react';
import { IconPlus, IconEdit, IconExperiment, IconEye } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 渠道管理：渠道商可自建分站、自定义品牌与客服联系方式。
// 创建渠道时同步创建渠道登录账号（role=channel），渠道登录后进入渠道后台。
// 平台可给渠道充值点数，渠道从自己余额拨付给旗下客户。
export default function Channels() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  // 渠道充值
  const [rechargeCh, setRechargeCh] = useState<any>(null);
  const [rechargeForm] = Form.useForm();
  // 渠道密码查看
  const [pwdView, setPwdView] = useState<any>(null);
  const [plainPwd, setPlainPwd] = useState('');
  const [plainLoading, setPlainLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setList((await api.listChannels()) || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setVisible(true);
  };
  const openEdit = (row: any) => {
    setEditing(row);
    form.setFieldsValue({
      name: row.name, brand_name: row.brand_name, copyright: row.copyright,
      service_phone: row.service_phone, service_wechat: row.service_wechat,
    });
    setVisible(true);
  };

  const submit = async () => {
    const v = await form.validate();
    setSaving(true);
    try {
      if (editing) {
        await api.updateChannel(editing.id, v);
        // 编辑时选填重置密码
        if (v.password) {
          await api.superChannelResetPassword(editing.id, v.password);
        }
        Message.success('渠道已更新');
      } else {
        const r: any = await api.createChannel(v);
        Message.success(`渠道已创建，登录账号：${r?.username || v.username}`);
      }
      setVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  // 查看渠道登录密码
  const showPlain = async (row: any) => {
    setPwdView(row);
    setPlainPwd('');
    setPlainLoading(true);
    try {
      const d: any = await api.superChannelPlaintext(row.id);
      setPlainPwd(d.plaintext || '');
    } catch (e: any) {
      Message.error(e.message);
      setPwdView(null);
    } finally {
      setPlainLoading(false);
    }
  };

  const toggle = async (row: any, checked: boolean) => {
    try {
      await api.updateChannelStatus(row.id, checked ? 1 : 0);
      Message.success(checked ? '已启用' : '已停用');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // ===== 一键登录渠道后台（备份总后台凭证，可一键切回） =====
  const simulate = async (row: any) => {
    if (row.status !== 1) return Message.warning('该渠道已停用，请先启用');
    try {
      const res: any = await api.superChannelSimulateLogin(row.id);
      if (!res || !res.token || !res.user) return Message.error('登录失败：未返回有效凭证');
      localStorage.setItem('geo_super_backup', JSON.stringify({
        token: localStorage.getItem('geo_token') || '',
        user: JSON.parse(localStorage.getItem('geo_user') || 'null'),
      }));
      localStorage.setItem('geo_token', res.token);
      localStorage.setItem('geo_user', JSON.stringify(res.user));
      Message.success(`已进入「${row.name}」渠道后台`);
      setTimeout(() => window.location.reload(), 600);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // ===== 渠道充值 =====
  const openRecharge = (row: any) => {
    setRechargeCh(row);
    rechargeForm.resetFields();
    rechargeForm.setFieldsValue({ amount: 100 });
  };
  const submitRecharge = async () => {
    const v = await rechargeForm.validate();
    setSaving(true);
    try {
      await api.channelRecharge(rechargeCh.id, { amount: Number(v.amount), remark: (v.remark || '').trim() });
      Message.success(`已为渠道「${rechargeCh.name}」充值 ${v.amount} 点`);
      setRechargeCh(null);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const columns = [
    {
      title: '渠道', width: 170,
      render: (_: any, row: any) => (
        <div>
          <div style={{ fontWeight: 600 }}>{row.name}</div>
          <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>账号：{row.username || '-'}</div>
        </div>
      ),
    },
    {
      title: '品牌', width: 180,
      render: (_: any, row: any) => (
        <div style={{ fontSize: 13 }}>
          {row.brand_name ? <div>{row.brand_name}</div> : <span style={{ color: 'var(--color-text-3)' }}>未设置（用默认品牌）</span>}
        </div>
      ),
    },
    {
      title: '客服', width: 180,
      render: (_: any, row: any) => (
        <div style={{ fontSize: 12 }}>
          {row.service_wechat ? <div>微信：{row.service_wechat}</div> : null}
          {row.service_phone ? <div>电话：{row.service_phone}</div> : null}
          {!row.service_wechat && !row.service_phone ? <span style={{ color: 'var(--color-text-3)' }}>未设置</span> : null}
        </div>
      ),
    },
    { title: '旗下分站', dataIndex: 'tenant_count', width: 90, render: (v: number) => <Tag color="arcoblue">{v} 个</Tag> },
    {
      title: '渠道点数余额', width: 130,
      render: (_: any, row: any) => (
        <div>
          <span style={{ fontWeight: 600 }}>{row.points ?? 0} 点</span>
          <Button size="mini" type="text" style={{ marginLeft: 4 }} onClick={() => openRecharge(row)}>充值</Button>
        </div>
      ),
    },
    {
      title: '状态', width: 90,
      render: (_: any, row: any) => (
        <Switch size="small" checked={row.status === 1} checkedText="启用" uncheckedText="停用" onChange={(c) => toggle(row, c)} />
      ),
    },
    {
      title: '操作', width: 200,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" icon={<IconEye />} onClick={() => showPlain(row)}>密码</Button>
          <Button size="mini" icon={<IconEdit />} onClick={() => openEdit(row)}>编辑</Button>
          <Button size="mini" icon={<IconExperiment />} onClick={() => simulate(row)}>一键登录</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
        <Typography.Paragraph type="secondary" style={{ fontSize: 13, margin: 0, flex: 1 }}>
          渠道商体系：每个渠道可自建分站、自定义品牌与客服联系方式；渠道登录自己的后台管理旗下客户。
        </Typography.Paragraph>
        <Button size="small" type="primary" icon={<IconPlus />} onClick={openCreate}>新建渠道</Button>
      </div>

      <Card title="渠道列表" style={{ borderRadius: 12 }} bordered>
        <Table rowKey="id" columns={columns} data={list} loading={loading} pagination={false} scroll={{ x: 1000 }} />
      </Card>

      {/* 渠道充值 */}
      <Modal
        title={`为渠道「${rechargeCh?.name || ''}」充值点数`}
        visible={!!rechargeCh}
        onCancel={() => setRechargeCh(null)}
        onOk={submitRecharge}
        confirmLoading={saving}
      >
        <Form form={rechargeForm} layout="vertical">
          <Form.Item label="充值点数" field="amount" rules={[{ required: true, message: '请输入点数' }]}>
            <InputNumber min={1} max={1000000} precision={0} style={{ width: '100%' }} placeholder="如 1000" />
          </Form.Item>
          <Form.Item label="备注（选填）" field="remark">
            <Input placeholder="如：9 月渠道采购" />
          </Form.Item>
        </Form>
        <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
          充值后进入渠道余额，渠道可在自己的后台把点数拨付给旗下客户。
        </Typography.Paragraph>
      </Modal>

      {/* 查看渠道登录密码 */}
      <Modal
        title={`渠道「${pwdView?.name || ''}」登录密码`}
        visible={!!pwdView}
        onCancel={() => setPwdView(null)}
        footer={<Button onClick={() => setPwdView(null)}>关闭</Button>}
      >
        <div style={{ fontSize: 13, marginBottom: 8 }}>
          登录账号：<b>{pwdView?.username || '-'}</b>
        </div>
        <div style={{ fontSize: 13 }}>
          明文密码：{plainLoading ? '查询中…' : <b style={{ fontSize: 16, letterSpacing: 1 }}>{plainPwd || '-'}</b>}
        </div>
        <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 10, marginBottom: 0 }}>
          密码可直接转发给渠道商用于登录渠道后台。
        </Typography.Paragraph>
      </Modal>

      <Modal
        title={editing ? '编辑渠道' : '新建渠道'}
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={submit}
        confirmLoading={saving}
        style={{ width: 560 }}
      >
        <Form form={form} layout="vertical">
          <Form.Item label="渠道名称" field="name" rules={[{ required: true, message: '请输入渠道名称' }]}>
            <Input placeholder="如：华东渠道商" />
          </Form.Item>
          {!editing && (
            <>
              <Form.Item label="渠道登录账号" field="username" rules={[{ required: true, message: '请输入登录账号' }]}>
                <Input placeholder="渠道登录后台使用的账号" />
              </Form.Item>
              <Form.Item label="渠道登录密码" field="password" rules={[{ required: true, message: '请输入登录密码' }]}>
                <Input.Password placeholder="至少 8 位，含字母和数字" />
              </Form.Item>
            </>
          )}
          {editing && (
            <Form.Item label="重置登录密码（选填）" field="password" extra="留空 = 保持原密码不变；填写则重置为新密码">
              <Input.Password placeholder="至少 8 位，含字母和数字" autoComplete="new-password" />
            </Form.Item>
          )}
          <Form.Item label="品牌名称（旗下分站默认品牌）" field="brand_name" extra="留空则其下分站使用平台默认品牌">
            <Input placeholder="如：XX 科技" />
          </Form.Item>
          <Form.Item label="版权文案" field="copyright">
            <Input placeholder="如：Copyright © XX All Rights Reserved" />
          </Form.Item>
          <Form.Item label="客服微信" field="service_wechat">
            <Input placeholder="显示在旗下分站客户端的客服入口" />
          </Form.Item>
          <Form.Item label="客服电话" field="service_phone">
            <Input placeholder="如：400-000-0000" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
