import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Message, Popconfirm,
  Space, Switch, TableColumnProps, Steps, Tag, Typography,
} from '@arco-design/web-react';
import { IconPlus, IconSettings, IconSync, IconDelete, IconCheckCircle, IconPlayArrow } from '@arco-design/web-react/icon';
import { api, getStoredUser } from '../api';

export default function Platforms() {
  const user = getStoredUser();
  const isSuper = user?.role === 'super';
  // 分站自有平台（tenant_id>0）可完整编辑/删除；继承的全局平台（tenant_id=0）只能覆盖 Key/启用、删除=恢复默认
  const isOwn = (row: any) => row && row.tenant_id > 0;
  const [list, setList] = useState<any[]>([]);
  const [templates, setTemplates] = useState<any[]>([]);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();
  const [testingId, setTestingId] = useState<number | null>(null);

  const load = async () => {
    const [p, t] = await Promise.all([api.listPlatforms(), api.platformTemplates()]);
    setList(p || []);
    setTemplates(t || []);
  };
  useEffect(() => { load(); }, []);

  const openAdd = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ enabled: true, sort_order: 0, interval_ms: 2000, sample_count: 1 });
    setVisible(true);
  };
  const openEdit = (row: any) => {
    setEditing(row);
    form.setFieldsValue({ ...row, api_key: '' });
    setVisible(true);
  };

  const applyTemplate = (tpl: any) => {
    form.setFieldsValue({ name: tpl.name, base_url: tpl.base_url, model: tpl.model });
  };

  const save = async () => {
    const values = await form.validate();
    // 排序/间隔/采样次数字段为数字输入框，Arco 取到的是 string，需转为 number 再提交
    for (const k of ['sort_order', 'interval_ms', 'sample_count']) {
      if (typeof values[k] === 'string') {
        const n = Number(values[k]);
        values[k] = Number.isNaN(n) || n < 0 ? 0 : n;
      }
    }
    // 采样次数约束 1~5
    if (typeof values.sample_count === 'number') {
      values.sample_count = Math.min(5, Math.max(1, values.sample_count || 1));
    }
    try {
      if (editing) {
        await api.updatePlatform(editing.id, values);
        Message.success('已更新');
      } else {
        await api.createPlatform(values);
        Message.success('已新增');
      }
      setVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const remove = async (id: number) => {
    try {
      await api.deletePlatform(id);
      Message.success('已删除');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const toggle = async (row: any, enabled: boolean) => {
    try {
      await api.updatePlatform(row.id, { ...row, enabled });
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const test = async (row: any) => {
    setTestingId(row.id);
    try {
      const r = await api.testPlatform(row.id);
      Message.success('连接正常: ' + (r.answer || ''));
      Message.success('连接正常');
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setTestingId(null);
    }
  };

  const columns: TableColumnProps[] = [
    { title: '平台名', dataIndex: 'name', render: (v) => <b>{v}</b> },
    { title: 'Base URL', dataIndex: 'base_url' },
    { title: '模型', dataIndex: 'model' },
    {
      title: '限流间隔', dataIndex: 'interval_ms', width: 100,
      render: (v: number) => (v ? `${(v / 1000).toFixed(v % 1000 ? 1 : 0)}s` : '默认'),
    },
    {
      title: 'API Key', dataIndex: 'api_key', width: 120,
      render: (v: string) => (v ? <Tag color="green">已配置</Tag> : <Tag color="red">未配置</Tag>),
    },
    {
      title: '启用', dataIndex: 'enabled', width: 80,
      render: (v: boolean, row: any) => <Switch checked={v} onChange={(c: any) => toggle(row, c)} />,
    },
    {
      title: '操作', width: 200,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" type="text" icon={<IconPlayArrow />} loading={testingId === row.id} onClick={() => test(row)}>
            测试
          </Button>
          <Button size="mini" type="text" icon={<IconSettings />} onClick={() => openEdit(row)}>编辑</Button>
          <Popconfirm content="确认删除该平台？" onOk={() => remove(row.id)}>
            <Button size="mini" type="text" status="danger" icon={<IconDelete />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card style={{ marginBottom: 16, borderRadius: 12 }}>
        <Steps current={3} style={{ maxWidth: 920 }} direction="horizontal" labelPlacement="vertical">
          <Steps.Step title="申请 API Key" description="在各平台官网开通" />
          <Steps.Step title="在此新增平台" description="可一键套用内置模板" />
          <Steps.Step title="测试连接" description="确认鉴权与网络通" />
          <Steps.Step title="开启巡检" description="勾选「启用」参与监控" />
        </Steps>
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 12 }}>
          基于 OpenAI 兼容协议统一接入，市面上主流 AI 平台（OpenAI / DeepSeek / Kimi / 通义 / 智谱 / 豆包 / 混元 / Ollama /
          以及任意提供 /v1/chat/completions 的服务）均可接入。
        </Typography.Text>
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          分站完全独立配置自己的 AI 平台与 API Key（独立计费），不继承总后台的全局平台配置。
        </Typography.Text>
      </Card>

      <Card
        title="AI 平台"
        extra={
          <Space>
            <Button icon={<IconSync />} onClick={load}>刷新</Button>
            <Button type="primary" icon={<IconPlus />} onClick={openAdd}>新增平台</Button>
          </Space>
        }
        style={{ borderRadius: 12 }}
      >
        <Table rowKey="id" columns={columns} data={list} pagination={false} />
      </Card>

      {/* 新增 / 编辑平台 */}
      <Modal
        title={editing ? '编辑平台' : '新增平台'}
        visible={visible}
        onOk={save}
        onCancel={() => setVisible(false)}
        style={{ width: 640 }}
        unmountOnExit
      >
        {!editing && (
          <div style={{ marginBottom: 16 }}>
            <Typography.Text style={{ display: 'block', marginBottom: 8, color: 'var(--color-text-2)' }}>
              快速套用模板：
            </Typography.Text>
            <Space wrap>
              {templates.map((t, i) => (
                <Button key={i} size="small" onClick={() => applyTemplate(t)}>{t.name}</Button>
              ))}
            </Space>
          </div>
        )}
        <Form form={form} layout="vertical">
          {(isSuper || !editing || isOwn(editing)) && (
            <>
              <Form.Item label="平台名" field="name" rules={[{ required: true, message: '请输入平台名' }]}>
                <Input placeholder="如：DeepSeek" />
              </Form.Item>
              <Form.Item label="API Base URL" field="base_url" rules={[{ required: true, message: '请输入 Base URL' }]}>
                <Input placeholder="如：https://api.deepseek.com" addBefore="POST" addAfter="/chat/completions" />
              </Form.Item>
              <Form.Item label="模型名" field="model" rules={[{ required: true, message: '请输入模型名' }]}>
                <Input placeholder="如：deepseek-chat" />
              </Form.Item>
            </>
          )}
          <Form.Item
            label="API Key"
            field="api_key"
          >
            <Input.Password placeholder="sk-..." autoComplete="new-password" />
          </Form.Item>
          {(isSuper || !editing || isOwn(editing)) && (
            <>
              <Form.Item label="排序" field="sort_order">
                <Input type="number" />
              </Form.Item>
              <Form.Item
                label="请求间隔（毫秒）"
                field="interval_ms"
                extra="同一平台两次请求的最小间隔，用于规避限流(429)。默认 2000ms；0=系统默认800ms节流；Kimi免费档建议20000"
              >
                <Input type="number" placeholder="如：2000" />
              </Form.Item>
              <Form.Item
                label="采样次数"
                field="sample_count"
                extra="同一问题向该平台提问多少次（1~5，默认1）。AI 回答有随机性，采样多次按命中比例判定，指标更稳定；次数>1 会按次扣点卡，显著增加巡检耗时"
              >
                <Input type="number" placeholder="1~5，默认 1" />
              </Form.Item>
            </>
          )}
          <Form.Item label="启用" field="enabled">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
