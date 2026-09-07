import { useEffect, useRef, useState } from 'react';
import { Card, Grid, Button, Message, Tag, Space, Typography, Empty, Skeleton, Radio, Dropdown, Menu, Spin } from '@arco-design/web-react';
import { IconRefresh, IconExport, IconDown, IconSync } from '@arco-design/web-react/icon';
import { api } from '../api';
import EChart from '../components/EChart';
import { platformLogoUrl } from './Dashboard';

const { Row: GridRow, Col: GridCol } = Grid;
const { Title, Text } = Typography;

// ============ 品牌调性：清爽淡蓝白底 + 彩色高可读数据 ============
const KPI_STYLE = [
  { color: '#165DFF', grad: 'var(--geo-grad-blue)' },
  { color: '#722ED1', grad: 'var(--geo-grad-purple)' },
  { color: '#00B42A', grad: 'var(--geo-grad-green)' },
  { color: '#FF7D00', grad: 'var(--geo-grad-orange)' },
  { color: '#F53F3F', grad: 'var(--geo-grad-red)' },
  { color: '#14C9C9', grad: 'var(--geo-grad-cyan)' },
  { color: '#F7BA1E', grad: 'var(--geo-grad-amber)' },
  { color: '#3491FA', grad: 'var(--geo-grad-blue)' },
];
const PLAT_COLORS = ['#165DFF', '#722ED1', '#00B42A', '#F53F3F', '#FF7D00', '#14C9C9', '#F7BA1E', '#F5319D', '#3491FA', '#9FDB1D', '#0FC6C2', '#D91AD9'];
const SCENE_COLORS = ['#165DFF', '#14C9C9', '#722ED1', '#FF7D00', '#F7BA1E', '#00B42A', '#F53F3F', '#3491FA', '#F5319D', '#9FDB1D', '#0FC6C2', '#D91AD9'];
const RANK_COLORS = ['#00B42A', '#165DFF', '#FF7D00', '#F57A7A', '#e5e6eb', '#C9CDD4'];
const colorOf = (i: number) => PLAT_COLORS[i % PLAT_COLORS.length];

const ADV_COLOR: Record<string, any> = {
  good: { tag: 'green', border: '#00B42A44', bg: '#F4FEF6', dot: '#00B42A' },
  warn: { tag: 'orange', border: '#FF7D0044', bg: 'fffBF4', dot: '#FF7D00' },
  bad: { tag: 'red', border: '#F53F3F44', bg: 'fff7F7', dot: '#F53F3F' },
};

// ============ 数字滚动动画 ============
function CountUpNumber({ value, duration = 750, color }: { value: number; duration?: number; color?: string }) {
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
      const eased = 1 - Math.pow(1 - p, 3);
      setDisplay(Math.round(from + (value - from) * eased));
      if (p < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [value, duration]);
  return <span style={{ color, fontVariantNumeric: 'tabular-nums' }}>{display.toLocaleString()}</span>;
}

// 平台图片徽标（失败回退字母圆标）
function PlatformLogo({ name, index, size = 30, rounded = 9 }: { name: string; index: number; size?: number; rounded?: number }) {
  const [failed, setFailed] = useState(false);
  const url = platformLogoUrl(name);
  if (!url || failed) {
    const c = colorOf(index);
    return (
      <span style={{
        width: size, height: size, borderRadius: rounded, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        fontWeight: 700, fontSize: Math.max(11, size * 0.4), color: '#fff', flexShrink: 0,
        background: `linear-gradient(135deg, ${c}, ${c}99)`, boxShadow: `0 4px 10px ${c}33`,
      }}>{name.slice(0, 1).toUpperCase()}</span>
    );
  }
  return (
    <img src={url} alt={name} onError={() => setFailed(true)}
      style={{
        width: size, height: size, borderRadius: rounded, objectFit: 'contain', flexShrink: 0, display: 'inline-block',
        background: 'var(--geo-surface)', padding: 2, border: '1px solid var(--color-border-2)', boxShadow: '0 2px 6px rgba(0,0,0,0.06)',
      }} />
  );
}

