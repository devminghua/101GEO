import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Grid, Button, Message, Tag, Space, Typography, Empty, Skeleton, Progress, Spin } from '@arco-design/web-react';
import { IconLaunch, IconRefresh, IconSync } from '@arco-design/web-react/icon';
import { api } from '../api';
import EChart from '../components/EChart';
import { useTranslation } from 'react-i18next';
import i18n from '../i18n';

const { Row: GridRow, Col: GridCol } = Grid;
const { Title, Text } = Typography;

// 品牌调性：清爽淡蓝白底 + 高可读大数字
const PLAT_COLORS = ['#165DFF', '#722ED1', '#00B42A', '#F53F3F', '#FF7D00', '#14C9C9', '#F7BA1E', '#F5319D', '#3491FA', '#9FDB1D', '#0FC6C2', '#D91AD9'];
const SCENE_COLORS = ['#165DFF', '#14C9C9', '#9FDB1D', '#FF7D00', '#F5319D', '#C9CDD4', '#722ED1', '#3491FA', '#F7BA1E', '#0FC6C2', '#D91AD9', '#00B42A'];
const DIST_COLORS = { top1: '#00B42A', top23: '#165DFF', top410: '#FF7D00', miss: '#e5e6eb' };
const RANK_COLORS = ['#165DFF', '#14C9C9', '#FF7D00', '#722ED1', '#F53F3F', '#C9CDD4'];
const colorOf = (i: number) => PLAT_COLORS[i % PLAT_COLORS.length];

// ============ 平台彩色 Logo（全部本地存储：frontend/public/assets/platform-icons/） ============
const LOGO = '/assets/platform-icons/';
// 顺序敏感：先精确、后宽泛；name 转小写后做子串匹配
const LOGO_RULES: [string, string][] = [
  ['deepseek', LOGO + 'deepseek.png'],
  ['openai', LOGO + 'openai.png'],
  ['gpt', LOGO + 'openai.png'],
  ['ollama', LOGO + 'ollama.png'],
  ['qwen', LOGO + 'qwen.png'],
  ['通义千问', LOGO + 'qwen.png'],
  ['通义', LOGO + 'qwen.png'],
  ['千问', LOGO + 'qwen.png'],
  ['kimi', LOGO + 'kimi-ai.png'],
  ['moonshot', LOGO + 'kimi-ai.png'],
  ['月之暗面', LOGO + 'kimi-ai.png'],
  ['claude', LOGO + 'claude-ai.png'],
  ['anthropic', LOGO + 'claude-ai.png'],
  ['gemini', LOGO + 'google.png'],
  ['google', LOGO + 'google.png'],
  ['grok', LOGO + 'grok.png'],
  ['x-ai', LOGO + 'grok.png'],
  ['xai', LOGO + 'grok.png'],
  ['讯飞星火', LOGO + 'spark.png'],
  ['讯飞', LOGO + 'spark.png'],
  ['星火', LOGO + 'spark.png'],
  ['spark', LOGO + 'spark.png'],
  ['minimax', LOGO + 'minimax.png'],
  ['豆包', LOGO + 'volcengine.png'],
  ['volcengine', LOGO + 'volcengine.png'],
  ['字节', LOGO + 'volcengine.png'],
  ['文心一言', LOGO + 'baidu.png'],
  ['文心', LOGO + 'baidu.png'],
  ['百度', LOGO + 'baidu.png'],
  ['baidu', LOGO + 'baidu.png'],
  ['chatglm', LOGO + 'zhipu.png'],
  ['智谱', LOGO + 'zhipu.png'],
  ['zhipu', LOGO + 'zhipu.png'],
  ['bigmodel', LOGO + 'zhipu.png'],
  ['混元', LOGO + 'hunyuan.png'],
  ['hunyuan', LOGO + 'hunyuan.png'],
  ['腾讯', LOGO + 'hunyuan.png'],
  ['tencent', LOGO + 'hunyuan.png'],
  ['mistral', LOGO + 'mistral-ai.png'],
  ['perplexity', LOGO + 'perplexity.png'],
  ['groq', LOGO + 'groq.png'],
  ['openrouter', LOGO + 'openrouter.png'],
  ['llama', LOGO + 'meta.png'],
  ['meta', LOGO + 'meta.png'],
  ['other', LOGO + 'google.png'],
];
export function platformLogoUrl(name: string): string | undefined {
  const n = (name || '').toLowerCase();
  for (const [key, url] of LOGO_RULES) {
    if (n.includes(key)) return url;
  }
  return undefined;
}

