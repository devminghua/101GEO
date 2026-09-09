import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Tag, Space, Message, Popconfirm,
  Switch, Typography, Checkbox, Divider, InputNumber, Select, Tooltip, Radio,
} from '@arco-design/web-react';
import { IconPlus, IconEye, IconExperiment, IconQrcode } from '@arco-design/web-react/icon';
import { api } from '../../api';
import { FEATURES, ALL_FEATURE_KEYS, featureLabel } from '../../features';

// 渠道端客户管理（拷贝总后台客户管理全部能力，范围限定自己渠道的分站）。
// 一个客户 = 一个分站 + 一个登录账号；开通/续费/充值/密码/停用/模拟登录/删除全在此页。
// 开通客户：填客户名称 + 登录账号 + 登录密码 + 勾选功能 + 开通时长，一键开通、立即可用。
// 后续管理（查看/重置密码、充值、续费、改功能、停用、模拟登录、删除）全部在同一张表内完成。
export default function ChannelCustomers() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  // 开通/编辑
  const [createVisible, setCreateVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [saving, setSaving] = useState(false);
  const [selectedFeatures, setSelectedFeatures] = useState<string[]>([]);
  const [form] = Form.useForm();
  const openMonths = Form.useWatch('open_months', form) ?? 12;
  const isTrial = Form.useWatch('trial', form) ?? false;

  // 密码查看 / 重置
  const [pwdViewUser, setPwdViewUser] = useState<any>(null);
  const [plainPwd, setPlainPwd] = useState('');
  const [plainLoading, setPlainLoading] = useState(false);
  const [pwdResetUser, setPwdResetUser] = useState<any>(null);
  const [pwdSaving, setPwdSaving] = useState(false);
  const [pwdForm] = Form.useForm();

  // 充值
  const [rechargeTenant, setRechargeTenant] = useState<any>(null);
  const [rechargeForm] = Form.useForm();
  // 渠道自己的点数余额（给客户充值从该余额扣除）
  const [channelPoints, setChannelPoints] = useState(0);

  // 续费
  const [extendUser, setExtendUser] = useState<any>(null);
  const [extendForm] = Form.useForm();
  const extMonths = Form.useWatch('months', extendForm) ?? 1;

  const load = async () => {
    setLoading(true);
    try {
      const cs = await api.channelListCustomers();
      setList(cs || []);
      // 顺带刷新渠道余额（给客户充值从该余额扣除）
      api.channelProfile().then((p: any) => setChannelPoints(p?.points || 0)).catch(() => {});
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  // ===== 开通客户 =====
  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ open_months: 12, trial: false });
    setSelectedFeatures([]); // 默认全量授权
    setCreateVisible(true);
  };

  const openEdit = (row: any) => {
    setEditing(row);
    form.setFieldsValue({ name: row.name, remark: row.remark, status: row.status === 1, daily_query_limit: row.daily_query_limit ?? 3 });
    setSelectedFeatures(Array.isArray(row.features) ? row.features : []);
    setCreateVisible(true);
  };

  const submit = async () => {
    const values = await form.validate();
    setSaving(true);
    try {
      if (editing) {
        await api.channelUpdateTenant(editing.id, {
          name: values.name,
          remark: values.remark,
          status: values.status ? 1 : 0,
          features: selectedFeatures,
          daily_query_limit: Number(values.daily_query_limit ?? 3),
        });
        Message.success('客户信息已更新');
      } else {
        const r = await api.channelCreateCustomer({
          name: values.name,
          username: values.username,
          password: values.password,
          nickname: values.nickname,
          open_months: Number(values.open_months) || 12,
          trial: !!values.trial,
          features: selectedFeatures,
          remark: values.remark,
        });
        Message.success(r?.msg || '客户已开通，可立即登录使用');
      }
      setCreateVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  // ===== 密码 =====
  const showPlain = async (row: any) => {
    if (!row.user_id) return Message.warning('该客户暂无登录账号');
    setPwdViewUser(row);
    setPlainPwd('');
    setPlainLoading(true);
    try {
      const d: any = await api.channelUserPlaintext(row.user_id);
      setPlainPwd(d.plaintext);
    } catch (e: any) {
      Message.error(e.message);
      setPwdViewUser(null);
    } finally {
      setPlainLoading(false);
    }
  };

  const resetPwd = async () => {
    const values = await pwdForm.validate();
    setPwdSaving(true);
    try {
      await api.channelResetPassword(pwdResetUser.user_id, values.password);
      Message.success(`已重置「${pwdResetUser.username}」的密码`);
      setPwdResetUser(null);
      pwdForm.resetFields();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setPwdSaving(false);
    }
  };

  // ===== 充值 =====
  const openRecharge = (row: any) => {
    setRechargeTenant(row);
    rechargeForm.resetFields();
    rechargeForm.setFieldsValue({ amount: 100 });
  };
  const submitRecharge = async () => {
    const values = await rechargeForm.validate();
    setSaving(true);
    try {
      await api.channelRechargeTenant(rechargeTenant.id, {
        amount: Number(values.amount),
        remark: (values.remark || '').trim(),
      });
      Message.success(`已为「${rechargeTenant.name}」充值 ${values.amount} 点`);
      setRechargeTenant(null);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  // ===== 续费 =====
  const openExtend = (row: any) => {
    if (!row.user_id) return Message.warning('该客户暂无登录账号');
    setExtendUser(row);
    extendForm.resetFields();
    extendForm.setFieldsValue({ months: 12, pay_method: '微信' });
  };
  const submitExtend = async () => {
    const values = await extendForm.validate();
    setSaving(true);
    try {
      await api.channelExtendUser(extendUser.user_id, Number(values.months), '', values.pay_method);
      Message.success(`已为「${extendUser.name}」续费 ${values.months} 个月`);
      setExtendUser(null);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  // ===== 停用 / 模拟登录 / 删除 =====
  const toggleStatus = async (row: any, checked: boolean) => {
    try {
      await api.channelUpdateTenant(row.id, { status: checked ? 1 : 0 });
      Message.success(checked ? '已启用该客户' : '已停用该客户');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const simulate = async (row: any) => {
    if (row.status !== 1) return Message.warning('该客户已停用，请先启用');
    try {
      const res = await api.channelSimulateLogin(row.id);
      if (!res || !res.token || !res.user) return Message.error('模拟登录失败：未返回有效登录信息');
      localStorage.setItem('geo_super_backup', JSON.stringify({
        token: localStorage.getItem('geo_token') || '',
        user: JSON.parse(localStorage.getItem('geo_user') || 'null'),
      }));
      localStorage.setItem('geo_token', res.token);
      localStorage.setItem('geo_user', JSON.stringify(res.user));
      Message.success(`已模拟登录「${row.name}」客户后台`);
      setTimeout(() => window.location.reload(), 600);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const remove = async (row: any) => {
    try {
      await api.channelDeleteTenant(row.id);
      Message.success('客户及其账号、业务数据已删除');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const renderFeatures = (v: any) => {
    const feats = Array.isArray(v) ? v : [];
    if (feats.length === 0) return <Tag color="green">全功能</Tag>;
    const text = feats.map(featureLabel).join('、');
    return <Tag color="arcoblue">{text.length > 16 ? text.slice(0, 16) + '…' : text}</Tag>;
  };

  const renderExpiry = (_: any, row: any) => {
    if (!row.open_months || row.open_months <= 0) return <Tag color="gray">不限</Tag>;
    const days = row.remain_days;
    if (days <= 0) return <Tag color="red">已到期</Tag>;
    const color = days <= 30 ? 'orange' : 'green';
    return (
      <div>
        <Tag color={color}>剩余 {days} 天</Tag>
        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
          {row.expire_at ? `至 ${new Date(row.expire_at).toLocaleDateString()}` : ''}
        </div>
      </div>
    );
  };

  const columns = [
    {
      title: '客户 / 账号', width: 200,
      render: (_: any, row: any) => (
        <div>
          <div style={{ fontWeight: 600 }}>{row.name}</div>
          <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
            {row.username ? `账号：${row.username}` : <Tag size="small" color="red">无登录账号</Tag>}
          </div>
        </div>
      ),
    },
    {
      title: '密码', width: 130,
      render: (_: any, row: any) => (
        <Space size={4}>
          <Tooltip content="查看客户当前明文密码">
            <Button size="mini" type="text" icon={<IconEye />} disabled={!row.user_id} onClick={() => showPlain(row)} />
          </Tooltip>
          <Button size="mini" type="text" disabled={!row.user_id} onClick={() => setPwdResetUser(row)}>
            重置
          </Button>
        </Space>
      ),
    },
    { title: '功能授权', dataIndex: 'features', width: 160, render: renderFeatures },
    {
      title: '点卡余额', width: 150,
      render: (_: any, row: any) => (
        <div>
          <span style={{ fontWeight: 600 }}>{row.points ?? 0} 点</span>
          <Button size="mini" type="text" style={{ marginLeft: 6 }} onClick={() => openRecharge(row)}>
            充值
          </Button>
        </div>
      ),
    },
    { title: '服务有效期', width: 150, render: renderExpiry },
    {
      title: '每日查询上限', width: 110,
      render: (_: any, row: any) => (
        <span style={{ fontWeight: 600, color: (row.daily_query_limit ?? 3) === 0 ? '#00B42A' : undefined }}>
          {(row.daily_query_limit ?? 3) === 0 ? '不限' : `${row.daily_query_limit ?? 3} 次`}
        </span>
      ),
    },
    {
      title: '状态', width: 90,
      render: (_: any, row: any) => (
        <Switch size="small" checked={row.status === 1} checkedText="启用" uncheckedText="停用" onChange={(c) => toggleStatus(row, c)} />
      ),
    },
    {
      title: '创建时间', width: 150,
      render: (_: any, row: any) => (row.created_at ? new Date(row.created_at).toLocaleDateString() : '-'),
    },
    {
      title: '操作', width: 240,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" onClick={() => openExtend(row)}>续费</Button>
          <Button size="mini" icon={<IconExperiment />} onClick={() => simulate(row)}>模拟登录</Button>
          <Button size="mini" onClick={() => openEdit(row)}>编辑</Button>
          <Popconfirm title={`确定删除客户「${row.name}」及其全部数据？`} onOk={() => remove(row)} okText="删除" cancelText="取消">
            <Button size="mini" status="danger">删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
        <Typography.Paragraph type="secondary" style={{ fontSize: 13, margin: 0, flex: 1 }}>
          一个客户 = 一个分站 + 一个登录账号。开通即填「账号 + 密码 + 勾选功能」，无需单独建站、单独建号。
        </Typography.Paragraph>
        <Space>
          <Button size="small" type="primary" icon={<IconPlus />} onClick={openCreate}>开通客户</Button>
        </Space>
      </div>

      <Card title="客户 / 账号" style={{ borderRadius: 12, marginBottom: 16 }} bordered>
        <Table rowKey="id" columns={columns} data={list} loading={loading} pagination={false} scroll={{ x: 1100 }} />
      </Card>


      {/* 开通 / 编辑客户 */}
      <Modal
        title={editing ? '编辑客户' : '开通客户'}
        visible={createVisible}
        onCancel={() => setCreateVisible(false)}
        onOk={submit}
        confirmLoading={saving}
        okText={editing ? '保存' : '立即开通'}
        cancelText="取消"
        maskClosable={false}
        style={{ width: 560 }}
        unmountOnExit
      >
        <Form form={form} layout="vertical">
          <Form.Item label="客户名称" field="name" rules={[{ required: true, message: '请输入客户名称' }]}>
            <Input placeholder="如：婚恋高定服务" />
          </Form.Item>
          {!editing && (
            <>
              <Form.Item
                label="登录账号"
                field="username"
                rules={[
                  { required: true, message: '请输入登录账号' },
                  {
                    validator: (v: any, cb: any) => {
                      if (!/^[a-zA-Z0-9_-]+$/.test(String(v || ''))) cb('账号仅限字母/数字/下划线/连字符');
                      else cb();
                    },
                  },
                ]}
                extra="客户用此账号登录客户端，全局唯一"
              >
                <Input placeholder="如 hongniang01" autoComplete="off" />
              </Form.Item>
              <Form.Item
                label="登录密码"
                field="password"
                rules={[
                  { required: true, message: '请设置登录密码' },
                  { minLength: 8, message: '密码至少 8 位' },
                  { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' },
                ]}
              >
                <Input.Password placeholder="至少 8 位，含字母和数字" autoComplete="new-password" />
              </Form.Item>
              <Form.Item label="昵称（选填）" field="nickname">
                <Input />
              </Form.Item>
              {!editing && (
                <Form.Item label="开通方式" field="trial" initialValue={false}>
                  <Radio.Group type="button">
                    <Radio value={false}>正式（按月）</Radio>
                    <Radio value={true}>试用 7 天</Radio>
                  </Radio.Group>
                </Form.Item>
              )}
              {!editing && !isTrial && (
                <Form.Item
                  label="开通时长"
                  field="open_months"
                  rules={[{ required: true, message: '请选择开通时长' }]}
                >
                  <InputNumber min={1} max={36} precision={0} suffix="个月" style={{ width: '100%' }} placeholder="1 - 36 个月" />
                </Form.Item>
              )}
              {!editing && !isTrial && !!openMonths && openMonths > 0 && (
                <div style={{ padding: '8px 12px', borderRadius: 8, background: 'var(--color-fill-2)', fontSize: 12, color: 'var(--color-text-2)', marginBottom: 12 }}>
                  服务有效期：从今日起开通 {openMonths} 个月，截止{' '}
                  {(() => { const d = new Date(); d.setMonth(d.getMonth() + Number(openMonths)); return d.toLocaleDateString(); })()}
                </div>
              )}
              {!editing && isTrial && (
                <div style={{ padding: '8px 12px', borderRadius: 8, background: 'var(--color-fill-2)', fontSize: 12, color: 'var(--color-text-2)', marginBottom: 12 }}>
                  试用期：7 天，到期后需续费（与自助注册客户同款试用）。
                </div>
              )}
            </>
          )}
          <Form.Item label="备注" field="remark">
            <Input placeholder="选填" />
          </Form.Item>
          <Form.Item label="每日查询上限" field="daily_query_limit" extra="小红书/抖音/百度每日查询总次数上限，0 = 不限。默认 3 次，高级版本可解锁更多。">
            <InputNumber min={0} max={999} style={{ width: 160 }} placeholder="3" />
          </Form.Item>
          {editing && (
            <Form.Item label="状态" field="status" triggerPropName="checked">
              <Switch checkedText="正常" uncheckedText="停用" />
            </Form.Item>
          )}
          <Divider style={{ margin: '4px 0 16px' }} />
          <Form.Item label="功能授权" extra="勾选开放的功能；不勾选任何功能 = 全部开放。未开放功能在客户端不显示菜单、路由拦截且接口返回 403。">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <Button size="mini" onClick={() => setSelectedFeatures([...ALL_FEATURE_KEYS])}>全选</Button>
                <Button size="mini" onClick={() => setSelectedFeatures([])}>全不选（全部开放）</Button>
                <Typography.Text type={selectedFeatures.length === 0 ? 'success' : 'secondary'}>
                  {selectedFeatures.length === 0 ? '已选择：全部开放' : `已勾选 ${selectedFeatures.length} 项`}
                </Typography.Text>
              </Space>
              <Checkbox.Group value={selectedFeatures} onChange={(val) => setSelectedFeatures((val as string[]) || [])} style={{ width: '100%' }}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  {FEATURES.map((f) => (
                    <Checkbox key={f.key} value={f.key}>{f.label}</Checkbox>
                  ))}
                </Space>
              </Checkbox.Group>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看明文密码 */}
      <Modal
        title={`查看密码 - ${pwdViewUser?.username || ''}`}
        visible={!!pwdViewUser}
        onCancel={() => setPwdViewUser(null)}
        footer={<Button type="primary" onClick={() => setPwdViewUser(null)}>知道了</Button>}
        maskClosable={false}
        style={{ width: 420 }}
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          该客户当前登录密码（SaaS 端可查看，便于协助客户登录）：
        </Typography.Paragraph>
        {plainLoading ? (
          <Typography.Text type="secondary">解密中…</Typography.Text>
        ) : (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '16px 0', borderRadius: 10, background: 'var(--color-fill-2)', fontSize: 24, fontWeight: 600, fontFamily: 'Consolas, Menlo, monospace', letterSpacing: 2 }}>
            {plainPwd}
          </div>
        )}
      </Modal>

      {/* 重置密码 */}
      <Modal
        title={`重置密码 - ${pwdResetUser?.username || ''}`}
        visible={!!pwdResetUser}
        onCancel={() => { setPwdResetUser(null); pwdForm.resetFields(); }}
        onOk={resetPwd}
        confirmLoading={pwdSaving}
        okText="重置"
        cancelText="取消"
        maskClosable={false}
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item label="新密码" field="password" rules={[{ required: true, message: '请输入新密码' }, { minLength: 8, message: '密码至少 8 位' }, { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' }]}>
            <Input.Password placeholder="至少 8 位，含字母和数字" autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 充值 */}
      <Modal
        title={`为「${rechargeTenant?.name || ''}」充值点卡`}
        visible={!!rechargeTenant}
        onCancel={() => setRechargeTenant(null)}
        onOk={submitRecharge}
        confirmLoading={saving}
        okText="确认充值"
        cancelText="取消"
        maskClosable={false}
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          客户当前余额：<b>{rechargeTenant?.points ?? 0}</b> 点。充值后 AI 调用按次扣点（每次 1 点）。
        </Typography.Paragraph>
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          你的渠道余额：<b style={{ color: channelPoints > 0 ? '#00B42A' : '#F53F3F' }}>{channelPoints}</b> 点。
          本次充值将<b>从渠道余额扣除</b>，余额不足请联系平台充值。
        </Typography.Paragraph>
        <Form form={rechargeForm} layout="vertical">
          <Form.Item label="充值点数" field="amount" rules={[{ required: true, message: '请输入充值点数' }]}>
            <InputNumber min={1} precision={0} placeholder="如 1000" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="备注" field="remark">
            <Input placeholder="选填" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 续费 */}
      <Modal
        title={`续费 - ${extendUser?.name || ''}（账号 ${extendUser?.username || ''}）`}
        visible={!!extendUser}
        onCancel={() => setExtendUser(null)}
        onOk={submitExtend}
        confirmLoading={saving}
        okText="确认续费"
        cancelText="取消"
        maskClosable={false}
      >
        <Form form={extendForm} layout="vertical">
          <Form.Item label="续费月数" field="months" rules={[{ required: true, message: '请输入续费月数' }]}>
            <InputNumber min={1} max={36} precision={0} suffix="个月" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="收款方式" field="pay_method">
            <Select>
              {['微信', '支付宝', '银行转账', '现金', '赠送'].map((m) => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            金额 = 月单价 × {extMonths} 个月（赠送记 0），续费记录可在「续费记录」页签对账。
          </Typography.Text>
        </Form>
      </Modal>
    </div>
  );
}
