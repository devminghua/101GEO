import { useEffect, useMemo, useState } from 'react';
import {
  Card, Grid, Button, Message, Tag, Space, Typography, Input, Tabs, Table, Select, Modal,
  Switch, Divider, Empty, Alert, Badge,
} from '@arco-design/web-react';
import {
  IconPlus, IconDelete, IconEdit, IconRefresh, IconSend, IconFile, IconSearch, IconCommon,
} from '@arco-design/web-react/icon';
import { api } from '../api';
import EChart from '../components/EChart';

const { Row, Col } = Grid;
const TextArea = Input.TextArea;

const TASK_STATUS: Record<string, { label: string; color: string }> = {
  pending: { label: '待发布', color: 'gray' },
  publishing: { label: '发布中', color: 'arcoblue' },
  published: { label: '已发布', color: 'green' },
  wait_manual: { label: '待人工发布', color: 'orange' },
  failed: { label: '发布失败', color: 'red' },
};

const SYNC_STATE: Record<string, { label: string; color: string }> = {
  unchecked: { label: '未检测', color: 'gray' },
  indexed: { label: '已收录', color: 'green' },
  not_indexed: { label: '未收录', color: 'red' },
  unknown: { label: '无法验证', color: 'orange' },
};

/* ================================================================
 * 内容投放模块（左侧栏独立主菜单 /content）
 *
 * 1. 媒体库：主流媒体账号池（内置预设 + 自定义），按租户隔离
 * 2. AI 软文：AI 自动写软文并入库（复用统一 AI 客户端，无平台提示配置）
 * 3. 发布任务：自动发布（对接发稿平台 API）/ 半自动（待人工发布后回填链接）
 *    百度收录检测：尽力而为，失败降级 estimate 标记
 * 4. 效果分析：发布成功率 / 收录率 / 质量分布 / 媒体分布 / 趋势
 *
 * 合规硬约束：不模拟登录媒体后台、不自动群发；人工环节由用户在官方后台完成。
 * ================================================================ */
export default function ContentPublish() {
  const [tab, setTab] = useState('media');

  return (
    <Card
      title="内容投放"
      style={{ borderRadius: 12, marginTop: 16 }}
      extra={<Tag color="arcoblue">媒体库 · AI软文 · 发布 · 百度收录 · 效果分析</Tag>}
    >
      <Tabs activeTab={tab} onChange={setTab} type="line">
        <Tabs.TabPane key="media" title={<span><IconCommon /> 媒体库</span>}>
          <MediaTab />
        </Tabs.TabPane>
        <Tabs.TabPane key="article" title={<span><IconFile /> AI 软文</span>}>
          <ArticleTab />
        </Tabs.TabPane>
        <Tabs.TabPane key="task" title={<span><IconSend /> 发布任务</span>}>
          <TaskTab />
        </Tabs.TabPane>
        <Tabs.TabPane key="analysis" title={<span><IconSearch /> 效果分析</span>}>
          <AnalysisTab />
        </Tabs.TabPane>
      </Tabs>
    </Card>
  );
}

