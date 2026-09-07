import { useState, useEffect } from 'react';
import { Card, Table, Button, Modal, Form, Input, InputNumber, Switch, Message, Space, Tag, Popconfirm } from '@arco-design/web-react';
import { IconPlus, IconEdit, IconDelete } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 总后台 · 价格套餐设置：客户端充值中心「选择套餐」所用套餐的增删改查
export default function Plans() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      setList((await api.listAllPlans()) || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const open = (row?: any) => {
    setEditing(row || null);
    if (row) {
      form.setFieldsValue({
        name: row.name, points: row.points,
        price: row.price_fen / 100,
        orig_price: row.orig_fen ? row.orig_fen / 100 : undefined,
        tag: row.tag, sort: row.sort, enabled: row.enabled,
        daily_query_limit: row.daily_query_limit ?? 0,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({ points: 100, price: 99, sort: 0, enabled: true, daily_query_limit: 0 });
    }
    setVisible(true);
  };

  const save = async () => {
    const v = await form.validate();
    const body = {
      name: v.name, points: v.points,
      price_fen: Math.round((v.price || 0) * 100),
      orig_fen: v.orig_price ? Math.round(v.orig_price * 100) : 0,
      tag: v.tag || '', sort: v.sort || 0, enabled: v.enabled,
      daily_query_limit: Number(v.daily_query_limit ?? 0),
    };
    try {
      if (editing) await api.updatePlan(editing.id, body);
      else await api.createPlan(body);
      Message.success('已保存');
      setVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const del = (row: any) => {
    Modal.confirm({
      title: '删除套餐',
      content: `确认删除套餐「${row.name}」？`,
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        try {
          await api.deletePlan(row.id);
          Message.success('已删除');
          load();
        } catch (e: any) {
          Message.error(e.message);
        }
      },
    });
  };

  const columns = [
    { title: '排序', dataIndex: 'sort', width: 60 },
    { title: '套餐名', dataIndex: 'name' },
    { title: 'token 数', dataIndex: 'points', width: 100, render: (v: number) => <b>{v}</b> },
    { title: '售价(元)', dataIndex: 'price_fen', width: 110, render: (v: number) => <b style={{ color: '#F53F3F' }}>¥{(v / 100).toFixed(2)}</b> },
    { title: '原价(元)', dataIndex: 'orig_fen', width: 110, render: (v: number) => v ? <span style={{ textDecoration: 'line-through', color: '#86909C' }}>¥{(v / 100).toFixed(2)}</span> : '-' },
    { title: '标签', dataIndex: 'tag', width: 120, render: (v: string) => v ? <Tag color="red">{v}</Tag> : '-' },
    { title: '每日上限', dataIndex: 'daily_query_limit', width: 90, render: (v: number) => (v && v > 0) ? <b style={{ color: '#4F46E5' }}>{v} 次</b> : '-' },
    { title: '启用', dataIndex: 'enabled', width: 80, render: (v: boolean) => v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag> },
    {
      title: '操作', width: 140,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" icon={<IconEdit />} onClick={() => open(row)}>编辑</Button>
          <Popconfirm title="确认删除该套餐？" onOk={() => del(row)}>
            <Button size="mini" status="danger" icon={<IconDelete />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="价格套餐设置"
      extra={<Button type="primary" icon={<IconPlus />} onClick={() => open()}>新增套餐</Button>}
      style={{ borderRadius: 12, marginTop: 16 }}
    >
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 12 }}>
        客户端充值中心会展示「选择套餐」，客户选择套餐后按套餐优惠价扫码充值。留空原价则不显示划线优惠。
      </div>
      <Table rowKey="id" columns={columns as any} data={list} loading={loading} pagination={false} />

      <Modal title={editing ? '编辑套餐' : '新增套餐'} visible={visible} onOk={save} onCancel={() => setVisible(false)} okText="保存" cancelText="取消" unmountOnExit>
        <Form form={form} layout="vertical" style={{ marginTop: 8 }}>
          <Form.Item label="套餐名" field="name" rules={[{ required: true, message: '请输入套餐名' }]}>
            <Input placeholder="如：基础包 / 专业包" />
          </Form.Item>
          <Form.Item label="token 数" field="points" rules={[{ required: true, message: '请输入 token 数' }]}>
            <InputNumber min={1} style={{ width: '100%' }} placeholder="如：100" />
          </Form.Item>
          <Form.Item label="售价（元）" field="price" rules={[{ required: true, message: '请输入售价' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} placeholder="如：99.00" />
          </Form.Item>
          <Form.Item label="原价（元，选填）" field="orig_price">
            <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="留空则不显示划线原价" />
          </Form.Item>
          <Form.Item label="标签（选填）" field="tag">
            <Input placeholder="如：最受欢迎 / 限时特惠" />
          </Form.Item>
          <Form.Item label="每日查询上限（购买后解锁）" field="daily_query_limit" extra="0 = 不改变客户当前上限；填 30 表示购买后客户每天可查 30 次。">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="如：30" />
          </Form.Item>
          <Form.Item label="排序（越小越靠前）" field="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="启用" field="enabled" triggerPropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
