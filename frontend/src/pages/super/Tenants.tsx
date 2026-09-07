import { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Tag,
  Space,
  Message,
  Popconfirm,
  Switch,
  Typography,
  Checkbox,
  Divider,
  InputNumber,
  Upload,
} from '@arco-design/web-react';
import { IconPlus, IconExperiment } from '@arco-design/web-react/icon';
import { api } from '../../api';
import { FEATURES, ALL_FEATURE_KEYS } from '../../features';

export default function Tenants({ embedded = false }: { embedded?: boolean }) {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [saving, setSaving] = useState(false);
  const [selectedFeatures, setSelectedFeatures] = useState<string[]>([]);
  const [form] = Form.useForm();
  // 分站 Logo（SaaS 端上传，客户端调用）
  const [logo, setLogo] = useState('');
  const [logoUploading, setLogoUploading] = useState(false);
  // 点卡充值
  const [rechargeVisible, setRechargeVisible] = useState(false);
  const [rechargeTenant, setRechargeTenant] = useState<any>(null);
  const [rechargeForm] = Form.useForm();
  // 支付通道信息（点卡单价 / 微信支付宝启用状态），用于充值弹窗提示
  const [payInfo, setPayInfo] = useState<any>(null);

  const load = async () => {
    setLoading(true);
    try {
      const data = await api.listTenants();
      setList(data || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // 支付通道状态（读取失败不影响充值，仅不展示提示）
    api
      .payGetConfig()
      .then((cfg: any) => setPayInfo(cfg || {}))
      .catch(() => {});
  }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setSelectedFeatures([]); // 新建分站默认全量授权
    setLogo('');
    setModalVisible(true);
  };

  const openEdit = (row: any) => {
    setEditing(row);
    form.setFieldsValue({ name: row.name, remark: row.remark, status: row.status === 1 });
    setSelectedFeatures(Array.isArray(row.features) ? row.features : []); // 空数组=全量授权
    setLogo(row.logo || '');
    setModalVisible(true);
  };

  // 上传分站 Logo
  const uploadLogo = async (file: File) => {
    setLogoUploading(true);
    try {
      const res = await api.uploadImage(file, 'tenant_logo');
      setLogo(res.url);
      Message.success('Logo 已上传');
    } catch (e: any) {
      Message.error(e.message || '上传失败');
    } finally {
      setLogoUploading(false);
    }
  };

  const submit = async () => {
    const values = await form.validate();
    setSaving(true);
    try {
      if (editing) {
        await api.updateTenant(editing.id, {
          name: values.name,
          logo: logo,
          remark: values.remark,
          status: values.status ? 1 : 0,
          features: selectedFeatures,
        });
        Message.success('分站已更新');
      } else {
        await api.createTenant({
          name: values.name,
          code: values.code,
          logo: logo,
          remark: values.remark,
          features: selectedFeatures,
        });
        Message.success('分站已创建');
      }
      setModalVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (row: any, checked: boolean) => {
    try {
      await api.updateTenant(row.id, { status: checked ? 1 : 0 });
      Message.success(checked ? '已启用该分站' : '已停用该分站');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const remove = async (row: any) => {
    try {
      await api.deleteTenant(row.id);
      Message.success('分站及其账号、业务数据已删除');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // 总后台一键模拟登录指定分站：备份当前总后台登录态，切换为分站账号
  const simulate = async (row: any) => {
    if (row.status !== 1) {
      Message.warning('该分站已停用，请先启用后再模拟登录');
      return;
    }
    try {
      const res = await api.simulateLogin(row.id);
      if (!res || !res.token || !res.user) {
        Message.error('模拟登录失败：未返回有效登录信息');
        return;
      }
      localStorage.setItem(
        'geo_super_backup',
        JSON.stringify({
          token: localStorage.getItem('geo_token') || '',
          user: JSON.parse(localStorage.getItem('geo_user') || 'null'),
        })
      );
      localStorage.setItem('geo_token', res.token);
      localStorage.setItem('geo_user', JSON.stringify(res.user));
      Message.success(`已模拟登录「${row.name}」分站后台`);
      setTimeout(() => window.location.reload(), 600);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // 总后台为分站点卡充值
  const openRecharge = (row: any) => {
    setRechargeTenant(row);
    rechargeForm.resetFields();
    rechargeForm.setFieldsValue({ amount: 100 });
    setRechargeVisible(true);
  };

  const submitRecharge = async () => {
    const values = await rechargeForm.validate();
    setSaving(true);
    try {
      await api.rechargeTenant(rechargeTenant.id, {
        amount: Number(values.amount),
        remark: (values.remark || '').trim(),
      });
      Message.success(`已为「${rechargeTenant.name}」充值 ${values.amount} 点`);
      setRechargeVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '分站标识', dataIndex: 'code', width: 120, render: (v: string) => <Tag color="arcoblue">{v}</Tag> },
    { title: '分站名称', dataIndex: 'name' },
    {
      title: '点卡余额',
      dataIndex: 'points',
      width: 110,
      render: (v: number, row: any) => <span style={{ fontWeight: 600 }}>{v ?? 0} 点</span>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v: number) => (v === 1 ? <Tag color="green">正常</Tag> : <Tag color="red">停用</Tag>),
    },
    {
      title: '功能授权',
      dataIndex: 'features',
      width: 200,
      render: (v: any) => {
        const feats = Array.isArray(v) ? v : [];
        if (feats.length === 0) return <Tag color="green">全功能</Tag>;
        const short = FEATURES.filter((f) => feats.includes(f.key)).map((f) => f.label);
        const text = short.join('、');
        return <Tag color="arcoblue">{text.length > 18 ? text.slice(0, 18) + '…' : text}</Tag>;
      },
    },
    { title: '备注', dataIndex: 'remark', render: (v: string) => v || '-' },
    {
      title: '账号数',
      dataIndex: 'account_count',
      width: 90,
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
      render: (_: any, row: any) => (
        <Space>
          <Switch
            size="small"
            checked={row.status === 1}
            checkedText="启用"
            uncheckedText="停用"
            onChange={(checked) => toggleStatus(row, checked)}
          />
          <Button size="mini" onClick={() => openRecharge(row)}>
            充值
          </Button>
          <Button size="mini" icon={<IconExperiment />} onClick={() => simulate(row)}>
            模拟登录
          </Button>
          <Button size="mini" onClick={() => openEdit(row)}>
            编辑
          </Button>
          <Popconfirm title={`确定删除分站「${row.name}」及其全部数据？`} onOk={() => remove(row)} okText="删除" cancelText="取消">
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
          <span>分站管理</span>
          <Button type="primary" icon={<IconPlus />} onClick={openCreate}>
            建立分站
          </Button>
        </div>
      }
      style={{ borderRadius: 12 }}
      bordered
    >
      {!embedded && (
        <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 16, fontSize: 13 }}>
          每个分站拥有独立的关键词、AI 平台、任务与结果数据，互不可见。建立分站后，可在「账号」页签中为其创建带密码的登录账号。
        </Typography.Text>
      )}
      <Table rowKey="id" columns={columns} data={list} loading={loading} pagination={false} />

      <Modal
        title={editing ? '编辑分站' : '建立分站'}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={submit}
        confirmLoading={saving}
        okText="保存"
        cancelText="取消"
        maskClosable={false}
      >
        <Form form={form} layout="vertical">
          <Form.Item label="分站名称" field="name" rules={[{ required: true, message: '请输入分站名称' }]}>
            <Input placeholder="如：婚恋高定服务" />
          </Form.Item>
          <Form.Item
            label="分站标识（唯一）"
            field="code"
            rules={[{ required: true, message: '请输入分站标识' }]}
            extra={editing ? '创建后不可修改' : '由字母/数字组成，用于分站区分与账号归属，如 match01'}
          >
            <Input placeholder="如 match01" disabled={!!editing} />
          </Form.Item>
          <Form.Item label="分站 Logo" extra="客户端侧栏头像将调用此 Logo（未设置则用账号头像）">
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              {logo ? (
                <img src={logo} alt="logo" style={{ width: 40, height: 40, borderRadius: 8, objectFit: 'cover', border: '1px solid var(--color-border-2)' }} />
              ) : (
                <div style={{ width: 40, height: 40, borderRadius: 8, background: 'var(--color-fill-2)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#C9CDD4', fontSize: 11 }}>无</div>
              )}
              <Upload
                showUploadList={false}
                accept="image/png,image/jpeg,image/webp,image/gif"
                beforeUpload={(file: File) => { uploadLogo(file); return false; }}
              >
                <Button loading={logoUploading}>{logo ? '更换 Logo' : '上传 Logo'}</Button>
              </Upload>
              {logo && <Button type="text" onClick={() => setLogo('')}>清除</Button>}
            </div>
          </Form.Item>
          <Form.Item label="备注" field="remark">
            <Input.TextArea placeholder="选填" rows={2} />
          </Form.Item>
          {editing && (
            <Form.Item label="状态" field="status" triggerPropName="checked">
              <Switch checkedText="正常" uncheckedText="停用" />
            </Form.Item>
          )}
          <Divider style={{ margin: '4px 0 16px' }} />
          <Form.Item label="功能授权" extra="勾选开放的功能；不勾选任何功能 = 全部开放。未开放功能在该分站客户端不显示菜单、路由拦截且接口返回 403。">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <Button size="mini" onClick={() => setSelectedFeatures([...ALL_FEATURE_KEYS])}>
                  全选
                </Button>
                <Button size="mini" onClick={() => setSelectedFeatures([])}>
                  全不选（全部开放）
                </Button>
                <Typography.Text type={selectedFeatures.length === 0 ? 'success' : 'secondary'}>
                  {selectedFeatures.length === 0 ? '已选择：全部开放' : `已勾选 ${selectedFeatures.length} 项`}
                </Typography.Text>
              </Space>
              <Checkbox.Group
                value={selectedFeatures}
                onChange={(val) => setSelectedFeatures((val as string[]) || [])}
                style={{ width: '100%' }}
              >
                <Space direction="vertical" style={{ width: '100%' }}>
                  {FEATURES.map((f) => (
                    <Checkbox key={f.key} value={f.key}>
                      {f.label}
                    </Checkbox>
                  ))}
                </Space>
              </Checkbox.Group>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 点卡充值弹窗 */}
      <Modal
        title={`为「${rechargeTenant?.name || ''}」充值点卡`}
        visible={rechargeVisible}
        onCancel={() => setRechargeVisible(false)}
        onOk={submitRecharge}
        confirmLoading={saving}
        okText="确认充值"
        cancelText="取消"
        maskClosable={false}
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          当前余额：<b>{rechargeTenant?.points ?? 0}</b> 点。充值后分站 AI 调用按次扣点（每次 1 点）。
        </Typography.Paragraph>
        {payInfo && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              flexWrap: 'wrap',
              padding: '6px 12px',
              marginBottom: 12,
              borderRadius: 8,
              background: 'var(--color-fill-2)',
              fontSize: 12,
              color: 'var(--color-text-2)',
            }}
          >
            <span>
              点卡单价：
              <b style={{ color: '#F77234' }}>
                ¥{((Number(payInfo.point_price_fen) || 100) / 100).toFixed(2)}
              </b>
              /点
            </span>
            <span>
              客户自助扫码：
              <Tag size="small" color={payInfo.wechat_pay_enabled === '1' ? 'green' : 'gray'}>
                微信{payInfo.wechat_pay_enabled === '1' ? '已启用' : '未配置'}
              </Tag>
              <Tag size="small" color={payInfo.alipay_pay_enabled === '1' ? 'green' : 'gray'}>
                支付宝{payInfo.alipay_pay_enabled === '1' ? '已启用' : '未配置'}
              </Tag>
            </span>
            <span style={{ color: 'var(--color-text-3)' }}>密钥配置见「支付设置」</span>
          </div>
        )}
        <Form form={rechargeForm} layout="vertical">
          <Form.Item
            label="充值点数"
            field="amount"
            rules={[
              { required: true, message: '请输入充值点数' },
              {
                validator: (v: any, cb: any) => {
                  const n = Number(v);
                  if (!Number.isInteger(n) || n <= 0) cb('充值点数须为正整数');
                  else cb();
                },
              },
            ]}
          >
            <InputNumber min={1} precision={0} placeholder="如 1000" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="备注" field="remark">
            <Input placeholder="选填，如：月度充值" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
