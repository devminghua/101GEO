import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api';
import {
  Card, Grid, Button, Message, Tag, Space, Typography, Empty, Input, Select, Tabs, Spin, Progress, Tooltip, Switch, Alert, Form, Modal, Popconfirm,
} from '@arco-design/web-react';
import { IconSearch, IconPlayArrow, IconRefresh, IconBulb, IconThunderbolt, IconExpand, IconUp, IconPlus, IconDelete, IconEdit, IconCheck, IconArrowRise, IconUserGroup, IconFile } from '@arco-design/web-react/icon';
import EChart from '../components/EChart';
import QuotaBadge, { notifyQuotaChanged } from '../components/QuotaBadge';

const { Row: GridRow, Col: GridCol } = Grid;
const { Title, Text } = Typography;
const TabPane = Tabs.TabPane;

/* ================================================================
 * 百度分析页 · 接入后端实时抓取 SERP，后端不可用时回退本地 mock 演示
 * 模块：
 *  0. 「我的网站 / 监控关键词」配置面板（按租户持久化到后端）
 *  1. 搜索抓取区（关键词下拉 + 深度 + 自动携带已配置域名 my_domains）
 *  2. 竞争指标卡
 *  3. 近 30 天排名趋势折线（echarts）
 *  4. 前十页分布矩阵（点击行展开 10 条，标注 同行归属 / 广告·自然 / 标题 / URL）
 *  5. 同行排行对比表（客户自己的网站高亮「我方」）
 *  6. 优化建议面板（P0/P1/P2 + SEO 优化建议）
 * 关键点：广告与自然排名分开识别；多关键词对比视图；后端不可用时自动回退 mock
 * ================================================================ */

// ---------- 品牌调色（沿用报告页） ----------
const KPI_STYLE = [
  { color: '#165DFF', grad: 'var(--geo-grad-blue)' },
  { color: '#722ED1', grad: 'var(--geo-grad-purple)' },
  { color: '#00B42A', grad: 'var(--geo-grad-green)' },
  { color: '#FF7D00', grad: 'var(--geo-grad-orange)' },
  { color: '#F53F3F', grad: 'var(--geo-grad-red)' },
  { color: '#14C9C9', grad: 'var(--geo-grad-cyan)' },
];
const DOMAIN_COLORS = ['#165DFF', '#722ED1', '#00B42A', '#F53F3F', '#FF7D00', '#14C9C9', '#F5319D', '#3491FA', '#F7BA1E', '#D91AD9'];

// 数字滚动动画（复用报告页实现）
function CountUpNumber({ value, duration = 650, color }: { value: number; duration?: number; color?: string }) {
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

function AnimStyle() {
  return (
    <style>{`
      @keyframes geoKbFadeUp { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: translateY(0); } }
      .geo-kb-reveal { animation: geoKbFadeUp .45s ease both; }
      @keyframes geoKbBar { 0% { background-position: 0 0; } 100% { background-position: 40px 0; } }
    `}</style>
  );
}

// 域名徽标（首字母圆标 + 颜色）
function DomainBadge({ name, text, size = 30 }: { name: string; text: string; size?: number }) {
  const h = Math.abs(name.split('').reduce((s, c) => s + c.charCodeAt(0), 0));
  const c = DOMAIN_COLORS[h % DOMAIN_COLORS.length];
  return (
    <span style={{
      width: size, height: size, borderRadius: 9, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
      fontWeight: 700, fontSize: Math.max(11, size * 0.4), color: '#fff', flexShrink: 0,
      background: `linear-gradient(135deg, ${c}, ${c}99)`, boxShadow: `0 4px 10px ${c}33`,
    }}>{text.slice(0, 1).toUpperCase()}</span>
  );
}

/* ================================================================
 * Mock 数据生成（确定性伪随机，保证每次一致）
 * ================================================================ */
function lcg(seed: number) {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) % 4294967296;
    return s / 4294967296;
  };
}
function pick<T>(rnd: () => number, arr: T[]): T { return arr[Math.floor(rnd() * arr.length)]; }

// 同行域名库（手工维护 + 自动聚类结果）：婚恋行业示例
const PEER_LIB: { d: string; name: string }[] = [
  { d: 'jiayuan.com', name: '世纪佳缘' },
  { d: 'baihe.com', name: '百合网' },
  { d: 'zhenai.com', name: '珍爱网' },
  { d: 'marryu.cn', name: 'MarryU 客户端' },
  { d: 'wozhuliangyuan.com', name: '我主良缘婚恋' },
  { d: 'qinglian.com', name: '青藤之恋' },
];
// 非同业强站（不计入统计，仅展示区分）
const GEN_STO = ['zhihu.com', 'xiaohongshu.com', 'sohu.com', '163.com', 'bilibili.com', 'mafengwo.cn'];

// 标题套路库（按同行域名预置，供"标题套路"归纳展示）
const PEER_PATTERNS: Record<string, { pattern: string; feats: string[] }> = {
  'jiayuan.com': { pattern: '「数字会员背书 + 疑问句 + 免费诱饵」', feats: ['会员突破X万', '疑问句式', '免费注册'] },
  'baihe.com': { pattern: '「实名认证信任 + 地域词 + 限时优惠」', feats: ['实名认证', '地域词「深圳」', '限时优惠'] },
  'zhenai.com': { pattern: '「成功率数字 + 成功案例 + 点击咨询CTA」', feats: ['成功率98%', '成功案例', '免费咨询'] },
  'marryu.cn': { pattern: '「社交App风格 + 下载引导 + 场景词」', feats: ['下载领福利', '场景词「同城」', '年轻化'] },
  'wozhuliangyuan.com': { pattern: '「品牌词 + 服务承诺 + 门店/线下」', feats: ['品牌词直出', '服务承诺', '线下门店'] },
  'qinglian.com': { pattern: '「学历/圈层人群标签 + 小程序入口」', feats: ['学历标签「硕博」', '小程序入口', '圈层词'] },
};
const GEN_PATTERN = { pattern: '「内容平台科普/攻略，无直接转化」', feats: ['攻略向', '无电话', '泛流量'] };

const INSERT_TRUST = ['实名认证平台', '民政局备案机构', '10年口碑', '红娘1对1服务', '本地直营门店', '成功率数据可查'];

interface RankItem {
  domain: string; name: string; peer: boolean; ad: boolean;
  title: string; url: string; snippet: string; rank: number; patternFeats?: string[];
}
interface PageInfo {
  page: number; ads: number; organic: number; organicPeers: number; density: number; items: RankItem[];
}
interface PeerRow {
  domain: string; name: string; count: number; pages: number[]; bestRank: number; avgRank: number;
  pattern: string; feats: string[];
}
interface AdviceRow { level: 'P0' | 'P1' | 'P2' | 'SEO'; tag: string; title: string; desc: string; basis: string }
interface MockResult {
  keyword: string; depth: number; peers: number; dominant: PeerRow | null; myBest: number; heat: number;
  pages: PageInfo[]; peerRows: PeerRow[]; advices: AdviceRow[]; trendSummary?: string;
}

// ---- 配置面板：客户网站 / 监控关键词（按租户持久化）----
interface BaiduSite {
  id: number; domain: string; name: string; enabled: boolean; tenant_id?: number;
  created_at?: string; updated_at?: string;
}
interface BaiduMonitorKeyword {
  id: number; keyword: string; site_id: number; site_domain: string; enabled: boolean;
  site_name?: string; tenant_id?: number;
  created_at?: string; updated_at?: string;
}
// ---- 排名趋势点（后端 /api/baidu/rank-history 返回）----
interface RankPoint { date: string; site_domain: string; best_rank: number; occurrences: number }
interface RankHistory { keyword: string; days: number; points: RankPoint[] }

// ---- 后端 /api/baidu/analyze 返回结构（与 services/baidu Result 对齐）----
interface ApiRankItem {
  rank: number; type: string; peer: boolean; name: string; domain: string;
  title: string; url: string; snippet: string; suspected?: boolean; suspectHints?: string;
}
interface ApiPageInfo {
  page: number; density: number; ads: number; organicPeers: number; organicOther: number;
  items: ApiRankItem[]; parseErrTag?: string;
}
interface ApiPeerRow {
  domain: string; name: string; count: number; pages: number[]; bestRank: number; avgRank: number;
  pattern: string; feats: string[]; hits?: string[];
}
interface ApiResult {
  keyword: string; depth: number; peers: number; myBest: number; heat: number;
  pages: ApiPageInfo[]; peerRows: ApiPeerRow[]; advices: AdviceRow[];
  suspicious?: { domain: string; title: string; url: string; pages: number[]; hints: string }[];
  totalItems?: number; myDomains?: string[]; costMs?: number; partialFail?: string; trendSummary?: string;
}