// 出现率小徽章
function RateBadge({ rate }: { rate: number }) {
  const c = rate >= 80 ? '#00B42A' : rate >= 40 ? '#FF7D00' : '#F53F3F';
  return <span style={{ color: c, fontWeight: 700 }}>{rate.toFixed(1)}%</span>;
}

// 插入动画关键帧
function AnimStyle() {
  return (
    <style>{`
      @keyframes geoReportFadeUp { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: translateY(0); } }
      .geo-report-reveal { animation: geoReportFadeUp .5s ease both; }
    `}</style>
  );
}

export default function Report() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [days, setDays] = useState(7);
  const dataRef = useRef<any>(null);
  const daysRef = useRef(7);

  const load = async (d: number = daysRef.current) => {
    const isFirst = !dataRef.current;
    if (isFirst) setLoading(true); else setRefreshing(true);
    try {
      const res = await api.reportData(d);
      dataRef.current = res;
      setData(res);
    } catch (e: any) {
      Message.error('加载失败: ' + e.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };
  useEffect(() => { load(); }, []);

  const changeDays = (d: number) => {
    daysRef.current = d;
    setDays(d);
    load(d);
  };

  const doExport = async (format: 'md' | 'html' | 'docx') => {
    try {
      const name = await api.exportReport(format, days);
      Message.success('已导出 ' + name);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const exportDropList = (
    <Menu
      onClickMenuItem={(k) => {
        if (k === 'docx') doExport('docx');
        else if (k === 'html') doExport('html');
        else doExport('md');
      }}
    >
      <Menu.Item key="docx">Word 文档 (.docx)</Menu.Item>
      <Menu.Item key="html">网页 (.html)</Menu.Item>
      <Menu.Item key="md">Markdown (.md)</Menu.Item>
    </Menu>
  );

  if (loading) return <Skeleton text={{ rows: 12 }} animation />;

  const hasData = !!data?.has_data;
  const kpi = data?.kpi || {};
  const trend = data?.trend || [];
  const rankDist = data?.rank_dist || [];
  const scenes = data?.scene_dist || [];
  const platforms = data?.platforms || [];
  const kwTop = data?.keywords_top || [];
  const weakWords = data?.weak_words || [];
  const insights = data?.insights || [];
  const advice = data?.advice || [];

  // KPI 卡片列表
  const kpiCards = [
    { label: '品牌出现率', num: kpi.brand_rate ?? 0, tail: '%', desc: 'AI 回答中品牌出现占比', idx: 0 },
    { label: 'TOP3 覆盖率', num: kpi.top3_rate ?? 0, tail: '%', desc: '进入推荐前 3 位占比', idx: 1 },
    { label: '查询总数', num: kpi.total ?? 0, tail: '次', desc: `${kpi.platform_num ?? 0} 平台 × ${kpi.keyword_num ?? 0} 关键词`, idx: 2 },
    { label: '命中品牌', num: kpi.hit ?? 0, tail: '次', desc: '品牌出现在回答中', idx: 3 },
    { label: '平均提及', num: kpi.avg_mention ?? 0, tail: '次', desc: '命中时品牌被提及次数', idx: 4 },
    { label: '未命中', num: kpi.miss ?? 0, tail: '次', desc: kpi.errors ? `另有 ${kpi.errors} 条请求错误` : '需重点优化的部分', idx: 5 },
  ];

  // 波形图 option：品牌率 + 查询量 双轴
  const trendOption = {
    tooltip: { trigger: 'axis', backgroundColor: 'rgba(29,33,41,0.92)', borderWidth: 0, textStyle: { color: '#fff' } },
    legend: { top: 0, right: 4, textStyle: { fontSize: 12 } },
    grid: { left: 44, right: 44, top: 34, bottom: 28 },
    xAxis: {
      type: 'category' as const, data: trend.map((t: any) => t.day), boundaryGap: false,
      axisLine: { lineStyle: { color: '#e5e6eb' } }, axisLabel: { color: '#86909C', fontSize: 11, formatter: (v: string) => v },
    },
    yAxis: [
      {
        type: 'value' as const, name: '出现率 %', min: 0, max: 100,
        splitLine: { lineStyle: { color: '#f2f3f5' } }, axisLabel: { color: '#86909C', fontSize: 11 },
      },
      {
        type: 'value' as const, name: '查询量', minInterval: 1,
        splitLine: { show: false }, axisLabel: { color: '#86909C', fontSize: 11 },
      },
    ],
    series: [
      {
        name: '品牌出现率', type: 'line' as const, smooth: true, symbol: 'circle', symbolSize: 6, data: trend.map((t: any) => t.rate),
        itemStyle: { color: '#165DFF' }, lineStyle: { color: '#165DFF', width: 3 },
        areaStyle: {
          color: {
            type: 'linear' as const, x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(22,93,255,0.35)' },
              { offset: 1, color: 'rgba(22,93,255,0.02)' },
            ],
          },
        },
      },
      {
        name: '查询量', type: 'bar' as const, yAxisIndex: 1, barWidth: 10, data: trend.map((t: any) => t.queries),
        itemStyle: { color: 'rgba(20,201,201,0.55)', borderRadius: [4, 4, 0, 0] },
      },
    ],
  };

  // 排名分布环形
  const rankOption = {
    tooltip: { trigger: 'item' as const, formatter: '{b}: {c} 次 ({d}%)' },
    legend: { bottom: 0, type: 'scroll' as const, icon: 'circle', textStyle: { fontSize: 12 } },
    series: [{
      type: 'pie' as const, radius: ['58%', '76%'], center: ['50%', '43%'],
      avoidLabelOverlap: true, itemStyle: { borderRadius: 6, borderColor: 'var(--geo-surface)', borderWidth: 2 },
      label: { show: false },
      data: rankDist.map((d: any, i: number) => ({ name: d.label, value: d.value, itemStyle: { color: RANK_COLORS[i % RANK_COLORS.length] } })),
    }],
  };

  // 场景覆盖环形
  const sceneOption = {
    tooltip: { trigger: 'item' as const, formatter: '{b}: {c} 个关键词 ({d}%)' },
    legend: { bottom: 0, type: 'scroll' as const, icon: 'circle', textStyle: { fontSize: 12 } },
    series: [{
      type: 'pie' as const, radius: ['58%', '76%'], center: ['50%', '43%'],
      avoidLabelOverlap: true, itemStyle: { borderRadius: 6, borderColor: 'var(--geo-surface)', borderWidth: 2 },
      label: { show: false },
      data: scenes.map((s: any, i: number) => ({ name: s.name, value: s.value, itemStyle: { color: SCENE_COLORS[i % SCENE_COLORS.length] } })),
    }],
  };

  const avgRate = platforms.length ? platforms.reduce((s: number, p: any) => s + p.rate, 0) / platforms.length : 0;

  return (
    <div>
      <AnimStyle />
      {/* 刷新进度条 */}
      {refreshing && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, height: 3, zIndex: 1000, overflow: 'hidden' }}>
          <div style={{
            height: '100%', width: '40%', background: 'linear-gradient(90deg,#165DFF,#14C9C9,#FF7D00)',
            animation: 'geoRefreshingBar 1.1s linear infinite',
          }} />
          <style>{`@keyframes geoRefreshingBar { 0%{ margin-left:-40%; } 100%{ margin-left:100%; } }`}</style>
        </div>
      )}

      {/* 页首 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 12, marginBottom: 16 }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#165DFF,#14C9C9,#FF7D00)' }} />
            <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>GEO 生成报告</span>
            {refreshing && <Spin size={14} style={{ marginLeft: 4 }} />}
          </div>
          <div style={{ color: '#86909C', fontSize: 13, marginTop: 4 }}>
            统计周期 {data?.period || '-'} · 生成于 {data?.generated_at || '-'}
          </div>
        </div>
        <Space>
          <Radio.Group type="button" value={days} onChange={(v: any) => changeDays(v)}>
            <Radio value={7}>近 7 天</Radio>
            <Radio value={30}>近 30 天</Radio>
          </Radio.Group>
          <Button icon={<IconSync />} loading={refreshing} onClick={() => load()}>刷新</Button>
          <Dropdown droplist={exportDropList} position="br">
            <Button type="primary" icon={<IconExport />}>导出文档 <IconDown style={{ marginLeft: 2 }} /></Button>
          </Dropdown>
        </Space>
      </div>

      <div style={{ opacity: refreshing ? 0.55 : 1, transition: 'opacity .35s ease', pointerEvents: refreshing ? 'none' : 'auto' }}>
        {!hasData ? (
          <div className="geo-report-reveal">
            <Card style={{ borderRadius: 16, padding: '60px 0' }}>
              <Empty description="暂无巡检数据，先配置关键词与 AI 平台，运行巡检后即可生成可视化报告" />
            </Card>
          </div>
        ) : (
          <>
            {/* 一、彩色 KPI 数据带 */}
            <div className="geo-report-reveal">
              <GridRow gutter={[14, 14]}>
                {kpiCards.map((k, i) => (
                  <GridCol key={k.label} xs={12} sm={8} md={6} lg={4}>
                    <div
                      style={{
                        position: 'relative', overflow: 'hidden', borderRadius: 16, padding: '14px 16px 12px',
                        background: KPI_STYLE[k.idx % KPI_STYLE.length].grad, border: '1px solid var(--color-border-2)',
                        boxShadow: '0 4px 14px rgba(0,0,0,0.04)',
                      }}
                    >
                      <span style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 4, background: KPI_STYLE[k.idx % KPI_STYLE.length].color }} />
                      <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 2 }}>{k.label}</div>
                      <div style={{ fontSize: 26, fontWeight: 800, lineHeight: 1.2 }}>
                        <CountUpNumber value={k.num} color={KPI_STYLE[k.idx % KPI_STYLE.length].color} />
                        <span style={{ fontSize: 14, fontWeight: 600, color: '#86909C', marginLeft: 2 }}>{k.tail}</span>
                      </div>
                      <div style={{ fontSize: 11, color: '#86909C', marginTop: 4 }}>{k.desc}</div>
                    </div>
                  </GridCol>
                ))}
              </GridRow>
            </div>

            {/* 二、波形图 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <Card style={{ borderRadius: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>品牌出现率趋势波形</span>}>
                <EChart option={trendOption} height={300} />
              </Card>
            </div>

            {/* 三、占比图 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <GridRow gutter={[16, 16]}>
                <GridCol xs={24} lg={12}>
                  <Card style={{ borderRadius: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>排名分布占比</span>}>
                    {rankDist.length ? <EChart option={rankOption} height={270} /> : <Empty style={{ padding: '40px 0' }} description="暂无排名数据" />}
                  </Card>
                </GridCol>
                <GridCol xs={24} lg={12}>
                  <Card style={{ borderRadius: 16 }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>关键词场景占比</span>}>
                    {scenes.length ? <EChart option={sceneOption} height={270} /> : <Empty style={{ padding: '40px 0' }} description="暂无场景分类" />}
                  </Card>
                </GridCol>
              </GridRow>
            </div>

            {/* 四、图文表格：平台表现 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <Card
                style={{ borderRadius: 16 }}
                title={
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 15, fontWeight: 600 }}>各 AI 平台表现</span>
                    <Tag color="arcoblue" size="small">平均出现率 {avgRate.toFixed(1)}%</Tag>
                  </div>
                }
              >
                {platforms.length === 0 ? (
                  <Empty description="暂无平台数据" style={{ padding: '30px 0' }} />
                ) : (
                  <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                      <thead>
                        <tr style={{ color: '#86909C', fontSize: 12, textAlign: 'left' }}>
                          <th style={{ padding: '8px 10px' }}>平台</th>
                          <th style={{ padding: '8px 10px' }}>查询数</th>
                          <th style={{ padding: '8px 10px' }}>命中</th>
                          <th style={{ padding: '8px 10px', minWidth: 150 }}>出现率</th>
                          <th style={{ padding: '8px 10px' }}>TOP3</th>
                          <th style={{ padding: '8px 10px' }}>平均提及</th>
                          <th style={{ padding: '8px 10px' }}>错误</th>
                        </tr>
                      </thead>
                      <tbody>
                        {platforms.map((p: any, i: number) => {
                          const rateColor = p.rate >= 80 ? '#00B42A' : p.rate >= 40 ? '#FF7D00' : '#F53F3F';
                          return (
                            <tr key={p.name} style={{ borderTop: '1px solid var(--color-border-1)' }}>
                              <td style={{ padding: '10px' }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                  <PlatformLogo name={p.name} index={i} />
                                  <span style={{ fontWeight: 600 }}>{p.name}</span>
                                </div>
                              </td>
                              <td style={{ padding: '10px' }}>{p.queries}</td>
                              <td style={{ padding: '10px' }}><RateBadge rate={p.hit} /> 次</td>
                              <td style={{ padding: '10px' }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                  <div style={{ flex: 1, height: 8, borderRadius: 4, background: 'var(--color-fill-2)', overflow: 'hidden' }}>
                                    <div style={{ width: `${Math.min(p.rate, 100)}%`, height: '100%', borderRadius: 4, background: `linear-gradient(90deg, ${rateColor}88, ${rateColor})`, transition: 'width .6s ease' }} />
                                  </div>
                                  <span style={{ width: 52, textAlign: 'right', fontWeight: 700, color: rateColor }}>{p.rate.toFixed(1)}%</span>
                                </div>
                              </td>
                              <td style={{ padding: '10px' }}>{p.top3}</td>
                              <td style={{ padding: '10px' }}>{p.avg_mention}</td>
                              <td style={{ padding: '10px' }}>
                                {p.errors > 0 ? <Tag color="red">{p.errors}</Tag> : <span style={{ color: '#C9CDD4' }}>-</span>}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </Card>
            </div>

            {/* 五、图文表格：关键词 TOP10 + 薄弱词 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <GridRow gutter={[16, 16]}>
                <GridCol xs={24} lg={14}>
                  <Card style={{ borderRadius: 16, height: '100%' }} title={<span style={{ fontSize: 15, fontWeight: 600 }}>关键词查询 TOP10</span>}>
                    {kwTop.length === 0 ? <Empty description="暂无关键词数据" style={{ padding: '30px 0' }} /> : (
                      <div style={{ overflowX: 'auto' }}>
                        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                          <thead>
                            <tr style={{ color: '#86909C', fontSize: 12, textAlign: 'left' }}>
                              <th style={{ padding: '8px 10px' }}>#</th>
                              <th style={{ padding: '8px 10px' }}>问题</th>
                              <th style={{ padding: '8px 10px' }}>查询</th>
                              <th style={{ padding: '8px 10px' }}>命中</th>
                              <th style={{ padding: '8px 10px', minWidth: 120 }}>出现率</th>
                              <th style={{ padding: '8px 10px' }}>最佳平台</th>
                            </tr>
                          </thead>
                          <tbody>
                            {kwTop.map((k: any, i: number) => {
                              const rc = k.rate >= 80 ? '#00B42A' : k.rate >= 40 ? '#FF7D00' : '#F53F3F';
                              return (
                                <tr key={k.question + i} style={{ borderTop: '1px solid var(--color-border-1)' }}>
                                  <td style={{ padding: '10px', color: '#86909C' }}>{i + 1}</td>
                                  <td style={{ padding: '10px', fontWeight: 500, maxWidth: 220 }}>{k.question}</td>
                                  <td style={{ padding: '10px' }}>{k.queries}</td>
                                  <td style={{ padding: '10px' }}>{k.hit}</td>
                                  <td style={{ padding: '10px' }}>
                                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                      <div style={{ flex: 1, height: 8, borderRadius: 4, background: 'var(--color-fill-2)', overflow: 'hidden' }}>
                                        <div style={{ width: `${Math.min(k.rate, 100)}%`, height: '100%', borderRadius: 4, background: `linear-gradient(90deg, ${rc}88, ${rc})`, transition: 'width .6s ease' }} />
                                      </div>
                                      <span style={{ width: 52, textAlign: 'right', fontWeight: 700, color: rc }}>{k.rate.toFixed(1)}%</span>
                                    </div>
                                  </td>
                                  <td style={{ padding: '10px' }}>
                                    {k.best_platform ? <Tag color="arcoblue" size="small">{k.best_platform}</Tag> : <span style={{ color: '#C9CDD4' }}>-</span>}
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </Card>
                </GridCol>
                <GridCol xs={24} lg={10}>
                  <Card
                    style={{ borderRadius: 16, height: '100%' }}
                    title={
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ fontSize: 15, fontWeight: 600 }}>薄弱关键词</span>
                        <Tag color={weakWords.length ? 'red' : 'green'} size="small">{weakWords.length} 个需优化</Tag>
                      </div>
                    }
                  >
                    {weakWords.length === 0 ? (
                      <Empty description="暂无薄弱关键词，表现良好" style={{ padding: '30px 0' }} />
                    ) : (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        {weakWords.map((w: any, i: number) => (
                          <div key={w.question + i} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px', borderRadius: 12, background: 'fff7F7', border: '1px solid #F53F3F33' }}>
                            <span style={{ width: 24, height: 24, borderRadius: 7, background: 'linear-gradient(135deg,#F53F3F,#FF7D00)', color: '#fff', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: 12 }}>{i + 1}</span>
                            <div style={{ flex: 1, minWidth: 0 }}>
                              <div style={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{w.question}</div>
                              <div style={{ fontSize: 12, color: '#86909C', marginTop: 2 }}>{w.queries} 次查询 · 命中 {w.hit} 次 · 出现率 <b style={{ color: '#F53F3F' }}>{w.rate.toFixed(1)}%</b></div>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </Card>
                </GridCol>
              </GridRow>
            </div>

            {/* 六、分析总结 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <Card
                style={{ borderRadius: 16 }}
                title={
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 15, fontWeight: 600 }}>分析总结</span>
                    <Tag color="purple" size="small">AI Insights</Tag>
                  </div>
                }
              >
                {insights.length === 0 ? <Empty description="暂无洞察" style={{ padding: '20px 0' }} /> : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    {insights.map((s: string, i: number) => (
                      <div key={i} style={{ display: 'flex', gap: 10, padding: '12px 16px', borderRadius: 12, background: 'var(--geo-surface-2)', border: '1px solid var(--color-border-2)' }}>
                        <span style={{ width: 22, height: 22, borderRadius: '50%', background: 'linear-gradient(135deg,#722ED1,#14C9C9)', color: '#fff', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: 12, flexShrink: 0 }}>{i + 1}</span>
                        <span style={{ fontSize: 14, lineHeight: 1.6, color: 'var(--geo-text)' }}>{s}</span>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            </div>

            {/* 七、给予建议 */}
            <div className="geo-report-reveal" style={{ marginTop: 16 }}>
              <Card
                style={{ borderRadius: 16 }}
                title={
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 15, fontWeight: 600 }}>优化建议</span>
                    <Tag color="orangered" size="small">行动指南</Tag>
                  </div>
                }
              >
                {advice.length === 0 ? <Empty description="暂无建议" style={{ padding: '20px 0' }} /> : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    {advice.map((a: any, i: number) => {
                      const st = ADV_COLOR[a.level] || ADV_COLOR.warn;
                      const iconMap: Record<string, string> = { good: '✓', warn: '!', bad: '✕' };
                      return (
                        <div key={i} style={{ display: 'flex', gap: 12, padding: '14px 16px', borderRadius: 12, background: st.bg, border: `1px solid ${st.border}`, borderLeft: `4px solid ${st.dot}` }}>
                          <span style={{ width: 24, height: 24, borderRadius: '50%', background: st.dot, color: '#fff', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: 13, flexShrink: 0 }}>{iconMap[a.level] || '!'}</span>
                          <div>
                            <div style={{ fontSize: 14, fontWeight: 700, color: 'var(--geo-text)' }}>{a.title}</div>
                            <div style={{ fontSize: 13, lineHeight: 1.6, color: 'var(--color-text-2)', marginTop: 2 }}>{a.desc}</div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </Card>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
