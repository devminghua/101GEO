import { useState, useEffect } from 'react';
import {
  Card, Form, Input, InputNumber, Button, Radio, Message, Modal, Table, Typography, Space, Tag,
} from '@arco-design/web-react';
import { IconCopy, IconDownload, IconRefresh, IconSafe } from '@arco-design/web-react/icon';
import { api } from '../../api';

interface CardRow {
  id: number;
  tier: number;
  code: string;
  points: number;
  status: number;
  remark: string;
  created_at: string;
}

const tierName = (t: number) => (t === 1 ? '3 天试用' : t === 2 ? '12 个月' : t === 3 ? '充值 token' : '未知');

// 总后台卡密管理：密钥对（公钥供打包）+ 批量生成 + 列表
export default function Cards() {
  const [pubKey, setPubKey] = useState('');
  const [hasPriv, setHasPriv] = useState(false);
  const [genForm] = Form.useForm();
  const [genTier, setGenTier] = useState<number>(2);
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<{ tier_name: string; codes: string[] } | null>(null);
  const [list, setList] = useState<CardRow[]>([]);
  const [listLoading, setListLoading] = useState(false);
  const [importVisible, setImportVisible] = useState(false);
  const [importVal, setImportVal] = useState('');
  const [importing, setImporting] = useState(false);

  const load = async () => {
    try {
      const [k, c] = await Promise.all([api.licenseKey(), api.listCards()]);
      setPubKey(k.public_key);
      setHasPriv(k.has_private);
      setList(c || []);
    } catch {
      /* 忽略 */
    }
  };

  useEffect(() => { load(); }, []);

  const doGenerate = async () => {
    const v = await genForm.validate();
    setGenerating(true);
    try {
      const r = await api.generateCards({ tier: v.tier, count: v.count, points: v.points || 0, remark: v.remark || '' });
      setResult({ tier_name: r.tier_name, codes: r.codes });
      Message.success(`已生成 ${r.count} 张卡密`);
      load();
    } catch (e: any) {
      Message.error(e.message || '生成失败');
    } finally {
      setGenerating(false);
    }
  };

  const doRegenKey = () => {
    Modal.confirm({
      title: '重新生成密钥对',
      content: '重新生成后，之前生成的所有卡密将全部失效，需重新生成卡密并重新打包客户端。确认继续？',
      okText: '确认重新生成',
      cancelText: '取消',
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        try {
          const r = await api.regenerateLicenseKey();
          setPubKey(r.public_key);
          setHasPriv(true);
          setResult(null);
          Message.success('密钥对已重新生成，请复制新公钥用于打包客户端');
        } catch (e: any) {
          Message.error(e.message || '生成失败');
        }
      },
    });
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      Message.success('已复制');
    } catch {
      Message.warning('复制失败，请手动复制');
    }
  };

  const download = () => {
    if (!result || !result.codes.length) return;
    const blob = new Blob([result.codes.join('\r\n')], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `卡密-${result.tier_name}-${Date.now()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const doImportKey = async () => {
    if (!importVal.trim()) {
      Message.warning('请粘贴私钥');
      return;
    }
    setImporting(true);
    try {
      const r = await api.importLicenseKey(importVal.trim());
      setPubKey(r.public_key);
      setHasPriv(true);
      setImportVisible(false);
      setImportVal('');
      Message.success('私钥已导入，现在生成的卡密可与客户端配对');
      load();
    } catch (e: any) {
      Message.error(e.message || '导入失败');
    } finally {
      setImporting(false);
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 80 },
    { title: '类型', dataIndex: 'tier', width: 110, render: (t: number) => <Tag color={t === 1 ? 'orange' : t === 3 ? 'green' : 'purple'}>{tierName(t)}</Tag> },
    {
      title: '卡密串', dataIndex: 'code',
      render: (code: string) => (
        <Space>
          <Typography.Text code style={{ fontSize: 12 }}>{code}</Typography.Text>
          <Button size="mini" icon={<IconCopy />} onClick={() => copy(code)} />
        </Space>
      ),
    },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (s: number, rec: CardRow) => {
        const usedLabel = rec.tier === 3 ? '已兑换' : '已激活';
        const idleLabel = rec.tier === 3 ? '未使用' : '未激活';
        return s === 1 ? <Tag color="green">{usedLabel}</Tag> : <Tag>{idleLabel}</Tag>;
      },
    },
    { title: '备注', dataIndex: 'remark', width: 120 },
    { title: '生成时间', dataIndex: 'created_at', width: 170, render: (t: string) => (t ? t.replace('T', ' ').slice(0, 19) : '') },
  ];

  return (
    <>
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title="密钥对" style={{ borderRadius: 12 }}>
        <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 12 }}>
          公钥用于打包客户端时写入 <Typography.Text code>GEO_LICENSE_PUBLIC_KEY</Typography.Text>。
          私钥已加密存储在本系统，仅生成卡密时使用。
        </div>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <Typography.Text code copyable style={{ fontSize: 12, maxWidth: 520, wordBreak: 'break-all' }}>
              {pubKey || '（尚未生成，生成卡密时自动创建）'}
            </Typography.Text>
          </Space>
          <Space>
            <Button size="small" icon={<IconCopy />} disabled={!pubKey} onClick={() => copy(pubKey)}>复制公钥</Button>
            <Button size="small" icon={<IconSafe />} onClick={() => setImportVisible(true)}>导入私钥</Button>
            <Button size="small" status="danger" icon={<IconRefresh />} onClick={doRegenKey}>重新生成密钥对</Button>
          </Space>
          {hasPriv && <Tag color="green">私钥已就绪</Tag>}
        </Space>
      </Card>

      <Card title="批量生成卡密" style={{ borderRadius: 12 }}>
        <Form form={genForm} layout="inline" initialValues={{ tier: 2, count: 10 }}>
          <Form.Item label="类型" field="tier" rules={[{ required: true }]}>
            <Radio.Group value={genTier} onChange={(v) => setGenTier(v as number)}>
              <Radio value={2}>12 个月</Radio>
              <Radio value={1}>3 天试用</Radio>
              <Radio value={3}>充值 token</Radio>
            </Radio.Group>
          </Form.Item>
          {genTier === 3 && (
            <Form.Item label="token 数" field="points" rules={[{ required: true, message: '充值卡密需指定 token 数' }]}>
              <InputNumber min={1} max={999999} style={{ width: 140 }} placeholder="兑换 token 数" />
            </Form.Item>
          )}
          <Form.Item label="数量" field="count" rules={[{ required: true, message: '请输入数量' }]}>
            <InputNumber min={1} max={1000} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item label="备注" field="remark">
            <Input placeholder="批次备注（可选）" style={{ width: 180 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" icon={<IconSafe />} loading={generating} onClick={doGenerate}>生成</Button>
          </Form.Item>
        </Form>

        {result && (
          <div style={{ marginTop: 16 }}>
            <Space style={{ marginBottom: 8 }}>
              <Typography.Text type="secondary">已生成 {result.codes.length} 张「{result.tier_name}」卡密：</Typography.Text>
              <Button size="small" icon={<IconCopy />} onClick={() => copy(result.codes.join('\n'))}>复制全部</Button>
              <Button size="small" icon={<IconDownload />} onClick={download}>下载 txt</Button>
            </Space>
            <div style={{ maxHeight: 260, overflow: 'auto', background: 'var(--color-fill-2)', borderRadius: 8, padding: 12 }}>
              {result.codes.map((c) => (
                <div key={c} style={{ fontFamily: 'var(--font-mono)', fontSize: 12, lineHeight: 1.9 }}>{c}</div>
              ))}
            </div>
          </div>
        )}
      </Card>

      <Card title="卡密列表（最近 500 条）" style={{ borderRadius: 12 }}>
        <Table
          rowKey="id"
          columns={columns as any}
          data={list}
          loading={listLoading}
          pagination={{ pageSize: 20, showTotal: true }}
          scroll={{ x: 900 }}
        />
      </Card>
    </Space>
      <Modal
        title="导入私钥"
        visible={importVisible}
        onCancel={() => setImportVisible(false)}
        onOk={doImportKey}
        confirmLoading={importing}
        okText="导入"
        cancelText="取消"
        unmountOnExit
      >
        <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 8 }}>
          粘贴打包客户端时使用的私钥（base64），导入后总后台生成的卡密才能被客户端激活。
        </div>
        <Input.TextArea
          value={importVal}
          onChange={setImportVal}
          placeholder="私钥 base64，如 KZfT6YqpnAosvmn9WMe1Mp1/..."
          autoSize={{ minRows: 3, maxRows: 6 }}
        />
      </Modal>
    </>
  );
}
