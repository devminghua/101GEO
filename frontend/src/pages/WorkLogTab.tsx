import { useEffect, useMemo, useState, type CSSProperties } from 'react';
import {
  Card, Button, Message, Tag, Space, Typography, Empty, Spin, Select, Grid, Alert, Timeline,
} from '@arco-design/web-react';
import {
  IconRefresh, IconThunderbolt, IconSafe, IconFile, IconClockCircle, IconCheckCircle,
  IconLink, IconSync,
} from '@arco-design/web-react/icon';
import { api } from '../api';

const { Text } = Typography;
const { Row: GridRow, Col: GridCol } = Grid;

const cardStyle: CSSProperties = {
  borderRadius: 16,
  boxShadow: '0 2px 14px rgba(0, 0, 0, 0.05)',
};

type WorkLogEvent = {
  time: string; date: string; kind: string; icon: string; auto: boolean;
  title: string; detail: string; cost: number; status: 'success' | 'partial' | 'failed';
};

type WorkLogPace = {
  auto_enabled: boolean; interval_min: number; window_text: string;
  last_auto_at: string; next_auto_at: string; today_count: number; week_count: number;
  running: boolean; platforms: number; keywords: number; points: number;
  ready: boolean; ready_hint: string;
};

// 事件类型 → 展示元数据（图标 / 配色 / 中文分类）
// 配色遵循后台统一语义：绿=完成、橙=部分/进行中、红=失败。
const KIND_META: Record<string, { icon: React.ReactNode; label: string; color: string }> = {
  patrol: { icon: <IconThunderbolt />, label: '自动巡检', color: '#165DFF' },
  action: { icon: <IconCheckCircle />, label: '优化行动', color: '#00B42A' },
  verify: { icon: <IconSync />, label: '效果复测', color: '#722ED1' },
  content: { icon: <IconFile />, label: '内容生成', color: '#FF7D00' },
  audit: { icon: <IconSafe />, label: '网站体检', color: '#0FC6C2' },
  cluster: { icon: <IconThunderbolt />, label: '话题聚类', color: '#F5319D' },
  citation: { icon: <IconLink />, label: '引用采集', color: '#86909C' },
};

const STATUS_META: Record<string, { color: string; text: string }> = {
  success: { color: '#00B42A', text: '已完成' },
  partial: { color: '#FF7D00', text: '部分完成' },
  failed: { color: '#F53F3F', text: '未完成' },
};

/** 相对时间：客户更关心「多久之前」，而不是精确到分的时间戳 */
function relTime(t: string): string {
  const d = new Date(t.replace(' ', 'T'));
  if (isNaN(d.getTime())) return '';
  const diff = Date.now() - d.getTime();
  const min = Math.floor(diff / 60000);
  if (min < 1) return '刚刚';
  if (min < 60) return `${min} 分钟前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} 小时前`;
  const day = Math.floor(h / 24);
  if (day < 30) return `${day} 天前`;
  return '';
}

/** 日期分组标题：今天 / 昨天 / 具体日期 */
function dayLabel(date: string): string {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
  if (date === today) return '今天';
  const y = new Date(now.getTime() - 86400000);
  const yest = `${y.getFullYear()}-${pad(y.getMonth() + 1)}-${pad(y.getDate())}`;
  if (date === yest) return '昨天';
  const [, m, d] = date.split('-');
  return `${Number(m)} 月 ${Number(d)} 日`;
}