// 后端 Result → 前端渲染结构（ad 布尔归一、organic 补全、宽松补默认值）
function toMockResult(api: ApiResult): MockResult {
  const pages: PageInfo[] = (api.pages || []).map((pg) => ({
    page: pg.page,
    ads: pg.ads || 0,
    organic: (pg.organicPeers || 0) + (pg.organicOther || 0),
    organicPeers: pg.organicPeers || 0,
    density: typeof pg.density === 'number' ? pg.density : 0,
    items: (pg.items || []).map((it) => ({
      rank: it.rank, domain: it.domain || '-', name: it.name || it.domain || '-',
      peer: !!it.peer, ad: it.type === 'ad', title: it.title || '', url: it.url || '', snippet: it.snippet || '',
    })),
  }));
  const peerRows: PeerRow[] = (api.peerRows || []).map((r) => ({
    domain: r.domain, name: r.name || r.domain, count: r.count, pages: r.pages || [],
    bestRank: r.bestRank, avgRank: r.avgRank, pattern: r.pattern || '', feats: r.feats || [],
  }));
  return {
    keyword: api.keyword, depth: api.depth || 0, peers: api.peers || 0,
    dominant: peerRows[0] || null, myBest: api.myBest || 0, heat: api.heat || 0,
    pages, peerRows, advices: api.advices || [], trendSummary: api.trendSummary || undefined,
  };
}

function buildMock(keyword: string, depth: number, seed: number): MockResult {
  const rnd = lcg(seed);
  const pageCount = depth;
  const pages: PageInfo[] = [];
  const peerCountMap = new Map<string, number>();
  const peerBestMap = new Map<string, number>();
  const peerAvgMap = new Map<string, number[]>();
  const peerPagesMap = new Map<string, number[]>();
  const TITLES: Record<string, string[]> = {
    'jiayuan.com': ['世纪佳缘 - 深圳站_会员突破8600万,同城真实征婚', '深圳单身男女都在世纪佳缘?免费注册试试', '世纪佳缘婚恋网_实名认证_找对象首选'],
    'baihe.com': ['百合网婚恋_深圳实名认证_8天免费体验', '深圳高端婚恋服务_百合网_2000+门店资源', '百合佳缘_真实会员库_同城相亲这么简单'],
    'zhenai.com': ['珍爱网_深圳_牵手成功率98%_免费咨询', '珍爱网婚恋顾问_成功案例36000+_点击咨询', '深圳高端征婚_珍爱网_红娘1对1服务'],
    'marryu.cn': ['MarryU_同城脱单_下载领新人福利', 'MarryU客户端_高学历同城交友_免费下载', '深圳上镜交友_MarryU_年轻人的脱单墙'],
    'wozhuliangyuan.com': ['我主良缘婚恋_深圳直营门店_服务承诺', '我主良缘_高端婚恋顾问_免费情感评估', '深圳婚恋机构_我主良缘_本地10年口碑'],
    'qinglian.com': ['青藤之恋_硕博本科学历交友_免费加入', '青藤之恋_深圳高知圈层交友_小程序入口', '学历匹配交友_青藤之恋_同城实名'],
  };
  const GEN_TITLES = [
    '2025深圳婚恋市场盘点:单身人群都在哪找对象?', '小红书疯传的脱单方法,到底靠不靠谱?', '深圳相亲攻略:首次见面去哪合适?_知乎',
    '婚恋平台哪家好?业内人这样选', '深圳高端婚恋服务对比,一篇讲清楚', '单身即自由?年轻人婚恋观正在变化',
  ];
  const SNIPS = [
    '提供同城实名征婚服务,累计会员逾千万。',
    '专注高端婚恋领域,红娘一对一精准匹配。',
    '真实会员资料库,支持身份/学历/收入认证。',
    '牵手成功率行业领先,服务流程透明收费。',
    '覆盖全国多个城市,线下门店提供面谈服务。',
  ];

  for (let p = 1; p <= pageCount; p++) {
    const items: RankItem[] = [];
    // 广告位（仅前几页出现，且大多数为同行大平台投放）
    const ads = p === 1 ? 3 : p <= 3 ? 2 : p <= 5 ? 1 : 0;
    for (let a = 0; a < ads; a++) {
      const dom = pick(rnd, PEER_LIB);
      const t = pick(rnd, TITLES[dom.d]);
      items.push({
        domain: dom.d, name: dom.name, peer: true, ad: true, rank: items.length + 1,
        title: t, url: `https://www.${dom.d}/shenzhen`, snippet: pick(rnd, SNIPS), patternFeats: PEER_PATTERNS[dom.d].feats,
      });
    }
    // 自然排名：混合同行 + 非同行
    const organicPeers = p === 1 ? 5 : 4;
    for (let o = 0; o < 10 - items.length; o++) {
      if (o < organicPeers) {
        const dom = pick(rnd, PEER_LIB);
        const t = pick(rnd, TITLES[dom.d]);
        items.push({
          domain: dom.d, name: dom.name, peer: true, ad: false, rank: items.length + 1,
          title: t, url: `https://www.${dom.d}/shenzhen`, snippet: pick(rnd, SNIPS), patternFeats: PEER_PATTERNS[dom.d].feats,
        });
      } else {
        const g = pick(rnd, GEN_STO);
        const t = pick(rnd, GEN_TITLES);
        items.push({
          domain: g, name: g.split('.')[0], peer: false, ad: false, rank: items.length + 1,
          title: t, url: `https://www.${g}/shenzhen-hunlian`, snippet: pick(rnd, SNIPS), patternFeats: GEN_PATTERN.feats,
        });
      }
    }
    // 统计
    const orgPeers = items.filter((i) => i.peer && !i.ad).length;
    items.forEach((it) => {
      if (!it.peer || it.ad) return;
      peerCountMap.set(it.domain, (peerCountMap.get(it.domain) || 0) + 1);
      peerBestMap.set(it.domain, Math.min(peerBestMap.get(it.domain) ?? 99, it.rank));
      if (!peerPagesMap.has(it.domain)) peerPagesMap.set(it.domain, []);
      if (!peerPagesMap.get(it.domain)!.includes(p)) peerPagesMap.get(it.domain)!.push(p);
      if (!peerAvgMap.has(it.domain)) peerAvgMap.set(it.domain, []);
      peerAvgMap.get(it.domain)!.push(it.rank);
    });
    const density = orgPeers / 10;
    pages.push({ page: p, ads, organic: 10 - ads, organicPeers: orgPeers, density, items });
  }

  const peerRows: PeerRow[] = [...peerCountMap.entries()]
    .map(([domain, count]) => {
      const avg = peerAvgMap.get(domain)!.reduce((s, v) => s + v, 0) / peerAvgMap.get(domain)!.length;
      return {
        domain, name: PEER_LIB.find((x) => x.d === domain)!.name, count,
        pages: peerPagesMap.get(domain)!, bestRank: peerBestMap.get(domain)!, avgRank: Math.round(avg * 10) / 10,
        pattern: PEER_PATTERNS[domain].pattern, feats: PEER_PATTERNS[domain].feats,
      };
    })
    .sort((a, b) => b.count - a.count);

  const dominant = peerRows[0] || null;
  const myBest = 3 + Math.floor(rnd() * 4); // 我方最好排名（原型演示固定区间）
  const heat = 62 + Math.floor(rnd() * 30);

  // 建议生成（对比我方 vs 前排同行）
  const advices: AdviceRow[] = [
    {
      level: 'P0', tag: 'TDK 重写',
      title: `标题缺失「数字信任」要素，前排名被同行霸占`,
      desc: `前排同行（${peerRows.slice(0, 3).map((r) => r.name).join(' / ')}）标题普遍带「成功率98%」「会员8600万」「实名认证」等硬背书。我方当前标题仅${myBest === 1 ? '1次' : '数次'}出现在自然排名顶部，缺乏数字与信任词，点击率被压制。`,
      basis: '对比 TOP10 标题高频词：数字(8/10)、品牌词(9/10)、疑问句(5/10)',
    },
    {
      level: 'P0', tag: '自然排名',
      title: `自然排名每页仅出现 ${(peerRows[0]?.count || 0)} 次，需提升站群铺位密度`,
      desc: `霸屏同行「${dominant?.name || '-'}」在前 ${pageCount} 页自然结果中出现 ${dominant?.count ?? '-'} 次，覆盖多个连续页码。建议围绕同义长尾重写落地页，按「主词-长尾-地域变体」三层布局在地域页补位。`,
      basis: `出现页码：${dominant?.pages.join('、') || '-'}，最好排名第 ${dominant?.bestRank ?? '-'} 位`,
    },
    {
      level: 'P1', tag: '长尾词切入',
      title: `低竞位长尾词机会：「深圳本地高端婚恋服务 TOP3」`,
      desc: `分析发现同行在「深圳 / 高端 / 本地门店 / 1对1」等修饰词组合上覆盖密度低（平均每页仅 2 条），而该类搜索意图强、转化率高。建议以品牌名 + 地域 + 服务词组合新开 3-5 个长尾落地页。`,
      basis: '该组合词在前5页自然结果中同行密度 < 30%',
    },
    {
      level: 'P1', tag: '描述要素',
      title: `Description 缺少地域定位与行动号召（CTA）`,
      desc: `同行描述普遍含「深圳」「同城」「免费咨询/免费注册」等地域词 + 行动号召。我方描述为通用介绍，需补充地域词与低门槛 CTA，提升自然排名的点击率。`,
      basis: '地域词出现率：同行 7/10 vs 我方 0/3',
    },
    {
      level: 'P2', tag: '信任元素',
      title: `落地页信任背书不足`,
      desc: `前排同行落地页具备「实名认证、成功案例数量、线下门店、备案资质」等信任元素。建议在首屏补齐：平台认证标识、真实成功案例截图、门店/服务资质展示，形成转化闭环。`,
      basis: `同行信任元素平均 ${4} 项 vs 我方 ${1} 项`,
    },
    {
      level: 'P2', tag: '广告位观察',
      title: `广告位由头部平台垄断，自然位是差异化主战场`,
      desc: `前 3 页广告位几乎全部为头部平台投放（共 ${pages.slice(0, 3).reduce((s, p2) => s + p2.ads, 0)} 条）。资金有限的场景下建议集中优化自然排名，避免与其广告正面竞争。`,
      basis: '广告位同行占比 > 90%',
    },
    {
      level: 'SEO', tag: '落地页 SEO',
      title: `围绕「${keyword}」重构落地页 TDK 与内外链`,
      desc: `建议以目标词作为主 H1 的唯一主题，Title 控制在 30 字内并前置地域/核心词；Description 融入同行高频的「成功率/会员数/认证」信任要素。同站内围绕该词的相关长尾页互链，并向行业目录/新闻源发布 2-3 条带锚文本外链，提升整站权重与收录速度。`,
      basis: `同行在 TOP10 命中该词的页面平均外链 ${8} 条、内链 ${5} 条；我方需补足技术型 SEO 动作`,
    },
  ];

  return { keyword, depth: pageCount, peers: peerRows.filter((r) => r.count > 0).length, dominant, myBest, heat, pages, peerRows, advices };
}

