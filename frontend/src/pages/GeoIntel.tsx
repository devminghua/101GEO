import { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import OpsReportTab from './OpsReportTab';
import {
  Card, Button, Message, Tag, Space, Typography, Table, Form, Input, Select, Tabs, Spin,
  Switch, Popconfirm, Modal, Empty, Progress, Alert, InputNumber, Statistic, Grid, Skeleton,
} from '@arco-design/web-react';
import {
  IconPlus, IconRefresh, IconThunderbolt, IconSafe, IconFile, IconCopy, IconDownload,
  IconCheckCircle, IconClockCircle, IconExclamationCircle, IconDelete, IconEdit, IconArrowRight, IconSave,
} from '@arco-design/web-react/icon';

const { Title, Text, Paragraph } = Typography;
const { Row: GridRow, Col: GridCol } = Grid;
const TabPane = Tabs.TabPane;
const FormItem = Form.Item;

const KPI_STYLE = [
  { label: '品牌出现率', key: 'brand_rate', color: '#165DFF', grad: 'var(--geo-grad-blue)', unit: '%' },
  { label: '推荐率 TOP3', key: 'top3_rate', color: '#722ED1', grad: 'var(--geo-grad-purple)', unit: '%' },
  { label: '引用率', key: 'citation_rate', color: '#00B42A', grad: 'var(--geo-grad-green)', unit: '%' },
  { label: '事实一致率', key: 'accuracy_rate', color: '#FF7D00', grad: 'var(--geo-grad-orange)', unit: '%' },
  { label: '竞品声量', key: 'competitor_sov', color: '#F53F3F', grad: 'var(--geo-grad-red)', unit: '%' },
  { label: '风险回答率', key: 'risk_rate', color: '#14C9C9', grad: 'var(--geo-grad-cyan)', unit: '%' },
];

function fmtNum(v: any, digits = 1) {
  const n = Number(v);
  if (Number.isNaN(n)) return '-';
  return n.toFixed(digits);
}

function DeltaTag({ before, after }: { before: number; after: number }) {
  const d = Number(after) - Number(before);
  if (Number.isNaN(d)) return null;
  const color = d > 0 ? 'green' : d < 0 ? 'red' : 'gray';
  const txt = d > 0 ? `+${fmtNum(d)}pt` : `${fmtNum(d)}pt`;
  return <Tag color={color} style={{ marginTop: 4 }}>{txt}</Tag>;
}

function rateColor(v: number) {
  if (v >= 80) return '#00B42A';
  if (v >= 50) return '#FF7D00';
  return '#F53F3F';
}

/* ============================================================
 * AI 可见度评分卡（AIVS）
 * 对标 Profound / Otterly.AI / CiteLens 的旗舰指标：
 * 一个客户看得懂的总分 + 四维拆解 + 行业基准 + 改进建议。
 * ============================================================ */
function gradeColorOf(grade: string) {
  switch (grade) {
    case 'A': return '#00B42A';
    case 'B': return '#165DFF';
    case 'C': return '#FF7D00';
    default: return '#F53F3F';
  }
}

function VisibilityScoreCard({ vs, sampleCount, refreshing }: { vs: any; sampleCount?: number; refreshing?: boolean }) {
  if (!vs || typeof vs.score !== 'number') return null;
  const gc = gradeColorOf(vs.grade);
  const dims: any[] = vs.dimensions || [];
  const bms: any[] = vs.benchmarks || [];
  const sugs: string[] = vs.suggestions || [];
  const delta = Number(vs.delta_30 || 0);
  const lowConf = vs.confidence === '低' || vs.confidence === '中';

  return (
    <Card style={{ borderRadius: 16, marginBottom: 16 }} bordered={false} bodyStyle={{ padding: '20px 24px' }}>
      <div style={{ display: 'flex', gap: 28, flexWrap: 'wrap' }}>
        {/* 左：总分环 */}
        <div style={{ flex: '0 0 auto', textAlign: 'center', minWidth: 132 }}>
          <Progress
            type="circle"
            percent={Number(vs.score)}
            width={112}
            strokeWidth={8}
            color={gc}
            formatText={() => (
              <div style={{ lineHeight: 1.2 }}>
                <div style={{ fontSize: 30, fontWeight: 700, color: gc }}>{fmtNum(vs.score, 0)}</div>
                <div style={{ fontSize: 11, color: '#86909C' }}>AI 可见度</div>
              </div>
            )}
          />
          <div style={{ marginTop: 10 }}>
            <Tag color={gc} size="large">{vs.grade} · {vs.grade_label}</Tag>
          </div>
          {delta !== 0 && (
            <div style={{ fontSize: 12, marginTop: 8, color: delta > 0 ? '#00B42A' : '#F53F3F' }}>
              较前期 {delta > 0 ? `↑ ${fmtNum(delta)}` : `↓ ${fmtNum(Math.abs(delta))}`} 分
            </div>
          )}
        </div>

        {/* 中：四维拆解 + 行业基准 */}
        <div style={{ flex: '1 1 300px', minWidth: 260 }}>
          <Text style={{ fontSize: 14, fontWeight: 600, display: 'block', marginBottom: 12 }}>
            得分拆解 <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>· 灰色刻度为行业基准</Text>
          </Text>
          {dims.map((d) => {
            const bm = bms.find((b) => b.key === d.key);
            const bmVal = bm ? Number(bm.benchmark) : 0;
            const gap = Number(d.score) - bmVal;
            const color = gap >= 0 ? '#00B42A' : Number(d.score) < bmVal - 15 ? '#F53F3F' : '#FF7D00';
            return (
              <div key={d.key} style={{ marginBottom: 12 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, marginBottom: 4 }}>
                  <span>
                    {d.name}
                    <Text type="secondary" style={{ fontSize: 12, marginLeft: 6 }}>
                      权重 {Math.round(Number(d.weight) * 100)}%
                    </Text>
                  </span>
                  <span style={{ fontWeight: 600, color }}>
                    {fmtNum(d.score, 0)}
                    <Text type="secondary" style={{ fontSize: 12, fontWeight: 400, marginLeft: 6 }}>
                      基准 {fmtNum(bmVal, 0)} · {gap >= 0 ? '+' : ''}{fmtNum(gap, 0)}
                    </Text>
                  </span>
                </div>
                <div style={{ position: 'relative' }}>
                  <Progress
                    percent={Math.min(100, Number(d.score))}
                    showText={false}
                    color={color}
                    strokeWidth={6}
                    size="small"
                  />
                  {/* 行业基准刻度 */}
                  <span
                    style={{
                      position: 'absolute',
                      top: -2,
                      left: `calc(${Math.min(100, bmVal)}% - 1px)`,
                      width: 2,
                      height: 10,
                      background: '#C9CDD4',
                      borderRadius: 1,
                    }}
                  />
                </div>
              </div>
            );
          })}
          {lowConf && (
            <div style={{ fontSize: 12, color: '#86909C', marginTop: 4 }}>
              样本量 {sampleCount ?? 0} 条，置信度{vs.confidence}——AI 回答有概率性，建议积累更多巡检数据后再看趋势。
            </div>
          )}
        </div>

        {/* 右：改进建议 */}
        <div style={{ flex: '1 1 260px', minWidth: 240 }}>
          <Text style={{ fontSize: 14, fontWeight: 600, display: 'block', marginBottom: 12 }}>优先改进项</Text>
          {sugs.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 13 }}>暂无建议，继续保持。</Text>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {sugs.map((s, i) => (
                <div key={i} style={{ display: 'flex', gap: 8, fontSize: 13, color: 'var(--color-text-2)', lineHeight: '20px', opacity: refreshing ? 0.5 : 1 }}>
                  <span
                    style={{
                      flex: '0 0 auto', width: 18, height: 18, borderRadius: '50%',
                      background: i === 0 ? '#F53F3F' : 'var(--color-fill-2)',
                      color: i === 0 ? '#fff' : '#86909C',
                      fontSize: 11, display: 'flex', alignItems: 'center', justifyContent: 'center', marginTop: 1,
                    }}
                  >
                    {i + 1}
                  </span>
                  <span>{s}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

/* ============================================================
 * Tab1 六项指标总览
 * ============================================================ */
function IntelTab() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [days, setDays] = useState(7);
  const dataRef = useRef<any>(null);
  // 默认品牌词（关键词未单独填写品牌词时的兜底，用于 AI 回答命中检测）
  const [brand, setBrand] = useState('');
  const [brandSaving, setBrandSaving] = useState(false);

  const loadBrand = () => {
    api.getSettings().then((s: any) => setBrand(s?.default_brand || '')).catch(() => {});
  };
  const saveBrand = async () => {
    const v = brand.trim();
    if (!v) { Message.warning('请输入优化的关键词'); return; }
    setBrandSaving(true);
    try {
      await api.saveSettings({ default_brand: v });
      Message.success('优化的关键词已保存');
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    } finally {
      setBrandSaving(false);
    }
  };

  const load = async (d = days) => {
    const isFirst = !dataRef.current;
    if (isFirst) setLoading(true); else setRefreshing(true);
    try {
      const res: any = await api.geoIntel(d);
      dataRef.current = res;
      setData(res);
    } catch (e: any) {
      Message.error('指标加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };
  useEffect(() => { load(); loadBrand(); /* eslint-disable-next-line */ }, []);

  const maxTrend = Math.max(1, ...(data?.trend || []).map((t: any) => t.rate || 0));
  const skeleton = loading || !data;
  const kpiCardStyle = { borderRadius: 16, border: '1px solid rgba(128,128,128,0.08)', height: '100%' };

  return (
    <>
      <style>{`
        .geo-kpi-scroll { scrollbar-width: thin; scrollbar-color: rgba(128,128,128,0.28) transparent; }
        .geo-kpi-scroll::-webkit-scrollbar { height: 6px; }
        .geo-kpi-scroll::-webkit-scrollbar-thumb { background: rgba(128,128,128,0.28); border-radius: 3px; }
        .geo-kpi-scroll::-webkit-scrollbar-track { background: transparent; }
        .geo-kpi-card { transition: all .25s ease; box-shadow: 0 4px 16px rgba(31,35,41,0.06); }
        .geo-kpi-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgba(31,35,41,0.10); }
        .geo-fade-in { animation: geoFadeIn .35s ease; }
        @keyframes geoFadeIn { from { opacity: 0; } to { opacity: 1; } }
      `}</style>

      {/* 优化的关键词设置（关键词未单独填写时的兜底，用于 AI 回答命中检测） */}
      <Card style={{ borderRadius: 16, marginBottom: 16 }} bordered={false} size="small">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <Text style={{ fontWeight: 600, flexShrink: 0 }}>优化的关键词</Text>
          <Input
            value={brand}
            onChange={(v) => setBrand(v)}
            placeholder="如：婚恋交友,同城相亲"
            style={{ width: 340, maxWidth: '100%' }}
            onPressEnter={saveBrand}
          />
          <Button type="primary" size="small" icon={<IconSave />} loading={brandSaving} onClick={saveBrand}>
            保存
          </Button>
          <Text type="secondary" style={{ fontSize: 12 }}>
            逗号分隔多个关键词；关键词未单独填写时，巡检将用它做 AI 回答命中检测
          </Text>
        </div>
      </Card>

      {/* 顶部操作栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          {skeleton ? (
            <Skeleton animation text={{ rows: 1, width: 200 }} />
          ) : (
            <Text style={{ fontSize: 14 }}>统计区间：{data.period}</Text>
          )}
          <Select value={days} onChange={(v) => { setDays(v); load(v); }} style={{ width: 120 }}>
            <Select.Option value={7}>近 7 天</Select.Option>
            <Select.Option value={30}>近 30 天</Select.Option>
          </Select>
        </Space>
        <Button icon={<IconRefresh />} onClick={() => load(days)}>刷新</Button>
      </div>

      {/* AI 可见度评分（最高优先级展示：一个总分 + 四维拆解 + 基准 + 建议） */}
      {!skeleton && <VisibilityScoreCard vs={data.visibility} sampleCount={data.sample_count} refreshing={refreshing} />}

      {/* KPI 自适应网格：根据屏幕宽度自动换行，全部显示在屏幕内。
          marginBottom 给下方「各 AI 平台表现明细」留出间距——不能写在 GridRow 上：
          Row 带 gutter 时会输出内联 margin:-8px 覆盖掉（净剩 8px 被 Col 的 padding 抵消），
          故此间距挂在网格容器上；视觉间距 = marginBottom 28 - Row 负 8 = 20px。 */}
      <div style={{ display: 'grid', gap: 16, gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', marginBottom: 28, opacity: refreshing ? 0.5 : 1, transition: 'opacity .3s ease' }}>
        {skeleton
          ? KPI_STYLE.map((k) => (
              <div key={k.key}>
                <Card style={{ borderRadius: 16, height: '100%' }}>
                  <Skeleton animation>
                    <Skeleton text={{ rows: 3, width: ['92%', '60%', '45%'] }} style={{ marginTop: 12 }} />
                  </Skeleton>
                </Card>
              </div>
            ))
          : (
            <>
              {KPI_STYLE.map((k) => {
                const v = Number(data[k.key] || 0);
                const delta = data?.deltas?.[k.key];
                return (
                  <div key={k.key} className="geo-fade-in">
                    <Card
                      className="geo-kpi-card"
                      style={{ ...kpiCardStyle, background: k.grad }}
                      bodyStyle={{ padding: '16px 20px' }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: k.color, display: 'inline-block', flex: '0 0 auto' }} />
                        <Text style={{ color: 'var(--color-text-2)', fontSize: 13 }}>{k.label}</Text>
                      </div>
                      <div style={{ fontSize: 30, fontWeight: 700, color: k.color, margin: '8px 0 4px' }}>
                        {fmtNum(v)}
                        <span style={{ fontSize: 16, marginLeft: 2 }}>{k.unit}</span>
                      </div>
                      {delta !== undefined && delta !== 0 && (
                        <div style={{ fontSize: 12, marginBottom: 6, fontWeight: 600, color: delta > 0 ? '#00B42A' : '#F53F3F' }}>
                          较前期 {delta > 0 ? `↑ ${delta}%` : `↓ ${Math.abs(delta)}%`}
                        </div>
                      )}
                      {delta !== undefined && delta === 0 && (
                        <div style={{ fontSize: 12, marginBottom: 6, color: '#86909C' }}>较前期 持平</div>
                      )}
                      <Progress percent={Math.min(100, v)} showText={false} color={k.color} strokeWidth={6} size="small" />
                    </Card>
                  </div>
                );
              })}
              {/* 声量对比卡 */}
              <div className="geo-fade-in">
                <Card
                  className="geo-kpi-card"
                  style={{ ...kpiCardStyle, background: 'var(--geo-grad-teal)' }}
                  bodyStyle={{ padding: '16px 20px' }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#14C9C9', display: 'inline-block', flex: '0 0 auto' }} />
                    <Text style={{ color: 'var(--color-text-2)', fontSize: 13 }}>品牌声量 vs 竞品声量</Text>
                  </div>
                  <div style={{ display: 'flex', gap: 20, margin: '8px 0 4px' }}>
                    <div>
                      <div style={{ fontSize: 26, fontWeight: 700, color: '#165DFF' }}>{fmtNum(data.brand_sov)}%</div>
                      <Text style={{ fontSize: 12, color: '#86909C' }}>品牌 SOV</Text>
                    </div>
                    <div>
                      <div style={{ fontSize: 26, fontWeight: 700, color: '#F53F3F' }}>{fmtNum(data.competitor_sov)}%</div>
                      <Text style={{ fontSize: 12, color: '#86909C' }}>竞品 SOV</Text>
                    </div>
                  </div>
                  {data.competitor_sov > data.brand_sov && (
                    <Tag color="red" icon={<IconExclamationCircle />}>竞品声量高于品牌，建议优先处理缺口</Tag>
                  )}
                </Card>
              </div>
            </>
          )}
      </div>

      {/* 下方两栏响应式（与上方间距由上方网格容器的 marginBottom 提供） */}
      <GridRow gutter={[16, 16]}>
        <GridCol xs={24} md={14}>
          <Card title="各 AI 平台表现明细" style={{ borderRadius: 16 }}>
            {skeleton ? (
              <Skeleton animation>
                <Skeleton text={{ rows: 1, width: '100%' }} />
                <Skeleton text={{ rows: 1, width: '100%' }} />
                <Skeleton text={{ rows: 1, width: '100%' }} />
                <Skeleton text={{ rows: 1, width: '100%' }} />
              </Skeleton>
            ) : (
              <div className="geo-fade-in">
                <Table
                  size="small" pagination={false} border={false}
                  data={data.platforms || []}
                  columns={[
                    { title: '平台', dataIndex: 'name' },
                    { title: '查询', dataIndex: 'queries', width: 70 },
                    {
                      title: '品牌出现率', dataIndex: 'brand_rate', width: 150,
                      render: (v) => <Progress percent={v} size="small" color={rateColor(v)} />,
                    },
                    {
                      title: '引用率', dataIndex: 'citation_rate', width: 150,
                      render: (v) => <Progress percent={v} size="small" color="#00B42A" />,
                    },
                    {
                      title: '事实一致率', dataIndex: 'accuracy_rate', width: 150,
                      render: (v) => <Progress percent={v} size="small" color="#FF7D00" />,
                    },
                    {
                      title: '竞品声量', dataIndex: 'competitor_sov', width: 150,
                      render: (v) => <Progress percent={v} size="small" color="#F53F3F" />,
                    },
                    {
                      title: '风险率', dataIndex: 'risk_rate', width: 150,
                      render: (v) => <Progress percent={v} size="small" color="#14C9C9" />,
                    },
                  ]}
                />
                {(data.platforms || []).length === 0 && <Empty description="暂无平台数据，请先运行巡检任务" />}
              </div>
            )}
          </Card>
        </GridCol>
        <GridCol xs={24} md={10}>
          <Card title="品牌出现率趋势（按天）" style={{ borderRadius: 16 }}>
            {skeleton ? (
              <Skeleton animation>
                <Skeleton text={{ rows: 5, width: '100%' }} />
              </Skeleton>
            ) : (
              <div className="geo-fade-in">
                {data.trend && data.trend.length > 1 ? (
                  <svg viewBox="0 0 320 160" width="100%" height={160}>
                    {[25, 50, 75, 100].map((p) => (
                      <line key={p} x1={0} y1={160 - p * 1.2} x2={320} y2={160 - p * 1.2} stroke="#E5E6EB" strokeDasharray="4 4" />
                    ))}
                    <polyline
                      fill="none" stroke="#165DFF" strokeWidth={2.5} strokeLinejoin="round" strokeLinecap="round"
                      points={data.trend.map((t: any, i: number) => `${10 + i * (300 / Math.max(1, data.trend.length - 1))},${160 - (t.rate / maxTrend) * 120}`).join(' ')}
                    />
                    {data.trend.map((t: any, i: number) => {
                      const x = 10 + i * (300 / Math.max(1, data.trend.length - 1));
                      const y = 160 - (t.rate / maxTrend) * 120;
                      return <circle key={i} cx={x} cy={y} r={3.5} fill="#165DFF" />;
                    })}
                  </svg>
                ) : (
                  <Empty description="数据不足，无法绘制趋势" style={{ padding: 40 }} />
                )}
                <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8 }}>
                  {(data.trend || []).map((t: any, i: number) => (
                    <Text key={i} style={{ fontSize: 11, color: '#86909C' }}>{t.day}</Text>
                  ))}
                </div>
                {(data.trend || []).length > 0 && (
                  <div style={{ marginTop: 12 }}>
                    <Text style={{ color: 'var(--color-text-2)', fontSize: 12 }}>
                      平均出现率 <b style={{ color: '#165DFF' }}>{fmtNum((data.trend || []).reduce((s: number, t: any) => s + (t.rate || 0), 0) / data.trend.length)}%</b>
                      ，每次被提及约 <b>{fmtNum(data.avg_mention, 1)}</b> 次，带引用的回答平均引用 <b>{fmtNum(data.avg_citation, 1)}</b> 个来源
                    </Text>
                  </div>
                )}
              </div>
            )}
          </Card>
        </GridCol>
      </GridRow>
    </>
  );
}

/* ============================================================
 * Tab2 竞品对比 + 品牌缺口分析
 * ============================================================ */
function GapTab() {
  const [data, setData] = useState<any>(null);
  const [cmp, setCmp] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const load = async () => {
    setLoading(true);
    try {
      const [res, c]: any[] = await Promise.all([api.geoGaps(7), api.geoCompare(30, 7)]);
      setData(res);
      setCmp(c);
    } catch (e: any) {
      Message.error('缺口分析加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const levelMap: any = {
    high: { color: 'red', text: '高危缺口' },
    mid: { color: 'orange', text: '中危' },
    low: { color: 'green', text: '正常' },
  };

  return (
    <Spin loading={loading} style={{ display: 'block' }}>
      {data && (
        <GridRow gutter={[16, 16]}>
          <GridCol span={8}>
            <Card title="竞品 AI 声量排行（近 7 天提及）">
              {data.competitor_sov && data.competitor_sov.length > 0 ? (
                <Table
                  size="small" pagination={false} border={false}
                  data={data.competitor_sov}
                  columns={[
                    { title: '竞品', dataIndex: 'name' },
                    { title: '提及次数', dataIndex: 'mentions', width: 84 },
                    { title: '首推次数', dataIndex: 'first_mentions', width: 84, render: (v, r: any) => `${v ?? 0} (${r.mentions ? Math.round((v ?? 0) / r.mentions * 100) : 0}%)` },
                    { title: '涉及问题数', dataIndex: 'questions', width: 96, render: (v) => (v || []).length },
                  ]}
                />
              ) : (
                <Empty description="未检测到竞品提及（需先在事实库维护竞品词）" />
              )}
              <Alert
                style={{ marginTop: 12 }}
                type="info"
                content="竞品声量越高，说明 AI 越倾向在相关问题上推荐竞品。建议在事实库补充差异化事实，抢占品牌出现率。"
              />
            </Card>
          </GridCol>
          <GridCol span={16}>
            <Card
              title="品牌缺口分析（品牌缺席 / 竞品抢占）"
              extra={<Tag color={data.high_gap > 0 ? 'red' : 'green'}>{data.high_gap} 个高危缺口</Tag>}
            >
              {data.gaps && data.gaps.length > 0 ? (
                <Table
                  size="small" pagination={false} border={false}
                  data={data.gaps}
                  rowKey={(r: any) => r.question}
                  columns={[
                    { title: '问题（关键词）', dataIndex: 'question' },
                    { title: '回答数', dataIndex: 'answered', width: 90, render: (_v, r) => Number(r.misses || 0) + Number(r.competitor_mention || 0) },
                    { title: '品牌缺席', dataIndex: 'misses', width: 90, render: (v) => <Tag color={v > 0 ? 'red' : 'green'}>{v} 次</Tag> },
                    { title: '竞品出现', dataIndex: 'competitor_mention', width: 90, render: (v) => (v > 0 ? <Tag color="orange">{v} 次</Tag> : '-') },
                    { title: '出现竞品', dataIndex: 'competitor_names', render: (v) => (v || []).map((n: string) => <Tag key={n} style={{ marginRight: 4 }}>{n}</Tag>) },
                    { title: '等级', dataIndex: 'level', width: 100, render: (v) => <Tag color={levelMap[v]?.color}>{levelMap[v]?.text}</Tag> },
                  ]}
                />
              ) : (
                <Empty description="暂无缺口数据，请先运行巡检任务" />
              )}
            </Card>
          </GridCol>
        </GridRow>
      )}

      {cmp && (
        <Card title="效果归因（前后期对比）" style={{ marginTop: 16 }}>
          <GridRow gutter={[16, 16]} style={{ marginBottom: 12 }}>
            <GridCol span={6}>
              <div style={{ textAlign: 'center', padding: '12px 0' }}>
                <Text style={{ fontSize: 12, color: '#86909C' }}>品牌出现率</Text>
                <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
                  {fmtNum(cmp.brand_rate?.[0])}% <IconArrowRight style={{ color: '#86909C' }} /> <span style={{ color: '#165DFF' }}>{fmtNum(cmp.brand_rate?.[1])}%</span>
                </div>
                <DeltaTag before={cmp.brand_rate?.[0]} after={cmp.brand_rate?.[1]} />
              </div>
            </GridCol>
            <GridCol span={6}>
              <div style={{ textAlign: 'center', padding: '12px 0' }}>
                <Text style={{ fontSize: 12, color: '#86909C' }}>首推率</Text>
                <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
                  {fmtNum(cmp.first_rate?.[0])}% <IconArrowRight style={{ color: '#86909C' }} /> <span style={{ color: '#00B42A' }}>{fmtNum(cmp.first_rate?.[1])}%</span>
                </div>
                <DeltaTag before={cmp.first_rate?.[0]} after={cmp.first_rate?.[1]} />
              </div>
            </GridCol>
            <GridCol span={12}>
              <Alert type="info" content={`对比前期（${cmp.before_days} 天）与后期（近 ${cmp.after_days} 天）的提及率变化，定位优化动作是否起效。`} />
            </GridCol>
          </GridRow>
          {cmp.questions && cmp.questions.length > 0 ? (
            <Table
              size="small" pagination={{ pageSize: 10, showTotal: true }}
              rowKey={(r: any) => r.question}
              data={cmp.questions}
              columns={[
                { title: '问题（关键词）', dataIndex: 'question' },
                { title: '前期出现率', dataIndex: 'before_rate', width: 110, render: (v, r) => `${fmtNum(v)}% (${r.before_hits}/${r.before_total})` },
                { title: '后期出现率', dataIndex: 'after_rate', width: 110, render: (v, r) => `${fmtNum(v)}% (${r.after_hits}/${r.after_total})` },
                { title: '变化', dataIndex: 'delta', width: 100, render: (v) => <Tag color={v > 0 ? 'green' : v < 0 ? 'red' : 'gray'}>{v > 0 ? `+${fmtNum(v)}` : fmtNum(v)}pt</Tag> },
              ]}
            />
          ) : (
            <Empty description="暂无前后期对比数据，需至少两个时间段的巡检记录" />
          )}
        </Card>
      )}
    </Spin>
  );
}

/* ============================================================
 * Tab3 优化行动清单
 * ============================================================ */
function ActionTab() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [genLoading, setGenLoading] = useState(false);
  const [filter, setFilter] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res: any = await api.listActions(filter ? `status=${filter}` : '');
      setList(res || []);
    } catch (e: any) {
      Message.error('行动清单加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); /* eslint-disable-next-line */ }, []);

  const generate = async () => {
    setGenLoading(true);
    try {
      const res: any = await api.generateActions();
      Message.success(`已基于最新巡检数据生成/更新行动清单，新增 ${res?.created ?? 0} 条`);
      load();
    } catch (e: any) {
      Message.error('生成失败：' + (e.message || e));
    } finally {
      setGenLoading(false);
    }
  };

  const updateStatus = async (id: number, status: string) => {
    await api.updateAction(id, { status });
    Message.success(status === 'done' ? '已标记完成' : status === 'doing' ? '已标记进行中' : '已重新打开');
    load();
  };

  const typeTag: any = {
    gap: { c: 'blue', t: '内容缺口' },
    competitor: { c: 'purple', t: '竞品反制' },
    risk: { c: 'red', t: '合规风险' },
    citation: { c: 'green', t: '引用提升' },
    audit: { c: 'orange', t: '网站优化' },
  };

  return (
    <Card
      title="优化行动清单（监测 → 建议 → 内容 → 复测 闭环）"
      extra={
        <Space>
          <Select value={filter} onChange={(v) => { setFilter(v); setTimeout(load, 0); }} style={{ width: 120 }} placeholder="全部状态">
            <Select.Option value="">全部</Select.Option>
            <Select.Option value="open">待处理</Select.Option>
            <Select.Option value="doing">进行中</Select.Option>
            <Select.Option value="done">已完成</Select.Option>
          </Select>
          <Button type="primary" icon={<IconThunderbolt />} loading={genLoading} onClick={generate}>
            一键生成行动清单
          </Button>
        </Space>
      }
    >
      <Table
        loading={loading} rowKey="id"
        data={list}
        columns={[
          { title: '类型', dataIndex: 'type', width: 100, render: (v) => <Tag color={typeTag[v]?.c}>{typeTag[v]?.t || v}</Tag> },
          { title: '任务', dataIndex: 'title' },
          { title: '详情与依据', dataIndex: 'detail' },
          {
            title: '优先级', dataIndex: 'priority', width: 90,
            render: (v) => <Tag color={v === 1 ? 'red' : v === 2 ? 'orange' : 'blue'}>P{v}</Tag>,
          },
          {
            title: '风险分级', dataIndex: 'risk_level', width: 100,
            render: (v) => v === 'high'
              ? <Tag color="red">高风险·技改</Tag>
              : v === 'observe'
                ? <Tag color="orange">需观察</Tag>
                : <Tag color="green">低风险·速优</Tag>,
          },
          {
            title: '状态', dataIndex: 'status', width: 110,
            render: (v) => v === 'done'
              ? <Tag color="green" icon={<IconCheckCircle />}>已完成</Tag>
              : v === 'doing'
                ? <Tag color="blue" icon={<IconClockCircle />}>进行中</Tag>
                : <Tag color="orange" icon={<IconExclamationCircle />}>待处理</Tag>,
          },
          {
            title: '操作', width: 200,
            render: (_v, r) => (
              <Space>
                {r.status !== 'done' && (
                  <Button size="mini" type="primary" status="success" onClick={() => updateStatus(r.id, 'done')}>完成</Button>
                )}
                {r.status === 'done' && <Button size="mini" onClick={() => updateStatus(r.id, 'open')}>重开</Button>}
                {r.status === 'open' && <Button size="mini" onClick={() => updateStatus(r.id, 'doing')}>开始</Button>}
                <Popconfirm title="确认删除该行动项？" onOk={async () => { await api.deleteAction(r.id); load(); }}>
                  <Button size="mini" status="danger" icon={<IconDelete />} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
        pagination={{ pageSize: 10, showTotal: true }}
      />
      {list.length === 0 && <Empty description="暂无行动项，点击「一键生成行动清单」基于巡检数据自动生成" />}
    </Card>
  );
}

/* ============================================================
 * Tab4 品牌事实库（GEO 准确率比对的知识底座）
 * ============================================================ */
function FactTab() {
  const [list, setList] = useState<any[]>([]);
  const [form] = Form.useForm();
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const res: any = await api.listFacts();
      setList(res || []);
    } finally { setLoading(false); }
  };
  useEffect(() => { load(); }, []);

  const openCreate = () => { setEditing(null); form.resetFields(); setModalOpen(true); };
  const openEdit = (r: any) => {
    setEditing(r);
    form.setFieldsValue({ category: r.category, question: r.question, fact: r.fact, not_fact: r.not_fact, enabled: r.enabled });
    setModalOpen(true);
  };

  const submit = async () => {
    const v = await form.validate();
    if (!v.fact) { Message.warning('请填写事实内容'); return; }
    try {
      if (editing) {
        await api.updateFact(editing.id, v);
        Message.success('已更新');
      } else {
        await api.createFact(v);
        Message.success('已添加');
      }
      setModalOpen(false);
      load();
    } catch (e: any) {
      Message.error('保存失败：' + (e.message || e));
    }
  };

  return (
    <Card
      title="品牌事实库（准确率比对基准 · 事实一致率 = 回答与事实库无冲突的比例）"
      extra={<Button type="primary" icon={<IconPlus />} onClick={openCreate}>添加事实</Button>}
    >
      <Table
        loading={loading} rowKey="id" size="small"
        data={list}
        columns={[
          { title: '分类', dataIndex: 'category', width: 110, render: (v) => <Tag>{v}</Tag> },
          { title: '适用问题', dataIndex: 'question', width: 200 },
          { title: '事实内容', dataIndex: 'fact' },
          { title: '边界 / 禁区表述', dataIndex: 'not_fact', render: (v) => (v ? <Text style={{ color: '#F53F3F' }}>{v}</Text> : '-') },
          {
            title: '启用', dataIndex: 'enabled', width: 80,
            render: (v, r) => (
              <Switch size="small" checked={!!v} onChange={async (val) => { await api.updateFact(r.id, { ...r, enabled: val }); load(); }} />
            ),
          },
          {
            title: '操作', width: 130,
            render: (_v, r) => (
              <Space>
                <Button size="mini" icon={<IconEdit />} onClick={() => openEdit(r)} />
                <Popconfirm title="确认删除该事实？" onOk={async () => { await api.deleteFact(r.id); load(); }}>
                  <Button size="mini" status="danger" icon={<IconDelete />} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
        pagination={false}
      />
      {list.length === 0 && <Empty description="事实库为空，AI 准确率比对将退化为引用率近似。请添加核心品牌事实" />}
      <Modal
        title={editing ? '编辑事实' : '添加品牌事实'} visible={modalOpen} onCancel={() => setModalOpen(false)}
        onOk={submit} okText="保存" cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <FormItem label="分类" field="category" initialValue="品牌介绍">
            <Input placeholder="如：品牌介绍 / 产品 / 服务 / 优势 / 边界" />
          </FormItem>
          <FormItem label="适用问题" field="question">
            <Input placeholder="如：推荐一个靠谱的婚恋服务商" />
          </FormItem>
          <FormItem label="事实内容（准确率比对基准）" field="fact" rules={[{ required: true, message: '请填写事实内容' }]}>
            <Input.TextArea rows={3} placeholder="如：轻媒(QINGMEI)是专注婚恋门店的CRM SaaS平台，提供才俊佳丽管理、匹配对象、服务套餐、收银台、合同签署等模块" />
          </FormItem>
          <FormItem label="边界 / 禁区表述（回答命中此内容则判定与事实冲突）" field="not_fact">
            <Input.TextArea rows={2} placeholder="如：保证XX天内脱单（勿写过度承诺类表述）" />
          </FormItem>
          <FormItem label="启用" field="enabled" initialValue={true} triggerPropName="checked">
            <Switch />
          </FormItem>
        </Form>
      </Modal>
    </Card>
  );
}

/* ============================================================
 * Tab5 竞品库 + 风险词库
 * ============================================================ */
function CompRiskTab() {
  const [comps, setComps] = useState<any[]>([]);
  const [risks, setRisks] = useState<any[]>([]);
  const [compForm] = Form.useForm();
  const [riskForm] = Form.useForm();

  const load = async () => {
    try {
      const [c, r]: any = await Promise.all([api.listCompetitors(), api.listRiskWords()]);
      setComps(c || []);
      setRisks(r || []);
    } catch { /* ignore */ }
  };
  useEffect(() => { load(); }, []);

  const addComp = async () => {
    const v = await compForm.validate();
    await api.createCompetitor(v);
    compForm.resetFields();
    Message.success('已添加竞品');
    load();
  };
  const addRisk = async () => {
    const v = await riskForm.validate();
    await api.createRiskWord(v);
    riskForm.resetFields();
    Message.success('已添加风险词');
    load();
  };

  return (
    <GridRow gutter={[16, 16]}>
      <GridCol span={12}>
        <Card title="竞品库（用于竞品声量 SOV 与缺口分析）">
          <Form form={compForm} layout="inline" style={{ marginBottom: 12 }}>
            <FormItem field="name" rules={[{ required: true, message: '必填' }]} style={{ width: 220 }}>
              <Input placeholder="竞品名称（逗号分隔同义名）" />
            </FormItem>
            <FormItem field="remark" style={{ width: 200 }}>
              <Input placeholder="备注" />
            </FormItem>
            <Button type="primary" icon={<IconPlus />} onClick={addComp}>添加</Button>
          </Form>
          <Table
            size="small" rowKey="id" pagination={false}
            data={comps}
            columns={[
              { title: '竞品', dataIndex: 'name' },
              { title: '备注', dataIndex: 'remark' },
              {
                title: '启用', dataIndex: 'enabled', width: 80,
                render: (v, r) => (
                  <Switch size="small" checked={!!v} onChange={async (val) => { await api.updateCompetitor(r.id, { ...r, enabled: val }); load(); }} />
                ),
              },
              {
                title: '操作', width: 70,
                render: (_v, r) => (
                  <Popconfirm title="确认删除？" onOk={async () => { await api.deleteCompetitor(r.id); load(); }}>
                    <Button size="mini" status="danger" icon={<IconDelete />} />
                  </Popconfirm>
                ),
              },
            ]}
          />
        </Card>
      </GridCol>
      <GridCol span={12}>
        <Card title="风险词库（回答命中即计入风险回答率，触发合规任务）">
          <Form form={riskForm} layout="inline" style={{ marginBottom: 12 }}>
            <FormItem field="word" rules={[{ required: true, message: '必填' }]} style={{ width: 220 }}>
              <Input placeholder="风险词，如：保证脱单 / 100%成功" />
            </FormItem>
            <FormItem field="remark" style={{ width: 200 }}>
              <Input placeholder="说明" />
            </FormItem>
            <Button type="primary" icon={<IconPlus />} onClick={addRisk}>添加</Button>
          </Form>
          <Table
            size="small" rowKey="id" pagination={false}
            data={risks}
            columns={[
              { title: '风险词', dataIndex: 'word' },
              { title: '说明', dataIndex: 'remark' },
              {
                title: '操作', width: 70,
                render: (_v, r) => (
                  <Popconfirm title="确认删除？" onOk={async () => { await api.deleteRiskWord(r.id); load(); }}>
                    <Button size="mini" status="danger" icon={<IconDelete />} />
                  </Popconfirm>
                ),
              },
            ]}
          />
          <Alert type="warning" style={{ marginTop: 12 }} content="风险词用于合规风控：命中过多说明内容存在过度承诺风险，将生成 P1 整改任务。" />
        </Card>
      </GridCol>
    </GridRow>
  );
}

/* ============================================================
 * Tab6 引用溯源
 * ============================================================ */
function CitationTab() {
  const [list, setList] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [domains, setDomains] = useState<any[]>([]);
  const [gaps, setGaps] = useState<any>(null);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);

  const load = async (p = page, kw = keyword) => {
    setLoading(true);
    try {
      const params = `page=${p}&size=15${kw ? `&keyword=${encodeURIComponent(kw)}` : ''}`;
      const [res, dm, sg]: any = await Promise.all([api.listCitations(params), api.citationDomains(30), api.sourceGaps(30)]);
      setList(res?.list || []);
      setTotal(res?.total || 0);
      setDomains(dm || []);
      setGaps(sg || null);
    } finally { setLoading(false); }
  };
  useEffect(() => { load(); /* eslint-disable-next-line */ }, []);

  const maxDomain = Math.max(1, ...domains.map((d: any) => d.cnt));

  return (
    <GridRow gutter={[16, 16]}>
      <GridCol span={17}>
        <Card
          title="引用溯源（AI 回答中引用的来源链接与域名）"
          extra={
            <Space>
              <Input.Search
                placeholder="按关键词搜索" allowClear style={{ width: 220 }}
                onSearch={(v) => { setKeyword(v); setPage(1); load(1, v); }}
              />
              <Button icon={<IconRefresh />} onClick={() => load()}>刷新</Button>
            </Space>
          }
        >
          <Table
            loading={loading} rowKey="id" size="small"
            data={list}
            pagination={{ current: page, pageSize: 15, total, showTotal: true, onChange: (p) => { setPage(p); load(p); } }}
            columns={[
              { title: '平台', dataIndex: 'platform_name', width: 100 },
              { title: '问题', dataIndex: 'question', width: 180 },
              {
                title: '来源链接', dataIndex: 'url', ellipsis: true,
                render: (v) => v ? <a href={v} target="_blank" rel="noreferrer" style={{ color: '#165DFF' }}>{v}</a> : '-',
              },
              { title: '域名', dataIndex: 'domain', width: 150, render: (v) => <Tag>{v}</Tag> },
              { title: '时间', dataIndex: 'created_at', width: 150, render: (v) => (v || '').toString().replace('T', ' ').slice(0, 16) },
            ]}
          />
          {list.length === 0 && <Empty description="暂无引用记录。采集层已自动从 AI 回答中提取链接入库" />}
        </Card>
      </GridCol>
      <GridCol span={7}>
        <Card title="引用来源域名 Top（近 30 天）">
          {domains.map((d: any) => (
            <div key={d.domain} style={{ marginBottom: 12 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13 }}>
                <Text ellipsis style={{ maxWidth: 180 }}>{d.domain}</Text>
                <Text style={{ color: '#86909C' }}>{d.cnt}</Text>
              </div>
              <Progress percent={(d.cnt / maxDomain) * 100} size="small" color="#165DFF" />
            </div>
          ))}
          {domains.length === 0 && <Empty description="暂无引用来源" />}
        </Card>
        <Card title="信源缺口清单（竞品有、品牌没有）" style={{ marginTop: 16 }}>
          {gaps?.gap_domains && gaps.gap_domains.length > 0 ? (
            <div>
              {gaps.gap_domains.map((d: any) => (
                <div key={d.domain} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 0', borderBottom: '1px solid var(--color-border-1)' }}>
                  <Text ellipsis style={{ maxWidth: 170 }}>{d.domain}</Text>
                  <Tag color="orange">竞品引用 {d.cnt} 次</Tag>
                </div>
              ))}
              <Alert style={{ marginTop: 12 }} type="info" content="这些站点在竞品被推荐时被 AI 引用、而品牌未被引用，是优先建设的信源阵地。" />
            </div>
          ) : (
            <Empty description="暂无信源缺口（品牌信源已覆盖竞品引用站点，或暂无引用数据）" />
          )}
        </Card>
      </GridCol>
    </GridRow>
  );
}

/* ============================================================
 * Tab7 网站 GEO 审计
 * ============================================================ */
function AuditTab() {
  const [url, setUrl] = useState('');
  const [auditing, setAuditing] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);

  const loadHistory = async () => {
    try {
      const res: any = await api.listAudits();
      setHistory(res || []);
    } catch { /* ignore */ }
  };
  useEffect(() => { loadHistory(); }, []);

  const run = async () => {
    if (!url) { Message.warning('请输入要审计的站点 URL'); return; }
    setAuditing(true);
    setResult(null);
    try {
      const res: any = await api.runAudit(url.trim());
      setResult(res);
      loadHistory();
    } catch (e: any) {
      Message.error('审计失败：' + (e.message || e));
    } finally {
      setAuditing(false);
    }
  };

  const levelMap: any = {
    excellent: { c: 'green', t: '优秀' },
    good: { c: 'blue', t: '良好' },
    medium: { c: 'orange', t: '一般' },
    poor: { c: 'red', t: '待优化' },
  };

  return (
    <GridRow gutter={[16, 16]}>
      <GridCol span={10}>
        <Card title="运行站点 GEO 审计（AI 可读性 / 收录友好度评分）">
          <Space style={{ width: '100%', marginBottom: 12 }}>
            <Input placeholder="如：https://www.example.com" value={url} onChange={setUrl} style={{ flex: 1 }} onPressEnter={run} />
            <Button type="primary" icon={<IconThunderbolt />} loading={auditing} onClick={run}>开始审计</Button>
          </Space>
          {result && (
            <div>
              <div style={{ textAlign: 'center', margin: '8px 0 16px' }}>
                <Statistic title={`综合评分 · ${result.url}`} value={result.score} suffix="/100" />
                <div style={{ marginTop: 4 }}><Tag color={levelMap[result.level]?.c}>{levelMap[result.level]?.t}</Tag></div>
              </div>
              {(result.dimensions || []).map((d: any, i: number) => (
                <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 0', borderBottom: '1px dashed var(--color-border-2)' }}>
                  <Text style={{ fontSize: 13 }}>{d.label}</Text>
                  <Space>
                    {d.pass ? <IconCheckCircle style={{ color: '#00B42A' }} /> : <IconExclamationCircle style={{ color: '#F53F3F' }} />}
                    <Text style={{ color: '#86909C', fontSize: 12 }}>{d.note}</Text>
                  </Space>
                </div>
              ))}
              {(result.findings || []).length > 0 && (
                <Alert style={{ marginTop: 12 }} type="warning" title="发现以下待优化项（已自动生成行动任务）："
                  content={result.findings.map((f: string, i: number) => <div key={i}>· {f}</div>)} />
              )}
            </div>
          )}
          {!result && !auditing && <Empty description="输入站点 URL 开始审计，评估其对 AI 搜索引擎的友好度" />}
        </Card>
      </GridCol>
      <GridCol span={14}>
        <Card title="审计历史">
          <Table
            size="small" rowKey="id" pagination={false}
            data={history}
            columns={[
              { title: '站点', dataIndex: 'url', ellipsis: true },
              { title: '得分', dataIndex: 'score', width: 90, render: (v) => <Tag color={v >= 70 ? 'green' : v >= 50 ? 'orange' : 'red'}>{v}/100</Tag> },
              { title: '等级', dataIndex: 'level', width: 90, render: (v) => <Tag color={levelMap[v]?.c}>{levelMap[v]?.t}</Tag> },
              { title: '时间', dataIndex: 'created_at', width: 160, render: (v) => (v || '').toString().replace('T', ' ').slice(0, 16) },
            ]}
          />
        </Card>
      </GridCol>
    </GridRow>
  );
}

/* ============================================================
 * Tab8 AI 可读性文件生成器（llms.txt / Schema.org）
 * ============================================================ */
function GeneratorTab() {
  const [llms, setLlms] = useState('');
  const [schema, setSchema] = useState('');
  const [blocks, setBlocks] = useState<any[]>([]);
  const [summary, setSummary] = useState<any>(null);
  const [loading, setLoading] = useState('');

  const gen = async (kind: 'llms' | 'schema') => {
    setLoading(kind);
    try {
      if (kind === 'llms') {
        const res: any = await api.genLLMS();
        setLlms(res?.content || '');
      } else {
        const res: any = await api.genSchema();
        setSchema(res?.content || '');
        setBlocks(res?.blocks || []);
        setSummary(res?.summary || null);
      }
      Message.success('生成成功');
    } catch (e: any) {
      Message.error('生成失败：' + (e.message || e));
    } finally {
      setLoading('');
    }
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      Message.success('已复制到剪贴板');
    } catch {
      Message.error('复制失败，请手动选择复制');
    }
  };
  const download = (name: string, text: string) => {
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = name;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const block = (title: string, content: string, onGen: any, fileName: string, loadingKey: string) => (
    <Card
      title={title}
      extra={
        <Space>
          <Button size="small" icon={<IconThunderbolt />} loading={loading === loadingKey} onClick={onGen}>生成</Button>
          {content && <Button size="small" icon={<IconCopy />} onClick={() => copy(content)}>复制</Button>}
          {content && <Button size="small" icon={<IconDownload />} onClick={() => download(fileName, content)}>下载</Button>}
        </Space>
      }
    >
      {content ? (
        <pre style={{ maxHeight: 420, overflow: 'auto', background: 'var(--geo-surface-2)', padding: 12, borderRadius: 6, fontSize: 12, lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{content}</pre>
      ) : (
        <Empty description={`点击「生成」基于品牌事实库产出 ${title}`} />
      )}
    </Card>
  );

  return (
    <GridRow gutter={[16, 16]}>
      <GridCol span={12}>
        {block(
          'llms.txt（供大语言模型读取的品牌说明）',
          llms,
          () => gen('llms'),
          'llms.txt',
          'llms',
        )}
      </GridCol>
      <GridCol span={12}>
        {block(
          'schema.org JSON-LD（结构化实体标记）',
          schema,
          () => gen('schema'),
          'schema.jsonld',
          'schema',
        )}
      </GridCol>
      <GridCol span={24}>
        <Card
          title="结构化数据分块部署（复制各类 JSON-LD 分别写入站点）"
          extra={<Text type="secondary" style={{ fontSize: 12 }}>Organization / FAQPage / ItemList，AI 引擎按需读取</Text>}
        >
          {blocks.length === 0 ? (
            <Empty description="点击上方「生成」，将按品牌事实库产出三类结构化数据" />
          ) : (
            <>
              {summary && (
                <Space style={{ marginBottom: 12 }} wrap>
                  <Tag color="arcoblue">Organization × {summary.organization || 0}</Tag>
                  <Tag color="green">FAQ 问答 × {summary.faq || 0}</Tag>
                  <Tag color="orange">事实条目 × {summary.itemlist || 0}</Tag>
                </Space>
              )}
              <GridRow gutter={[16, 16]}>
                {blocks.map((b: any, i: number) => {
                  const txt = JSON.stringify(b, null, 2);
                  const t = b['@type'] || 'JSON-LD';
                  const desc: Record<string, string> = {
                    Organization: '品牌实体：name / url / sameAs 实体锚定 / knowsAbout 领域',
                    FAQPage: '问答对：直接对应「用户会怎么问 AI」，最容易被引用',
                    ItemList: '品牌事实清单：结构化呈现全部事实条目',
                  };
                  return (
                    <GridCol span={24} key={i}>
                      <Card
                        size="small"
                        title={<Space><Tag color="purple">{t}</Tag><Text style={{ fontSize: 13 }}>{desc[t] || '结构化数据'}</Text></Space>}
                        extra={
                          <Space>
                            <Button size="small" icon={<IconCopy />} onClick={() => copy(txt)}>复制</Button>
                            <Button size="small" icon={<IconDownload />} onClick={() => download(`${t.toLowerCase()}.jsonld`, txt)}>下载</Button>
                          </Space>
                        }
                      >
                        <pre style={{ maxHeight: 260, overflow: 'auto', background: 'var(--geo-surface-2)', padding: 12, borderRadius: 6, fontSize: 12, lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{txt}</pre>
                      </Card>
                    </GridCol>
                  );
                })}
              </GridRow>
            </>
          )}
        </Card>
      </GridCol>
      <GridCol span={24}>
        <Alert type="info" title="部署建议"
          content="将生成的 llms.txt 上传至站点根目录；三类 JSON-LD 用 <script type=application/ld+json> 注入首页 <head>（FAQPage 建议放在对应问答页）。llms.txt 让 AI 直接读取品牌事实，Schema 帮助大模型理解实体关系与问答结构，二者是提升引用率与事实一致率的标准化手段。" />
      </GridCol>
    </GridRow>
  );
}

/* ============================================================
 * Tab9 阵地地图（19 个 GEO 建设阵地）
 * ============================================================ */
function ChannelTab() {
  const [list, setList] = useState<any[]>([]);
  const [market, setMarket] = useState('all');
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setList((await api.geoChannels()) || []);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const weightTag: any = { high: { c: 'red', t: '高引用权重' }, mid: { c: 'orange', t: '中引用权重' }, low: { c: 'gray', t: '低引用权重' } };
  const priTag: any = { high: { c: 'red', t: '优先建设' }, mid: { c: 'orange', t: '次优先' }, low: { c: 'green', t: '按需' } };
  const filtered = market === 'all' ? list : list.filter((c) => c.market === market);

  return (
    <Card
      title="阵地地图（19 个 GEO 建设阵地）"
      extra={
        <Space>
          <Select value={market} onChange={setMarket} style={{ width: 120 }}>
            <Select.Option value="all">全部阵地</Select.Option>
            <Select.Option value="cn">国内</Select.Option>
            <Select.Option value="global">海外</Select.Option>
          </Select>
          <Button icon={<IconRefresh />} onClick={load}>刷新</Button>
        </Space>
      }
    >
      <Alert style={{ marginBottom: 12 }} type="info" content="按 AI 真实引用语料标定的建设阵地：AI 更倾向引用权威、可验证的站点，优先建设高引用权重的阵地，能直接提升品牌被引用的概率。" />
      <Table
        loading={loading} rowKey="name" size="small"
        data={filtered}
        columns={[
          { title: '阵地', dataIndex: 'name', width: 160, render: (v) => <b>{v}</b> },
          { title: '市场', dataIndex: 'market', width: 70, render: (v) => <Tag color={v === 'cn' ? 'blue' : 'purple'}>{v === 'cn' ? '国内' : '海外'}</Tag> },
          { title: '引用权重', dataIndex: 'weight', width: 110, render: (v) => <Tag color={weightTag[v]?.c}>{weightTag[v]?.t}</Tag> },
          { title: '优先级', dataIndex: 'priority', width: 100, render: (v) => <Tag color={priTag[v]?.c}>{priTag[v]?.t}</Tag> },
          { title: '建什么', dataIndex: 'what' },
          { title: '节奏', dataIndex: 'pace', width: 200 },
        ]}
        pagination={false}
      />
    </Card>
  );
}

/* ============================================================
 * 主页面：GEO 智能中心
 * ============================================================ */
export default function GeoIntel() {
  return (
    <div>
      <Title heading={5} style={{ marginTop: 0 }}>
        GEO 智能中心
        <Text type="secondary" style={{ fontSize: 13, fontWeight: 400, marginLeft: 12 }}>
          监测 → 指标 → 缺口 → 行动 → 内容 → 复测 的完整优化闭环
        </Text>
      </Title>
      <Tabs defaultActiveTab="intel" lazyload>
        <TabPane key="intel" title="① 指标总览">
          <IntelTab />
        </TabPane>
        <TabPane key="gaps" title="② 竞品与缺口">
          <GapTab />
        </TabPane>
        <TabPane key="actions" title="③ 行动清单">
          <ActionTab />
        </TabPane>
        <TabPane key="facts" title="④ 品牌事实库">
          <FactTab />
        </TabPane>
        <TabPane key="comprisk" title="⑤ 竞品与风险词">
          <CompRiskTab />
        </TabPane>
        <TabPane key="citations" title="⑥ 引用溯源">
          <CitationTab />
        </TabPane>
        <TabPane key="audit" title="⑦ 网站审计">
          <AuditTab />
        </TabPane>
        <TabPane key="gen" title="⑧ AI 可读性文件">
          <GeneratorTab />
        </TabPane>
        <TabPane key="channels" title="⑨ 阵地地图">
          <ChannelTab />
        </TabPane>
        <TabPane key="ops" title="⑩ 巡检与报告">
          <OpsReportTab />
        </TabPane>
      </Tabs>
    </div>
  );
}