export default function WorkLogTab() {
  const [days, setDays] = useState(7);
  const [loading, setLoading] = useState(true);
  const [events, setEvents] = useState<WorkLogEvent[]>([]);
  const [pace, setPace] = useState<WorkLogPace | null>(null);
  const [kindFilter, setKindFilter] = useState<string>('all');

  const load = async (d = days) => {
    setLoading(true);
    try {
      const r: any = await api.geoWorkLog(d);
      setEvents(r?.events || []);
      setPace(r?.pace || null);
    } catch (e: any) {
      Message.error(e?.message || '工作日志加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(days); /* eslint-disable-next-line */ }, [days]);

  const filtered = useMemo(
    () => (kindFilter === 'all' ? events : events.filter((e) => e.kind === kindFilter)),
    [events, kindFilter],
  );

  // 按天分组（后端已倒序，这里保持顺序聚合）
  const groups = useMemo(() => {
    const out: { date: string; items: WorkLogEvent[] }[] = [];
    for (const ev of filtered) {
      const last = out[out.length - 1];
      if (last && last.date === ev.date) last.items.push(ev);
      else out.push({ date: ev.date, items: [ev] });
    }
    return out;
  }, [filtered]);

  // 出现过的类型 → 供筛选下拉（只列实际有数据的，避免选了空）
  const kindOptions = useMemo(() => {
    const seen = new Set(events.map((e) => e.kind));
    return [
      { label: '全部工作', value: 'all' },
      ...['patrol', 'verify', 'action', 'content', 'audit', 'cluster', 'citation']
        .filter((k) => seen.has(k))
        .map((k) => ({ label: KIND_META[k]?.label || k, value: k })),
    ];
  }, [events]);

  return (
    <div>
      {/* ── 工作节奏：一眼看清「系统什么时候干活」 ── */}
      <Card
        style={{ ...cardStyle, marginBottom: 16 }}
        bordered={false}
        title={
          <Space>
            <IconClockCircle style={{ color: '#165DFF' }} />
            系统工作节奏
          </Space>
        }
        extra={
          <Space>
            <Select
              size="small"
              value={kindFilter}
              onChange={setKindFilter}
              style={{ width: 132 }}
              options={kindOptions}
            />
            <Select
              size="small"
              value={days}
              onChange={setDays}
              style={{ width: 108 }}
              options={[1, 7, 14, 30].map((d) => ({ label: `近 ${d} 天`, value: d }))}
            />
            <Button size="small" icon={<IconRefresh />} loading={loading} onClick={() => load(days)}>
              刷新
            </Button>
          </Space>
        }
      >
        <Spin loading={loading && !pace} style={{ width: '100%' }}>
          {pace && (
            <>
              <GridRow gutter={[16, 16]}>
                <GridCol xs={24} sm={12} lg={6}>
                  <StatBox
                    label="自动巡检"
                    value={pace.auto_enabled ? '已开启' : '已关闭'}
                    color={pace.auto_enabled ? '#00B42A' : '#86909C'}
                    hint={pace.auto_enabled ? `每 ${pace.interval_min} 分钟一轮 · ${pace.window_text}` : '未开启定时巡检'}
                  />
                </GridCol>
                <GridCol xs={24} sm={12} lg={6}>
                  <StatBox
                    label="上次自动巡检"
                    value={pace.last_auto_at ? relTime(pace.last_auto_at) || pace.last_auto_at : '尚未开始'}
                    color={pace.last_auto_at ? '#165DFF' : '#86909C'}
                    hint={pace.last_auto_at ? pace.last_auto_at : '等待首轮自动巡检'}
                  />
                </GridCol>
                <GridCol xs={24} sm={12} lg={6}>
                  <StatBox
                    label="下次预计巡检"
                    value={pace.running ? '正在巡检中…' : pace.next_auto_at || '—'}
                    color={pace.running ? '#FF7D00' : '#722ED1'}
                    hint={pace.auto_enabled ? '系统会自动执行，无需开着浏览器' : '自动巡检未开启'}
                  />
                </GridCol>
                <GridCol xs={24} sm={12} lg={6}>
                  <StatBox
                    label="工作产出"
                    value={`今天 ${pace.today_count} 项`}
                    color="#0FC6C2"
                    hint={`近 7 天共 ${pace.week_count} 项`}
                  />
                </GridCol>
              </GridRow>

              {/* 能力不满足时必须说清楚「为什么不动」，否则客户会以为系统坏了 */}
              {pace.auto_enabled && !pace.ready && pace.ready_hint && (
                <Alert
                  style={{ marginTop: 16 }}
                  type="warning"
                  content={pace.ready_hint}
                />
              )}
              {pace.auto_enabled && pace.ready && pace.points >= 0 && pace.points <= 5 && (
                <Alert
                  style={{ marginTop: 16 }}
                  type="warning"
                  content={`点卡余额仅剩 ${pace.points} 点，每次 AI 提问扣 1 点，余额不足会导致巡检中断，请及时充值。`}
                />
              )}

              <div style={{ marginTop: 14, color: 'var(--color-text-3)', fontSize: 12, lineHeight: 1.8 }}>
                系统当前可用 AI 平台 <b>{pace.platforms}</b> 个、启用关键词 <b>{pace.keywords}</b> 个
                {pace.points >= 0 && <>、点卡余额 <b>{pace.points}</b> 点</>}。
                自动巡检由服务器定时执行，<b>关闭浏览器也会照常工作</b>；内容生成、网站体检等需要您在页面点击触发。
              </div>
            </>
          )}
        </Spin>
      </Card>

      {/* ── 工作日志时间线 ── */}
      <Card
        style={cardStyle}
        bordered={false}
        title={
          <Space>
            <IconThunderbolt style={{ color: '#165DFF' }} />
            工作日志
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
              软件什么时候干了什么，一目了然
            </Text>
          </Space>
        }
      >
        <Spin loading={loading} style={{ width: '100%' }}>
          {groups.length === 0 && !loading ? (
            <Empty
              description={
                events.length === 0
                  ? '这段时间还没有工作记录。开启自动巡检后，系统会按时自动执行并记录在这里。'
                  : '当前筛选类型下暂无记录'
              }
            />
          ) : (
            groups.map((g) => (
              <div key={g.date} style={{ marginBottom: 22 }}>
                <div
                  style={{
                    display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12,
                    fontWeight: 600, fontSize: 14,
                  }}
                >
                  <span
                    style={{
                      width: 8, height: 8, borderRadius: 4,
                      background: '#165DFF', display: 'inline-block',
                    }}
                  />
                  {dayLabel(g.date)}
                  <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
                    {g.items.length} 项工作
                  </Text>
                </div>

                <Timeline>
                  {g.items.map((ev, i) => {
                    const km = KIND_META[ev.kind] || { icon: <IconFile />, label: ev.kind, color: '#86909C' };
                    const sm = STATUS_META[ev.status] || STATUS_META.success;
                    return (
                      <Timeline.Item
                        key={`${g.date}-${i}-${ev.time}`}
                        dotColor={sm.color}
                        label={
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
                            {ev.time.slice(11)}
                          </span>
                        }
                      >
                        <div style={{ paddingBottom: 4 }}>
                          <Space size={8} wrap style={{ marginBottom: 4 }}>
                            <Tag
                              size="small"
                              style={{
                                color: km.color,
                                background: `${km.color}14`,
                                border: 'none',
                                borderRadius: 6,
                              }}
                            >
                              {km.label}
                            </Tag>
                            {ev.auto && (
                              <Tag size="small" style={{ color: '#165DFF', background: '#165DFF14', border: 'none' }}>
                                系统自动
                              </Tag>
                            )}
                            <span style={{ fontWeight: 600, fontSize: 13 }}>{ev.title}</span>
                          </Space>
                          <div
                            style={{
                              fontSize: 12.5, color: 'var(--color-text-2)',
                              lineHeight: 1.7, wordBreak: 'break-word',
                            }}
                          >
                            {ev.detail}
                          </div>
                          <div style={{ marginTop: 4, fontSize: 12, color: 'var(--color-text-3)' }}>
                            <IconCheckCircle
                              style={{ color: sm.color, marginRight: 4, verticalAlign: -3 }}
                            />
                            {sm.text}
                            {ev.cost > 0 && <span> · 消耗 {ev.cost} 点卡</span>}
                          </div>
                        </div>
                      </Timeline.Item>
                    );
                  })}
                </Timeline>
              </div>
            ))
          )}
        </Spin>
      </Card>
    </div>
  );
}

/** 节奏卡内的指标块：大字号数值 + 灰色说明，保证一眼可读 */
function StatBox({ label, value, hint, color }: { label: string; value: string; hint: string; color: string }) {
  return (
    <div
      style={{
        background: 'var(--color-fill-1)',
        borderRadius: 12,
        padding: '14px 16px',
        height: '100%',
      }}
    >
      <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color, lineHeight: 1.3, wordBreak: 'break-word' }}>
        {value}
      </div>
      <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 6, lineHeight: 1.6 }}>
        {hint}
      </div>
    </div>
  );
}
