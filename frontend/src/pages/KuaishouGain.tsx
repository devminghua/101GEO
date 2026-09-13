import { useEffect, useMemo, useState } from 'react';
import {
  Card, Grid, Button, Message, Tag, Space, Input, Tabs, Table, Switch, Slider, Divider, Tooltip,
  Timeline, Modal, Select, Radio, Progress, Spin, Empty,
} from '@arco-design/web-react';
import {
  IconUserGroup, IconLink, IconPlayArrow, IconBulb, IconCopy, IconRight, IconSafe, IconClockCircle,
  IconPlus, IconDelete, IconRedo, IconRefresh, IconEdit, IconThunderbolt,
} from '@arco-design/web-react/icon';
import { api } from '../api';
import QuotaBadge, { notifyQuotaChanged } from '../components/QuotaBadge';

const { Row, Col } = Grid;
const TextArea = Input.TextArea;

/* ================================================================
 * 快手获客页 · 完整内部功能（前后端全栈，已对接真实后端 API）
 *
 * 模块：
 *  1. 账号管理：多账号 CRUD + 今日动作计数 + 冷却/恢复计算
 *  2. 同行追踪：粘贴快手主页链接批量导入（公开页数据极少，同步为结构化估算）
 *  3. 视频数据分析：KPI + Top 爆款视频（后端聚合接口）
 *  4. 获客打招呼：评论线索解析/手动添加 + 话术库 + AI 自动生成话术 + 半自动打招呼
 *  5. 频率与安全设置：租户级 KV，动作前置校验在服务端权威判定
 *
 * 合规硬约束（与抖音/小红书一致）：
 *  - 后台不模拟登录任何快手账号、不自动群发；
 *  - 同步以结构化估算为主（sourced_from=estimate），严禁把估算冒充真实数据；
 *  - AI 只负责生成话术建议，发送动作需人工在官方快手客户端完成。
 * ================================================================ */

const fmtN = (n?: number | null): string => {
  const v = Number(n || 0);
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return String(v);
};

const LEAD_TAGS = ['男·单身', '女·单身', '家长', '其他'];
const LEAD_STATES = ['待跟进', '已打招呼', '已回复'];

