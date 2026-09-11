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

/** 查询动作成功后调用：清缓存并广播，所有已挂载的配额徽标立即刷新 */
export function notifyQuotaChanged() {
  cache = null;
  try { window.dispatchEvent(new Event('geo-quota-changed')); } catch { /* ignore */ }
}

/**
 * 每日查询配额徽标：百度 / 抖音 / 小红书三模块共用一个每日额度池，
 * 展示「今日剩余 x/y」。不限量（remain < 0）时不渲染。
 *
 * @param compact 紧凑模式（仅一个胶囊，适合嵌在页首标题行）
 */
export default function QuotaBadge({ compact = false }: { compact?: boolean }) {
  const [q, setQ] = useState<QuotaData | null>(cache);

  useEffect(() => {
    let alive = true;
    // stale-while-revalidate：先用缓存渲染（避免闪烁），同时后台拉取最新值。
    // 查询动作会消耗配额，页面重新挂载或切换回来时即自动刷新。
    fetchQuota(true).then((d) => { if (alive && d) setQ(d); });
    const onChanged = () => { fetchQuota(true).then((d) => { if (alive && d) setQ(d); }); };
    window.addEventListener('geo-quota-changed', onChanged);
    return () => { alive = false; window.removeEventListener('geo-quota-changed', onChanged); };
  }, []);

  if (!q || q.remain < 0) return null; // 不限量或未取到 → 不展示

  const pct = q.limit > 0 ? Math.min(100, Math.round((q.used / q.limit) * 100)) : 0;
  const exhausted = q.remain <= 0;
  const warn = !exhausted && q.remain <= Math.max(1, Math.floor(q.limit * 0.2));
  const color = exhausted ? '#F53F3F' : warn ? '#FF7D00' : '#00B42A';
  const tip = `今日查询次数已用 ${q.used}/${q.limit}（百度 · 抖音 · 小红书共用同一每日额度池）${
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
        <Text style={{ fontSize: 12, color: '#86909C', flexShrink: 0 }}>今日查询配额</Text>
        <Tag color={color} size="small" style={{ flexShrink: 0 }}>{q.used}/{q.limit}{exhausted ? ' · 已用完' : ''}</Tag>
        <Text style={{ fontSize: 12, color: '#86909C', wordBreak: 'break-word', minWidth: 0 }}>
          {exhausted ? '明天 0 点自动重置，或联系服务商升级解锁更多' : `剩余 ${q.remain} 次 · 百度/抖音/小红书共用`}
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