/* ================= 1) 媒体库 ================= */
function MediaTab() {
  const [list, setList] = useState<any[]>([]);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form, setForm] = useState({ name: '', category: '新闻门户', level: '中', url: '', remark: '' });

  const load = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (keyword) params.set('keyword', keyword);
      const r = await api.listContentMedia(params.toString());
      setList(r || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, [keyword]);

  const openModal = (row?: any) => {
    setEditing(row || null);
    setForm(row
      ? { name: row.name, category: row.category || '新闻门户', level: row.level || '中', url: row.url || '', remark: row.remark || '' }
      : { name: '', category: '新闻门户', level: '中', url: '', remark: '' });
    setModalVisible(true);
  };

  const submit = async () => {
    if (!form.name.trim()) { Message.warning('请填写媒体名称'); return; }
    try {
      if (editing) {
        await api.updateContentMedia(editing.id, form);
        Message.success('已更新');
      } else {
        await api.createContentMedia(form);
        Message.success('已新增');
      }
      setModalVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const del = (row: any) => {
    Modal.confirm({
      title: '删除媒体',
      content: `确认删除媒体「${row.name}」？`,
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        try {
          await api.deleteContentMedia(row.id);
          Message.success('已删除');
          load();
        } catch (e: any) {
          Message.error(e.message);
        }
      },
    });
  };

  const columns = [
    { title: '媒体名称', dataIndex: 'name', render: (v: string) => <b>{v}</b> },
    { title: '类型', dataIndex: 'category', width: 110, render: (v: string) => <Tag color="arcoblue">{v}</Tag> },
    { title: '级别', dataIndex: 'level', width: 80, render: (v: string) => <Tag color={v === '高' ? 'red' : v === '中' ? 'orange' : 'gray'}>{v}</Tag> },
    { title: '来源', dataIndex: 'source_type', width: 90, render: (v: string) => (v === 'builtin' ? <Tag>内置</Tag> : <Tag color="cyan">自定义</Tag>) },
    { title: '链接', dataIndex: 'url', ellipsis: true, render: (v: string) => (v ? <a href={v} target="_blank" rel="noreferrer">{v}</a> : '-') },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
    {
      title: '操作', width: 140,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" icon={<IconEdit />} onClick={() => openModal(row)}>编辑</Button>
          <Button size="mini" status="danger" icon={<IconDelete />} onClick={() => del(row)}>删除</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="搜索媒体名称"
          value={keyword}
          onChange={setKeyword}
          prefix={<IconSearch />}
          allowClear
          style={{ width: 220 }}
        />
        <Typography.Text type="secondary">内置 22 家主流媒体预设 + 自定义，按租户隔离</Typography.Text>
        <Button type="primary" icon={<IconPlus />} onClick={() => openModal()}>新增媒体</Button>
      </Space>
      <Table rowKey="id" columns={columns} data={list} loading={loading} border pagination={{ pageSize: 12, showTotal: true }} />

      <Modal
        title={editing ? '编辑媒体' : '新增媒体'}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={submit}
        okText="保存"
        cancelText="取消"
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div><Typography.Text bold>媒体名称 *</Typography.Text><Input value={form.name} onChange={(v) => setForm({ ...form, name: v })} placeholder="如：新浪新闻" /></div>
          <div><Typography.Text bold>类型</Typography.Text><Input value={form.category} onChange={(v) => setForm({ ...form, category: v })} placeholder="新闻门户 / 垂直媒体 / 自媒体" /></div>
          <div>
            <Typography.Text bold>级别</Typography.Text>
            <Select
              value={form.level}
              onChange={(v) => setForm({ ...form, level: v })}
              style={{ width: '100%' }}
              options={[{ label: '高', value: '高' }, { label: '中', value: '中' }, { label: '低', value: '低' }]}
            />
          </div>
          <div><Typography.Text bold>媒体链接（选填）</Typography.Text><Input value={form.url} onChange={(v) => setForm({ ...form, url: v })} placeholder="https://" /></div>
          <div><Typography.Text bold>备注</Typography.Text><Input value={form.remark} onChange={(v) => setForm({ ...form, remark: v })} /></div>
        </div>
      </Modal>
    </div>
  );
}

/* ================= 2) AI 软文 ================= */
function ArticleTab() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [genLoading, setGenLoading] = useState(false);
  const [topic, setTopic] = useState('');
  const [brandKeywords, setBrandKeywords] = useState('');
  const [category, setCategory] = useState('品牌软文');
  const [length, setLength] = useState(600);
  const [editing, setEditing] = useState<any>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editContent, setEditContent] = useState('');
  const [editVisible, setEditVisible] = useState(false);
  // 批量生成
  const [batchVisible, setBatchVisible] = useState(false);
  const [batchTopics, setBatchTopics] = useState('');
  const [batchLoading, setBatchLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const r = await api.listContentArticles();
      setList(r || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const generate = async () => {
    if (!topic.trim()) { Message.warning('请填写投放主题'); return; }
    setGenLoading(true);
    try {
      const art = await api.generateContentArticle({ topic, brand_keywords: brandKeywords, category, length });
      Message.success(`已生成《${art.title}》（${art.platform} · ${art.model}）`);
      setList([art, ...list]);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setGenLoading(false);
    }
  };

  const batchGenerate = async () => {
    const topics = String(batchTopics || '').split('\n').map((s: string) => s.trim()).filter(Boolean);
    if (!topics.length) { Message.warning('请至少输入一个主题'); return; }
    setBatchLoading(true);
    try {
      const r = await api.generateContentArticlesBatch({ topics, brand_keywords: brandKeywords, category, length });
      const n = r?.created_count ?? 0;
      const f = r?.failed_count ?? 0;
      Message.success(`批量生成完成：成功 ${n} 篇${f ? `，失败 ${f} 篇` : ''}`);
      setBatchVisible(false);
      setBatchTopics('');
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setBatchLoading(false);
    }
  };

  const openEdit = (row: any) => {
    setEditing(row);
    setEditTitle(row.title);
    setEditContent(row.content);
    setEditVisible(true);
  };

  const saveEdit = async () => {
    if (!editTitle.trim()) { Message.warning('标题不能为空'); return; }
    try {
      await api.updateContentArticle(editing.id, { title: editTitle, content: editContent });
      Message.success('已保存');
      setEditVisible(false);
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const del = (row: any) => {
    Modal.confirm({
      title: '删除软文',
      content: `确认删除《${row.title}》？`,
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        try {
          await api.deleteContentArticle(row.id);
          Message.success('已删除');
          load();
        } catch (e: any) {
          Message.error(e.message);
        }
      },
    });
  };

  const columns = [
    { title: '标题', dataIndex: 'title', ellipsis: true, render: (v: string) => <b>{v}</b> },
    { title: '主题', dataIndex: 'topic', ellipsis: true },
    { title: '分类', dataIndex: 'category', width: 100, render: (v: string) => <Tag color="arcoblue">{v || '未分类'}</Tag> },
    { title: '字数', dataIndex: 'length', width: 80 },
    { title: 'AI平台', dataIndex: 'platform', width: 90, render: (v: string) => (v ? <Tag>{v}</Tag> : '-') },
    { title: '状态', dataIndex: 'status', width: 90, render: (v: string) => <Tag color={v === 'published' ? 'green' : 'gray'}>{v === 'published' ? '已用' : '草稿'}</Tag> },
    { title: '生成时间', dataIndex: 'created_at', width: 170, render: (v: string) => (v ? new Date(v).toLocaleString() : '-') },
    {
      title: '操作', width: 140,
      render: (_: any, row: any) => (
        <Space>
          <Button size="mini" icon={<IconEdit />} onClick={() => openEdit(row)}>编辑</Button>
          <Button size="mini" status="danger" icon={<IconDelete />} onClick={() => del(row)}>删除</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card size="small" title="AI 自动写软文" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={8}>
            <div style={{ marginBottom: 6 }}><Typography.Text bold>投放主题 *</Typography.Text></div>
            <Input value={topic} onChange={setTopic} placeholder="如：新一代智能扫地机器人" />
          </Col>
          <Col span={6}>
            <div style={{ marginBottom: 6 }}><Typography.Text bold>品牌关键词</Typography.Text></div>
            <Input value={brandKeywords} onChange={setBrandKeywords} placeholder="品牌名，逗号分隔" />
          </Col>
          <Col span={4}>
            <div style={{ marginBottom: 6 }}><Typography.Text bold>分类</Typography.Text></div>
            <Select
              value={category}
              onChange={setCategory}
              style={{ width: '100%' }}
              options={['品牌软文', '产品测评', '行业资讯', '案例故事'].map((c) => ({ label: c, value: c }))}
            />
          </Col>
          <Col span={3}>
            <div style={{ marginBottom: 6 }}><Typography.Text bold>字数</Typography.Text></div>
            <Select value={length} onChange={setLength} style={{ width: '100%' }}
              options={[400, 600, 800, 1000, 1500].map((n) => ({ label: `${n}`, value: n }))} />
          </Col>
          <Col span={3} style={{ display: 'flex', alignItems: 'flex-end', gap: 8 }}>
            <Button type="primary" long loading={genLoading} icon={<IconFile />} onClick={generate}>生成软文</Button>
            <Button type="outline" icon={<IconFile />} onClick={() => setBatchVisible(true)}>批量生成</Button>
          </Col>
        </Row>
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          复用统一 AI 客户端（OpenAI 兼容），自动取当前租户首个启用平台；未配置 AI 平台时请先到「系统设置」配置。
          批量生成：每行一个主题，逐篇调用 AI 生成软文。
        </Typography.Text>
      </Card>

      <Table rowKey="id" columns={columns} data={list} loading={loading} border pagination={{ pageSize: 10, showTotal: true }} />

      {/* 批量生成软文 */}
      <Modal
        title="批量生成软文"
        visible={batchVisible}
        onOk={batchGenerate}
        confirmLoading={batchLoading}
        onCancel={() => { setBatchVisible(false); setBatchTopics(''); }}
        okText="开始批量生成"
        cancelText="取消"
        style={{ width: 560 }}
      >
        <div style={{ marginBottom: 6 }}><Typography.Text bold>主题（每行一个）</Typography.Text></div>
        <TextArea
          value={batchTopics}
          onChange={setBatchTopics}
          placeholder={'婚恋相亲平台怎么选\n红娘谈单系统推荐\n婚恋CRM软件哪个好'}
          autoSize={{ minRows: 6, maxRows: 14 }}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          将沿用上方设置的品牌关键词、分类与字数，逐篇生成；生成失败的主题会单独跳过并提示。
        </Typography.Text>
      </Modal>

      <Modal
        title={`编辑软文 · ${editing?.title || ''}`}
        visible={editVisible}
        onCancel={() => setEditVisible(false)}
        onOk={saveEdit}
        okText="保存"
        cancelText="取消"
        style={{ width: 720 }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div><Typography.Text bold>标题</Typography.Text><Input value={editTitle} onChange={setEditTitle} /></div>
          <div><Typography.Text bold>正文</Typography.Text><TextArea value={editContent} onChange={setEditContent} autoSize={{ minRows: 8, maxRows: 20 }} /></div>
        </div>
      </Modal>
    </div>
  );
}

/* ================= 3) 发布任务 ================= */
function TaskTab() {
  const [tasks, setTasks] = useState<any[]>([]);
  const [mediaList, setMediaList] = useState<any[]>([]);
  const [articles, setArticles] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [monitors, setMonitors] = useState<any[]>([]);
  const [monitorVisible, setMonitorVisible] = useState(false);
  const [activeTask, setActiveTask] = useState<any>(null);
  const [cfg, setCfg] = useState<any>(null);
  const [cfgVisible, setCfgVisible] = useState(false);

  // 创建任务弹窗
  const [createVisible, setCreateVisible] = useState(false);
  const [articleId, setArticleId] = useState<number | undefined>();
  const [mediaId, setMediaId] = useState<number | undefined>();
  const [publishMode, setPublishMode] = useState('auto');

  // 人工回填弹窗
  const [finishVisible, setFinishVisible] = useState(false);
  const [finishUrl, setFinishUrl] = useState('');
  const [finishTask, setFinishTask] = useState<any>(null);

  // 一键批量投放弹窗
  const [batchPubVisible, setBatchPubVisible] = useState(false);
  const [batchArticleIds, setBatchArticleIds] = useState<number[]>([]);
  const [batchMediaIds, setBatchMediaIds] = useState<number[]>([]);
  const [batchPubMode, setBatchPubMode] = useState('auto');
  const [batchPubLoading, setBatchPubLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const r = await api.listContentTasks();
      setTasks(r || []);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
    api.listContentMedia().then((r) => setMediaList(r || [])).catch(() => {});
    api.listContentArticles().then((r) => setArticles(r || [])).catch(() => {});
    api.getContentPublishConfig().then((r) => setCfg(r)).catch(() => {});
  }, []);

  const openCreate = () => {
    setArticleId(undefined);
    setMediaId(undefined);
    setPublishMode('auto');
    setCreateVisible(true);
  };

  const batchPublish = async () => {
    if (!batchArticleIds.length || !batchMediaIds.length) { Message.warning('请选择软文与媒体'); return; }
    setBatchPubLoading(true);
    try {
      const r = await api.createContentTasksBatch({ article_ids: batchArticleIds, media_ids: batchMediaIds, publish_mode: batchPubMode });
      Message.success(`已批量创建 ${r?.created ?? 0} 个发布任务`);
      setBatchPubVisible(false);
      setBatchArticleIds([]);
      setBatchMediaIds([]);
      load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setBatchPubLoading(false);
    }
  };

  const createTask = async () => {
    if (!articleId || !mediaId) { Message.warning('请选择软文与媒体'); return; }
    try {
      const t = await api.createContentTask({ article_id: articleId, media_id: mediaId, publish_mode: publishMode });
      Message.success('已创建任务');
      setCreateVisible(false);
      load();
      if (publishMode === 'auto') {
        api.publishContentTask(t.id).then(() => { Message.success('已触发发布'); load(); }).catch((e: any) => Message.error(e.message));
      }
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const publish = async (row: any) => {
    try {
      await api.publishContentTask(row.id);
      Message.success('已触发发布');
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const openFinish = (row: any) => {
    setFinishTask(row);
    setFinishUrl(row.publish_url || '');
    setFinishVisible(true);
  };

  const finish = async () => {
    if (!finishTask) return;
    try {
      await api.finishContentTask(finishTask.id, { publish_url: finishUrl });
      Message.success('已回填，标记为已发布');
      setFinishVisible(false);
      load();
      if (finishUrl.trim()) {
        api.checkContentTask(finishTask.id).then(() => { Message.success('已触发百度收录检测'); load(); }).catch((e: any) => Message.error(e.message));
      }
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const check = async (row: any) => {
    if (!row.publish_url) { Message.warning('该任务无发布链接，请先回填'); return; }
    try {
      const r = await api.checkContentTask(row.id);
      const m = r?.monitor;
      if (m?.sourced_from === 'estimate') {
        Message.warning(`无法验证：${m.note}`);
      } else if (m?.baidu_indexed) {
        Message.success('百度已收录');
      } else {
        Message.info('百度未收录');
      }
      load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const showMonitor = async (row: any) => {
    setActiveTask(row);
    setMonitorVisible(true);
    try {
      const r = await api.listContentMonitors(`task_id=${row.id}`);
      setMonitors(r || []);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const del = (row: any) => {
    Modal.confirm({
      title: '删除任务',
      content: `确认删除发布任务「${row.article_title}」？`,
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        try {
          await api.deleteContentTask(row.id);
          Message.success('已删除');
          load();
        } catch (e: any) {
          Message.error(e.message);
        }
      },
    });
  };

  const saveCfg = async () => {
    try {
      await api.saveContentPublishConfig({ enabled: cfg?.enabled, api: cfg?.api, key: cfg?.newKey || '' });
      Message.success('发稿平台配置已保存');
      setCfgVisible(false);
      api.getContentPublishConfig().then((r) => setCfg(r)).catch(() => {});
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const columns = [
    { title: '软文标题', dataIndex: 'article_title', ellipsis: true, render: (v: string) => <b>{v}</b> },
    { title: '媒体', dataIndex: 'media_name', width: 120, render: (v: string) => <Tag color="arcoblue">{v}</Tag> },
    { title: '模式', dataIndex: 'publish_mode', width: 90, render: (v: string) => (v === 'auto' ? <Tag color="cyan">自动</Tag> : <Tag>人工</Tag>) },
    { title: '状态', dataIndex: 'status', width: 110, render: (v: string) => {
      const s = TASK_STATUS[v] || { label: v, color: 'gray' };
      return <Badge status={s.color as any} text={s.label} />;
    } },
    { title: '收录', dataIndex: 'sync_state', width: 110, render: (v: string) => {
      const s = SYNC_STATE[v] || { label: v, color: 'gray' };
      return <Badge status={s.color as any} text={s.label} />;
    } },
    { title: '质量分', dataIndex: 'quality_score', width: 80, render: (v: number, row: any) => (
      <TooltipText text={row.quality_msg}><b style={{ color: v >= 80 ? '#00B42A' : v >= 60 ? '#FF7D00' : '#F53F3F' }}>{v}</b></TooltipText>
    ) },
    { title: '发布时间', dataIndex: 'published_at', width: 170, render: (v: string) => (v ? new Date(v).toLocaleString() : '-') },
    { title: '链接', dataIndex: 'publish_url', ellipsis: true, render: (v: string) => (v ? <a href={v} target="_blank" rel="noreferrer">{v}</a> : '-') },
    {
      title: '操作', width: 300,
      render: (_: any, row: any) => (
        <Space>
          {row.status === 'pending' && <Button size="mini" type="primary" icon={<IconSend />} onClick={() => publish(row)}>发布</Button>}
          {(row.status === 'wait_manual' || row.status === 'pending') && <Button size="mini" icon={<IconEdit />} onClick={() => openFinish(row)}>回填</Button>}
          {row.publish_url && row.status === 'published' && <Button size="mini" icon={<IconSearch />} onClick={() => check(row)}>百度检测</Button>}
          <Button size="mini" icon={<IconFile />} onClick={() => showMonitor(row)}>监控</Button>
          <Button size="mini" status="danger" icon={<IconDelete />} onClick={() => del(row)}>删除</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Button type="primary" icon={<IconPlus />} onClick={openCreate}>新建发布任务</Button>
        <Button icon={<IconSend />} onClick={() => setBatchPubVisible(true)}>一键批量投放</Button>
        <Button icon={<IconCommon />} onClick={() => { setCfgVisible(true); setCfg({ ...cfg, newKey: '' }); }}>发稿平台配置</Button>
        <Button icon={<IconRefresh />} onClick={load}>刷新</Button>
        <Typography.Text type="secondary">
          自动模式：配置发稿平台 API 后提交即对接；否则转「待人工发布」，在媒体后台发布后回填链接。
        </Typography.Text>
      </Space>

      {cfg && !cfg.enabled && (
        <Alert type="warning" style={{ marginBottom: 12 }} title="未启用发稿平台"
          content="当前未配置发稿平台接口，发布任务将进入「待人工发布」，请到媒体后台人工发布后回填链接。" />
      )}

      <Table rowKey="id" columns={columns} data={tasks} loading={loading} border pagination={{ pageSize: 10, showTotal: true }} />

      {/* 一键批量投放 */}
      <Modal title="一键批量投放" visible={batchPubVisible} onCancel={() => setBatchPubVisible(false)} onOk={batchPublish} confirmLoading={batchPubLoading} okText="批量创建" cancelText="取消" style={{ width: 560 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <Typography.Text bold>选择软文（可多选）*</Typography.Text>
            <Select mode="multiple" value={batchArticleIds} onChange={(v) => setBatchArticleIds(v as number[])} placeholder="请选择软文" style={{ width: '100%' }}
              options={articles.map((a) => ({ label: a.title, value: a.id }))} />
          </div>
          <div>
            <Typography.Text bold>选择媒体（可多选）*</Typography.Text>
            <Select mode="multiple" value={batchMediaIds} onChange={(v) => setBatchMediaIds(v as number[])} placeholder="请选择媒体" style={{ width: '100%' }}
              options={mediaList.map((m) => ({ label: `${m.name}（${m.category}·${m.level}）`, value: m.id }))} />
          </div>
          <div>
            <Typography.Text bold>发布模式</Typography.Text>
            <Space>
              <Switch checked={batchPubMode === 'auto'} onChange={(v) => setBatchPubMode(v ? 'auto' : 'manual')} checkedText="自动" uncheckedText="人工" />
              <Typography.Text type="secondary">将按「软文 × 媒体」组合批量创建发布任务</Typography.Text>
            </Space>
          </div>
        </div>
      </Modal>

      {/* 新建任务 */}
      <Modal title="新建发布任务" visible={createVisible} onCancel={() => setCreateVisible(false)} onOk={createTask} okText="创建并发布" cancelText="取消">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <Typography.Text bold>选择软文 *</Typography.Text>
            <Select value={articleId} onChange={setArticleId} placeholder="请选择软文" style={{ width: '100%' }}
              options={articles.map((a) => ({ label: a.title, value: a.id }))} />
          </div>
          <div>
            <Typography.Text bold>选择媒体 *</Typography.Text>
            <Select value={mediaId} onChange={setMediaId} placeholder="请选择媒体" style={{ width: '100%' }}
              options={mediaList.map((m) => ({ label: `${m.name}（${m.category}·${m.level}）`, value: m.id }))} />
          </div>
          <div>
            <Typography.Text bold>发布模式</Typography.Text>
            <Space>
              <Switch checked={publishMode === 'auto'} onChange={(v) => setPublishMode(v ? 'auto' : 'manual')} checkedText="自动" uncheckedText="人工" />
              <Typography.Text type="secondary">{publishMode === 'auto' ? '对接发稿平台自动发布（未配置则转待人工）' : '创建后由人工在媒体后台发布'}</Typography.Text>
            </Space>
          </div>
        </div>
      </Modal>

      {/* 人工回填 */}
      <Modal title={`人工发布回填 · ${finishTask?.article_title || ''}`} visible={finishVisible}
        onCancel={() => setFinishVisible(false)} onOk={finish} okText="标记已发布" cancelText="取消">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Alert type="info" title="请先在媒体后台完成人工发布" content="发布完成后，将文章链接粘贴到下方，系统会同时触发百度收录检测。" />
          <div><Typography.Text bold>发布链接</Typography.Text><Input value={finishUrl} onChange={setFinishUrl} placeholder="https://..." /></div>
        </div>
      </Modal>

      {/* 发稿平台配置 */}
      <Modal title="发稿平台配置" visible={cfgVisible} onCancel={() => setCfgVisible(false)} onOk={saveCfg} okText="保存" cancelText="取消" style={{ width: 600 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <Typography.Text bold>① 先到发稿平台注册（任选一家）</Typography.Text>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 8, background: 'var(--color-fill-1)', borderRadius: 8, padding: '12px 14px' }}>
              <Space wrap>
                <Typography.Text style={{ fontSize: 13 }}>
                  媒介盒子：<a href="https://www.meijiehezi.com/" target="_blank" rel="noreferrer">meijiehezi.com</a>
                </Typography.Text>
                <Typography.Text style={{ fontSize: 13 }}>
                  软文街：<a href="https://www.ruanwen.la/" target="_blank" rel="noreferrer">ruanwen.la</a>
                </Typography.Text>
                <Typography.Text style={{ fontSize: 13 }}>
                  投媒网：<a href="https://toumeiw.cn/" target="_blank" rel="noreferrer">toumeiw.cn</a>
                </Typography.Text>
                <Typography.Text style={{ fontSize: 13 }}>
                  优媒汇：<a href="http://www.youmeiwang.com/" target="_blank" rel="noreferrer">youmeiwang.com</a>
                </Typography.Text>
              </Space>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                注册后到平台后台申请/获取「API 接口地址 + 鉴权 Key」，填入下方即可自动发稿；不配置则走人工发布。
              </Typography.Text>
            </div>
          </div>
          <div>
            <Typography.Text bold>② 填写 API 配置</Typography.Text>
            <Space style={{ marginTop: 8 }}><Switch checked={!!cfg?.enabled} onChange={(v) => setCfg({ ...cfg, enabled: v })} checkedText="启用" uncheckedText="停用" />
              <Typography.Text type="secondary">启用后，发布任务将自动对接下方接口</Typography.Text></Space>
          </div>
          <div><Typography.Text bold>发稿接口 URL</Typography.Text>
            <Input value={cfg?.api || ''} onChange={(v) => setCfg({ ...cfg, api: v })} placeholder="https://your-publish-platform.com/api/submit" /></div>
          <div>
            <Typography.Text bold>鉴权 Key（可选）</Typography.Text>
            <Input value={cfg?.newKey || ''} onChange={(v) => setCfg({ ...cfg, newKey: v })} placeholder={cfg?.has_key ? `已配置（${cfg.key_masked}），留空保持不变` : '未配置'} />
          </div>
          <Typography.Text type="secondary">接口约定：POST {`{title, content, media_name}`} → 200 返回 data.url 或 url 即视为发布成功并回填链接。</Typography.Text>
        </div>
      </Modal>

      {/* 监控记录 */}
      <Modal title={`监控记录 · ${activeTask?.article_title || ''}`} visible={monitorVisible}
        onCancel={() => setMonitorVisible(false)} footer={null} style={{ width: 760 }}>
        <Table
          rowKey="id"
          size="small"
          border
          pagination={false}
          data={monitors}
          columns={[
            { title: '媒体', dataIndex: 'media_name', width: 110 },
            { title: '检测方式', dataIndex: 'check_method', width: 100 },
            { title: '来源', dataIndex: 'sourced_from', width: 90, render: (v: string) => (v === 'real' ? <Tag color="green">真实</Tag> : <Tag color="orange">估算</Tag>) },
            { title: '百度收录', dataIndex: 'baidu_indexed', width: 100, render: (v: any) => (v === true ? <Tag color="green">已收录</Tag> : v === false ? <Tag color="red">未收录</Tag> : <Tag>无法验证</Tag>) },
            { title: '质量分', dataIndex: 'quality_score', width: 70 },
            { title: '说明', dataIndex: 'note', ellipsis: true },
            { title: '时间', dataIndex: 'created_at', width: 160, render: (v: string) => (v ? new Date(v).toLocaleString() : '-') },
          ]}
        />
      </Modal>
    </div>
  );
}

function TooltipText({ text, children }: { text: string; children: React.ReactNode }) {
  if (!text) return <>{children}</>;
  return <span title={text}>{children}</span>;
}

/* ================= 4) 效果分析 ================= */
function AnalysisTab() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const r = await api.contentAnalysis();
      setData(r);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const mediaBarOption = useMemo(() => {
    const ps = (data?.media_dist || []).slice(0, 10).reverse();
    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: 8, right: 40, top: 8, bottom: 4, containLabel: true },
      xAxis: { type: 'value', minInterval: 1 },
      yAxis: { type: 'category', data: ps.map((p: any) => p.name), axisLabel: { fontSize: 11 } },
      series: [{
        type: 'bar', barWidth: 12,
        data: ps.map((p: any) => p.count),
        itemStyle: { borderRadius: [0, 6, 6, 0], color: '#14C9C9' },
        label: { show: true, position: 'right', formatter: '{c}', fontSize: 10 },
      }],
    };
  }, [data]);

  const categoryPieOption = useMemo(() => {
    const src = (data?.category_dist || []).filter((d: any) => d.count > 0);
    const COLORS = ['#165DFF', '#14C9C9', '#FF7D00', '#722ED1', '#F53F3F', '#00B42A', '#F7BA1E', '#C9CDD4'];
    return {
      tooltip: { trigger: 'item', formatter: '{b}：{c} 篇（{d}%）' },
      legend: { bottom: 0, type: 'scroll', textStyle: { fontSize: 11 } },
      series: [{
        type: 'pie', radius: ['42%', '68%'], center: ['50%', '44%'],
        itemStyle: { borderRadius: 4, borderColor: 'var(--geo-surface)', borderWidth: 2 },
        data: src.map((d: any, i: number) => ({ name: d.category, value: d.count, itemStyle: { color: COLORS[i % COLORS.length] } })),
        label: { show: false },
      }],
    };
  }, [data]);

  const qualityPieOption = useMemo(() => {
    const src = (data?.quality_dist || []).filter((d: any) => d.count > 0);
    const COLORS = { '0-59': '#F53F3F', '60-79': '#FF7D00', '80-100': '#00B42A' } as Record<string, string>;
    return {
      tooltip: { trigger: 'item', formatter: '{b}：{c} 个（{d}%）' },
      legend: { bottom: 0, textStyle: { fontSize: 11 } },
      series: [{
        type: 'pie', radius: ['42%', '68%'], center: ['50%', '44%'],
        itemStyle: { borderRadius: 4, borderColor: 'var(--geo-surface)', borderWidth: 2 },
        data: src.map((d: any) => ({ name: d.range, value: d.count, itemStyle: { color: COLORS[d.range] || '#C9CDD4' } })),
        label: { show: false },
      }],
    };
  }, [data]);

  const trendOption = useMemo(() => {
    const t = [...(data?.trend || [])].reverse();
    return {
      tooltip: { trigger: 'axis' },
      legend: { data: ['发布数', '收录数'], top: 0 },
      grid: { left: 36, right: 16, top: 30, bottom: 24 },
      xAxis: { type: 'category', data: t.map((p: any) => p.day.slice(5)), axisLabel: { fontSize: 10 } },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        { name: '发布数', type: 'line', smooth: true, symbolSize: 6, data: t.map((p: any) => p.published), lineStyle: { color: '#165DFF' }, itemStyle: { color: '#165DFF' } },
        { name: '收录数', type: 'line', smooth: true, symbolSize: 6, data: t.map((p: any) => p.indexed), lineStyle: { color: '#00B42A' }, itemStyle: { color: '#00B42A' } },
      ],
    };
  }, [data]);

  const stat = (label: string, value: any, unit = '', color = '') => (
    <Card size="small" style={{ textAlign: 'center' }}>
      <Typography.Text type="secondary" style={{ fontSize: 13 }}>{label}</Typography.Text>
      <div style={{ fontSize: 26, fontWeight: 600, color: color || 'var(--color-text-1)', marginTop: 4 }}>
        {value}{unit && <span style={{ fontSize: 13, marginLeft: 2 }}>{unit}</span>}
      </div>
    </Card>
  );

  return (
    <div>
      <Space style={{ marginBottom: 12 }} wrap>
        <Button icon={<IconRefresh />} onClick={load}>刷新</Button>
        <Typography.Text type="secondary">收录率仅按真实检测（sourced_from=real）统计，无法验证的记录不纳入分母。</Typography.Text>
      </Space>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col span={4}>{stat('发布任务', data?.total_tasks ?? '-')}</Col>
        <Col span={4}>{stat('已发布', data?.published ?? '-')}</Col>
        <Col span={4}>{stat('发布成功率', data?.publish_rate ?? '-', '%', '#165DFF')}</Col>
        <Col span={4}>{stat('真实收录率', data?.indexed_rate ?? '-', '%', '#00B42A')}</Col>
        <Col span={4}>{stat('平均质量分', data?.avg_quality ?? '-', '', '#FF7D00')}</Col>
        <Col span={4}>{stat('待人工/失败', `${data?.wait_manual ?? '-'}/${data?.failed ?? '-'}`)}</Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col span={10}>
          <Card size="small" title="媒体发布分布 TOP10">
            {data?.media_dist?.length ? <EChart option={mediaBarOption} height={260} /> : <Empty />}
          </Card>
        </Col>
        <Col span={7}>
          <Card size="small" title="软文场景分布">
            {data?.category_dist?.length ? <EChart option={categoryPieOption} height={260} /> : <Empty />}
          </Card>
        </Col>
        <Col span={7}>
          <Card size="small" title="任务质量分布">
            {data?.quality_dist?.length ? <EChart option={qualityPieOption} height={260} /> : <Empty />}
          </Card>
        </Col>
      </Row>

      <Card size="small" title="近 30 天发布与收录趋势" style={{ marginTop: 16 }}>
        <EChart option={trendOption} height={280} />
      </Card>

      <Divider />
      <Alert type="warning" title="合规说明"
        content="本模块不模拟登录任何媒体后台、不自动群发；发布动作由用户在官方媒体后台完成（半自动回填）或由租户自备的发稿平台 API 对接。百度收录检测为尽力而为，失败时标记为估算（estimate），请以百度搜索结果为准。" />
    </div>
  );
}