// 平台真实图片徽标（失败回退为字母圆标）
function PlatformLogo({ name, index, size = 34, rounded = 10 }: { name: string; index: number; size?: number; rounded?: number }) {
  const [failed, setFailed] = useState(false);
  const url = platformLogoUrl(name);
  if (!url || failed) {
    const c = colorOf(index);
    return (
      <span
        style={{
          width: size, height: size, borderRadius: rounded, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
          fontWeight: 700, fontSize: Math.max(12, size * 0.4), color: '#fff', flexShrink: 0,
          background: `linear-gradient(135deg, ${c}, ${c}99)`, boxShadow: `0 4px 10px ${c}33`,
        }}
      >
        {name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return (
    <img
      src={url}
      alt={name}
      onError={() => setFailed(true)}
      style={{
        width: size, height: size, borderRadius: rounded, objectFit: 'contain', flexShrink: 0, display: 'inline-block',
        background: 'var(--geo-surface)', padding: 3, border: '1px solid var(--color-border-2)', boxShadow: '0 2px 6px rgba(0,0,0,0.06)',
      }}
    />
  );
}

// ============ 数字滚动动画 ============
function CountUpNumber({ value, duration = 750 }: { value: number; duration?: number }) {
  const [display, setDisplay] = useState(0);
  const prevRef = useRef<number>(NaN);
  useEffect(() => {
    const from = Number.isNaN(prevRef.current) ? 0 : prevRef.current;
    prevRef.current = value;
    if (from === value) { setDisplay(value); return; }
    const start = performance.now();
    let raf = 0;
    const tick = (t: number) => {
      const p = Math.min((t - start) / duration, 1);
      const eased = 1 - Math.pow(1 - p, 3); // easeOutCubic
      setDisplay(Math.round(from + (value - from) * eased));
      if (p < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [value, duration]);
  return <>{display.toLocaleString()}</>;
}

// 排名徽章（TOP1 / TOP2-3 / TOP4-10 / 未进入）
function RankBadge({ p }: { p: number }) {
  if (p >= 99) {
    return <span style={{ display: 'inline-block', padding: '2px 10px', borderRadius: 12, fontSize: 12, fontWeight: 600, color: '#86909C', background: 'var(--color-fill-2)' }}>{i18n.t('dashboard.notInTop10')}</span>;
  }
  const color = p === 1 ? '#00B42A' : p <= 3 ? '#165DFF' : p <= 10 ? '#FF7D00' : '#86909C';
  const bg = p === 1 ? '#00B42A14' : p <= 3 ? '#165DFF14' : p <= 10 ? '#FF7D0014' : '#f2f3f5';
  return (
    <span style={{ display: 'inline-block', padding: '2px 10px', borderRadius: 12, fontSize: 12, fontWeight: 600, color, background: bg }}>
      TOP{p}
    </span>
  );
}

// 较上次变化
function DeltaTag({ d }: { d: number }) {
  if (d > 0) return <span style={{ color: '#00B42A', fontWeight: 600, fontSize: 13 }}>↑ {d.toFixed(1)}%</span>;
  if (d < 0) return <span style={{ color: '#F53F3F', fontWeight: 600, fontSize: 13 }}>↓ {Math.abs(d).toFixed(1)}%</span>;
  return <span style={{ color: '#86909C', fontSize: 13 }}>— {i18n.t('dashboard.flat')}</span>;
}

// 平台对接健康度 → 展示映射
const HEALTH_META: Record<string, { color: string; text: string }> = {
  healthy: { color: '#00B42A', text: i18n.t('dashboard.health_healthy') },
  bad_key: { color: '#F53F3F', text: i18n.t('dashboard.health_bad_key') },
  no_key: { color: '#FF7D00', text: i18n.t('dashboard.health_no_key') },
  timeout: { color: '#F53F3F', text: i18n.t('dashboard.health_timeout') },
  empty: { color: '#FF7D00', text: i18n.t('dashboard.health_empty') },
  error: { color: '#F53F3F', text: i18n.t('dashboard.health_error') },
  disabled: { color: '#C9CDD4', text: i18n.t('dashboard.health_disabled') },
};

// 迷你趋势线（动态纵轴：让波形撑满卡片）
function Spark({ data, color }: { data: number[]; color: string }) {
  const peak = Math.max(...data, 0) || 1;
  const yMax = Math.ceil(peak * 1.2 * 100) / 100;
  const option = {
    grid: { left: 2, right: 2, top: 6, bottom: 2 },
    xAxis: { show: false, type: 'category' as const },
    yAxis: { show: false, min: 0, max: yMax },
    tooltip: { show: false },
    series: [{
      type: 'line' as const, smooth: true, symbol: 'none', data,
      lineStyle: { color, width: 2 },
      areaStyle: { color, opacity: 0.10 },
    }],
  };
  return <EChart option={option} height={42} />;
}

export default function Dashboard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [running, setRunning] = useState(false);
  const [health, setHealth] = useState<any[]>([]);
  const [setup, setSetup] = useState<any>(null);
  const dataRef = useRef<any>(null);

  // 登录后自检各 AI 平台对接健康度
  const loadHealth = async () => {
    try {
      const h: any = await api.platformHealth();
      setHealth(h || []);
    } catch (e: any) {
      // 自检失败不打扰用户
    }
  };
  useEffect(() => { loadHealth(); /* eslint-disable-next-line */ }, []);

  // 登录后自检 GEO 配置完整性（品牌词/关键词/平台/Key）
  const loadSetup = async () => {
    try {
      const s: any = await api.geoSetupCheck();
      setSetup(s || null);
    } catch (e: any) {
      // 自检失败不打扰用户
    }
  };
  useEffect(() => { loadSetup(); /* eslint-disable-next-line */ }, []);

  const load = async () => {
    const isFirst = !dataRef.current;
    if (isFirst) setLoading(true); else setRefreshing(true);
    try {
      const d = await api.dashboardOverview();
      dataRef.current = d;
      setData(d);
    } catch (e: any) {
      Message.error('加载失败: ' + e.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };
  useEffect(() => { load(); }, []);

  // 首页数据自动刷新：每 60 秒拉取最新数据
  useEffect(() => {
    const timer = setInterval(() => { load(); }, 60000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const runCheck = async () => {
    setRunning(true);
    try {
      await api.runTask();
      Message.success('巡检任务已启动，完成后自动刷新');
      setTimeout(load, 8000);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setRunning(false);
    }
  };

  if (loading) return <Skeleton text={{ rows: 8 }} animation />;

  const kpi = data?.kpi || {};
  const coverage = data?.coverage || [];
  const platforms = data?.platform_table || [];
  const scenes = data?.scene_dist || [];
  const ranks = data?.kw_rank_top || [];
  const rankDist = data?.rank_dist || {};
  const hasData = !!kpi.has_data;

  // 较上次检测变化（百分比）：取趋势序列最后两个非零点
  const pctDelta = (seq: number[]) => {
    const vals = (seq || []).filter((v) => v > 0);
    if (vals.length < 2) return 0;
    const prev = vals[vals.length - 2];
    const last = vals[vals.length - 1];
    if (prev <= 0) return 0;
    return Math.round(((last - prev) / prev) * 100);
  };
  const coveredCount = coverage.filter((p: any) => p.covered).length;

  const kpiCards = [
    { label: t('dashboard.kpi_visibility'), num: kpi.avg_visibility ?? 0, tail: '%', color: '#165DFF', delta: pctDelta(kpi.visibility_trend || kpi.trend || []), trend: (kpi.visibility_trend || kpi.trend || []) as number[] },
    { label: t('dashboard.kpi_top3'), num: kpi.top3_rate ?? 0, tail: '%', color: '#00B42A', delta: pctDelta(kpi.top3_trend || []), trend: (kpi.top3_trend || []) as number[] },
    { label: t('dashboard.kpi_exposure'), num: kpi.exposure ?? 0, tail: '', color: '#722ED1', delta: pctDelta(kpi.exposure_trend || []), trend: (kpi.exposure_trend || []) as number[] },
    { label: t('dashboard.kpi_covered'), num: kpi.covered ?? 0, tail: `/${kpi.total_platform ?? 0}`, color: '#722ED1', delta: pctDelta(kpi.covered_trend || []), trend: (kpi.covered_trend || []) as number[] },
    { label: t('dashboard.kpi_keywords'), num: kpi.keyword_count ?? 0, tail: t('dashboard.keywordsUnit'), color: '#F7BA1E', delta: pctDelta(kpi.keyword_trend || []), trend: (kpi.keyword_trend || []) as number[] },
  ];

  // 场景覆盖分布（环形，中间总场景数；label 靠右显示百分比）
  const sceneTotal = scenes.reduce((s: number, x: any) => s + (x.value || 0), 0);
  const sceneOption = {
    tooltip: { trigger: 'item' as const, formatter: '{b}: {c} 个关键词 ({d}%)' },
    legend: { bottom: 0, type: 'scroll' as const, icon: 'circle', textStyle: { fontSize: 12 } },
    // 动画时长放顶层才生效（series 级会被默认值覆盖）
    animation: true,
    animationDuration: 2600,
    animationDurationUpdate: 800,
    animationEasing: 'cubicInOut',
    title: {
      text: String(sceneTotal), subtext: '总场景数',
      left: '38%', top: '35%', textAlign: 'center',
      textStyle: { fontSize: 22, fontWeight: 700, color: 'var(--geo-text)' },
      subtextStyle: { fontSize: 12, color: '#86909C' },
    },
    series: [{
      type: 'pie' as const, radius: ['52%', '70%'], center: ['38%', '43%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: 'var(--geo-surface)', borderWidth: 2 },
      label: { show: true, position: 'right', formatter: '{d}%', fontSize: 12, color: 'var(--color-text-2)' },
      labelLine: { length: 10, length2: 8 },
      // 顺时针慢加载动画：扇区逐个顺时针展开
      animationType: 'expansion' as const,
      animationDelay: (idx: number) => idx * 320,
      data: scenes.map((s: any, i: number) => ({ name: s.name, value: s.value, itemStyle: { color: SCENE_COLORS[i % SCENE_COLORS.length] } })),
    }],
  };

  // 各平台出现率 TOP8（横向条形图，参考 Results.tsx platformBarOption，数据用 platform_table.visibility）
  const platformTop8 = (platforms || []).slice(0, 8).reverse();
  const platformBarOption = {
    tooltip: { trigger: 'axis' as const, axisPointer: { type: 'shadow' } },
    grid: { left: 8, right: 36, top: 12, bottom: 4, containLabel: true },
    xAxis: { type: 'value' as const, max: 100, axisLabel: { formatter: '{value}%', fontSize: 10 } },
    yAxis: { type: 'category' as const, data: platformTop8.map((p: any) => p.platform), axisLabel: { fontSize: 11 } },
    series: [{
      type: 'bar' as const, barWidth: 12,
      data: platformTop8.map((p: any) => p.visibility),
      itemStyle: { borderRadius: [0, 6, 6, 0], color: '#14C9C9' },
      label: { show: true, position: 'right', formatter: '{c}%', fontSize: 10 },
      // 滚动到视口后条形从左往右生长
      animationDuration: 1800,
      animationDurationUpdate: 700,
      animationEasing: 'cubicOut',
    }],
  };

  // {t('dashboard.rankDist')}（环形，中间总数）
  const distData = [
    { name: 'TOP1', value: rankDist.top1 || 0, color: '#165DFF' },
    { name: 'TOP2-3', value: rankDist.top23 || 0, color: '#FF7D00' },
    { name: 'TOP4-10', value: rankDist.top410 || 0, color: '#B37FEB' },
    { name: '11+', value: rankDist.miss || 0, color: '#86909C' },
  ];
  const distTotal = distData.reduce((s, d) => s + d.value, 0);
  const distOption = {
    tooltip: { trigger: 'item' as const, formatter: '{b}: {c} 次 ({d}%)' },
    legend: { bottom: 0, type: 'scroll' as const, icon: 'circle', textStyle: { fontSize: 12 } },
    animation: true,
    animationDuration: 2400,
    animationEasing: 'cubicInOut',
    title: {
      text: String(distTotal), subtext: '关键词总数',
      left: 'center', top: '35%',
      textStyle: { fontSize: 22, fontWeight: 700, color: 'var(--geo-text)' },
      subtextStyle: { fontSize: 12, color: '#86909C' },
    },
    series: [{
      type: 'pie' as const, radius: ['58%', '76%'], center: ['50%', '43%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: 'var(--geo-surface)', borderWidth: 2 },
      label: { show: false },
      animationType: 'expansion' as const,
      animationDelay: (idx: number) => idx * 300,
      data: distData.map((d) => ({ name: d.name, value: d.value, itemStyle: { color: d.color } })),
    }],
  };

  // {t('dashboard.rankPieTitle')}（环形，参考 Results.tsx rankPieOption，数据用 rank_dist 构造 label/value）
  const rankPieData = [
    { label: '第 1 位', value: rankDist.top1 || 0 },
    { label: '第 2-3 位', value: rankDist.top23 || 0 },
    { label: '第 4-10 位', value: rankDist.top410 || 0 },
    { label: '未命中', value: rankDist.miss || 0 },
  ].filter((d) => (d.value || 0) > 0);
  const rankPieOption = {
    tooltip: { trigger: 'item' as const, formatter: '{b}：{c} 条（{d}%）' },
    legend: { bottom: 0, type: 'scroll' as const, textStyle: { fontSize: 11 } },
    animation: true,
    animationDuration: 2400,
    animationEasing: 'cubicInOut',
    series: [{
      type: 'pie' as const, radius: ['42%', '68%'], center: ['50%', '44%'],
      itemStyle: { borderRadius: 4, borderColor: 'var(--geo-surface)', borderWidth: 2 },
      data: rankPieData.map((d, i) => ({ name: d.label, value: d.value, itemStyle: { color: RANK_COLORS[i % RANK_COLORS.length] } })),
      label: { show: false },
      animationType: 'expansion' as const,
      animationDelay: (idx: number) => idx * 300,
    }],
  };

  const trendColor = (rate: number) => {
    if (rate >= 80) return '#00B42A';
    if (rate >= 40) return '#FF7D00';
    return '#F53F3F';
  };

  return (
    <div>
      {/* 刷新过渡动画：顶部细进度条 */}
      {refreshing && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, height: 3, zIndex: 1000, background: 'transparent', overflow: 'hidden' }}>
          <div style={{
            height: '100%', width: '40%',
            background: 'linear-gradient(90deg,#165DFF,#14C9C9)',
            animation: 'geoRefreshingBar 1.1s linear infinite',
          }} />
          <style>{`@keyframes geoRefreshingBar { 0%{ margin-left:-40%; } 100%{ margin-left:100%; } }`}</style>
        </div>
      )}

      {/* GEO 配置自检提示：未就绪时醒目标出缺失项，引导去配置 */}
      {setup && !setup.ready && (
        <div style={{
          background: '#FFF7E6', border: '1px solid #FFD591', borderRadius: 16,
          padding: '16px 20px', marginBottom: 16,
        }}>
          <div style={{ fontWeight: 700, color: '#D46B08', marginBottom: 8, fontSize: 14 }}>
            {t('dashboard.setupTitle')}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {(setup.checks || []).filter((c: any) => !c.ok).map((c: any) => (
              <div key={c.key} style={{ display: 'flex', alignItems: 'flex-start', gap: 8, fontSize: 13, color: '#D46B08', lineHeight: 1.6 }}>
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#FAAD14', flexShrink: 0, marginTop: 7 }} />
                <span><b>{c.label}</b>：{c.hint}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 首次巡检引导：配置就绪但尚未巡检时，醒目引导一键开始 */}
      {setup && setup.ready && !hasData && !running && (
        <div
          onClick={runCheck}
          style={{
            background: 'linear-gradient(135deg,#4F46E5,#7B61FF)', color: '#fff', borderRadius: 16,
            padding: '20px 24px', marginBottom: 16, cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16,
            boxShadow: '0 6px 20px rgba(79,70,229,0.25)',
          }}
        >
          <div>
            <div style={{ fontWeight: 700, fontSize: 15 }}>{t('dashboard.startTitle')}</div>
            <div style={{ opacity: 0.88, fontSize: 13, marginTop: 4 }}>
              {t('dashboard.startSub')}
            </div>
          </div>
          <div style={{ background: 'rgba(255,255,255,0.22)', borderRadius: 20, padding: '8px 20px', fontWeight: 600, whiteSpace: 'nowrap' }}>
            立即巡检 →
          </div>
        </div>
      )}

      {/* {t('dashboard.platformCoverage')}横幅：横向滚动平台覆盖状态 */}
      <div style={{
        background: 'var(--geo-surface)',
        borderRadius: 16,
        padding: '18px 20px',
        marginBottom: 16,
        border: '1px solid var(--color-border-2)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', marginBottom: 14 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 17, fontWeight: 700, color: 'var(--geo-text)' }}>{t('dashboard.platformCoverage')}</span>
            <span style={{
              background: hasData ? '#165DFF' : '#FF7D00', color: '#fff', fontSize: 12, fontWeight: 600,
              borderRadius: 12, padding: '2px 12px', whiteSpace: 'nowrap',
            }}>{hasData ? t('dashboard.coveredCount', { a: coveredCount, b: coverage.length || 0 }) : t('dashboard.notChecked')}</span>
            {hasData && data?.generated_at && (
              <span style={{ color: '#86909C', fontSize: 12 }}>{t('dashboard.updatedAt', { t: data.generated_at })}</span>
            )}
            {!hasData && <span style={{ color: '#FF7D00', fontSize: 12 }}>{t('dashboard.hintNotChecked')}</span>}
          </div>
          <Space>
            <Button icon={<IconSync />} loading={refreshing} onClick={load}>{t('dashboard.refreshData')}</Button>
            <Button type="primary" icon={<IconLaunch />} loading={running} onClick={runCheck}>
              {running ? t('dashboard.checking') : t('dashboard.startCheck')}
            </Button>
          </Space>
        </div>
        <div style={{ display: 'flex', gap: 12, overflowX: 'auto', paddingBottom: 4 }}>
          {(coverage.length ? coverage : platforms).map((p: any, i: number) => {
            const h = health.find((x: any) => x.name === (p.name || p.platform));
            const hm = h ? HEALTH_META[h.status] : null;
            const dotColor = hm ? hm.color : (p.enabled === false ? '#C9CDD4' : p.covered ? '#00B42A' : (hasData ? '#F53F3F' : '#FF7D00'));
            const statusText = hm ? hm.text : (p.enabled === false ? t('dashboard.disabled') : p.covered ? t('dashboard.covered') : (hasData ? t('dashboard.notCovered') : t('dashboard.pending')));
            return (
              <div key={(p.name || p.platform) + i} style={{
                flex: '0 0 auto', minWidth: 148, padding: '12px 14px',
                borderRadius: 12, background: 'var(--color-fill-2)',
                display: 'flex', alignItems: 'center', gap: 10,
              }}>
                <PlatformLogo name={p.name || p.platform} index={i} size={34} rounded={9} />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 92 }}>{p.name || p.platform}</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 5, marginTop: 3, fontSize: 12 }}>
                    <span style={{
                      width: 7, height: 7, borderRadius: '50%', display: 'inline-block',
                      background: dotColor,
                      boxShadow: hm && h.status === 'healthy' ? '0 0 0 2px rgba(0,180,42,.15)' : 'none',
                    }} />
                    <span style={{ color: dotColor, fontWeight: 500, whiteSpace: 'nowrap' }}>
                      {statusText}{h && h.latency_ms > 0 && hm ? ` ${h.latency_ms}ms` : ''}
                    </span>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div style={{ opacity: refreshing ? 0.55 : 1, transition: 'opacity .35s ease', pointerEvents: refreshing ? 'none' : 'auto' }}>
      {!hasData ? (
        <Card style={{ borderRadius: 16, padding: '60px 0' }}>
          <Empty description={t('dashboard.emptyMain')} />
        </Card>
      ) : (
        <>
          {/* 第一排：五大 KPI 统计卡（自适应网格：窄屏自动换行） */}
          <div style={{ display: 'grid', gap: 16, gridTemplateColumns: 'repeat(auto-fit, minmax(168px, 1fr))', marginBottom: 16 }}>
            {kpiCards.map((k) => (
              <Card key={k.label} style={{ borderRadius: 16 }} bodyStyle={{ padding: 16 }}>
                <div style={{ color: '#86909C', fontSize: 13 }}>{k.label}</div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginTop: 6 }}>
                  <span style={{ fontSize: 30, fontWeight: 700, color: 'var(--geo-text)', lineHeight: 1 }}>
                    <CountUpNumber value={k.num} />
                  </span>
                  {k.tail !== undefined && k.tail !== '' && (
                    <span style={{ fontSize: 14, color: 'var(--geo-text)', fontWeight: 600, marginBottom: 3 }}>{k.tail}</span>
                  )}
                </div>
                {k.delta !== undefined && k.delta !== 0 ? (
                  <div style={{ fontSize: 12, marginTop: 6, fontWeight: 500, color: k.delta > 0 ? '#00B42A' : '#F53F3F' }}>
                    {t('dashboard.vsLast')} {k.delta > 0 ? `↑ ${k.delta}%` : `↓ ${Math.abs(k.delta)}%`}
                  </div>
                ) : (
                  <div style={{ fontSize: 12, marginTop: 6, color: '#86909C' }}>{t('dashboard.vsLast')} {t('dashboard.flat')}</div>
                )}
                <div style={{ marginTop: 10 }}>
                  <Spark data={(k.trend || []) as number[]} color={k.color} />
                </div>
              </Card>
            ))}
          </div>

          {/* 第二排：{t('dashboard.platformPerf')} + 场景覆盖分布 */}
          <GridRow gutter={[16, 16]} style={{ marginBottom: 0 }}>
            <GridCol xs={24} xl={15}>
              <Card style={{ borderRadius: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.platformPerf')}</span>}>
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 620 }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--color-border-2)' }}>
                        {[t('dashboard.col_platform'), t('dashboard.col_visibility'), t('dashboard.col_top3'), t('dashboard.col_exposure'), t('dashboard.col_delta'), t('dashboard.col_trend')].map((h, i) => (
                          <th key={h} style={{ textAlign: i === 0 ? 'left' : 'center', padding: '10px 8px', color: '#86909C', fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap' }}>
                            {h}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {platforms.map((p: any, i: number) => (
                        <tr key={p.platform} style={{ borderBottom: '2px solid var(--color-fill-2)' }}>
                          <td style={{ padding: '12px 8px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                              <PlatformLogo name={p.platform} index={i} size={32} />
                              <div>
                                <div style={{ fontWeight: 600 }}>{p.platform}</div>
                                <div style={{ fontSize: 11, color: p.enabled ? '#00B42A' : '#C9CDD4' }}>{p.enabled ? t('dashboard.enabled') : t('dashboard.disabled')}</div>
                              </div>
                            </div>
                          </td>
                          <td style={{ padding: '12px 8px', textAlign: 'center', whiteSpace: 'nowrap' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                              <div style={{ flex: 1, minWidth: 70 }}>
                                <Progress percent={Math.min(100, p.visibility)} size="small" color={trendColor(p.visibility)} />
                              </div>
                              <span style={{ fontWeight: 700, width: 42, textAlign: 'right' }}>{p.visibility}%</span>
                            </div>
                          </td>
                          <td style={{ padding: '12px 8px', textAlign: 'center', fontWeight: 600 }}>{p.top3_rate}%</td>
                          <td style={{ padding: '12px 8px', textAlign: 'center' }}>{p.exposure?.toLocaleString?.() ?? p.exposure}</td>
                          <td style={{ padding: '12px 8px', textAlign: 'center' }}><DeltaTag d={p.delta} /></td>
                          <td style={{ padding: '12px 8px', minWidth: 120 }}>
                            <Spark data={p.trend || []} color={trendColor(p.visibility)} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div style={{ textAlign: 'right', marginTop: 10 }}>
                  <span onClick={() => navigate('/platforms')} style={{ color: '#165DFF', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>
                    {t('dashboard.viewAllPlatforms', { n: kpi.total_platform ?? 0 })}
                  </span>
                </div>
              </Card>
            </GridCol>
            <GridCol xs={24} xl={9}>
              <Card style={{ borderRadius: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.sceneDist')}</span>}>
                {scenes.length > 0
                  ? <EChart option={sceneOption} height={240} />
                  : <Empty description={t('dashboard.sceneEmpty')} style={{ padding: '60px 0' }} />}
              </Card>
              <Card style={{ borderRadius: 16, marginTop: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.occurrenceTop')}</span>}>
                {platformTop8.length > 0
                  ? <EChart option={platformBarOption} height={220} />
                  : <Empty description={t('dashboard.occurrenceEmpty')} style={{ padding: '24px 0' }} />}
              </Card>
            </GridCol>
          </GridRow>

          {/* 第三排：关键词排名监测 TOP5 + {t('dashboard.rankDist')} */}
          <GridRow gutter={[16, 0]}>
            <GridCol xs={24} xl={15}>
              <Card style={{ borderRadius: 16, marginTop: 8 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.rankMonitorTop')}</span>}>
                {(ranks || []).slice(0, 8).length > 0 ? (
                  <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 620 }}>
                      <thead>
                        <tr style={{ borderBottom: '1px solid var(--color-border-2)' }}>
                          {[t('dashboard.col_keyword'), t('dashboard.col_belong'), t('dashboard.col_rank'), t('dashboard.col_inTop3'), t('dashboard.col_delta'), t('dashboard.col_checkTime')].map((h) => (
                            <th key={h} style={{ textAlign: h === '关键词' ? 'left' : 'center', padding: '10px 8px', color: '#86909C', fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap' }}>
                              {h}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {(ranks || []).slice(0, 8).map((r: any, i: number) => (
                          <tr key={i} style={{ borderBottom: '2px solid var(--color-fill-2)' }}>
                            <td style={{ padding: '12px 8px', minWidth: 200 }}>
                              <span style={{ display: 'block', fontWeight: 600, wordBreak: 'break-all', lineHeight: 1.4 }}>{r.keyword}</span>
                            </td>
                            <td style={{ padding: '12px 8px', whiteSpace: 'nowrap', textAlign: 'center' }}>
                              <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                                <PlatformLogo name={r.platform} index={i} size={26} rounded={7} />
                                <span style={{ fontSize: 13 }}>{r.platform}</span>
                              </div>
                            </td>
                            <td style={{ padding: '12px 8px', textAlign: 'center', whiteSpace: 'nowrap' }}>
                              {r.miss ? <span style={{ color: '#C9CDD4' }}>—</span>
                                : <span style={{ fontWeight: 700, fontSize: 14, color: r.position === 1 ? '#00B42A' : r.position <= 3 ? '#165DFF' : '#1d2129' }}>{r.position}</span>}
                            </td>
                            <td style={{ padding: '12px 8px', textAlign: 'center', whiteSpace: 'nowrap' }}>
                              <span style={{
                                display: 'inline-block', padding: '2px 12px', borderRadius: 12, fontSize: 12, fontWeight: 600,
                                color: r.top3 ? '#00B42A' : '#86909C', background: r.top3 ? '#00B42A14' : 'var(--color-fill-2)',
                              }}>
                                {r.top3 ? t('dashboard.yes') : t('dashboard.no')}
                              </span>
                            </td>
                            <td style={{ padding: '12px 8px', textAlign: 'center' }}>
                              {r.miss || r.delta === 0 ? <span style={{ color: '#86909C', fontSize: 13 }}>{t('dashboard.flat')}</span>
                                : r.delta > 0
                                  ? <span style={{ color: '#00B42A', fontWeight: 600, fontSize: 13 }}>↑ {r.delta}</span>
                                  : <span style={{ color: '#F53F3F', fontWeight: 600, fontSize: 13 }}>↓ {Math.abs(r.delta)}</span>}
                            </td>
                            <td style={{ padding: '12px 8px', color: '#86909C', fontSize: 12, whiteSpace: 'nowrap' }}>
                              {r.check_time ? String(r.check_time).slice(0, 10) : '-'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <Empty description={t('dashboard.rankEmpty')} style={{ padding: '60px 0' }} />
                )}
                <div style={{ textAlign: 'right', marginTop: 10 }}>
                  <span onClick={() => navigate('/keywords')} style={{ color: '#165DFF', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>
                    {t('dashboard.viewAllKeywords', { n: kpi.keyword_count ?? 0 })}
                  </span>
                </div>
              </Card>
            </GridCol>
            <GridCol xs={24} xl={9}>
              <Card style={{ borderRadius: 16, marginTop: 11 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.rankDist')}</span>}>
                <EChart option={distOption} height={220} />
              </Card>
              <Card style={{ borderRadius: 16, marginTop: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t('dashboard.rankPieTitle')}</span>}>
                {rankPieData.length > 0
                  ? <EChart option={rankPieOption} height={200} />
                  : <Empty description="暂无{t('dashboard.rankPieTitle')}数据" style={{ padding: '24px 0' }} />}
              </Card>
            </GridCol>
          </GridRow>

          {/* 数据说明 */}
          <div style={{ textAlign: 'right', marginTop: 16, fontSize: 12, lineHeight: 1.9, color: '#C9CDD4' }}>
            <div>{t('dashboard.autoRefresh', { n: kpi.total_platform ?? 0 })}</div>
            <div>{t('dashboard.disclaimer')}</div>
          </div>
        </>
      )}
      </div>
    </div>
  );
}
