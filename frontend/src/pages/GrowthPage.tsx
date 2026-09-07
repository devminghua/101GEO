import { useEffect, useState } from 'react';
import { Card, Button, Message, Tag } from '@arco-design/web-react';
import { IconGift, IconCheckCircle } from '@arco-design/web-react/icon';
import { api } from '../api';

const LEVEL_META: Record<number, { color: string; bg: string }> = {
  1: { color: '#B0743A', bg: 'linear-gradient(135deg,#8D6E63,#A1887F)' },
  2: { color: '#8A9AA8', bg: 'linear-gradient(135deg,#90A4AE,#B0BEC5)' },
  3: { color: '#D4A017', bg: 'linear-gradient(135deg,#F7BA1E,#FFB300)' },
  4: { color: '#4F46E5', bg: 'linear-gradient(135deg,#4F46E5,#7B61FF)' },
};

export default function GrowthPage() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const r: any = await api.growthSummary();
      setData(r);
    } catch (e: any) {
      Message.error('加载失败：' + (e.message || e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); /* eslint-disable-next-line */ }, []);

  const checkin = async () => {
    setChecking(true);
    try {
      const r: any = await api.growthCheckin();
      if (r && r.points !== undefined) {
        Message.success(`签到成功，+${r.points} 积分（连续 ${r.streak} 天）`);
      }
      load();
    } catch (e: any) {
      Message.error(e.message || '签到失败');
    } finally {
      setChecking(false);
    }
  };

  const meta = LEVEL_META[data?.level] || LEVEL_META[1];
  const progress = data?.next_need !== undefined && data.next_need > 0
    ? Math.max(0, Math.min(100, Math.round((1 - data.next_need / 1000) * 100)))
    : 100;

  return (
    <div>
      {/* 等级卡 */}
      <Card style={{ marginBottom: 16, borderRadius: 12, background: meta.bg, color: '#fff' }} bodyStyle={{ padding: '24px' }} loading={loading}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 20, flexWrap: 'wrap' }}>
          <div style={{ width: 64, height: 64, borderRadius: '50%', background: 'rgba(255,255,255,0.2)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 30, fontWeight: 800, flexShrink: 0 }}>
            {data?.level || 1}
          </div>
          <div style={{ flex: 1, minWidth: 200 }}>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#fff' }}>{data?.level_name || '青铜会员'}</div>
            <div style={{ fontSize: 13, color: 'rgba(255,255,255,0.9)', marginTop: 4 }}>
              当前 {data?.points ?? 0} 积分
              {data?.next_need !== undefined && data.next_need > 0 ? `，再得 ${data.next_need} 积分升级` : '，已达最高等级'}
            </div>
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'rgba(255,255,255,0.85)' }}>连续签到</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: '#fff', marginTop: 4 }}>{data?.streak ?? 0} 天</div>
          </div>
        </div>
      </Card>

      {/* 签到卡 */}
      <Card title="每日签到" style={{ marginBottom: 16, borderRadius: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
          <Button
            type="primary"
            size="large"
            icon={<IconGift />}
            loading={checking}
            disabled={data?.today_checked}
            onClick={checkin}
          >
            {data?.today_checked ? '今日已签到' : '立即签到'}
          </Button>
          <div style={{ fontSize: 13, color: '#86909C' }}>
            连续签到奖励递增：第 1 天 5 积分，之后每天 +2（封顶 50 积分/天）。
          </div>
        </div>
      </Card>

      {/* 本月签到记录 */}
      <Card title="本月签到记录" style={{ borderRadius: 12 }} loading={loading}>
        {!data?.checkins?.length ? (
          <div style={{ padding: '24px 0', textAlign: 'center', color: '#86909C' }}>
            <IconCheckCircle style={{ fontSize: 36, marginBottom: 8 }} />
            <div>本月还没有签到记录，从今天开始坚持吧</div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
            {data.checkins.map((c: any) => (
              <Tag key={c.day} color="green" style={{ marginRight: 0 }}>
                {c.day.slice(5)} · +{c.points}分 · 连{c.streak}天
              </Tag>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
