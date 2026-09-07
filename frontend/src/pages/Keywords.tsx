import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Message, Popconfirm,
  Space, Tag, Switch, Select, Typography,
} from '@arco-design/web-react';
import { IconPlus, IconSettings, IconSync, IconUserAdd, IconSearch, IconDelete } from '@arco-design/web-react/icon';
import { api } from '../api';

export default function Keywords() {
  const [list, setList] = useState<any[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [bulkVisible, setBulkVisible] = useState(false);
  const [filterCat, setFilterCat] = useState<string>('全部');
  // 批量删除
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>([]);
  // 拓词选题
  const [suggestVisible, setSuggestVisible] = useState(false);
  const [suggestKeyword, setSuggestKeyword] = useState('');
  const [suggestSource, setSuggestSource] = useState('baidu');
  const [suggestWords, setSuggestWords] = useState<string[]>([]);
  const [suggestChecked, setSuggestChecked] = useState<string[]>([]);
  const [suggestCategory, setSuggestCategory] = useState('拓词');
  const [suggestLoading, setSuggestLoading] = useState(false);
  const [suggestImporting, setSuggestImporting] = useState(false);
  const [form] = Form.useForm();
  const [bulkForm] = Form.useForm();

  const load = async () => {
    const [k, c] = await Promise.all([api.listKeywords(), api.getCategories()]);
    setList(k || []);
    setCategories(c || []);
  };
  useEffect(() => { load(); }, []);

  const openAdd = () => {
    setEditing(null);
    form.resetFields();
    setFormDefault();
    setVisible(true);
  };
  const setFormDefault = () => {
    form.setFieldsValue({ category: '默认', enabled: true });
  };

  const openEdit = (row: any) => {
    setEditing(row);
    form.setFieldsValue(row);
    setVisible(true);
  };

  const save = async () => {
    const values = await form.validate();
    try {
      if (editing) {
        await api.updateKeyword(editing.id, { ...values });
        Message.success('已更新');
      } else {
        await api.createKeyword({ ...values });
        Message.success('已新增');
      }
      setVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const saveBulk = async () => {
    const values = await bulkForm.validate();
    const questions = String(values.questions || '')
      .split('\n')
      .map((s: string) => s.trim())
      .filter(Boolean);
    if (!questions.length) {
      Message.error('请输入至少一个问题');
      return;
    }
    try {
      const r = await api.bulkCreateKeywords({
        brand_keywords: values.brand_keywords || '',
        category: values.category || '默认',
        questions,
      });
      Message.success(`批量导入成功（共 ${r.created} 条）`);
      setBulkVisible(false);
      bulkForm.resetFields();
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const remove = async (id: number) => {
    try {
      await api.deleteKeyword(id);
      Message.success('已删除');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // 批量删除
  const batchRemove = () => {
    if (selectedRowKeys.length === 0) {
      Message.warning('请先勾选要删除的关键词');
      return;
    }
    Modal.confirm({
      title: '批量删除',
      content: `确认删除选中的 ${selectedRowKeys.length} 个关键词？此操作不可恢复。`,
      onOk: async () => {
        try {
          await api.batchDeleteKeywords(selectedRowKeys);
          Message.success(`已删除 ${selectedRowKeys.length} 个关键词`);
          setSelectedRowKeys([]);
          load();
        } catch (e: any) {
          Message.error(e.message);
        }
      },
    });
  };

  // 拓词：拉取候选词
  const fetchSuggest = async () => {
    if (!suggestKeyword.trim()) { Message.warning('请输入拓词词根'); return; }
    setSuggestLoading(true);
    try {
      const r = await api.baiduSuggest({ keyword: suggestKeyword.trim(), source: suggestSource });
      setSuggestWords(r.words || []);
      setSuggestChecked([]);
      if (!(r.words || []).length) Message.info('未获取到候选词，换个词根试试');
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSuggestLoading(false);
    }
  };

  // 拓词：勾选候选词入库
  const importSuggest = async () => {
    if (!suggestChecked.length) { Message.warning('请勾选要入库的候选词'); return; }
    setSuggestImporting(true);
    try {
      const r = await api.bulkCreateKeywords({
        brand_keywords: '',
        category: suggestCategory || '拓词',
        questions: suggestChecked,
      });
      Message.success(`已入库 ${r.created} 条候选词`);
      setSuggestVisible(false);
      setSuggestWords([]);
      setSuggestChecked([]);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSuggestImporting(false);
    }
  };

  const toggle = async (row: any, enabled: boolean) => {
    try {
      await api.updateKeyword(row.id, { ...row, enabled });
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const filtered = filterCat === '全部' ? list : list.filter((k) => k.category === filterCat);

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '监控问题', dataIndex: 'question' },
    {
      title: '品牌词', dataIndex: 'brand_keywords',
      render: (v: string) => (
        <Space>
          {String(v || '').split(',').filter(Boolean).map((w, i) => (
            <Tag key={i} color="arcoblue">{w}</Tag>
          ))}
        </Space>
      ),
    },
    { title: '分类', dataIndex: 'category', render: (v: string) => <Tag>{v}</Tag> },
    {
      title: '启用', dataIndex: 'enabled', width: 80,
      render: (v: boolean, row: any) => <Switch checked={v} onChange={(c: any) => toggle(row, c)} />,
    },
    {
      title: '操作', width: 140,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" type="text" icon={<IconSettings />} onClick={() => openEdit(row)}>编辑</Button>
          <Popconfirm content="确认删除该关键词？" onOk={() => remove(row.id)}>
            <Button size="mini" type="text" status="danger" icon={<IconSettings />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="关键词监控"
      extra={
        <Space>
          <Button icon={<IconSync />} onClick={load}>刷新</Button>
          <Button icon={<IconUserAdd />} onClick={() => setBulkVisible(true)}>批量导入</Button>
          <Button icon={<IconSearch />} onClick={() => setSuggestVisible(true)}>拓词</Button>
          <Button type="primary" icon={<IconPlus />} onClick={openAdd}>新增关键词</Button>
          <Button status="danger" icon={<IconDelete />} disabled={selectedRowKeys.length === 0} onClick={batchRemove}>
            批量删除{selectedRowKeys.length > 0 ? `(${selectedRowKeys.length})` : ''}
          </Button>
        </Space>
      }
      style={{ borderRadius: 12 }}
    >
      <Space style={{ marginBottom: 16 }}>
        <Select
          value={filterCat}
          onChange={setFilterCat}
          style={{ width: 160 }}
          options={['全部', ...categories].map((c) => ({ label: c, value: c }))}
        />
        <Typography.Text type="secondary">
          提示：巡检会向每个启用平台的 AI 提问「监控问题」，并检测回答中是否出现「品牌词」。
        </Typography.Text>
      </Space>
      <Table
        rowKey="id"
        columns={columns}
        data={filtered}
        pagination={{ pageSize: 10 }}
        rowSelection={{
          type: 'checkbox',
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys as number[]),
        }}
      />

      {/* 新增 / 编辑 */}
      <Modal
        title={editing ? '编辑关键词' : '新增关键词'}
        visible={visible}
        onOk={save}
        onCancel={() => setVisible(false)}
        unmountOnExit
      >
        <Form form={form} layout="vertical">
          <Form.Item label="监控问题（用于向 AI 提问）" field="question" rules={[{ required: true, message: '请输入问题' }]}>
            <Input placeholder="如：婚恋门店管理软件哪个好？" />
          </Form.Item>
          <Form.Item label="品牌词（逗号分隔，用于命中检测）" field="brand_keywords" rules={[{ required: true, message: '请输入品牌词' }]}>
            <Input placeholder="如：轻媒,QINGMEI" />
          </Form.Item>
          <Form.Item label="分类" field="category">
            <Input placeholder="如：对比选型 / 功能咨询" />
          </Form.Item>
          <Form.Item label="启用" field="enabled">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* 批量导入 */}
      <Modal
        title="批量导入监控问题"
        visible={bulkVisible}
        onOk={saveBulk}
        onCancel={() => setBulkVisible(false)}
        unmountOnExit
      >
        <Form form={bulkForm} layout="vertical" initialValues={{ category: '默认' }}>
          <Form.Item label="每个问题一行" field="questions" rules={[{ required: true, message: '请输入至少一个问题' }]}>
            <Input.TextArea
              rows={8}
              placeholder={'婚恋门店管理软件哪个好？\n红娘谈单系统推荐一下\n婚恋CRM系统怎么选？'}
            />
          </Form.Item>
          <Form.Item label="品牌词（统一应用，逗号分隔）" field="brand_keywords">
            <Input placeholder="如：轻媒,QINGMEI（留空则使用系统默认品牌词）" />
          </Form.Item>
          <Form.Item label="分类" field="category">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      {/* 拓词选题 */}
      <Modal
        title="拓词选题（下拉联想词扩充）"
        visible={suggestVisible}
        onCancel={() => { setSuggestVisible(false); setSuggestWords([]); setSuggestChecked([]); }}
        footer={null}
        unmountOnExit
        style={{ width: 560 }}
      >
        <Space style={{ width: '100%', marginBottom: 12 }}>
          <Input
            value={suggestKeyword}
            onChange={setSuggestKeyword}
            placeholder="输入品牌 / 竞品 / 品类词根，如：婚恋系统"
            style={{ width: 240 }}
            onPressEnter={fetchSuggest}
          />
          <Select
            value={suggestSource}
            onChange={setSuggestSource}
            style={{ width: 120 }}
            options={[
              { label: '百度下拉', value: 'baidu' },
              { label: 'Google 补全', value: 'google' },
              { label: '两者都拉', value: 'both' },
            ]}
          />
          <Button type="primary" loading={suggestLoading} onClick={fetchSuggest}>获取候选词</Button>
        </Space>

        {suggestWords.length > 0 && (
          <>
            <div style={{ marginBottom: 8, fontSize: 13, color: '#86909C' }}>
              共 {suggestWords.length} 个候选词，勾选后入库为监控问题（品牌词留空，沿用系统默认）。
            </div>
            <div style={{ maxHeight: 320, overflow: 'auto', border: '1px solid var(--color-border-2)', borderRadius: 8, padding: 12 }}>
              {suggestWords.map((w) => (
                <label key={w} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', cursor: 'pointer', borderBottom: '1px solid var(--color-border-1)' }}>
                  <input
                    type="checkbox"
                    checked={suggestChecked.includes(w)}
                    onChange={(e) => {
                      if (e.target.checked) setSuggestChecked([...suggestChecked, w]);
                      else setSuggestChecked(suggestChecked.filter((x) => x !== w));
                    }}
                  />
                  <span style={{ fontSize: 13 }}>{w}</span>
                </label>
              ))}
            </div>
            <div style={{ marginTop: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
              <Input
                value={suggestCategory}
                onChange={setSuggestCategory}
                placeholder="分类（如：拓词 / 竞品词）"
                style={{ width: 160 }}
              />
              <Button type="primary" loading={suggestImporting} onClick={importSuggest} disabled={!suggestChecked.length}>
                入库选中（{suggestChecked.length}）
              </Button>
            </div>
          </>
        )}
      </Modal>
    </Card>
  );
}
