import { useEffect, useMemo, useState } from 'react';
import {
  Card, Grid, Button, Message, Tag, Space, Typography, Input, Tabs, Table, Switch, Slider, Divider, Tooltip,
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
 * 抖音获客页 · 完整内部功能（前后端全栈，已对接真实后端 API）
 *
 * 模块：
 *  1. 账号管理：多账号 CRUD + 今日动作计数 + 冷却/恢复计算
 *  2. 同行追踪：粘贴主页链接批量导入 → 公开页尽力抓取，失败降级估算
 *  3. 数据分析：KPI + Top 爆款视频 + 近7天发布分布（后端聚合接口）
 *  4. 获客打招呼：评论线索解析/手动添加 + 话术库 + 半自动打招呼
 *  5. 频率与安全设置：租户级 KV，动作前置校验在服务端权威判定
 *
 * 合规硬约束（代码注释 & 页面提示双重体现）：
 *  - 后台不模拟登录任何抖音账号、不自动群发；
 *  - 同步抓取仅面向公开数据且尽力而为，失败降级为结构化估算数据，
 *    接口/页面均标记 sourced_from=real/estimate，严禁把估算冒充真实数据；
 *  - 发送动作需人工在官方客户端执行：本页只做「复制话术 + 标记已打招呼」。
 * ================================================================ */

const fmtN = (n?: number | null): string => {
  const v = Number(n || 0);
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return String(v);
};

// 播放量：抖音平台不公开播放数（API 恒为 0），0 时显示「不公开」
const fmtPlay = (n?: number | null): string => {
  const v = Number(n || 0);
  if (v <= 0) return '不公开';
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return String(v);
};

const fmtTime = (t?: string | null): string => {
  if (!t) return '--';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return '--';
  const p = (x: number) => (x < 10 ? `0${x}` : String(x));
  return `${d.getMonth() + 1}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

// 互动率格式化：基于算法机制 (点赞+评论)/播放*100，统一精确到小数点后1位（如 12.7）
const fmtRate = (n?: number | null): string => {
  const v = Number(n || 0);
  if (!Number.isFinite(v) || v <= 0) return '0.0';
  if (v > 999.9) return '999.9+';
  return v.toFixed(1);
};

const srcTag = (s?: string) =>
  s === 'real' ? <Tag color="green" size="small">真实数据</Tag> : <Tag color="orange" size="small">估算数据</Tag>;

function Kpi({ label, value, color, suffix }: { label: string; value: string; color: string; suffix?: string }) {
  return (
    <Card style={{ textAlign: 'center' }} bodyStyle={{ padding: '16px 8px' }}>
      <div style={{ color: '#86909C', fontSize: 13 }}>{label}</div>
      <div style={{ fontSize: 28, fontWeight: 700, color, marginTop: 6, fontVariantNumeric: 'tabular-nums' }}>
        {value}
        {suffix && <span style={{ fontSize: 14, fontWeight: 400, marginLeft: 2 }}>{suffix}</span>}
      </div>
    </Card>
  );
}

export default function DouyinGain() {
  /* ---------------- 数据 ---------------- */
  const [accounts, setAccounts] = useState<any[]>([]);
  const [peers, setPeers] = useState<any[]>([]);
  const [leads, setLeads] = useState<any[]>([]);
  const [slogans, setSlogans] = useState<any[]>([]);
  const [logs, setLogs] = useState<any[]>([]);
  const [analysis, setAnalysis] = useState<any>({ kpi: {}, top_videos: [], weekly: [] });
  const [loading, setLoading] = useState(true);

  /* ---------------- 频率与安全设置 ---------------- */
  const [settings, setSettings] = useState<any>({
    daily_limit: 30, interval_min: 5, active_start: '09:00', active_end: '22:00', cool_on: true, repeat_on: true,
  });

  /* ---------------- 账号弹窗 ---------------- */
  const [acctModal, setAcctModal] = useState<{ visible: boolean; editing: any | null }>({ visible: false, editing: null });
  const [acctForm, setAcctForm] = useState({ nickname: '', region: '', daily_limit: 30, status: '在线' });

  /* ---------------- 话术弹窗 ---------------- */
  const [slgModal, setSlgModal] = useState<{ visible: boolean; editing: any | null }>({ visible: false, editing: null });
  const [slgForm, setSlgForm] = useState({ category: '开场白', text: '' });

  /* ---------------- 线索弹窗 ---------------- */
  const [leadModal, setLeadModal] = useState(false);
  const [leadForm, setLeadForm] = useState({ nickname: '', peer_id: 0, tag: '男·单身', comment: '' });

  /* ---------------- 解析评论弹窗 ---------------- */
  const [parseModal, setParseModal] = useState(false);
  const [parseForm, setParseForm] = useState({ peer_id: 0, count: 8 });
  const [parsing, setParsing] = useState(false);

  /* ---------------- 打招呼流程 ---------------- */
  const [greetLead, setGreetLead] = useState<any | null>(null);
  const [greetAccountId, setGreetAccountId] = useState(0);
  const [greetSloganId, setGreetSloganId] = useState(0);
  const [precheck, setPrecheck] = useState<any | null>(null);
  const [prechecking, setPrechecking] = useState(false);
  const [greeting, setGreeting] = useState(false);

  /* ---------------- AI 话术生成 ---------------- */
  const [aiGen, setAiGen] = useState<{ loading: boolean; slogans: any[]; platform: string; model: string; error: string }>({
    loading: false, slogans: [], platform: '', model: '', error: '',
  });

  /* ---------------- 视频详情 ---------------- */
  const [videoPeer, setVideoPeer] = useState<any | null>(null);
  const [videos, setVideos] = useState<any[]>([]);
  const [videoLoading, setVideoLoading] = useState(false);

  const loadAll = async () => {
    try {
      const [accts, ps, ls, sg, st, lg] = await Promise.all([
        api.douyinListAccounts(), api.douyinListPeers(), api.douyinListLeads(),
        api.douyinListSlogans(), api.douyinGetSettings(), api.douyinListLogs('page=1&page_size=20'),
      ]);
      setAccounts(accts || []);
      setPeers(ps || []);
      setLeads(ls || []);
      setSlogans(sg || []);
      if (st) setSettings((s: any) => ({ ...s, ...st }));
      setLogs(lg?.list || []);
    } catch (e: any) {
      Message.error(e.message || '加载数据失败');
    }
  };

  const loadAnalysis = async () => {
    try {
      const a = await api.douyinAnalysis();
      setAnalysis(a || { kpi: {}, top_videos: [], weekly: [] });
    } catch (e: any) {
      Message.error(e.message || '加载分析数据失败');
    }
  };

  useEffect(() => {
    setLoading(true);
    Promise.all([loadAll(), loadAnalysis()]).finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* ---------------- 账号管理 ---------------- */
  const openAddAccount = () => {
    setAcctForm({ nickname: '', region: '', daily_limit: settings.daily_limit || 30, status: '在线' });
    setAcctModal({ visible: true, editing: null });
  };
  const openEditAccount = (a: any) => {
    setAcctForm({ nickname: a.nickname, region: a.region || '', daily_limit: a.daily_limit, status: a.status });
    setAcctModal({ visible: true, editing: a });
  };
  const submitAccount = async () => {
    if (!acctForm.nickname.trim()) {
      Message.warning('请填写账号昵称');
      return;
    }
    try {
      if (acctModal.editing) {
        await api.douyinUpdateAccount(acctModal.editing.id, acctForm);
        Message.success('账号已更新');
      } else {
        await api.douyinCreateAccount(acctForm);
        Message.success('账号已添加');
      }
      setAcctModal({ visible: false, editing: null });
      setAccounts(await api.douyinListAccounts());
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    }
  };
  const removeAccount = (a: any) => {
    Modal.confirm({
      title: '删除账号',
      content: `确认移除账号「${a.nickname}」？其动作日志将保留。`,
      onOk: async () => {
        try {
          await api.douyinDeleteAccount(a.id);
          Message.success('已删除');
          setAccounts(await api.douyinListAccounts());
        } catch (e: any) {
          Message.error(e.message || '删除失败');
        }
      },
    });
  };

  /* ---------------- 同行追踪 ---------------- */
  const [peerInput, setPeerInput] = useState('');
  const [importing, setImporting] = useState(false);
  const importPeers = async () => {
    if (!peerInput.trim()) {
      Message.warning('请先粘贴同行主页链接');
      return;
    }
    setImporting(true);
    try {
      const res = await api.douyinImportPeers(peerInput);
      setPeerInput('');
      Message.success(`已加入 ${res.created?.length || 0} 个同行${res.skipped?.length ? `，跳过 ${res.skipped.length} 个（重复/失败）` : ''}`);
      const ps = await api.douyinListPeers();
      setPeers(ps || []);
      await loadAnalysis();
    } catch (e: any) {
      Message.error(e.message || '导入失败');
    } finally {
      setImporting(false);
    }
  };
  const refreshPeer = async (p: any) => {
    try {
      Message.loading(`正在刷新「${p.nickname}」...`);
      const res = await api.douyinRefreshPeer(p.id);
      Message.success(`已刷新（${res.sourced_from === 'real' ? '真实数据' : '估算数据'}）`);
      notifyQuotaChanged(); // 刷新同行消耗每日查询配额，立即刷新余量显示
      const ps = await api.douyinListPeers();
      setPeers(ps || []);
      await loadAnalysis();
    } catch (e: any) {
      Message.error(e.message || '刷新失败');
    }
  };
  const removePeer = (p: any) => {
    Modal.confirm({
      title: '移除同行',
      content: `确认移除「${p.nickname}」？其视频数据将一并删除，客户线索保留。`,
      onOk: async () => {
        try {
          await api.douyinDeletePeer(p.id);
          Message.success('已移除');
          const ps = await api.douyinListPeers();
          setPeers(ps || []);
          await loadAnalysis();
        } catch (e: any) {
          Message.error(e.message || '删除失败');
        }
      },
    });
  };
  // 一键催更新全部同行（重抓数据 + 刷新 Top 爆款视频）
  const [refreshingAll, setRefreshingAll] = useState(false);
  const refreshAllPeers = async () => {
    setRefreshingAll(true);
    try {
      const res = await api.douyinRefreshAllPeers();
      Message.success(`已更新 ${res.updated || 0} 个同行`);
      notifyQuotaChanged(); // 批量更新消耗每日查询配额（算 1 次）
      const ps = await api.douyinListPeers();
      setPeers(ps || []);
      await loadAnalysis();
    } catch (e: any) {
      Message.error(e.message || '更新失败');
    } finally {
      setRefreshingAll(false);
    }
  };
  const showVideos = async (p: any) => {
    setVideoPeer(p);
    setVideoLoading(true);
    try {
      const vs = await api.douyinListVideos(p.id);
      setVideos(vs || []);
    } catch (e: any) {
      Message.error(e.message || '加载视频失败');
    } finally {
      setVideoLoading(false);
    }
  };

  /* ---------------- 话术库 ---------------- */
  const copySlogan = (text: string) => {
    navigator.clipboard?.writeText(text).catch(() => undefined);
    Message.success('话术已复制，前往抖音客户端私信粘贴发送');
  };
  const openAddSlogan = () => {
    setSlgForm({ category: '开场白', text: '' });
    setSlgModal({ visible: true, editing: null });
  };
  const openEditSlogan = (s: any) => {
    setSlgForm({ category: s.category, text: s.text });
    setSlgModal({ visible: true, editing: s });
  };
  const submitSlogan = async () => {
    if (!slgForm.text.trim()) {
      Message.warning('请填写话术内容');
      return;
    }
    try {
      if (slgModal.editing) {
        await api.douyinUpdateSlogan(slgModal.editing.id, slgForm);
        Message.success('话术已更新');
      } else {
        await api.douyinCreateSlogan(slgForm);
        Message.success('话术已添加');
      }
      setSlgModal({ visible: false, editing: null });
      setSlogans(await api.douyinListSlogans());
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    }
  };
  const removeSlogan = (s: any) => {
    Modal.confirm({
      title: '删除话术',
      content: '确认删除这条话术？',
      onOk: async () => {
        try {
          await api.douyinDeleteSlogan(s.id);
          Message.success('已删除');
          setSlogans(await api.douyinListSlogans());
        } catch (e: any) {
          Message.error(e.message || '删除失败');
        }
      },
    });
  };
  const useSloganCount = async (s: any) => {
    try {
      await api.douyinUseSlogan(s.id);
      setSlogans(await api.douyinListSlogans());
    } catch { /* ignore */ }
  };

  /* ---------------- 线索：解析 / 添加 / 状态 ---------------- */
  const submitParse = async () => {
    if (!parseForm.peer_id) {
      Message.warning('请选择来源同行');
      return;
    }
    setParsing(true);
    try {
      const res = await api.douyinParseLeads(parseForm);
      Message.success(`已解析 ${res.created} 条线索${res.skipped ? `（跳过重复 ${res.skipped}）` : ''}`);
      setParseModal(false);
      setLeads(await api.douyinListLeads());
    } catch (e: any) {
      Message.error(e.message || '解析失败');
    } finally {
      setParsing(false);
    }
  };
  const submitLead = async () => {
    if (!leadForm.nickname.trim()) {
      Message.warning('请填写客户昵称');
      return;
    }
    try {
      await api.douyinCreateLead(leadForm);
      Message.success('已添加线索');
      setLeadModal(false);
      setLeadForm({ nickname: '', peer_id: 0, tag: '男·单身', comment: '' });
      setLeads(await api.douyinListLeads());
    } catch (e: any) {
      Message.error(e.message || '添加失败');
    }
  };
  const changeLeadState = async (l: any, state: string) => {
    try {
      await api.douyinUpdateLead(l.id, { state });
      Message.success(state === '已打招呼' ? '已标记为已打招呼（请到官方客户端完成发送）' : '状态已更新');
      setLeads(await api.douyinListLeads());
      setLogs((await api.douyinListLogs('page=1&page_size=20'))?.list || []);
    } catch (e: any) {
      Message.error(e.message || '更新失败');
    }
  };
  const removeLead = (l: any) => {
    Modal.confirm({
      title: '删除线索',
      content: `确认删除客户「${l.nickname}」？`,
      onOk: async () => {
        try {
          await api.douyinDeleteLead(l.id);
          Message.success('已删除');
          setLeads(await api.douyinListLeads());
        } catch (e: any) {
          Message.error(e.message || '删除失败');
        }
      },
    });
  };

  /* ---------------- 打招呼（半自动：复制话术 + 标记已打招呼） ---------------- */
  const openGreet = async (l: any) => {
    setGreetLead(l);
    setGreetAccountId(0);
    setGreetSloganId(0);
    setPrecheck(null);
    if (accounts.length === 0) {
      Message.warning('请先在「账号管理」中添加抖音账号');
      return;
    }
    setGreetAccountId(accounts[0].id);
    setPrechecking(true);
    try {
      const res = await api.douyinPrecheck({ account_id: accounts[0].id, lead_id: l.id });
      setPrecheck(res);
    } catch (e: any) {
      Message.error(e.message || '前置校验失败');
    } finally {
      setPrechecking(false);
    }
  };
  const changeGreetAccount = async (id: any) => {
    setGreetAccountId(Number(id));
    setPrechecking(true);
    try {
      const res = await api.douyinPrecheck({ account_id: Number(id), lead_id: greetLead?.id });
      setPrecheck(res);
    } catch (e: any) {
      Message.error(e.message || '前置校验失败');
    } finally {
      setPrechecking(false);
    }
  };
  const runAiGenerate = async () => {
    const lead = greetLead;
    if (!lead) return;
    const acct = accounts.find((a) => a.id === greetAccountId);
    setAiGen({ loading: true, slogans: [], platform: '', model: '', error: '' });
    try {
      const res = await api.douyinAiGenerate({
        nickname: lead.nickname,
        comment: lead.comment || '',
        tag: lead.tag || '',
        account_style: acct ? `${acct.nickname || ''}${acct.region ? '（' + acct.region + '）' : ''}` : '',
        count: 3,
      });
      setAiGen({ loading: false, slogans: res.slogans || [], platform: res.platform || '', model: res.model || '', error: '' });
    } catch (e: any) {
      setAiGen({ loading: false, slogans: [], platform: '', model: '', error: e.message || 'AI 生成失败' });
    }
  };
  const doGreet = async () => {
    if (!greetAccountId) {
      Message.warning('请选择动作账号');
      return;
    }
    setGreeting(true);
    try {
      const res = await api.douyinGreet({ account_id: greetAccountId, lead_id: greetLead.id, slogan_id: greetSloganId });
      Message.success('已标记已打招呼，请到官方抖音客户端完成发送');
      setGreetLead(null);
      setAccounts(await api.douyinListAccounts());
      setLeads(await api.douyinListLeads());
      setSlogans(await api.douyinListSlogans());
      setLogs((await api.douyinListLogs('page=1&page_size=20'))?.list || []);
    } catch (e: any) {
      Message.error(e.message || '操作失败');
    } finally {
      setGreeting(false);
    }
  };

  /* ---------------- 频率与安全 ---------------- */
  const saveSettings = async () => {
    try {
      await api.douyinSaveSettings(settings);
      Message.success('设置已保存，动作前置校验即时生效');
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    }
  };

  const stats = useMemo(() => {
    const k = analysis.kpi || {};
    return {
      peers: k.peers ?? peers.length,
      totalVideos: k.total_videos ?? 0,
      totalComments: k.total_comments ?? 0,
      avgRate: k.avg_rate ?? 0,
      realVideos: k.real_videos ?? 0,
      estimateVideos: k.estimate_videos ?? 0,
    };
  }, [analysis, peers]);

  const weekly = useMemo(() => {
    const list = analysis.weekly || [];
    const max = Math.max(1, ...list.map((w: any) => Number(w.count) || 0));
    return list.map((w: any) => ({
      weekday: w.weekday,
      count: Number(w.count) || 0,
      height: Math.max(8, Math.round((Number(w.count) || 0) / max * 100)),
    }));
  }, [analysis]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Spin loading={loading} style={{ display: 'block' }}>
      {/* 页首 */}
      <Card bodyStyle={{ padding: '18px 20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#165DFF,#14C9C9,#FF7D00)' }} />
          <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>抖音获客</span>
          <Tag color="arcoblue" size="small">Beta</Tag>
          <Tag color="green" size="small">合规半自动</Tag>
          <QuotaBadge compact />
        </div>
        <div style={{ color: '#86909C', fontSize: 13, marginTop: 4 }}>
          多账号管理 · 同行视频数据分析 · 评论区客户获取 · 打招呼话术辅助（人工确认执行）
        </div>
        <div style={{ color: '#F77234', fontSize: 12, marginTop: 6, background: 'rgba(247,114,52,.08)', padding: '6px 10px', borderRadius: 4 }}>
          合规说明：本系统不模拟登录任何抖音账号、不自动群发；同步数据为公开页尽力抓取，失败降级为«估算数据»（页面已标注真实/估算）；
          打招呼动作需您在官方抖音客户端人工完成，本页仅提供话术复制与状态记录。
        </div>
      </Card>

      {/* 1. 账号管理 */}
      <Card
        title={<Space><IconUserGroup />账号管理（多账号）</Space>}
        extra={<Button size="small" type="primary" icon={<IconPlus />} onClick={openAddAccount}>添加账号</Button>}
      >
        <Table
          rowKey="id"
          data={accounts}
          pagination={false}
          noDataElement={<Empty description="暂无账号，点击右上角添加" />}
          columns={[
            {
              title: '账号昵称', dataIndex: 'nickname', render: (v, r) => (
                <div>
                  <div style={{ fontWeight: 600 }}>{v}</div>
                  <div style={{ color: '#86909C', fontSize: 12 }}>累计获客 {r.total_leads || 0} · 最后动作 {fmtTime(r.last_action_at)}</div>
                </div>
              ),
            },
            { title: '地区城市', dataIndex: 'region', width: 130, render: (v) => (v ? <Tag color="arcoblue">{v}</Tag> : '--') },
            {
              title: '状态', dataIndex: 'status', width: 90, render: (v) => (
                <Tag color={v === '在线' ? 'green' : v === '冷却中' ? 'orange' : 'gray'}>{v}</Tag>
              ),
            },
            {
              title: '今日动作', dataIndex: 'today_used', render: (v, r) => (
                <div style={{ width: 120 }}>
                  <Progress
                    percent={Math.min(100, Math.round((Number(v) / Math.max(1, r.daily_limit)) * 100))}
                    formatText={() => `${r.today_used || 0}/${r.daily_limit}`} size="small"
                  />
                </div>
              ),
            },
            { title: '关联同行', dataIndex: 'peer_ids', width: 90, render: (v) => (Array.isArray(v) ? v.length : 0) },
            { title: '冷却截止', dataIndex: 'cooldown_until', width: 110, render: (v) => (v ? fmtTime(v) : '--') },
            {
              title: '操作', width: 150, render: (_, r) => (
                <Space>
                  <Button size="mini" icon={<IconEdit />} onClick={() => openEditAccount(r)}>编辑</Button>
                  <Button size="mini" status="warning" icon={<IconDelete />} onClick={() => removeAccount(r)}>移除</Button>
                </Space>
              ),
            },
          ]}
        />
        <div style={{ color: '#86909C', fontSize: 12, marginTop: 8 }}>
          提示：账号仅作状态记录与数据分析，请在官方抖音客户端/网页正常登录后使用，后台不模拟登录账号。
        </div>
      </Card>

      {/* 2. 同行追踪 */}
      <Card
        title={<Space><IconLink />同行追踪</Space>}
        extra={<Button size="mini" icon={<IconRefresh />} loading={refreshingAll} onClick={refreshAllPeers}>一键更新全部</Button>}
      >
        <Space style={{ marginBottom: 12, width: '100%' }} align="start">
          <TextArea
            placeholder="粘贴同行抖音主页链接（可多个，每行一个，如 https://www.douyin.com/user/xxx 或 v.douyin.com 短链）"
            value={peerInput}
            onChange={(v) => setPeerInput(v)}
            style={{ flex: 1, minHeight: 44 }}
            autoSize={{ minRows: 1, maxRows: 4 }}
          />
          <Button type="primary" icon={<IconPlus />} loading={importing} onClick={importPeers}>加入追踪</Button>
        </Space>
        <Table
          rowKey="id"
          data={peers}
          pagination={false}
          noDataElement={<Empty description="暂无同行，粘贴主页链接加入跟踪" />}
          columns={[
            {
              title: '同行账号', dataIndex: 'nickname', render: (v, r) => (
                <div>
                  <div style={{ fontWeight: 600 }}>{v} {srcTag(r.sourced_from)}</div>
                  <div style={{ color: '#86909C', fontSize: 12, maxWidth: 300, wordBreak: 'break-all' }}>{r.link}</div>
                </div>
              ),
            },
            { title: '主页 ID', dataIndex: 'home_id', width: 140, ellipsis: true, render: (v) => v || '--' },
            { title: '粉丝', dataIndex: 'fans_count', width: 90, render: (v) => fmtN(v) },
            { title: '视频数', dataIndex: 'video_count', width: 80 },
            { title: '最近同步', dataIndex: 'last_sync_at', width: 120, render: (v) => fmtTime(v) },
            {
              title: '操作', width: 220, render: (_, r) => (
                <Space>
                  <Button size="mini" icon={<IconRedo />} onClick={() => refreshPeer(r)}>刷新</Button>
                  <Button size="mini" type="text" icon={<IconPlayArrow />} onClick={() => showVideos(r)}>数据详情</Button>
                  <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => removePeer(r)}>移除</Button>
                </Space>
              ),
            },
          ]}
        />
        <div style={{ color: '#86909C', fontSize: 12, marginTop: 8 }}>
          说明：同步仅抓取抖音公开页面，尽力而为；抓取失败或网络不可达时降级为可视化估算（标«估算数据»），不可作为真实运营依据。
        </div>
      </Card>

      {/* 3. 数据分析 */}
      <Card
        title={<Space><IconPlayArrow />同行数据分析</Space>}
        extra={
          <Space>
            <Tag color="gold">真实 {stats.realVideos} 条 / 估算 {stats.estimateVideos} 条</Tag>
            <Button size="mini" icon={<IconRefresh />} onClick={loadAnalysis}>刷新</Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}><Kpi label="追踪同行总数" value={String(stats.peers)} color="#165DFF" suffix="家" /></Col>
          <Col span={6}><Kpi label="总视频量" value={fmtN(stats.totalVideos)} color="#14C9C9" suffix="条" /></Col>
          <Col span={6}><Kpi label="累计评论互动" value={fmtN(stats.totalComments)} color="#F77234" suffix="条" /></Col>
          <Col span={6}><Kpi label="平均赞评比" value={String(stats.avgRate)} color="#C9CDD4" suffix="%" /></Col>
        </Row>
        <Row gutter={[16, 16]}>
          <Col span={14}>
            <Card title="Top 爆款视频" size="small" bodyStyle={{ padding: '8px 12px' }}>
              <Table
                rowKey="id"
                data={analysis.top_videos || []}
                pagination={false}
                noDataElement={<Empty description="暂无视频数据" />}
                columns={[
                  { title: '标题', dataIndex: 'title', ellipsis: true, render: (v: any, r: any) => <div><div style={{ color: 'var(--geo-text)' }}>{v}</div>{srcTag(r.sourced_from)}</div> },
                  { title: '来源', dataIndex: 'peer_name', width: 110, ellipsis: true },
                  { title: '播放', dataIndex: 'play_count', width: 80, render: (v) => fmtPlay(v) },
                  { title: '点赞', dataIndex: 'like_count', width: 70, render: (v) => fmtN(v) },
                  { title: '评论', dataIndex: 'comment_count', width: 70, render: (v) => fmtN(v) },
                  { title: '赞评比', dataIndex: 'interaction_rate', width: 80, render: (v) => <Tag color="green" size="small">{fmtRate(v)}%</Tag> },
                  { title: '发布时间', dataIndex: 'publish_time', width: 110, render: (v) => fmtTime(v) },
                ]}
              />
            </Card>
          </Col>
          <Col span={10}>
            <Card title="发布时间分布（近7天 按星期）" size="small">
              <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10, height: 140, padding: '4px 8px' }}>
                {weekly.map((w: any) => (
                  <div key={w.weekday} style={{ flex: 1, textAlign: 'center' }}>
                    <div style={{ fontSize: 12, color: '#86909C' }}>{w.count}</div>
                    <div style={{ background: 'linear-gradient(180deg,#165DFF,#69B1FF)', borderRadius: 4, height: w.height, margin: '4px auto 0', width: 22 }} />
                    <div style={{ fontSize: 11, color: '#C9CDD4', marginTop: 2 }}>{w.weekday}</div>
                  </div>
                ))}
              </div>
              <Divider style={{ margin: '12px 0 8px' }} />
              <div style={{ color: '#86909C', fontSize: 12 }}>
                洞察：观察同行发布高峰星期，把高意向内容安排到对应时段发布（统计数据含估算样本）。
              </div>
            </Card>
          </Col>
        </Row>
      </Card>

      {/* 4. 获客打招呼 */}
      <Card
        title={<Space><IconBulb />获客打招呼（留言区引客）</Space>}
        extra={
          <Space>
            <Tag color="orange">半自动 · 需人工确认发送</Tag>
            <Button size="mini" icon={<IconThunderbolt />} onClick={() => setParseModal(true)}>解析视频评论</Button>
            <Button size="mini" icon={<IconPlus />} onClick={() => setLeadModal(true)}>手动添加</Button>
          </Space>
        }
      >
        <Tabs defaultActiveTab="queue">
          <Tabs.TabPane
            key="queue"
            title={`待跟进客户（${leads.filter((l) => l.state === '待跟进').length}）`}
          >
            <Table
              rowKey="id"
              data={leads}
              pagination={{ pageSize: 10 }}
              columns={[
                {
                  title: '客户昵称', dataIndex: 'nickname', width: 130, render: (v: any, r: any) => (
                    <div style={{ fontWeight: 600 }}>{v}{r.sourced_from === 'estimate' && <Tag color="orange" size="small" style={{ marginLeft: 4 }}>估算</Tag>}</div>
                  ),
                },
                { title: '来源同行', dataIndex: 'peer_name', width: 140, ellipsis: true, render: (v) => v || '--' },
                { title: '来源视频', dataIndex: 'video_title', width: 170, ellipsis: true, render: (v) => v || '--' },
                { title: '评论/备注', dataIndex: 'comment', ellipsis: true },
                { title: '标签', dataIndex: 'tag', width: 90, render: (v) => <Tag color="purple">{v || '其他'}</Tag> },
                {
                  title: '状态', dataIndex: 'state', width: 100, render: (v) => (
                    <Tag color={v === '待跟进' ? 'red' : v === '已打招呼' ? 'arcoblue' : 'green'}>{v}</Tag>
                  ),
                },
                {
                  title: '操作', width: 250, render: (_, r) => (
                    <Space>
                      {r.state === '待跟进' && (
                        <Button size="mini" type="primary" icon={<IconRight />} onClick={() => openGreet(r)}>去打招呼</Button>
                      )}
                      {r.state === '已打招呼' && (
                        <Button size="mini" icon={<IconRight />} onClick={() => changeLeadState(r, '已回复')}>标记已回复</Button>
                      )}
                      {r.state === '已回复' && (
                        <Button size="mini" type="text" onClick={() => changeLeadState(r, '待跟进')}>重开</Button>
                      )}
                      <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => removeLead(r)} />
                    </Space>
                  ),
                },
              ]}
            />
          </Tabs.TabPane>

          <Tabs.TabPane key="slogan" title={`话术库（${slogans.length}）`}>
            <Space direction="vertical" style={{ width: '100%' }}>
              {slogans.map((s) => (
                <div key={s.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', border: '1px solid var(--color-border-2)', borderRadius: 8, padding: '10px 12px' }}>
                  <div style={{ display: 'flex', gap: 10, alignItems: 'center', flex: 1 }}>
                    <Tag color="arcoblue">{s.category}</Tag>
                    <span style={{ color: 'var(--geo-text)', flex: 1 }}>{s.text}</span>
                    <Tag color="gray" size="small">使用 {s.used_count || 0} 次</Tag>
                  </div>
                  <Space>
                    <Button size="mini" icon={<IconCopy />} onClick={() => useSloganCount(s).then(() => copySlogan(s.text))}>复制</Button>
                    <Button size="mini" icon={<IconEdit />} onClick={() => openEditSlogan(s)} />
                    <Button size="mini" status="danger" icon={<IconDelete />} onClick={() => removeSlogan(s)} />
                  </Space>
                </div>
              ))}
              {slogans.length === 0 && <Empty description="暂无话术，点击下方添加" />}
              <Space>
                <Button size="small" type="outline" icon={<IconPlus />} onClick={openAddSlogan}>新增话术</Button>
              </Space>
            </Space>
          </Tabs.TabPane>

          <Tabs.TabPane key="history" title="动作日志">
            <Timeline>
              {logs.map((lg) => (
                <Timeline.Item key={lg.id}>
                  <span style={{ color: '#86909C', fontSize: 12 }}>{fmtTime(lg.created_at)}</span>
                  &nbsp;账号「{lg.account_name}」{lg.action_type === 'greet' ? '向客户' : '执行'}「{lg.target}」{lg.result}
                </Timeline.Item>
              ))}
              {logs.length === 0 && <div style={{ color: '#86909C' }}>暂无动作记录</div>}
            </Timeline>
          </Tabs.TabPane>
        </Tabs>
      </Card>

      {/* 5. 频率与安全设置 */}
      <Card title={<Space><IconSafe />频率与安全设置</Space>}>
        <Row gutter={[24, 16]}>
          <Col span={12}>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>每账号每日打招呼上限</span>
              <Tag color="arcoblue">{settings.daily_limit} 次</Tag>
            </div>
            <Slider value={settings.daily_limit} min={5} max={100} step={5} onChange={(v) => setSettings({ ...settings, daily_limit: Number(v) })} />
            <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8 }}>
              <span>两次动作最小间隔</span>
              <Tag color="arcoblue">{settings.interval_min} 分钟</Tag>
            </div>
            <Slider value={settings.interval_min} min={1} max={60} onChange={(v) => setSettings({ ...settings, interval_min: Number(v) })} />
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 8 }}>
              <span>每天活跃时段</span>
              <Space>
                <Input size="mini" value={settings.active_start} maxLength={5} onChange={(v) => setSettings({ ...settings, active_start: v })} style={{ width: 76 }} />
                <span>—</span>
                <Input size="mini" value={settings.active_end} maxLength={5} onChange={(v) => setSettings({ ...settings, active_end: v })} style={{ width: 76 }} />
              </Space>
            </div>
            <div style={{ color: '#86909C', fontSize: 12, marginTop: 6 }}>格式 HH:MM，如 09:00；动作仅允许在活跃时段内执行。</div>
          </Col>
          <Col span={12}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 0' }}>
              <span><IconClockCircle /> 达到上限后自动进入冷却</span>
              <Switch checked={!!settings.cool_on} onChange={(v) => setSettings({ ...settings, cool_on: v })} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 0' }}>
              <span><IconRedo /> 同日陌生人重复忽略（已打招呼客户跳过）</span>
              <Switch checked={!!settings.repeat_on} onChange={(v) => setSettings({ ...settings, repeat_on: v })} />
            </div>
            <Divider style={{ margin: '10px 0' }} />
            <div style={{ color: '#86909C', fontSize: 12, lineHeight: 1.7 }}>
              <div>• 全程使用浏览器所在地区真实网络访问，不启用任何代理/加速器伪装。</div>
              <div>• 建议使用官方客户端完成发送动作，单账号保持日常自然节奏。</div>
              <div>• 打招呼内容建议真人化、个性化，避免广告话术与频率轰炸。</div>
            </div>
            <Button type="primary" style={{ marginTop: 12 }} onClick={saveSettings}>保存设置</Button>
          </Col>
        </Row>
      </Card>
      </Spin>

      {/* ---------------- 账号弹窗 ---------------- */}
      <Modal
        title={acctModal.editing ? '编辑账号' : '添加账号'}
        visible={acctModal.visible}
        onOk={submitAccount}
        onCancel={() => setAcctModal({ visible: false, editing: null })}
        simple
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="账号昵称（如：轻媒）" value={acctForm.nickname} onChange={(v) => setAcctForm({ ...acctForm, nickname: v })} />
          <Input placeholder="地区城市（如：苏州）" value={acctForm.region} onChange={(v) => setAcctForm({ ...acctForm, region: v })} />
          <Input placeholder="每日打招呼上限" type="number" value={String(acctForm.daily_limit)} onChange={(v) => setAcctForm({ ...acctForm, daily_limit: Number(v || 30) })} />
          <div>
            状态：
            <Radio.Group value={acctForm.status} onChange={(v) => setAcctForm({ ...acctForm, status: String(v) })}>
              <Radio value="在线">在线</Radio>
              <Radio value="离线">离线</Radio>
            </Radio.Group>
          </div>
        </Space>
      </Modal>

      {/* ---------------- 话术弹窗 ---------------- */}
      <Modal
        title={slgModal.editing ? '编辑话术' : '新增话术'}
        visible={slgModal.visible}
        onOk={submitSlogan}
        onCancel={() => setSlgModal({ visible: false, editing: null })}
        simple
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Select
            placeholder="分类" value={slgForm.category}
            onChange={(v) => setSlgForm({ ...slgForm, category: String(v) })}
            style={{ width: '100%' }}
          >
            <Select.Option value="开场白">开场白</Select.Option>
            <Select.Option value="进阶">进阶</Select.Option>
            <Select.Option value="邀约">邀约</Select.Option>
          </Select>
          <TextArea placeholder="话术文案（真人化、个性化，避免广告化与频率轰炸）" value={slgForm.text} onChange={(v) => setSlgForm({ ...slgForm, text: v })} autoSize={{ minRows: 3, maxRows: 6 }} />
        </Space>
      </Modal>

      {/* ---------------- 手动添加线索 ---------------- */}
      <Modal title="手动添加客户线索" visible={leadModal} onOk={submitLead} onCancel={() => setLeadModal(false)} simple>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="客户昵称" value={leadForm.nickname} onChange={(v) => setLeadForm({ ...leadForm, nickname: v })} />
          <Select
            placeholder="来源同行（可选）" style={{ width: '100%' }} allowClear
            value={leadForm.peer_id || undefined}
            onChange={(v) => setLeadForm({ ...leadForm, peer_id: Number(v || 0) })}
          >
            {peers.map((p) => <Select.Option key={p.id} value={p.id}>{p.nickname}</Select.Option>)}
          </Select>
          <Select placeholder="标签" value={leadForm.tag} onChange={(v) => setLeadForm({ ...leadForm, tag: String(v) })} style={{ width: '100%' }}>
            <Select.Option value="男·单身">男·单身</Select.Option>
            <Select.Option value="女·单身">女·单身</Select.Option>
            <Select.Option value="家长">家长</Select.Option>
            <Select.Option value="其他">其他</Select.Option>
          </Select>
          <Input placeholder="评论/备注（选填）" value={leadForm.comment} onChange={(v) => setLeadForm({ ...leadForm, comment: v })} />
        </Space>
      </Modal>

      {/* ---------------- 解析视频评论 ---------------- */}
      <Modal
        title="从视频评论解析线索"
        visible={parseModal}
        onOk={submitParse}
        onCancel={() => setParseModal(false)}
        confirmLoading={parsing}
        simple
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Select
            placeholder="选择来源同行" style={{ width: '100%' }}
            value={parseForm.peer_id || undefined}
            onChange={(v) => setParseForm({ ...parseForm, peer_id: Number(v || 0) })}
          >
            {peers.map((p) => <Select.Option key={p.id} value={p.id}>{p.nickname}</Select.Option>)}
          </Select>
          <Input placeholder="解析条数（1-20，默认 8）" type="number" value={String(parseForm.count)}
            onChange={(v) => setParseForm({ ...parseForm, count: Number(v || 8) })} />
          <div style={{ color: '#F77234', fontSize: 12 }}>
            说明：评论数据需登录态，公开页无法可靠抓取，系统将生成结构化估算线索并标记«估算»，仅作获客参考，请以官方客户端实际留言为准。
          </div>
        </Space>
      </Modal>

      {/* ---------------- 视频数据详情 ---------------- */}
      <Modal
        title={`「${videoPeer?.nickname || ''}」视频数据${videoPeer?.sourced_from === 'real' ? '（真实）' : '（估算）'}`}
        visible={!!videoPeer}
        footer={null}
        onCancel={() => setVideoPeer(null)}
        style={{ width: 720 }}
      >
        <Spin loading={videoLoading} style={{ display: 'block' }}>
          <Table
            rowKey="id"
            data={videos}
            pagination={false}
            noDataElement={<Empty description="暂无视频数据" />}
            columns={[
              { title: '标题', dataIndex: 'title', ellipsis: true },
              { title: '播放', dataIndex: 'play_count', width: 80, render: (v) => fmtPlay(v) },
              { title: '点赞', dataIndex: 'like_count', width: 70, render: (v) => fmtN(v) },
              { title: '评论', dataIndex: 'comment_count', width: 70, render: (v) => fmtN(v) },
              { title: '赞评比', dataIndex: 'interaction_rate', width: 80, render: (v) => `${fmtRate(v)}%` },
              { title: '发布时间', dataIndex: 'publish_time', width: 110, render: (v) => fmtTime(v) },
            ]}
          />
        </Spin>
      </Modal>

      {/* ---------------- 打招呼半自动流程 ---------------- */}
      <Modal
        title="去打招呼（合规半自动）"
        visible={!!greetLead}
        footer={null}
        onCancel={() => setGreetLead(null)}
      >
        {greetLead && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <div style={{ color: '#86909C', fontSize: 13 }}>
              客户：<b>{greetLead.nickname}</b>（来源：{greetLead.peer_name || '--'}）
            </div>
            <div>
              执行账号：
              <Select
                style={{ width: '100%' }}
                value={greetAccountId || undefined}
                onChange={changeGreetAccount}
                loading={prechecking}
              >
                {accounts.map((a) => (
                  <Select.Option key={a.id} value={a.id}>
                    {a.nickname}（{a.status} · {a.today_used}/{a.daily_limit}）
                  </Select.Option>
                ))}
              </Select>
            </div>
            <Card size="small" bodyStyle={{ background: 'rgba(0,0,0,.02)', fontSize: 12 }}>
              {precheck?.can ? (
                <div style={{ color: '#00B42A' }}>✓ 可以执行：今日已用 {precheck.today_used}/{precheck.daily_limit}，非冷却中，间隔满足。</div>
              ) : (
                <div style={{ color: precheck?.not_cooling === false ? '#F77234' : '#F53F3F' }}>
                  {precheck ? precheck.reason : '校验中...'}
                </div>
              )}
            </Card>
            <Divider style={{ margin: '4px 0' }} />
            <Space direction="vertical" style={{ width: '100%' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <span style={{ fontSize: 13, fontWeight: 600 }}>AI 自动生成话术</span>
                <Button size="mini" type="primary" icon={<IconThunderbolt />} loading={aiGen.loading} onClick={runAiGenerate}>
                  生成 3 条
                </Button>
                {aiGen.platform && <Tag size="small" color="arcoblue">{aiGen.platform}/{aiGen.model}</Tag>}
              </div>
              {aiGen.error && <div style={{ color: '#F53F3F', fontSize: 12 }}>AI 生成失败：{aiGen.error}（请先在「AI 平台」中配置启用的平台）</div>}
              {(aiGen.slogans || []).map((s: any, idx: number) => (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, border: '1px solid var(--color-border-2)', borderRadius: 8, padding: '8px 10px', background: 'rgba(22,93,255,.03)' }}>
                  <span style={{ flex: 1, fontSize: 13, color: 'var(--geo-text)' }}>{typeof s === 'string' ? s : s.text}</span>
                  <Button size="mini" icon={<IconCopy />} onClick={() => copySlogan(typeof s === 'string' ? s : s.text)}>复制</Button>
                </div>
              ))}
              <div style={{ color: '#C9CDD4', fontSize: 11 }}>
                AI 基于客户昵称与评论内容生成真人化、个性化开场话术（无广告语），仅作建议；发送仍由人工完成。
              </div>
            </Space>

            <div>
              从话术库选择（可选，不选则仅复制 AI 生成内容）：
              <Select style={{ width: '100%' }} value={greetSloganId || undefined} allowClear
                onChange={(v) => setGreetSloganId(Number(v || 0))}
                placeholder="可选"
              >
                {slogans.map((s) => <Select.Option key={s.id} value={s.id}>[{s.category}] {s.text}</Select.Option>)}
              </Select>
            </div>
            <div style={{ color: '#F77234', fontSize: 12, lineHeight: 1.7 }}>
              合规提醒：本系统不代发消息。请先在官方抖音客户端打开该用户私信，复制话术完成发送，再在下方确认标记为「已打招呼」。
            </div>
            <Space>
              <Button
                icon={<IconCopy />}
                onClick={() => {
                  const s = slogans.find((x) => x.id === greetSloganId);
                  if (s) copySlogan(s.text);
                  else Message.warning('请选择一条话术（或使用上方 AI 生成）');
                }}
              >
                复制话术
              </Button>
              <Button type="primary" loading={greeting} disabled={!precheck?.can} onClick={doGreet}>确认已打招呼</Button>
            </Space>
          </Space>
        )}
      </Modal>
    </div>
  );
}