/* ==================== 页面主组件 ==================== */
export default function BaiduKeywords() {
  const [rootTab, setRootTab] = useState('analyze'); // 根 Tab：analyze / config
  const [keywordInput, setKeywordInput] = useState('深圳高端婚恋交友');
  // 抓取深度固定 3（老板 2026-09-03 拍板）：深度是百度反爬命中率的主要变量，
  // 每多 1 页就多 1 次 HTTP 请求。前端已去掉「抓取页数」控件，后端 Normalize 也会强制收敛到 3。
  const [depth] = useState(3);
  const [ignoredAd, setIgnoredAd] = useState(true); // 广告位是否参与霸屏统计（默认剔除，防失真）
  const [analyzing, setAnalyzing] = useState(false);
  const [progress, setProgress] = useState({ done: 0, total: 0, peers: 0, seconds: 0 });
  const [results, setResults] = useState<Record<string, MockResult>>({}); // 多关键词对比视图
  const [fallbackNotes, setFallbackNotes] = useState<Record<string, string>>({}); // 各关键词抓取失败/受限提示
  const [activeKey, setActiveKey] = useState('');
  const [expandedPage, setExpandedPage] = useState<number | null>(1);
  const resultsRef = useRef<Record<string, MockResult>>({});
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // ---- 「我的网站 / 监控关键词」配置（后端不可用回退本地 state，不报错）----
  const [sites, setSites] = useState<BaiduSite[]>([]);
  const [monitorKeywords, setMonitorKeywords] = useState<BaiduMonitorKeyword[]>([]);
  const [configBackendOk, setConfigBackendOk] = useState<boolean | null>(null); // null=检测中
  const [trendData, setTrendData] = useState<RankHistory | null>(null);
  const [trendLoading, setTrendLoading] = useState(false);
  // 添加对比关键词弹窗
  const [compareModal, setCompareModal] = useState(false);
  const [compareKeyword, setCompareKeyword] = useState('');

  // 加载已配置网站 + 监控关键词；任一失败则保持空并标记后端不可用
  const loadConfig = useCallback(async () => {
    try {
      const [siteRes, kwRes] = await Promise.all([
        api.baiduListSites(),
        api.baiduListMonitorKeywords(),
      ]);
      const siteList: BaiduSite[] = Array.isArray(siteRes) ? siteRes : siteRes?.data || [];
      const kwList: BaiduMonitorKeyword[] = Array.isArray(kwRes) ? kwRes : kwRes?.data || [];
      const siteMap = new Map(siteList.map((s) => [s.id, s.domain]));
      setSites(siteList);
      setMonitorKeywords(kwList.map((k) => ({ ...k, site_domain: k.site_domain || siteMap.get(k.site_id) || '' })));
      setConfigBackendOk(true);
    } catch (err) {
      console.warn('[baidu-keywords] 配置接口不可用，使用本地演示配置:', err);
      setConfigBackendOk(false);
    }
  }, []);

  useEffect(() => { loadConfig(); }, [loadConfig]);

  // 加载近 30 天排名趋势（后端不可用则回退演示序列）
  const loadTrend = useCallback(async (kw: string, domains: string[]) => {
    if (!kw || !domains.length) { setTrendData(null); return; }
    setTrendLoading(true);
    try {
      const res = await api.baiduRankHistory(kw, 30);
      setTrendData(res as RankHistory);
    } catch (err) {
      console.warn('[baidu-keywords] 趋势接口不可用，使用演示趋势:', err);
      // 演示趋势：近 30 天，为我方域名各生成一条确定性伪随机序列
      const rnd = lcg(kw.split('').reduce((s, c) => s + c.charCodeAt(0), 7));
      const today = new Date();
      const points: RankPoint[] = [];
      domains.forEach((d) => {
        let r = 8 + Math.floor(rnd() * 5);
        for (let i = 29; i >= 0; i--) {
          const dt = new Date(today);
          dt.setDate(dt.getDate() - i);
          r = Math.max(2, Math.min(15, r + (rnd() > 0.5 ? 1 : -1)));
          points.push({ date: dt.toISOString().slice(0, 10), site_domain: d, best_rank: r, occurrences: 1 });
        }
      });
      setTrendData({ keyword: kw, days: 30, points });
    }
    setTrendLoading(false);
  }, []);

  const runAnalysis = async (words: string[]) => {
    const wl = words.slice(0, 5);
    if (!wl.length) return;
    setAnalyzing(true);
    const total = depth * 10 * wl.length;
    let done = 0, sec = 0;
    setProgress({ done: 0, total, peers: 0, seconds: 0 });
    if (timerRef.current) clearInterval(timerRef.current);
    let settled = false;
    // 进度条仅作加载动画，真实进度以请求返回为准
    timerRef.current = setInterval(() => {
      if (settled) { if (timerRef.current) clearInterval(timerRef.current); return; }
      sec += 0.15;
      done = Math.min(done + 4, total);
      setProgress({ done, total, peers: Math.round((done / total) * 6), seconds: Math.round(sec * 10) / 10 });
    }, 150);

    // 自动携带客户已配置且启用的网站域名（my_domains）
    const myDomains = sites.filter((s) => s.enabled).map((s) => s.domain).filter(Boolean);

    // 每个关键词都调用后端实时抓取接口（Promise 并行，词数上限 5）；
    // 单个词失败时仅该词回退本地演示数据并明确标注，不影响其它关键词的真实结果
    const tasks = wl.map((w, i) =>
      api.baiduAnalyze({ keyword: w, depth: effDepth, my_domains: myDomains })
        .then((data: any) => {
          const d = data as ApiResult;
          const pf = d && d.partialFail;
          let note = pf ? `部分页面抓取受限：${pf}` : '';
          const real = d && Array.isArray(d.pages) && d.pages.length > 0;
          if (!note && !real) note = '未解析到排名数据（可能该词无收录或抓取为空）';
          return { key: w, res: toMockResult(d), note };
        })
        .catch((err: Error) => {
          console.warn('[baidu-keywords] 关键词抓取失败，回退演示数据:', w, err);
          const reason = err && err.message ? err.message : '后端不可用';
          return {
            key: w,
            res: buildMock(w, depth, (Date.now() % 100000) + i * 7919),
            note: `该词抓取失败已回退演示数据（${reason}）`,
          };
        })
    );

    const settledList = await Promise.all(tasks);
    settled = true;
    if (timerRef.current) clearInterval(timerRef.current);
    notifyQuotaChanged(); // 查询消耗每日配额，立即刷新余量显示
    const mapped: Record<string, MockResult> = {};
    const notes: Record<string, string> = {};
    settledList.forEach((it) => {
      mapped[it.key] = it.res;
      if (it.note) notes[it.key] = it.note;
    });
    resultsRef.current = mapped;
    setResults(mapped);
    setFallbackNotes(notes);
    setActiveKey(wl[0]);
    setExpandedPage(1);
    setAnalyzing(false);
    // 分析完成后加载首词近 30 天排名趋势
    loadTrend(wl[0], myDomains);
    const failed = Object.keys(notes).filter((k) => notes[k].startsWith('该词抓取失败')).length;
    const ok = wl.length - failed;
    Message.success(
      failed > 0
        ? `${ok}/${wl.length} 个关键词实时查询成功${failed ? `，${failed} 个已回退演示数据` : ''}`
        : `「${wl.join('、')}」前 ${effDepth} 页实时查询完成，共 ${wl.length} 个关键词`
    );
  };

  useEffect(() => () => { if (timerRef.current) clearInterval(timerRef.current); }, []);

  const startAnalyze = () => {
    const words = keywordInput.split(/[,，]/).map((s) => s.trim()).filter(Boolean);
    if (!words.length) { Message.warning('请先输入关键词'); return; }
    runAnalysis(words.slice(0, 5));
  };

  const cur: MockResult | undefined = results[activeKey];

  // 抓取深度固定 3：不再有「自定义页数」分支（99 已废弃），直接等于 depth
  const effDepth = depth;

  // ---- 「+ 对比关键词」：点击 Tab 弹窗输入新词，增量抓取后合入对比视图 ----
  const onTabChange = (key: string) => {
    if (key === '__add') {
      setCompareKeyword('');
      setCompareModal(true);
      return; // 不切换 activeKey，避免落到空 Tab
    }
    setActiveKey(key);
  };

  const addCompareKeyword = async () => {
    const w = compareKeyword.trim();
    if (!w) { Message.warning('请输入要对比的关键词'); return; }
    if (resultsRef.current[w]) { Message.info(`「${w}」已在对比列表中`); setCompareModal(false); return; }
    setCompareModal(false);
    setAnalyzing(true);
    const myDomains = sites.filter((s) => s.enabled).map((s) => s.domain).filter(Boolean);
    try {
      const d = (await api.baiduAnalyze({ keyword: w, depth: effDepth, my_domains: myDomains })) as ApiResult;
      const pf = d && d.partialFail;
      const merged = { ...resultsRef.current, [w]: toMockResult(d) };
      resultsRef.current = merged;
      setResults(merged);
      setFallbackNotes((prev) => (pf ? { ...prev, [w]: `部分页面抓取受限：${pf}` } : prev));
      setActiveKey(w);
      setExpandedPage(1);
      loadTrend(w, myDomains);
      notifyQuotaChanged(); // 对比词也消耗配额
      Message.success(`「${w}」已加入对比`);
    } catch (err: any) {
      const reason = err && err.message ? err.message : '后端不可用';
      const merged = { ...resultsRef.current, [w]: buildMock(w, depth, Date.now() % 100000) };
      resultsRef.current = merged;
      setResults(merged);
      setFallbackNotes((prev) => ({ ...prev, [w]: `该词抓取失败已回退演示数据（${reason}）` }));
      setActiveKey(w);
      setExpandedPage(1);
      Message.warning(`「${w}」抓取失败，已回退演示数据`);
    } finally {
      setAnalyzing(false);
    }
  };

  // ---- 配置面板 CRUD（后端不可用则在本地 state 上模拟增删改，保持演示可用）----
  const isBackend = () => configBackendOk !== false;
  const upsertSites = (list: BaiduSite[]) => {
    setSites(list);
    // 同步已删除站点关联的关键词
    const aliveIds = new Set(list.map((s) => s.id));
    setMonitorKeywords((prev) => prev.filter((k) => aliveIds.has(k.site_id)));
  };

  const handleAddSite = async (domain: string, name: string) => {
    if (!domain) { Message.warning('请输入网站域名'); return; }
    if (!isBackend()) {
      const id = Math.max(0, ...sites.map((s) => s.id)) + 1;
      upsertSites([...sites, { id, domain, name, enabled: true }]);
      Message.success('已添加网站（演示模式，未持久化）');
      return;
    }
    try {
      const res = await api.baiduCreateSite({ domain, name });
      await loadConfig();
      Message.success(`已添加网站：${domain}`);
    } catch (e: any) {
      Message.error(`添加失败：${e?.message || '网络错误'}`);
    }
  };
  const handleUpdateSite = async (id: number, patch: Partial<BaiduSite>) => {
    if (!isBackend()) {
      upsertSites(sites.map((s) => (s.id === id ? { ...s, ...patch } : s)));
      Message.success('已更新网站（演示模式）');
      return;
    }
    try {
      await api.baiduUpdateSite(id, patch);
      await loadConfig();
      Message.success('已更新网站');
    } catch (e: any) {
      Message.error(`更新失败：${e?.message || '网络错误'}`);
    }
  };
  const handleDeleteSite = async (id: number) => {
    if (!isBackend()) {
      upsertSites(sites.filter((s) => s.id !== id));
      Message.success('已删除网站（演示模式）');
      return;
    }
    try {
      await api.baiduDeleteSite(id);
      await loadConfig();
      Message.success('已删除网站及其关联关键词');
    } catch (e: any) {
      Message.error(`删除失败：${e?.message || '网络错误'}`);
    }
  };
  const handleAddKeyword = async (keyword: string, siteId: number) => {
    if (!keyword) { Message.warning('请输入关键词'); return; }
    if (!siteId) { Message.warning('请选择关联网站'); return; }
    if (!isBackend()) {
      const id = Math.max(0, ...monitorKeywords.map((k) => k.id)) + 1;
      const site = sites.find((s) => s.id === siteId);
      setMonitorKeywords([...monitorKeywords, { id, keyword, site_id: siteId, site_domain: site?.domain || '', enabled: true }]);
      Message.success('已添加监控关键词（演示模式）');
      return;
    }
    try {
      await api.baiduCreateMonitorKeyword({ keyword, site_id: siteId });
      await loadConfig();
      Message.success(`已添加监控关键词：${keyword}`);
    } catch (e: any) {
      Message.error(`添加失败：${e?.message || '网络错误'}`);
    }
  };
  const handleUpdateKeyword = async (id: number, patch: Partial<BaiduMonitorKeyword>) => {
    if (!isBackend()) {
      setMonitorKeywords(monitorKeywords.map((k) => (k.id === id ? { ...k, ...patch } : k)));
      Message.success('已更新关键词（演示模式）');
      return;
    }
    try {
      await api.baiduUpdateMonitorKeyword(id, patch);
      await loadConfig();
      Message.success('已更新关键词');
    } catch (e: any) {
      Message.error(`更新失败：${e?.message || '网络错误'}`);
    }
  };
  const handleDeleteKeyword = async (id: number) => {
    if (!isBackend()) {
      setMonitorKeywords(monitorKeywords.filter((k) => k.id !== id));
      Message.success('已删除关键词（演示模式）');
      return;
    }
    try {
      await api.baiduDeleteMonitorKeyword(id);
      await loadConfig();
      Message.success('已删除监控关键词');
    } catch (e: any) {
      Message.error(`删除失败：${e?.message || '网络错误'}`);
    }
  };

  // 切到某个关键词时联动加载其近 30 天趋势
  const onSelectKeyword = (kw: string) => {
    setKeywordInput(kw);
    const myDomains = sites.filter((s) => s.enabled).map((s) => s.domain).filter(Boolean);
    loadTrend(kw, myDomains);
  };

  // ---- 配置弹窗状态与保存 ----
  const [siteModal, setSiteModal] = useState<Partial<BaiduSite> | null>(null);
  const [keywordModal, setKeywordModal] = useState<Partial<BaiduMonitorKeyword> | null>(null);
  const openSiteModal = (s?: BaiduSite) => setSiteModal(s ? { ...s } : { enabled: true });
  const openKeywordModal = (k?: BaiduMonitorKeyword) =>
    setKeywordModal(k ? { ...k } : { enabled: true, site_id: sites[0]?.id });
  const saveSiteModal = () => {
    if (!siteModal) return;
    if (siteModal.id) {
      handleUpdateSite(siteModal.id, { domain: siteModal.domain, name: siteModal.name });
    } else {
      handleAddSite(siteModal.domain || '', siteModal.name || '');
    }
    setSiteModal(null);
  };
  const saveKeywordModal = () => {
    if (!keywordModal) return;
    if (keywordModal.id) {
      handleUpdateKeyword(keywordModal.id, { keyword: keywordModal.keyword, site_id: keywordModal.site_id });
    } else {
      handleAddKeyword(keywordModal.keyword || '', keywordModal.site_id || 0);
    }
    setKeywordModal(null);
  };

  // ---- 近 30 天趋势折线图 option ----
  const trendOption = (() => {
    if (!trendData || !trendData.points || !trendData.points.length) return null;
    const domains = [...new Set(trendData.points.map((p) => p.site_domain))];
    const dates = [...new Set(trendData.points.map((p) => p.date))].sort();
    const series = domains.map((d, i) => {
      const byDate = new Map<string, number>();
      trendData.points.filter((p) => p.site_domain === d).forEach((p) => { byDate.set(p.date, p.best_rank); });
      return {
        name: d,
        type: 'line' as const,
        smooth: true,
        connectNulls: true,
        symbolSize: 6,
        lineStyle: { width: 2.5, color: DOMAIN_COLORS[i % DOMAIN_COLORS.length] },
        itemStyle: { color: DOMAIN_COLORS[i % DOMAIN_COLORS.length] },
        data: dates.map((dt) => byDate.get(dt) ?? null),
      };
    });
    return {
      tooltip: {
        trigger: 'axis' as const,
        formatter: (params: any) => {
          const arr = Array.isArray(params) ? params : [params];
          let html = `<b>${arr[0]?.axisValue || ''}</b><br/>`;
          arr.forEach((p: any) => {
            if (p.value == null) return;
            const arrow = p.value <= 5 ? '↑ 靠前' : p.value <= 10 ? '→ 中游' : '↓ 靠后';
            html += `${p.marker}${p.seriesName}：第 <b>${p.value}</b> 名（${arrow}）<br/>`;
          });
          return html;
        },
      },
      legend: { top: 0, data: domains },
      grid: { top: 40, left: 10, right: 10, bottom: 0, containLabel: true },
      xAxis: { type: 'category' as const, data: dates, boundaryGap: false, axisLabel: { color: '#86909C', fontSize: 11 } },
      yAxis: {
        type: 'value' as const, name: '排名(越小越靠前)', nameTextStyle: { color: '#86909C', fontSize: 11 }, inverse: true,
        min: 1, axisLabel: { color: '#86909C', fontSize: 11 }, splitLine: { lineStyle: { color: '#f2f3f5' } },
      },
      series,
    };
  })();

  return (
    <div>
      <AnimStyle />
      {/* 刷新进度条 */}
      {analyzing && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, height: 3, zIndex: 1000, overflow: 'hidden' }}>
          <div style={{
            height: '100%', width: '40%', background: 'linear-gradient(90deg,#165DFF,#14C9C9,#FF7D00)',
            animation: 'geoKbBar 1.1s linear infinite',
          }} />
        </div>
      )}

      {/* 根 Tab：分析 / 我的网站·关键词配置 */}
      <Tabs activeTab={rootTab} onChange={setRootTab} style={{ marginBottom: 8 }} type="line" size="large">
        <TabPane key="analyze" title={<span><IconSearch style={{ marginRight: 6 }} />百度优化</span>} />
        <TabPane key="config" title={<span><IconUserGroup style={{ marginRight: 6 }} />我的网站 / 关键词配置</span>} />
      </Tabs>

      {rootTab === 'analyze' && (
      <div>
      {/* 页首 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 12, marginBottom: 16 }}>
        <div style={{ minWidth: 0, maxWidth: '100%' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#165DFF,#14C9C9,#FF7D00)' }} />
            <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>百度优化</span>
            <Tag color="arcoblue" size="small">Beta</Tag>
          </div>
          <div style={{ color: '#86909C', fontSize: 13, marginTop: 4 }}>
            搜索同行动态 · 抓取前 {effDepth} 页 · 给出 P0/P1/P2 优化建议（已接通后端实时抓取）
          </div>
          <QuotaBadge />
        </div>
        <Tag style={{ background: 'fff7E8', border: '1px solid #FF7D0033', color: '#D25F00', borderRadius: 6 }}>
          数据来自后端实时抓取；后端不可用时自动回退本地演示数据
        </Tag>
      </div>

      {/* 一、搜索抓取区 */}
      <div className="geo-kb-reveal">
        <Card style={{ borderRadius: 16, marginBottom: 16 }}>
          <GridRow gutter={16} align="center">
            <GridCol xs={24} md={14}>
              <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 6 }}>关键词（支持多个，逗号分隔 → 对比视图）</div>
              <Space style={{ width: '100%' }} direction="vertical" size={8}>
                <Input
                  value={keywordInput}
                  onChange={setKeywordInput}
                  placeholder="如：深圳高端婚恋交友, 深圳婚介所推荐"
                  prefix={<IconSearch style={{ color: '#86909C' }} />}
                  size="large"
                  style={{ borderRadius: 10 }}
                />
                {monitorKeywords.filter((k) => k.enabled).length > 0 && (
                  <Select
                    value={undefined}
                    placeholder="从已配置的监控关键词中快速选择"
                    onChange={(v: any) => { if (v) onSelectKeyword(String(v)); }}
                    size="small"
                    style={{ width: '100%' }}
                    options={monitorKeywords.filter((k) => k.enabled).map((k) => ({
                      label: k.site_domain ? `${k.keyword}（${k.site_domain}）` : k.keyword,
                      value: k.keyword,
                    }))}
                  />
                )}
              </Space>
            </GridCol>
            <GridCol xs={24} md={5}>
              <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 6 }}>广告位处理</div>
              <Space size={8}>
                <Switch checked={ignoredAd} onChange={setIgnoredAd} size="small" />
                <Text style={{ fontSize: 12, color: 'var(--color-text-2)' }}>霸屏统计剔除广告（自然排名为准）</Text>
              </Space>
            </GridCol>
            <GridCol xs={24} md={5} style={{ display: 'flex', alignItems: 'flex-end' }}>
              <Button type="primary" size="large" icon={<IconPlayArrow />} loading={analyzing} onClick={startAnalyze} style={{ width: '100%', borderRadius: 10 }}>
                开始分析
              </Button>
            </GridCol>
          </GridRow>

          {/* 抓取进度条与实时状态 */}
          {analyzing && (
            <div style={{ marginTop: 16, padding: '14px 16px', borderRadius: 12, background: 'var(--geo-surface-2)', border: '1px solid var(--color-border-2)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#86909C', marginBottom: 8 }}>
                <span><Spin size={12} /> 正在搜索抓取…</span>
                <span>已解析 <b style={{ color: '#165DFF' }}>{progress.done}</b> / {progress.total} 条 · 识别同行 <b style={{ color: '#722ED1' }}>{progress.peers}</b> 个 · 耗时 {progress.seconds}s</span>
              </div>
              <Progress percent={(progress.done / progress.total) * 100} color="#165DFF" style={{ width: '100%' }} showText={false} />
            </div>
          )}
        </Card>
      </div>

      {!cur ? (
        <Card style={{ borderRadius: 16, padding: '50px 0' }}>
          <Empty description="输入关键词并点击「开始分析」" />
        </Card>
      ) : (
        /* ==================== 分析结果 ==================== */
        <div style={{ opacity: analyzing ? 0.5 : 1, transition: 'opacity .3s', pointerEvents: analyzing ? 'none' : 'auto' }}>
          {/* 多关键词对比视图 */}
          <Tabs activeTab={activeKey} onChange={onTabChange} style={{ marginBottom: 16 }}>
            {Object.keys(results).map((k) => (
              <TabPane
                key={k}
                title={
                  <span>
                    {k}
                    {fallbackNotes[k] && (
                      <Tag size="small" color="orange" style={{ marginLeft: 6 }}>回退</Tag>
                    )}
                  </span>
                }
              />
            ))}
            {Object.keys(results).length > 0 && (
              <TabPane key="__add" title="+ 对比关键词" />
            )}
          </Tabs>

          {/* 当前关键词抓取状态提示：失败回退 / 部分受限 */}
          {fallbackNotes[activeKey] && (
            <Alert
              type={fallbackNotes[activeKey].startsWith('该词抓取失败') ? 'warning' : 'info'}
              style={{ marginBottom: 16, borderRadius: 10 }}
              title={`「${activeKey}」未实时查询成功`}
              content={fallbackNotes[activeKey]}
            />
          )}

          {/* 二、竞争指标卡 */}
          <div className="geo-kb-reveal">
            <GridRow gutter={[14, 14]}>
              {[
                { label: '同行站点数', num: cur.peers, tail: '个', desc: '自然排名中识别出的同业站点', idx: 0 },
                { label: '霸屏同行', num: cur.peers, tail: '', desc: cur.dominant ? `${cur.dominant.name} · 出现 ${cur.dominant.count} 次` : '暂无', idx: 1, text: cur.dominant?.name || '-' },
                { label: '我方最佳排名', num: cur.myBest, tail: '名', desc: '自然结果中我方站点最好名次', idx: 2 },
                { label: '竞争热度', num: cur.heat, tail: '/100', desc: cur.heat >= 80 ? '激烈：建议差异化切入' : cur.heat >= 65 ? '中等：有机会卡位' : '偏低：抢占窗口期', idx: 3 },
              ].map((k: any, i) => (
                <GridCol key={k.label} xs={12} sm={6} lg={3}>
                  <div style={{
                    position: 'relative', overflow: 'hidden', borderRadius: 16, padding: '14px 16px 12px',
                    background: KPI_STYLE[k.idx % KPI_STYLE.length].grad, border: '1px solid var(--color-border-2)',
                    boxShadow: '0 4px 14px rgba(0,0,0,0.04)',
                  }}>
                    <span style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 4, background: KPI_STYLE[k.idx % KPI_STYLE.length].color }} />
                    <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 2 }}>{k.label}</div>
                    <div style={{ fontSize: 24, fontWeight: 800, lineHeight: 1.2, display: 'flex', alignItems: 'baseline', gap: 4 }}>
                      {k.text ? (
                        <span style={{ fontSize: 16, color: KPI_STYLE[i % KPI_STYLE.length].color, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 150 }}>{k.text}</span>
                      ) : (
                        <CountUpNumber value={k.num} color={KPI_STYLE[k.idx % KPI_STYLE.length].color} />
                      )}
                      <span style={{ fontSize: 13, fontWeight: 600, color: '#86909C' }}>{k.tail}</span>
                    </div>
                    <div style={{ fontSize: 11, color: '#86909C', marginTop: 4 }}>{k.desc}</div>
                  </div>
                </GridCol>
              ))}
            </GridRow>
          </div>

          {/* 三、近 30 天排名趋势（echarts） */}
          <div className="geo-kb-reveal" style={{ marginTop: 16 }}>
            <Card
              style={{ borderRadius: 16 }}
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <IconArrowRise style={{ color: '#165DFF' }} />
                  <span style={{ fontSize: 15, fontWeight: 600 }}>近 30 天排名趋势</span>
                  <span style={{ fontSize: 12, color: '#86909C' }}>「{activeKey}」各客户网站每日最佳排名（越小越靠前）</span>
                  {cur.trendSummary && (
                    <Tag color={cur.trendSummary.includes('上升') ? 'red' : cur.trendSummary.includes('下降') ? 'green' : 'arcoblue'} size="small" style={{ marginLeft: 'auto' }}>
                      趋势：{cur.trendSummary}
                    </Tag>
                  )}
                </div>
              }
            >
              {trendLoading ? (
                <div style={{ padding: '40px 0', textAlign: 'center' }}><Spin tip="加载排名趋势…" /></div>
              ) : trendOption ? (
                <EChart option={trendOption} height={300} />
              ) : (
                <Empty description="暂无排名趋势数据（先配置「我的网站」并完成一次分析，将写入每日快照）" />
              )}
            </Card>
          </div>

          {/* 四、前十页分布矩阵（核心模块） */}
          <div className="geo-kb-reveal" style={{ marginTop: 16 }}>
            <Card
              style={{ borderRadius: 16 }}
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 15, fontWeight: 600 }}>前十页分布矩阵</span>
                  <Tooltip content="颜色深浅 = 该页自然结果中同行密度">
                    <Tag color="arcoblue" size="small">同行密度热力</Tag>
                  </Tooltip>
                  <span style={{ marginLeft: 'auto', fontSize: 12, color: '#86909C' }}>
                    <Tag size="small" color="green" style={{ marginRight: 6 }}>自然·同行</Tag>
                    <Tag size="small" color="orange" style={{ marginRight: 6 }}>广告(已剔除统计)</Tag>
                    <Tag size="small">自然·其他</Tag>
                  </span>
                </div>
              }
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {cur.pages.map((pg) => {
                  const opacity = 0.18 + pg.density * 0.75;
                  const expanded = expandedPage === pg.page;
                  return (
                    <div key={pg.page} style={{ borderRadius: 12, overflow: 'hidden', border: expanded ? '1px solid #165DFF55' : '1px solid var(--color-border-2)' }}>
                      {/* 每页一行 */}
                      <div
                        onClick={() => setExpandedPage(expanded ? null : pg.page)}
                        style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 12, padding: '9px 14px', background: expanded ? 'var(--geo-tint-blue)' : 'var(--geo-surface-2)', transition: 'background .2s' }}
                      >
                        <span style={{ width: 30, textAlign: 'center', fontWeight: 700, color: expanded ? '#165DFF' : '#4e5969' }}>P{pg.page}</span>
                        <div style={{ flex: 1, height: 16, borderRadius: 8, background: 'var(--geo-track-strong)', overflow: 'hidden', position: 'relative' }}>
                          <div style={{
                            width: `${pg.density * 100}%`, height: '100%', borderRadius: 8,
                            background: `rgba(22,93,255,${opacity})`, transition: 'width .4s ease',
                          }} />
                          <span style={{ position: 'absolute', right: 6, top: 0, lineHeight: '16px', fontSize: 10, color: pg.density > 0.45 ? '#fff' : '#86909C' }}>
                            同行 {pg.organicPeers}/10
                          </span>
                        </div>
                        <span style={{ width: 90, fontSize: 12, color: '#86909C' }}>
                          广告 {pg.ads} · 自然 {pg.organic - pg.organicPeers}
                        </span>
                        {expanded ? <IconUp style={{ color: '#165DFF' }} /> : <IconExpand style={{ color: '#86909C' }} />}
                      </div>
                      {/* 展开本页 10 条 */}
                      {expanded && (
                        <div style={{ padding: '6px 14px 12px' }}>
                          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                            <thead>
                              <tr style={{ color: '#86909C', fontSize: 11, textAlign: 'left' }}>
                                <th style={{ padding: '6px 8px', width: 40 }}>位次</th>
                                <th style={{ padding: '6px 8px', width: 90 }}>类型</th>
                                <th style={{ padding: '6px 8px', width: 130 }}>同行归属</th>
                                <th style={{ padding: '6px 8px' }}>标题</th>
                                <th style={{ padding: '6px 8px', width: 260 }}>落地页 URL</th>
                              </tr>
                            </thead>
                            <tbody>
                              {pg.items.map((it) => {
                                const peerLib = PEER_LIB.find((x) => x.d === it.domain);
                                const typeColor = it.ad ? 'orange' : it.peer ? 'green' : 'default';
                                const typeText = it.ad ? '广告' : it.peer ? '自然·同行' : '自然·其他';
                                return (
                                  <tr key={it.rank} style={{ borderTop: '1px solid var(--color-border-1)' }}>
                                    <td style={{ padding: '7px 8px', fontWeight: 700, color: it.rank <= 3 ? '#165DFF' : '#4e5969' }}>{it.rank}</td>
                                    <td style={{ padding: '7px 8px' }}>
                                      <Tag size="small" color={typeColor}>{typeText}</Tag>
                                      {it.peer && <span style={{ fontSize: 9, color: '#C9CDD4', marginLeft: 2 }}>{it.ad ? '不计入' : '计入'}</span>}
                                    </td>
                                    <td style={{ padding: '7px 8px' }}>
                                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                        <DomainBadge name={it.domain} text={it.name} size={20} />
                                        {it.peer ? (
                                          <span style={{ fontWeight: 600 }}>{it.name}</span>
                                        ) : (
                                          <span style={{ color: '#86909C' }}>{it.domain}</span>
                                        )}
                                        {it.peer && it.ad && <Tag size="small" color="orange">头部投放</Tag>}
                                      </div>
                                    </td>
                                    <td style={{ padding: '7px 8px', color: 'var(--geo-text)', maxWidth: 360 }}>
                                      <Tooltip content={<div style={{ maxWidth: 420 }}>{it.snippet}</div>}>
                                        <span style={{ display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{it.title}</span>
                                      </Tooltip>
                                    </td>
                                    <td style={{ padding: '7px 8px' }}>
                                      <a href={it.url} target="_blank" rel="noreferrer" style={{ color: '#165DFF', fontSize: 12, wordBreak: 'break-all' }}>{it.url}</a>
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </Card>
          </div>

          {/* 四、同行排行对比表 */}
          <div className="geo-kb-reveal" style={{ marginTop: 16 }}>
            <Card
              style={{ borderRadius: 16 }}
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 15, fontWeight: 600 }}>同行排行对比</span>
                  <Tag color={cur.peerRows[0]?.count >= 8 ? 'red' : 'orange'} size="small">霸屏 TOP1：{cur.peerRows[0]?.name}</Tag>
                  <span style={{ marginLeft: 'auto', fontSize: 12, color: '#86909C' }}>按自然排名出现次数排序</span>
                  <Tag color="green" size="small" style={{ background: 'var(--geo-tint-green)', border: '1px solid #00B42A33' }}>绿色 = 我方网站</Tag>
                </div>
              }
            >
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                  <thead>
                    <tr style={{ color: '#86909C', fontSize: 12, textAlign: 'left' }}>
                      <th style={{ padding: '8px 12px' }}>排行</th>
                      <th style={{ padding: '8px 12px' }}>同行站点</th>
                      <th style={{ padding: '8px 12px' }}>出现次数</th>
                      <th style={{ padding: '8px 12px' }}>出现页码</th>
                      <th style={{ padding: '8px 12px' }}>最好排名</th>
                      <th style={{ padding: '8px 12px' }}>平均排名</th>
                      <th style={{ padding: '8px 12px' }}>标题套路</th>
                    </tr>
                  </thead>
                  <tbody>
                    {cur.peerRows.map((r, i) => {
                      const myDomain = sites.find((s) => s.domain === r.domain);
                      return (
                      <tr
                        key={r.domain}
                        style={{
                          borderTop: '1px solid var(--color-border-1)',
                          background: myDomain ? 'var(--geo-tint-green)' : undefined,
                          boxShadow: myDomain ? 'inset 3px 0 0 #00B42A' : undefined,
                        }}
                      >
                        <td style={{ padding: '10px 12px' }}>
                          <span style={{
                            display: 'inline-flex', width: 22, height: 22, borderRadius: 7, alignItems: 'center', justifyContent: 'center',
                            fontWeight: 700, fontSize: 12, color: '#fff', background: i < 3 ? 'linear-gradient(135deg,#165DFF,#14C9C9)' : '#C9CDD4',
                          }}>{i + 1}</span>
                        </td>
                        <td style={{ padding: '10px 12px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                            <DomainBadge name={r.domain} text={r.name} />
                            <div>
                              <div style={{ fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                                {r.name}
                                {myDomain && <Tag size="small" color="green" style={{ borderRadius: 4 }}>我方</Tag>}
                              </div>
                              <div style={{ fontSize: 11, color: '#86909C' }}>{r.domain}</div>
                            </div>
                          </div>
                        </td>
                        <td style={{ padding: '10px 12px' }}>
                          <b style={{ color: myDomain ? '#00B42A' : r.count >= 8 ? '#F53F3F' : '#FF7D00', fontSize: 16 }}>{r.count}</b>
                          <span style={{ color: '#C9CDD4', fontSize: 12 }}> 次</span>
                        </td>
                        <td style={{ padding: '10px 12px' }}>
                          <Space size={3} wrap>
                            {r.pages.map((p2) => (
                              <Tag key={p2} size="small" color={p2 <= 3 ? 'arcoblue' : 'default'}>{p2}</Tag>
                            ))}
                          </Space>
                        </td>
                        <td style={{ padding: '10px 12px', fontWeight: 700, color: myDomain ? '#00B42A' : r.bestRank <= 3 ? '#00B42A' : '#4e5969' }}>第 {r.bestRank} 名</td>
                        <td style={{ padding: '10px 12px', color: 'var(--color-text-2)' }}>{r.avgRank} 位</td>
                        <td style={{ padding: '10px 12px', maxWidth: 360 }}>
                          <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--geo-text)' }}>{r.pattern}</div>
                          <div style={{ marginTop: 4 }}>
                            <Space size={4} wrap>
                              {r.feats.map((f) => <Tag key={f} size="small" color="arcoblue">{f}</Tag>)}
                            </Space>
                          </div>
                        </td>
                      </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </Card>
          </div>

          {/* 五、优化建议面板 P0/P1/P2 + SEO */}
          <div className="geo-kb-reveal" style={{ marginTop: 16 }}>
            <Card
              style={{ borderRadius: 16 }}
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 15, fontWeight: 600 }}>优化建议</span>
                  <Tag color="orangered" size="small">行动梯度</Tag>
                  <Tag color="purple" size="small">SEO 专项</Tag>
                </div>
              }
            >
              {['P0', 'P1', 'P2', 'SEO'].map((lv) => {
                const list = cur.advices.filter((a) => a.level === lv);
                const meta = {
                  P0: { color: '#F53F3F', bg: 'fff7F7', border: '#F53F3F33', desc: '立即处理 · 直接影响排名与转化' },
                  P1: { color: '#FF7D00', bg: 'fffBF4', border: '#FF7D0033', desc: '一周内排期 · 竞争间隙的卡位机会' },
                  P2: { color: '#00B42A', bg: '#F4FEF6', border: '#00B42A33', desc: '持续优化 · 信任与体验升级' },
                  SEO: { color: '#722ED1', bg: '#F7F0FF', border: '#722ED133', desc: '技术型 SEO · 标题/落地页/外链与内容' },
                } as any;
                const m = meta[lv];
                return (
                  <div key={lv} style={{ marginBottom: list.length ? 14 : 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                      <span style={{
                        width: lv === 'SEO' ? 40 : 30, height: 22, borderRadius: 6, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                        fontWeight: 800, fontSize: 12, color: '#fff', background: m.color,
                      }}>{lv}</span>
                      <span style={{ fontSize: 12, color: '#86909C' }}>{m.desc}</span>
                    </div>
                    {list.length ? (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 8 }}>
                        {list.map((a, i) => (
                          <div key={i} style={{ display: 'flex', gap: 12, padding: '12px 14px', borderRadius: 12, background: m.bg, border: `1px solid ${m.border}`, borderLeft: `4px solid ${m.color}` }}>
                            <div style={{ flex: 1, minWidth: 0 }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                                <span style={{ fontSize: 13, fontWeight: 700, color: 'var(--geo-text)' }}>{a.title}</span>
                                <Tag size="small" color="gold">{a.tag}</Tag>
                              </div>
                              <div style={{ fontSize: 12.5, lineHeight: 1.6, color: 'var(--color-text-2)', marginTop: 4 }}>{a.desc}</div>
                              <div style={{ fontSize: 11.5, color: '#86909C', marginTop: 6, display: 'flex', alignItems: 'center', gap: 4 }}>
                                <IconBulb style={{ color: '#FF7D00' }} /> 依据：{a.basis}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div style={{ fontSize: 12, color: '#C9CDD4', padding: '4px 0 10px' }}>暂无 {lv} 级项</div>
                    )}
                  </div>
                );
              })}
            </Card>
          </div>

          {/* 多关键词对比提示 */}
          <div style={{ marginTop: 12, textAlign: 'center', color: '#86909C', fontSize: 12 }}>
            <IconThunderbolt style={{ color: '#FF7D00', marginRight: 4 }} />
            多关键词对比：在抓取区输入多个关键词，可在上方 Tab 间切换，查看同一批同行在不同关键词下的排名变化，辅助选题决策
          </div>
        </div>
      )}
      </div>
      )}

      {rootTab === 'config' && (
        <div>
          {/* ==================== 我的网站 / 关键词配置 ==================== */}
          {configBackendOk === false && (
            <Alert
              type="warning" style={{ marginBottom: 12, borderRadius: 10 }}
              title="后端不可用，当前为演示模式"
              content="增删改仅在当前页面内存中生效，不会持久化到数据库；连接后端后配置将自动同步。"
            />
          )}

          {/* 我的网站 */}
          <Card
            style={{ borderRadius: 16, marginBottom: 16 }}
            title={
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 15, fontWeight: 600 }}>我的网站</span>
                <Tag color="arcoblue" size="small">{sites.length} 个</Tag>
              </div>
            }
            extra={
              <Button type="primary" size="small" icon={<IconPlus />} onClick={() => openSiteModal()}>
                新增网站
              </Button>
            }
          >
            {sites.length ? (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                  <thead>
                    <tr style={{ color: '#86909C', fontSize: 12, textAlign: 'left' }}>
                      <th style={{ padding: '8px 12px' }}>域名</th>
                      <th style={{ padding: '8px 12px' }}>名称</th>
                      <th style={{ padding: '8px 12px' }}>启用</th>
                      <th style={{ padding: '8px 12px' }}>监控关键词数</th>
                      <th style={{ padding: '8px 12px' }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sites.map((s) => (
                      <tr key={s.id} style={{ borderTop: '1px solid var(--color-border-1)' }}>
                        <td style={{ padding: '10px 12px', fontWeight: 600, color: '#165DFF' }}>{s.domain}</td>
                        <td style={{ padding: '10px 12px', color: 'var(--color-text-2)' }}>{s.name || '-'}</td>
                        <td style={{ padding: '10px 12px' }}>
                          <Switch
                            size="small" checked={s.enabled}
                            onChange={(v) => handleUpdateSite(s.id, { enabled: v })}
                          />
                        </td>
                        <td style={{ padding: '10px 12px' }}>{monitorKeywords.filter((k) => k.site_id === s.id).length}</td>
                        <td style={{ padding: '10px 12px' }}>
                          <Space>
                            <Button size="mini" icon={<IconEdit />} onClick={() => openSiteModal(s)} />
                            <Popconfirm
                              title="删除该网站及其关联监控关键词？"
                              onOk={() => handleDeleteSite(s.id)}
                              okText="删除" cancelText="取消"
                            >
                              <Button size="mini" status="danger" icon={<IconDelete />} />
                            </Popconfirm>
                          </Space>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <Empty description={configBackendOk === false ? '暂无网站（演示模式）' : '暂无网站，点击右上角「新增网站」添加'} />
            )}
          </Card>

          {/* 监控关键词 */}
          <Card
            style={{ borderRadius: 16 }}
            title={
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 15, fontWeight: 600 }}>监控关键词</span>
                <Tag color="orangered" size="small">{monitorKeywords.length} 个</Tag>
              </div>
            }
            extra={
              <Button type="primary" size="small" icon={<IconPlus />} onClick={() => openKeywordModal()}>
                新增关键词
              </Button>
            }
          >
            {monitorKeywords.length ? (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                  <thead>
                    <tr style={{ color: '#86909C', fontSize: 12, textAlign: 'left' }}>
                      <th style={{ padding: '8px 12px' }}>关键词</th>
                      <th style={{ padding: '8px 12px' }}>关联网站</th>
                      <th style={{ padding: '8px 12px' }}>启用</th>
                      <th style={{ padding: '8px 12px' }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {monitorKeywords.map((k) => (
                      <tr key={k.id} style={{ borderTop: '1px solid var(--color-border-1)' }}>
                        <td style={{ padding: '10px 12px', fontWeight: 600 }}>{k.keyword}</td>
                        <td style={{ padding: '10px 12px', color: 'var(--color-text-2)' }}>{k.site_domain || k.site_name || '-'}</td>
                        <td style={{ padding: '10px 12px' }}>
                          <Switch
                            size="small" checked={k.enabled}
                            onChange={(v) => handleUpdateKeyword(k.id, { enabled: v })}
                          />
                        </td>
                        <td style={{ padding: '10px 12px' }}>
                          <Space>
                            <Button size="mini" icon={<IconEdit />} onClick={() => openKeywordModal(k)} />
                            <Popconfirm
                              title="删除该监控关键词？"
                              onOk={() => handleDeleteKeyword(k.id)}
                              okText="删除" cancelText="取消"
                            >
                              <Button size="mini" status="danger" icon={<IconDelete />} />
                            </Popconfirm>
                          </Space>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <Empty description={configBackendOk === false ? '暂无监控关键词（演示模式）' : '暂无监控关键词，点击右上角「新增关键词」添加'} />
            )}
          </Card>
        </div>
      )}

      {/* 网站新增/编辑弹窗 */}
      <Modal
        title={siteModal && siteModal.id ? '编辑网站' : '新增网站'}
        visible={!!siteModal}
        onCancel={() => setSiteModal(null)}
        onOk={() => saveSiteModal()}
        okText="保存" cancelText="取消"
      >
        <Form layout="vertical" style={{ marginTop: 8 }}>
          <Form.Item label="网站域名" required>
            <Input
              value={siteModal?.domain || ''}
              onChange={(v) => setSiteModal((p) => (p ? { ...p, domain: v } : p))}
              placeholder="如 www.example.com"
            />
          </Form.Item>
          <Form.Item label="网站名称">
            <Input
              value={siteModal?.name || ''}
              onChange={(v) => setSiteModal((p) => (p ? { ...p, name: v } : p))}
              placeholder="如：我的官网"
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 关键词新增/编辑弹窗 */}
      <Modal
        title={keywordModal && keywordModal.id ? '编辑监控关键词' : '新增监控关键词'}
        visible={!!keywordModal}
        onCancel={() => setKeywordModal(null)}
        onOk={() => saveKeywordModal()}
        okText="保存" cancelText="取消"
      >
        <Form layout="vertical" style={{ marginTop: 8 }}>
          <Form.Item label="关键词" required>
            <Input
              value={keywordModal?.keyword || ''}
              onChange={(v) => setKeywordModal((p) => (p ? { ...p, keyword: v } : p))}
              placeholder="如 深圳高端婚恋交友"
            />
          </Form.Item>
          <Form.Item label="关联网站" required>
            <Select
              value={keywordModal?.site_id}
              onChange={(v) => setKeywordModal((p) => (p ? { ...p, site_id: v } : p))}
              placeholder="选择该关键词关联的网站"
              style={{ width: '100%' }}
              options={sites.map((s) => ({ label: `${s.domain}${s.name ? `（${s.name}）` : ''}`, value: s.id }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 添加对比关键词弹窗：点击「+ 对比关键词」Tab 触发 */}
      <Modal
        title="添加对比关键词"
        visible={compareModal}
        onCancel={() => setCompareModal(false)}
        onOk={() => addCompareKeyword()}
        okText="分析并加入对比" cancelText="取消"
        okButtonProps={{ loading: analyzing }}
      >
        <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 8 }}>
          输入一个新关键词，将实时抓取前 {effDepth} 页并加入上方对比视图
        </div>
        <Input
          value={compareKeyword}
          onChange={setCompareKeyword}
          placeholder="如 高端婚恋一对一"
          onPressEnter={() => addCompareKeyword()}
        />
      </Modal>
    </div>
  );
}
