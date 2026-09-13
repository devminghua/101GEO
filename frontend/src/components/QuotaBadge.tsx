import { useEffect, useState } from 'react';
import { Tag, Tooltip, Typography, Progress } from '@arco-design/web-react';
import { api } from '../api';

const { Text } = Typography;

interface QuotaData {
  limit: number;
  used: number;
  remain: number;
}

// 模块级缓存：同一次会话内多个页面共享，避免每个页面都打一次接口。
let cache: QuotaData | null = null;
let inflight: Promise<QuotaData | null> | null = null;
// 短视频配额（抖音/小红书/快手共用池，每日 10 次）独立缓存
let svCache: QuotaData | null = null;
let svInflight: Promise<QuotaData | null> | null = null;

async function fetchQuota(force = false): Promise<QuotaData | null> {
  if (!force && cache) return cache;
  if (!force && inflight) return inflight;
  inflight = (async () => {
    try {
      const r: any = await api.quotaInfo();
      cache = r as QuotaData;
      return cache;
    } catch {
      return null; // 配额接口失败不打扰用户
    } finally {
      inflight = null;
    }
  })();
  return inflight;
}

async function fetchShortVideoQuota(force = false): Promise<QuotaData | null> {
  if (!force && svCache) return svCache;
  if (!force && svInflight) return svInflight;
  svInflight = (async () => {
    try {
      const r: any = await api.shortVideoQuota();
      svCache = r as QuotaData;
      return svCache;
    } catch {
      return null;
    } finally {
      svInflight = null;
    }
  })();
  return svInflight;
}

/** 查询动作成功后调用：清缓存并广播，所有已挂载的配额徽标立即刷新 */
export function notifyQuotaChanged() {
  cache = null;
  svCache = null;
  try { window.dispatchEvent(new Event('geo-quota-changed')); } catch { /* ignore */ }
}

/**
 * 每日查询配额徽标。两种口径：
 *  - scope="total"（默认）：百度模块共用池，展示「今日剩余 x/y」
 *  - scope="short_video"：抖音/小红书/快手共用池（每日 10 次，超限联系官方解锁）
 * 不限量（remain < 0）时不渲染。
 *
 * @param compact 紧凑模式（仅一个胶囊，适合嵌在页首标题行）
 * @param scope 配额池口径：total | short_video
 */
export default function QuotaBadge({ compact = false, scope = 'total' }: { compact?: boolean; scope?: 'total' | 'short_video' }) {
  const isSV = scope === 'short_video';
  const [q, setQ] = useState<QuotaData | null>(isSV ? svCache : cache);

  useEffect(() => {
    let alive = true;
    // stale-while-revalidate：先用缓存渲染（避免闪烁），同时后台拉取最新值。
    // 查询动作会消耗配额，页面重新挂载或切换回来时即自动刷新。
    const fetcher = isSV ? fetchShortVideoQuota : fetchQuota;
    fetcher(true).then((d) => { if (alive && d) setQ(d); });
    const onChanged = () => { fetcher(true).then((d) => { if (alive && d) setQ(d); }); };
    window.addEventListener('geo-quota-changed', onChanged);
    return () => { alive = false; window.removeEventListener('geo-quota-changed', onChanged); };
  }, [isSV]);

  if (!q || q.remain < 0) return null; // 不限量或未取到 → 不展示

  const pct = q.limit > 0 ? Math.min(100, Math.round((q.used / q.limit) * 100)) : 0;
  const exhausted = q.remain <= 0;
  const warn = !exhausted && q.remain <= Math.max(1, Math.floor(q.limit * 0.2));
  const color = exhausted ? '#F53F3F' : warn ? '#FF7D00' : '#00B42A';
  const tip = isSV
    ? `今日短视频查询次数已用 ${q.used}/${q.limit}（抖音 · 小红书 · 快手共用）${
        exhausted ? '，已用完，明天 0 点自动重置；如需更多次数请联系官方客服解锁' : '，剩余 ' + q.remain + ' 次'
      }`
    : `今日查询次数已用 ${q.used}/${q.limit}（百度模块）${
        exhausted ? '，已用完，明天 0 点自动重置，或联系服务商升级版本解锁更多次数' : '，剩余 ' + q.remain + ' 次'
      }`;

  if (compact) {
    return (
      <Tooltip content={tip}>
        <Tag color={color} size="small" style={{ cursor: 'help' }}>
          今日查询 {q.used}/{q.limit}
        </Tag>
      </Tooltip>
    );
  }

  return (
    <div style={{ marginTop: 8, maxWidth: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <Text style={{ fontSize: 12, color: '#86909C', flexShrink: 0 }}>
          {isSV ? '今日短视频查询' : '今日查询配额'}
        </Text>
        <Tag color={color} size="small" style={{ flexShrink: 0 }}>{q.used}/{q.limit}{exhausted ? ' · 已用完' : ''}</Tag>
        <Text style={{ fontSize: 12, color: '#86909C', wordBreak: 'break-word', minWidth: 0 }}>
          {exhausted
            ? isSV ? '明天 0 点自动重置；如需更多次数请联系官方客服解锁' : '明天 0 点自动重置，或联系服务商升级解锁更多'
            : `剩余 ${q.remain} 次 · ${isSV ? '抖音/小红书/快手共用' : '百度模块'}`}
        </Text>
      </div>
      <Progress
        percent={pct}
        size="small"
        color={color}
        style={{ maxWidth: 320, marginTop: 4 }}
        formatText={() => ''}
      />
    </div>
  );
}