export default function KuaishouGain() {
  const [tab, setTab] = useState('account');

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1200, margin: '0 auto' }}>
      {/* 页头 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#FF5F00,#FF8A3D,#FFC069)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>快手获客</span>
        <Tag color="orange" size="small">Beta</Tag>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 12 }}>
        账号管理 · 同行追踪 · 视频分析 · 话术打招呼（半自动：AI 生成话术，人工官方客户端发送）
        <QuotaBadge />
      </div>

      <Tabs activeTab={tab} onChange={(k) => setTab(String(k))} type="line" style={{ marginBottom: 16 }}>
        <Tabs.TabPane key="account" title={<span><IconUserGroup style={{ marginRight: 6 }} />账号管理</span>} />
        <Tabs.TabPane key="peer" title={<span><IconLink style={{ marginRight: 6 }} />同行追踪</span>} />
        <Tabs.TabPane key="video" title={<span><IconPlayArrow style={{ marginRight: 6 }} />视频分析</span>} />
        <Tabs.TabPane key="lead" title={<span><IconBulb style={{ marginRight: 6 }} />获客线索</span>} />
        <Tabs.TabPane key="settings" title={<span><IconSafe style={{ marginRight: 6 }} />频率与安全</span>} />
        <Tabs.TabPane key="log" title={<span><IconClockCircle style={{ marginRight: 6 }} />动作日志</span>} />
      </Tabs>

      {tab === 'account' && <AccountTab />}
      {tab === 'peer' && <PeerTab />}
      {tab === 'video' && <VideoTab />}
      {tab === 'lead' && <LeadTab />}
      {tab === 'settings' && <SettingsTab />}
      {tab === 'log' && <LogTab />}
    </div>
  );
}

/* ============ 1. 账号管理 ============ */
function AccountTab() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form, setForm] = useState({ nickname: '', region: '', daily_limit: 30 });

  const load = async () => {
    setLoading(true);
    try {
      const d: any = await api.ksListAccounts();
      setList(d || []);
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const submit = async () => {
    if (!form.nickname.trim()) { Message.warning('请填写账号昵称'); return; }
    try {
      if (editing) {
        await api.ksUpdateAccount(editing.id, form);
        Message.success('已更新');
      } else {
        await api.ksCreateAccount(form);
        Message.success('已添加');
      }
      setVisible(false); setEditing(null);
      setForm({ nickname: '', region: '', daily_limit: 30 });
      load();
    } catch (e: any) {
      Message.error(e?.message || '保存失败');
    }
  };

  const remove = async (id: number) => {
    try {
      await api.ksDeleteAccount(id);
      Message.success('已删除');
      load();
    } catch {
      Message.error('删除失败');
    }
  };

  return (
    <Card title="快手获客账号" bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
      extra={<Button type="primary" size="small" icon={<IconPlus />} onClick={() => { setEditing(null); setForm({ nickname: '', region: '', daily_limit: 30 }); setVisible(true); }}>添加账号</Button>}>
      <Table
        rowKey="id"
        loading={loading}
        data={list}
        pagination={false}
        columns={[
          { title: '昵称', dataIndex: 'nickname', width: 160, render: (v) => <b>{v}</b> },
          { title: '地区', dataIndex: 'region', width: 120, render: (v) => v || '—' },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (v) => <Tag color={v === '在线' ? 'green' : v === '冷却中' ? 'orange' : 'gray'}>{v}</Tag>,
          },
          { title: '今日已用', dataIndex: 'today_used', width: 110, render: (v, r) => `${v}/${r.daily_limit}` },
          { title: '累计获客', dataIndex: 'total_leads', width: 100, render: (v) => v || 0 },
          {
            title: '操作', width: 160,
            render: (_, r) => (
              <Space>
                <Button size="mini" type="text" icon={<IconEdit />} onClick={() => { setEditing(r); setForm({ nickname: r.nickname, region: r.region || '', daily_limit: r.daily_limit }); setVisible(true); }}>编辑</Button>
                <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => remove(r.id)}>删除</Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑账号' : '添加账号'} visible={visible} onCancel={() => setVisible(false)} onOk={submit} okText="保存">
        <Space direction="vertical" style={{ width: '100%' }} size="medium">
          <Input placeholder="账号昵称（如：红娘小婷）" value={form.nickname} onChange={(v) => setForm((f) => ({ ...f, nickname: v }))} />
          <Input placeholder="地区（如：河南·郑州）" value={form.region} onChange={(v) => setForm((f) => ({ ...f, region: v }))} />
          <div>
            <div style={{ marginBottom: 6, color: '#4e5969', fontSize: 13 }}>每日打招呼上限：{form.daily_limit}</div>
            <Slider min={10} max={100} value={form.daily_limit} onChange={(v) => setForm((f) => ({ ...f, daily_limit: v as number }))} />
          </div>
        </Space>
      </Modal>
    </Card>
  );
}

/* ============ 2. 同行追踪 ============ */
function PeerTab() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [links, setLinks] = useState('');
  const [importing, setImporting] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const d: any = await api.ksListPeers();
      setList(d || []);
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const doImport = async () => {
    if (!links.trim()) { Message.warning('请粘贴快手主页链接'); return; }
    setImporting(true);
    try {
      const r: any = await api.ksImportPeers(links);
      Message.success(`成功导入 ${r.created?.length || 0} 个同行${r.skipped?.length ? `，跳过 ${r.skipped.length} 个` : ''}`);
      setLinks('');
      notifyQuotaChanged();
      load();
    } catch (e: any) {
      Message.error(e?.message || '导入失败');
    } finally {
      setImporting(false);
    }
  };

  const remove = async (id: number) => {
    try {
      await api.ksDeletePeer(id);
      Message.success('已删除');
      load();
    } catch {
      Message.error('删除失败');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
        <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <TextArea
            value={links}
            onChange={setLinks}
            placeholder={'粘贴快手同行主页链接，每行一个\n支持 kuaishou.com/profile/xxx 格式'}
            style={{ flex: 1, minWidth: 300 }}
            rows={3}
          />
          <Button type="primary" icon={<IconLink />} loading={importing} onClick={doImport}>导入同行</Button>
        </div>
        <div style={{ marginTop: 10, color: '#86909c', fontSize: 12 }}>
          提示：快手公开主页数据极少，同步以结构化估算为主（sourced_from=estimate），估算数据不可作为真实运营依据，请以官方客户端为准
        </div>
      </Card>

      <Card title={`已追踪同行（${list.length}）`} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
        <Table
          rowKey="id"
          loading={loading}
          data={list}
          pagination={{ pageSize: 10, showTotal: true }}
          columns={[
            { title: '昵称', dataIndex: 'nickname', width: 160, render: (v) => <b>{v}</b> },
            { title: '主页链接', dataIndex: 'link', ellipsis: true },
            { title: '粉丝', dataIndex: 'fans_count', width: 100, render: (v) => fmtN(v) },
            { title: '视频', dataIndex: 'video_count', width: 80, render: (v) => v || 0 },
            {
              title: '数据来源', dataIndex: 'sourced_from', width: 100,
              render: (v) => <Tag color={v === 'real' ? 'green' : 'orange'}>{v === 'real' ? '真实抓取' : '估算'}</Tag>,
            },
            {
              title: '操作', width: 100,
              render: (_, r) => (
                <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => remove(r.id)}>删除</Button>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

/* ============ 3. 视频分析 ============ */
function VideoTab() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setData(await api.ksAnalysis());
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  if (loading && !data) {
    return <div style={{ padding: '60px 0', textAlign: 'center' }}><Spin size={24} /></div>;
  }
  const top = data?.top_videos || [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Row gutter={16}>
        <Col span={8}>
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', textAlign: 'center' }}>
            <div style={{ fontSize: 30, fontWeight: 700, color: '#FF5F00' }}>{data?.peer_count || 0}</div>
            <div style={{ color: '#86909c', fontSize: 13, marginTop: 4 }}>追踪同行</div>
          </Card>
        </Col>
        <Col span={8}>
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', textAlign: 'center' }}>
            <div style={{ fontSize: 30, fontWeight: 700, color: '#165DFF' }}>{data?.video_count || 0}</div>
            <div style={{ color: '#86909c', fontSize: 13, marginTop: 4 }}>监控视频</div>
          </Card>
        </Col>
        <Col span={8}>
          <Card bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', textAlign: 'center' }}>
            <div style={{ fontSize: 30, fontWeight: 700, color: '#00B42A' }}>
              {(data?.avg_interaction_rate || 0).toFixed(2)}%
            </div>
            <div style={{ color: '#86909c', fontSize: 13, marginTop: 4 }}>平均互动率</div>
          </Card>
        </Col>
      </Row>

      <Card title="Top 爆款视频（按点赞排序）" bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
        {top.length === 0 ? (
          <Empty description="暂无视频数据，请先在「同行追踪」导入同行" />
        ) : (
          <Table
            rowKey="id"
            data={top}
            pagination={false}
            columns={[
              { title: '视频标题', dataIndex: 'title', ellipsis: true },
              { title: '发布人', dataIndex: 'peer_name', width: 130 },
              { title: '播放', dataIndex: 'play_count', width: 100, render: (v) => fmtN(v) },
              { title: '点赞', dataIndex: 'like_count', width: 100, render: (v) => fmtN(v) },
              { title: '评论', dataIndex: 'comment_count', width: 90, render: (v) => fmtN(v) },
              { title: '互动率', dataIndex: 'interaction_rate', width: 90, render: (v) => (v || 0).toFixed(2) + '%' },
              {
                title: '来源', dataIndex: 'sourced_from', width: 80,
                render: (v) => <Tag color={v === 'real' ? 'green' : 'orange'}>{v === 'real' ? '真实' : '估算'}</Tag>,
              },
            ]}
          />
        )}
      </Card>
    </div>
  );
}

/* ============ 4. 获客线索 + 话术 ============ */
function LeadTab() {
  const [leads, setLeads] = useState<any[]>([]);
  const [slogans, setSlogans] = useState<any[]>([]);
  const [accounts, setAccounts] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [addVisible, setAddVisible] = useState(false);
  const [genVisible, setGenVisible] = useState(false);
  const [genTarget, setGenTarget] = useState<any>(null);
  const [genForm, setGenForm] = useState({ nickname: '', comment: '', tag: '', account_style: '', count: 3 });
  const [genLoading, setGenLoading] = useState(false);
  const [genResult, setGenResult] = useState<string[]>([]);
  const [greetLoading, setGreetLoading] = useState<number | null>(null);
  const [newLead, setNewLead] = useState({ nickname: '', comment: '', tag: '女·单身' });

  const loadAll = async () => {
    setLoading(true);
    try {
      const [l, s, a]: any[] = await Promise.all([api.ksListLeads(), api.ksListSlogans(), api.ksListAccounts()]);
      setLeads(l || []);
      setSlogans(s || []);
      setAccounts(a || []);
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { loadAll(); }, []);

  const copy = (t: string) => {
    navigator.clipboard?.writeText(t);
    Message.success('已复制');
  };

  const openGen = (lead: any) => {
    setGenTarget(lead);
    setGenForm({
      nickname: lead.nickname,
      comment: lead.comment || '',
      tag: lead.tag || '',
      account_style: '',
      count: 3,
    });
    setGenResult([]);
    setGenVisible(true);
  };

  const doGen = async () => {
    setGenLoading(true);
    setGenResult([]);
    try {
      const r: any = await api.ksGenerateSlogan(genForm);
      setGenResult(r.slogans || []);
      notifyQuotaChanged();
    } catch (e: any) {
      Message.error(e?.message || '生成失败');
    } finally {
      setGenLoading(false);
    }
  };

  const doGreet = async (lead: any, sloganText?: string) => {
    if (!lead) return;
    setGreetLoading(lead.id);
    try {
      const r: any = await api.ksPrecheck({ account_id: lead._acctId || 0 });
      if (!r.ok) {
        Message.warning(r.reason || '不满足打招呼条件');
        return;
      }
      await api.ksGreet({ account_id: lead._acctId || 0, lead_id: lead.id, note: sloganText ? `使用话术：${sloganText}` : '' });
      Message.success('已标记打招呼（请在快手官方客户端发送）');
      setGenVisible(false);
      loadAll();
    } catch (e: any) {
      Message.error(e?.message || '操作失败');
    } finally {
      setGreetLoading(null);
    }
  };

  const addLead = async () => {
    if (!newLead.nickname.trim()) { Message.warning('请填写客户昵称'); return; }
    try {
      await api.ksCreateLead(newLead);
      Message.success('已添加');
      setAddVisible(false);
      setNewLead({ nickname: '', comment: '', tag: '女·单身' });
      loadAll();
    } catch (e: any) {
      Message.error(e?.message || '添加失败');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card title={`获客线索（${leads.length}）`} bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
        extra={<Button type="primary" size="small" icon={<IconPlus />} onClick={() => setAddVisible(true)}>手动添加客户</Button>}>
        <Table
          rowKey="id"
          loading={loading}
          data={leads}
          pagination={{ pageSize: 10, showTotal: true }}
          columns={[
            { title: '昵称', dataIndex: 'nickname', width: 130, render: (v) => <b>{v}</b> },
            { title: '评论', dataIndex: 'comment', ellipsis: true },
            { title: '标签', dataIndex: 'tag', width: 90, render: (v) => <Tag color="arcoblue">{v || '—'}</Tag> },
            {
              title: '状态', dataIndex: 'state', width: 100,
              render: (v) => <Tag color={v === '已回复' ? 'green' : v === '已打招呼' ? 'arcoblue' : 'orange'}>{v}</Tag>,
            },
            {
              title: '操作', width: 230,
              render: (_, r) => (
                <Space>
                  <Button size="mini" type="primary" icon={<IconBulb />} onClick={() => openGen(r)}>AI 话术</Button>
                  <Select
                    size="mini"
                    placeholder="选择账号"
                    style={{ width: 110 }}
                    value={r._acctId || undefined}
                    onChange={(v) => { r._acctId = v; setLeads([...leads]); }}
                    options={accounts.map((a: any) => ({ label: a.nickname, value: a.id }))}
                  />
                  <Button size="mini" status="success" icon={<IconThunderbolt />} loading={greetLoading === r.id} onClick={() => doGreet(r)}>打招呼</Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Card title="话术库" bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}>
        {slogans.length === 0 ? (
          <Empty description="暂无话术，可从客户线索点击「AI 话术」生成，或直接生成开场白话术" />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {slogans.slice(0, 20).map((s: any) => (
              <div key={s.id} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 10 }}>
                <Tag size="small">{s.category || '开场白'}</Tag>
                <span style={{ flex: 1, fontSize: 13, color: '#4e5969' }}>{s.text}</span>
                <span style={{ fontSize: 11, color: '#a9aeb8' }}>使用 {s.used_count || 0} 次</span>
                <Button size="mini" type="text" icon={<IconCopy />} onClick={() => copy(s.text)}>复制</Button>
                <Button size="mini" type="text" icon={<IconDelete />} onClick={async () => { await api.ksDeleteSlogan(s.id); loadAll(); }} />
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* 手动添加客户 */}
      <Modal title="手动添加客户" visible={addVisible} onCancel={() => setAddVisible(false)} onOk={addLead} okText="添加">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="客户昵称" value={newLead.nickname} onChange={(v) => setNewLead((f) => ({ ...f, nickname: v }))} />
          <TextArea placeholder="客户评论/意向（可选）" value={newLead.comment} onChange={(v) => setNewLead((f) => ({ ...f, comment: v }))} rows={3} />
          <Select value={newLead.tag} onChange={(v) => setNewLead((f) => ({ ...f, tag: String(v) }))} options={LEAD_TAGS.map((t) => ({ label: t, value: t }))} style={{ width: '100%' }} />
        </Space>
      </Modal>

      {/* AI 话术弹窗 */}
      <Modal
        title={`AI 生成话术 · ${genTarget?.nickname || ''}`}
        visible={genVisible}
        onCancel={() => setGenVisible(false)}
        footer={
          <Space>
            <Button onClick={() => setGenVisible(false)}>取消</Button>
            {genResult.length > 0 && (
              <Button type="primary" status="success" onClick={() => doGreet(genTarget, genResult[0])}>用第 1 条打招呼</Button>
            )}
            <Button type="primary" icon={<IconBulb />} loading={genLoading} onClick={doGen}>
              {genResult.length > 0 ? '重新生成' : '生成话术'}
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div style={{ color: '#4e5969', fontSize: 13 }}>客户评论：{genForm.comment || '（无）'}</div>
          <TextArea placeholder="打招呼账号风格（可选，如：红娘小婷｜郑州婚恋）" value={genForm.account_style} onChange={(v) => setGenForm((f) => ({ ...f, account_style: v }))} rows={2} />
          {genLoading && <div style={{ textAlign: 'center', padding: 20 }}><Spin /></div>}
          {genResult.map((s, i) => (
            <div key={i} style={{ display: 'flex', gap: 8, alignItems: 'center', padding: '10px 12px', background: 'var(--color-fill-1)', borderRadius: 10 }}>
              <span style={{ flex: 1, fontSize: 13 }}>{s}</span>
              <Button size="mini" type="text" icon={<IconCopy />} onClick={() => copy(s)}>复制</Button>
            </div>
          ))}
        </Space>
      </Modal>
    </div>
  );
}

/* ============ 5. 频率与安全 ============ */
function SettingsTab() {
  const [f, setF] = useState<any>(null);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    try {
      setF(await api.ksGetSettings());
    } catch {
      Message.error('加载失败');
    }
  };
  useEffect(() => { load(); }, []);

  const save = async () => {
    setSaving(true);
    try {
      await api.ksSaveSettings(f);
      Message.success('已保存');
    } catch (e: any) {
      Message.error(e?.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (!f) return <Spin />;

  return (
    <Card title="频率与安全设置" bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)', maxWidth: 560 }}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <div>
          <div style={{ marginBottom: 6, color: '#4e5969', fontSize: 13 }}>每账号每日打招呼上限：{f.daily_limit}</div>
          <Slider min={1} max={200} value={f.daily_limit} onChange={(v) => setF((x: any) => ({ ...x, daily_limit: v as number }))} />
        </div>
        <div>
          <div style={{ marginBottom: 6, color: '#4e5969', fontSize: 13 }}>两次动作最小间隔（分钟）：{f.interval_min}</div>
          <Slider min={0} max={60} value={f.interval_min} onChange={(v) => setF((x: any) => ({ ...x, interval_min: v as number }))} />
        </div>
        <Space>
          <div style={{ color: '#4e5969', fontSize: 13 }}>活跃时段</div>
          <Input style={{ width: 90 }} value={f.active_start} onChange={(v) => setF((x: any) => ({ ...x, active_start: v }))} placeholder="09:00" />
          <span style={{ color: '#86909c' }}>~</span>
          <Input style={{ width: 90 }} value={f.active_end} onChange={(v) => setF((x: any) => ({ ...x, active_end: v }))} placeholder="22:00" />
        </Space>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ color: '#4e5969', fontSize: 13 }}>达到上限后自动进入冷却</span>
          <Switch checked={f.cool_on} onChange={(v) => setF((x: any) => ({ ...x, cool_on: v }))} />
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ color: '#4e5969', fontSize: 13 }}>同日陌生人重复忽略</span>
          <Switch checked={f.repeat_on} onChange={(v) => setF((x: any) => ({ ...x, repeat_on: v }))} />
        </div>
        <Button type="primary" loading={saving} onClick={save} style={{ width: 120 }}>保存设置</Button>
      </Space>
    </Card>
  );
}

/* ============ 6. 动作日志 ============ */
function LogTab() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const d: any = await api.ksListLogs();
      setList(d || []);
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  return (
    <Card title="动作日志" bordered={false} style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
      extra={<Button size="small" icon={<IconRefresh />} onClick={load}>刷新</Button>}>
      {list.length === 0 ? (
        <Empty description="暂无动作记录" />
      ) : (
        <Timeline>
          {list.map((l: any) => (
            <Timeline.Item key={l.id} label={new Date(l.created_at).toLocaleString('zh-CN', { hour12: false })}>
              <div style={{ fontSize: 13 }}>
                <Tag size="small" color={l.action_type === 'greet' ? 'green' : 'arcoblue'}>
                  {l.action_type === 'greet' ? '打招呼' : l.action_type === 'ai_gen' ? 'AI 生成' : '标记'}
                </Tag>
                <b>{l.account_name}</b> → {l.target}
              </div>
              <div style={{ fontSize: 12, color: '#86909c', marginTop: 2 }}>{l.result}</div>
            </Timeline.Item>
          ))}
        </Timeline>
      )}
    </Card>
  );
}
